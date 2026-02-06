package fetcher

import (
	"encoding/base64"
	"fmt"
	"log"
	"strings"

	"github.com/cjeanner/kustomap/internal/repository"
	gitlab "gitlab.com/gitlab-org/api/client-go"
)

type GitLabFetcher struct {
	client    *gitlab.Client
	info      *repository.RepositoryInfo
	projectID string
}

func NewGitLabFetcher(info *repository.RepositoryInfo, token string) (*GitLabFetcher, error) {
	// Create GitLab client
	var client *gitlab.Client
	var err error

	if token != "" {
		client, err = gitlab.NewClient(token, gitlab.WithBaseURL(info.BaseURL+"/api/v4"))
	} else {
		client, err = gitlab.NewClient("", gitlab.WithBaseURL(info.BaseURL+"/api/v4"))
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create GitLab client: %w", err)
	}

	// GitLab uses "owner/repo" as project ID
	projectID := fmt.Sprintf("%s/%s", info.Owner, info.Repo)

	return &GitLabFetcher{
		client:    client,
		info:      info,
		projectID: projectID,
	}, nil
}

// FetchFile retrieves a single file content
func (f *GitLabFetcher) FetchFile(path string) ([]byte, error) {
	log.Printf("Fetching file from GitLab: %s/%s @ %s",
		f.projectID, path, f.info.Ref)

	file, _, err := f.client.RepositoryFiles.GetFile(
		f.projectID,
		path,
		&gitlab.GetFileOptions{
			Ref: gitlab.Ptr(f.info.Ref),
		},
	)

	if err != nil {
		return nil, fmt.Errorf("failed to fetch file %s: %w", path, err)
	}

	// Decode base64 content
	decoded, err := base64.StdEncoding.DecodeString(file.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64 content: %w", err)
	}

	return decoded, nil
}

// ListFiles lists all files recursively in the repository
func (f *GitLabFetcher) ListFiles() ([]string, error) {
	log.Printf("Listing files from GitLab: %s @ %s",
		f.projectID, f.info.Ref)

	opts := &gitlab.ListTreeOptions{
		Ref:       gitlab.Ptr(f.info.Ref),
		Recursive: gitlab.Ptr(true),
		ListOptions: gitlab.ListOptions{
			PerPage: 100,
			Page:    1,
		},
	}

	var allFiles []string

	for {
		tree, resp, err := f.client.Repositories.ListTree(f.projectID, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to list repository tree: %w", err)
		}

		for _, node := range tree {
			if node.Type == "blob" {
				allFiles = append(allFiles, node.Path)
			}
		}

		// Check if there are more pages
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	log.Printf("Found %d files in repository", len(allFiles))
	return allFiles, nil
}

// kustomizationFilenamesLower are known kustomization file names (lowercase for matching).
var kustomizationFilenamesLower = map[string]bool{
	"kustomization.yaml": true,
	"kustomization.yml":  true,
	"kustomization":      true,
}

func isKustomizationFileName(name string) bool {
	return kustomizationFilenamesLower[strings.ToLower(name)]
}

// FindKustomizationInPath finds kustomization.yaml in a specific path.
// It tries the path as a file first, then lists the directory (when path is a directory)
// and picks a kustomization file by name (case-insensitive), matching GitHub fetcher behavior.
func (f *GitLabFetcher) FindKustomizationInPath(path string) (string, error) {
	path = strings.Trim(path, "/")

	log.Printf("Trying to fetch path as-is: %s", path)
	content, err := f.FetchFile(path)
	if err == nil {
		return string(content), nil
	}

	// Path may be a directory: list it and look for a kustomization file by name (case-insensitive)
	opts := &gitlab.ListTreeOptions{
		Path:        gitlab.Ptr(path),
		Ref:         gitlab.Ptr(f.info.Ref),
		Recursive:   gitlab.Ptr(false),
		ListOptions: gitlab.ListOptions{PerPage: 100, Page: 1},
	}
	tree, _, err := f.client.Repositories.ListTree(f.projectID, opts)
	if err == nil {
		for _, node := range tree {
			if node.Type != "blob" {
				continue
			}
			name := node.Name
			if name == "" && node.Path != "" {
				parts := strings.Split(node.Path, "/")
				name = parts[len(parts)-1]
			}
			if !isKustomizationFileName(name) {
				continue
			}
			filePath := node.Path
			if filePath == "" {
				if path != "" {
					filePath = path + "/" + name
				} else {
					filePath = name
				}
			}
			content, err := f.FetchFile(filePath)
			if err == nil {
				log.Printf("✅ Found kustomization file: %s", filePath)
				return string(content), nil
			}
		}
	}

	return "", fmt.Errorf("no kustomization file found in path: %s", strings.Clone(path))
}
