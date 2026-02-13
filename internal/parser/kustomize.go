package parser

import (
	"fmt"
	"log"
	"path"
	"path/filepath"
	"strings"

	"github.com/cjeanner/kustomap/internal/fetcher"
	"github.com/cjeanner/kustomap/internal/repository"
	"github.com/cjeanner/kustomap/internal/types"
	"github.com/cjeanner/kustomap/internal/validation"
	"gopkg.in/yaml.v3"
)

// copyLogArgs makes independent copies of strings used in log/error messages
// to avoid any risk of corruption from shared buffers when handling concurrent requests.
func copyLogArgs(s string) string { return strings.Clone(s) }

// Kustomization represents a kustomization.yaml file structure
type Kustomization struct {
	Resources  []string      `yaml:"resources"`
	Components []string      `yaml:"components"`
	Patches    []interface{} `yaml:"patches"`

	// Deprecated but still supported for backward compatibility
	Bases []string `yaml:"bases"`
}

// FetcherFactory creates a fetcher for a given repo and token.
// When set on Parser (e.g. in tests), it is used instead of fetcher.NewFetcher
// when resolving references that require a fetcher for a different repo.
type FetcherFactory func(repo *repository.RepositoryInfo, token string) (fetcher.Fetcher, error)

// Parser handles the parsing and graph building
type Parser struct {
	fetcher        fetcher.Fetcher
	repoInfo       *repository.RepositoryInfo
	tokens         map[repository.RepositoryType]string // GitHub and GitLab tokens
	graph          *types.Graph
	visitedURLs    map[string]bool // Prevent infinite loops
	FetcherFactory FetcherFactory // optional; used in tests to inject mock fetchers
}

// sameRepoAsEntry reports whether current is the same repo as entry.
// For GitHub/GitLab: owner+repo. For Local: RootPath.
func sameRepoAsEntry(entry, current *repository.RepositoryInfo) bool {
	if entry == nil || current == nil {
		return false
	}
	if entry.Type == repository.Local && current.Type == repository.Local {
		return entry.RootPath != "" && entry.RootPath == current.RootPath
	}
	return entry.Owner == current.Owner && entry.Repo == current.Repo
}

// getFetcherForRepo returns a fetcher for the given repo, using FetcherFactory if set (e.g. in tests).
func (p *Parser) getFetcherForRepo(repo *repository.RepositoryInfo, token string) (fetcher.Fetcher, error) {
	if p.FetcherFactory != nil {
		return p.FetcherFactory(repo, token)
	}
	return fetcher.NewFetcher(repo, token)
}

// NewParser creates a new Kustomize parser
func NewParser(f fetcher.Fetcher, repoInfo *repository.RepositoryInfo) *Parser {
	return &Parser{
		fetcher:     f,
		repoInfo:    repoInfo,
		tokens:         make(map[repository.RepositoryType]string),
		graph:          &types.Graph{Elements: []types.Element{}, BaseURLs: make(map[string]string), LocalRootPaths: make(map[string]string)},
		visitedURLs:    make(map[string]bool),
	}
}

// SetToken sets authentication token for a repository type
func (p *Parser) SetToken(repoType repository.RepositoryType, token string) {
	p.tokens[repoType] = token
}

// Parse starts parsing from the initial path
func (p *Parser) Parse(startPath string) (*types.Graph, error) {
	log.Printf("Starting parse from path: %s", startPath)

	// Fetch the initial kustomization.yaml
	content, err := p.fetcher.FindKustomizationInPath(startPath)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch initial kustomization: %w", err)
	}

	// Parse and process recursively (entry point is an overlay)
	nodeID := p.buildNodeID(p.repoInfo, startPath)
	err = p.processKustomization(nodeID, content, startPath, p.repoInfo, "overlay")
	if err != nil {
		return nil, err
	}

	log.Printf("✅ Graph built with %d elements", len(p.graph.Elements))
	return p.graph, nil
}

// processKustomization parses a kustomization.yaml and processes its dependencies.
// nodeType is the kind of this node: "overlay" for the entry point, "resource" when
// reached via resources/bases, or "component" when reached via components.
func (p *Parser) processKustomization(nodeID, content, currentPath string, currentRepo *repository.RepositoryInfo, nodeType string) error {
	// Check if already visited to prevent loops
	if p.visitedURLs[nodeID] {
		log.Printf("Already visited: %s", nodeID)
		return nil
	}
	p.visitedURLs[nodeID] = true

	log.Printf("Processing kustomization at: %s (type: %s)", nodeID, nodeType)

	// Parse YAML
	var kust Kustomization
	if err := yaml.Unmarshal([]byte(content), &kust); err != nil {
		return fmt.Errorf("failed to parse kustomization YAML: %w", err)
	}

	// Create node for this kustomization (type reflects how it was referenced)
	p.addNode(nodeID, nodeType, currentPath, &kust, currentRepo.BaseURL)

	// Merge bases into resources (backward compatibility)
	allResources := append(kust.Resources, kust.Bases...)

	// Process all resources (files + kustomizations)
	for _, resource := range allResources {
		if err := p.processResource(nodeID, resource, currentPath, currentRepo); err != nil {
			log.Printf("Warning: failed to process resource %s: %v", resource, err)
		}
	}

	// Process components (reusable components)
	for _, component := range kust.Components {
		if err := p.processReference(nodeID, component, "component", currentPath, currentRepo); err != nil {
			log.Printf("Warning: failed to process component %s: %v", component, err)
		}
	}

	return nil
}

// processReference handles bases and components (both can be remote or local)
func (p *Parser) processReference(parentID, ref, refType, currentPath string, currentRepo *repository.RepositoryInfo) error {
	log.Printf("Processing %s: %s", refType, ref)

	// Check if it's a YAML file
	if isYAMLFile(ref) {
		resourcePath := path.Join(currentPath, ref)
		childID := p.buildNodeID(currentRepo, resourcePath)
		p.addNode(childID, "resource", resourcePath, nil, currentRepo.BaseURL)
		p.addEdge(parentID, childID, refType)
		return nil
	}

	// Parse the reference
	token := p.tokens[currentRepo.Type]
	kustomizeRef, err := ParseReference(ref, token)
	if err != nil {
		childID := fmt.Sprintf("error:%s", ref)
		p.addErrorNode(childID, ref, fmt.Sprintf("Failed to parse reference: %v", err), currentRepo.BaseURL)
		p.addEdge(parentID, childID, refType) // Edge AFTER node creation
		return nil
	}

	var childFetcher fetcher.Fetcher
	var childRepo *repository.RepositoryInfo
	var childPath string

	switch kustomizeRef.Type {
	case ReferenceRelative:
		childPath = resolvePath(currentPath, kustomizeRef.RelativePath)
		childRepo = currentRepo

		// When current repo is Local, check if the resolved path escapes the root.
		// If so, treat as an external local repo and create a fetcher for it.
		if currentRepo.Type == repository.Local && currentRepo.RootPath != "" {
			baseDir := filepath.Join(currentRepo.RootPath, filepath.FromSlash(currentPath))
			absPath := filepath.Clean(filepath.Join(baseDir, filepath.FromSlash(kustomizeRef.RelativePath)))
			rootClean := filepath.Clean(currentRepo.RootPath)
			rootPrefix := rootClean + string(filepath.Separator)
			escapes := absPath != rootClean && !strings.HasPrefix(absPath, rootPrefix)

			if escapes {
				// Path references a directory outside the current repo; validate and resolve as external local repo
				validatedPath, err := validation.ValidateLocalPath(absPath)
				if err != nil {
					childID := p.buildNodeID(currentRepo, childPath)
					p.addErrorNode(childID, childPath, fmt.Sprintf("Invalid local path: %v", err), currentRepo.BaseURL)
					p.addEdge(parentID, childID, refType)
					return nil
				}
				extRepo, err := repository.DetectLocalRepository(validatedPath)
				if err != nil {
					childID := p.buildNodeID(currentRepo, childPath)
					p.addErrorNode(childID, childPath, fmt.Sprintf("Failed to detect repository: %v", err), currentRepo.BaseURL)
					p.addEdge(parentID, childID, refType)
					return nil
				}
				childRepo = extRepo
				childPath = extRepo.Path
				lf, err := fetcher.NewLocalFetcher(extRepo, "")
				if err != nil {
					childID := p.buildNodeID(extRepo, childPath)
					p.addErrorNode(childID, childPath, fmt.Sprintf("Failed to create fetcher: %v", err), extRepo.BaseURL)
					p.addEdge(parentID, childID, refType)
					return nil
				}
				childFetcher = lf
			} else {
				// Path stays within current repo
				if sameRepoAsEntry(p.repoInfo, currentRepo) {
					childFetcher = p.fetcher
				} else {
					tok := p.tokens[currentRepo.Type]
					var err error
					childFetcher, err = p.getFetcherForRepo(currentRepo, tok)
					if err != nil {
						childID := p.buildNodeID(currentRepo, childPath)
						p.addErrorNode(childID, childPath, fmt.Sprintf("Failed to create fetcher: %v", err), currentRepo.BaseURL)
						p.addEdge(parentID, childID, refType)
						return nil
					}
				}
			}
		} else {
			// Remote repo: use fetcher for current repo
			if sameRepoAsEntry(p.repoInfo, currentRepo) {
				childFetcher = p.fetcher
			} else {
				tok := p.tokens[currentRepo.Type]
				var err error
				childFetcher, err = p.getFetcherForRepo(currentRepo, tok)
				if err != nil {
					childID := p.buildNodeID(currentRepo, childPath)
					p.addErrorNode(childID, childPath, fmt.Sprintf("Failed to create fetcher: %v", err), currentRepo.BaseURL)
					p.addEdge(parentID, childID, refType)
					return nil
				}
			}
		}

	case ReferenceRemote:
		childRepo = kustomizeRef.RepoInfo
		childPath = kustomizeRef.Path

		token := p.tokens[childRepo.Type]
		var err error
		childFetcher, err = p.getFetcherForRepo(childRepo, token)
		if err != nil {
			childID := p.buildNodeID(childRepo, childPath)
			p.addErrorNode(childID, childPath, fmt.Sprintf("Failed to create fetcher: %v", err), childRepo.BaseURL)
			p.addEdge(parentID, childID, refType) // Edge AFTER node creation
			return nil
		}
	}

	// Build unique node ID
	childID := p.buildNodeID(childRepo, childPath)

	// Store per-node local root for nodes in external local repos (used by build endpoint)
	if childRepo.Type == repository.Local && childRepo.RootPath != "" && !sameRepoAsEntry(p.repoInfo, childRepo) {
		if p.graph.LocalRootPaths == nil {
			p.graph.LocalRootPaths = make(map[string]string)
		}
		p.graph.LocalRootPaths[childID] = childRepo.RootPath
	}

	// Try to fetch the child kustomization
	content, err := childFetcher.FindKustomizationInPath(childPath)
	if err != nil {
		// Use explicit copies for log and stored error to avoid corruption from
		// shared buffers when multiple requests log concurrently.
		pathCopy := copyLogArgs(childPath)
		errStr := copyLogArgs(err.Error())
		log.Printf("⚠️  Warning: failed to fetch kustomization at %s: %s", pathCopy, errStr)
		p.addErrorNode(childID, pathCopy, "File not found or inaccessible: "+errStr, childRepo.BaseURL)
		p.addEdge(parentID, childID, refType)
		return nil
	}

	// Add edge BEFORE processing (so the node will exist after processKustomization)
	p.addEdge(parentID, childID, refType)

	// Recursively process the child (creates the node with type = refType: "resource" or "component")
	return p.processKustomization(childID, content, childPath, childRepo, refType)
}

// addErrorNode adds an error node to the graph
func (p *Parser) addErrorNode(id, path, errorMessage, baseURL string) {
	// Check if node already exists
	for _, elem := range p.graph.Elements {
		if elem.Group == "nodes" && elem.Data.ID == id {
			return
		}
	}

	content := map[string]interface{}{
		"error": errorMessage,
	}

	label := getShortLabel(path)
	p.graph.Elements = append(p.graph.Elements, types.Element{
		Group: "nodes",
		Data: types.ElementData{
			ID:      id,
			Label:   label,
			Type:    "error",
			Path:    path,
			Content: content,
		},
	})

	if baseURL != "" {
		p.graph.BaseURLs[id] = baseURL
	}
	log.Printf("Added error node: %s (error: %s)", copyLogArgs(id), copyLogArgs(errorMessage))
}

// processResource handles individual YAML resources or kustomization directories
func (p *Parser) processResource(parentID, resource, currentPath string, currentRepo *repository.RepositoryInfo) error {
	log.Printf("Processing resource: %s", resource)

	// Check if it's a directory (needs kustomization) or a file
	if isYAMLFile(resource) {
		// Direct YAML file - create a resource node
		resourcePath := path.Join(currentPath, resource)
		resourceID := p.buildNodeID(currentRepo, resourcePath)
		p.addNode(resourceID, "resource", resourcePath, nil, currentRepo.BaseURL)
		p.addEdge(parentID, resourceID, "resource")
		return nil
	}

	// It's a directory (or remote repo), treat as a kustomization reference
	return p.processReference(parentID, resource, "resource", currentPath, currentRepo)
}

// buildNodeID creates a unique identifier for a node.
// Format: github:owner/repo/path@ref, gitlab:owner/repo/path@ref, local:path@ref
func (p *Parser) buildNodeID(repoInfo *repository.RepositoryInfo, nodePath string) string {
	if repoInfo == nil {
		return nodePath
	}
	if repoInfo.Type == repository.Local {
		return fmt.Sprintf("local:%s@%s", nodePath, repoInfo.Ref)
	}
	return fmt.Sprintf("%s:%s/%s/%s@%s",
		repoInfo.Type, repoInfo.Owner, repoInfo.Repo, nodePath, repoInfo.Ref)
}

// addNode adds a node to the graph
func (p *Parser) addNode(id, nodeType, nodePath string, kust *Kustomization, baseURL string) {
	var content map[string]interface{}
	if kust != nil {
		content = map[string]interface{}{
			"resources":  kust.Resources,
			"bases":      kust.Bases,
			"components": kust.Components,
			"patches":    kust.Patches,
		}
	}
	label := getShortLabel(nodePath)
	newData := types.ElementData{
		ID:      id,
		Label:   label,
		Type:    nodeType,
		Path:    nodePath,
		Content: content,
	}

	// If a node with this ID already exists, replace it only if it was an error node
	// (so that a later successful resolution wins over an earlier failed fetch).
	for i := range p.graph.Elements {
		elem := &p.graph.Elements[i]
		if elem.Group == "nodes" && elem.Data.ID == id {
			if elem.Data.Type == "error" {
				elem.Data = newData
				log.Printf("Replaced error node with success node: %s (type: %s)", id, nodeType)
			}
			if baseURL != "" {
				p.graph.BaseURLs[id] = baseURL
			}
			return
		}
	}

	if baseURL != "" {
		p.graph.BaseURLs[id] = baseURL
	}
	p.graph.Elements = append(p.graph.Elements, types.Element{
		Group: "nodes",
		Data:  newData,
	})
	log.Printf("Added node: %s (type: %s)", id, nodeType)
}

// addEdge adds an edge to the graph
func (p *Parser) addEdge(sourceID, targetID, edgeType string) {
	edgeID := fmt.Sprintf("%s->%s", sourceID, targetID)

	// Check if edge already exists
	for _, elem := range p.graph.Elements {
		if elem.Group == "edges" && elem.Data.ID == edgeID {
			return // Already exists
		}
	}

	p.graph.Elements = append(p.graph.Elements, types.Element{
		Group: "edges",
		Data: types.ElementData{
			ID:       edgeID,
			Source:   sourceID,
			Target:   targetID,
			EdgeType: edgeType,
		},
	})

	log.Printf("Added edge: %s -> %s (type: %s)", sourceID, targetID, edgeType)
}

// Helper functions

// isYAMLFile checks if a path points to a YAML file
func isYAMLFile(path string) bool {
	lower := strings.ToLower(path)
	return strings.HasSuffix(lower, ".yaml") || strings.HasSuffix(lower, ".yml")
}

// resolvePath resolves a relative path against a base path
// Similar to os.path.join but for URL paths
func resolvePath(basePath, relativePath string) string {
	// Clean the paths
	basePath = path.Clean(basePath)
	relativePath = path.Clean(relativePath)

	// Join and clean
	return path.Join(basePath, relativePath)
}

// maxLabelLenMulti is the max length when the label is multiple path segments (e.g. "base/app").
const maxLabelLenMulti = 35

// maxLabelLenSingle is the max length for a single segment (filename); kept higher so long filenames show in full.
const maxLabelLenSingle = 50

// getShortLabel creates a short, readable label from a path by taking the last N
// path segments such that the joined string fits within the max length. Single-segment
// labels (e.g. filenames like "openstackcontrolplane.yaml") use a higher limit so they aren't truncated.
func getShortLabel(fullPath string) string {
	fullPath = strings.Trim(fullPath, "/")
	if fullPath == "" {
		return "unknown"
	}
	parts := strings.Split(fullPath, "/")
	var segs []string
	for _, p := range parts {
		if p != "" {
			segs = append(segs, p)
		}
	}
	if len(segs) == 0 {
		return "unknown"
	}
	// Use as many trailing segments as fit in maxLabelLenMulti
	label := segs[len(segs)-1]
	for n := 2; n <= len(segs); n++ {
		candidate := strings.Join(segs[len(segs)-n:], "/")
		if len(candidate) <= maxLabelLenMulti {
			label = candidate
		} else {
			break
		}
	}
	maxLen := maxLabelLenMulti
	if !strings.Contains(label, "/") {
		maxLen = maxLabelLenSingle // single segment = filename, allow longer
	}
	if len(label) > maxLen {
		label = label[:maxLen-3] + "..."
	}
	return label
}
