package parser

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/cjeanner/kustomap/internal/repository"
)

// KustomizeReference represents a reference in kustomization.yaml
type KustomizeReference struct {
	Type     ReferenceType
	Original string

	// For remote references
	RepoInfo *repository.RepositoryInfo
	Path     string

	// For relative references
	RelativePath string
}

type ReferenceType string

const (
	ReferenceRemote   ReferenceType = "remote"
	ReferenceRelative ReferenceType = "relative"
)

// ParseReference parses a reference from kustomization.yaml
// Formats supported:
// - https://github.com/org/repo//path?ref=branch
// - git@github.com:org/repo.git//path?ref=branch
// - ../relative/path (explicit relative)
// - ./relative/path (explicit relative)
// - relative/path (implicit relative - no prefix)
func ParseReference(ref string, token string) (*KustomizeReference, error) {
	// Remote references (HTTP/HTTPS)
	if strings.HasPrefix(ref, "https://") || strings.HasPrefix(ref, "http://") {
		return parseHTTPReference(ref, token)
	}

	// Git SSH format
	if strings.HasPrefix(ref, "git@") {
		return parseGitSSHReference(ref, token)
	}

	// Explicit relative paths
	if strings.HasPrefix(ref, "./") || strings.HasPrefix(ref, "../") {
		return &KustomizeReference{
			Type:         ReferenceRelative,
			Original:     ref,
			RelativePath: ref,
		}, nil
	}

	// Implicit relative path (no prefix = relative to current directory)
	// Examples: "deployment-02", "nodeset", "components/foo"
	// These are treated as "./deployment-02", "./nodeset", etc.
	return &KustomizeReference{
		Type:         ReferenceRelative,
		Original:     ref,
		RelativePath: ref, // Will be resolved with path.Join in processReference
	}, nil
}

// parseHTTPReference parses HTTP(S) Kustomize references
// Format: https://github.com/org/repo//path?ref=branch
func parseHTTPReference(ref string, token string) (*KustomizeReference, error) {
	var repoURL string
	var path string
	var refOverride string

	// Compter le nombre de "//" dans l'URL
	slashCount := strings.Count(ref, "//")

	if slashCount > 1 {
		// Format Kustomize: https://github.com/org/repo//path?ref=branch
		// Trouver le second "//" pour séparer repo et path
		idx := strings.Index(ref, "//")
		remaining := ref[idx+2:]
		secondIdx := strings.Index(remaining, "//")

		if secondIdx != -1 {
			repoURL = ref[:idx+2+secondIdx]
			pathWithRef := remaining[secondIdx+2:]

			if strings.Contains(pathWithRef, "?ref=") {
				subparts := strings.SplitN(pathWithRef, "?ref=", 2)
				path = subparts[0]
				refOverride = subparts[1]
			} else {
				path = pathWithRef
			}
		} else {
			repoURL = ref
		}
	} else {
		// Format standard: https://github.com/org/repo/path?ref=branch
		// Extraire le repo URL (schéma + host + jusqu'à 2 parties du path pour org/repo)
		u, err := url.Parse(ref)
		if err != nil {
			return nil, fmt.Errorf("invalid URL: %w", err)
		}

		pathParts := strings.Split(strings.Trim(u.Path, "/"), "/")

		// Extract ref query parameter (branch/tag for fetching)
		if q := u.Query().Get("ref"); q != "" {
			refOverride = q
		}

		if len(pathParts) >= 2 {
			// Repo URL = scheme + host + /owner/repo
			repoURL = fmt.Sprintf("%s://%s/%s/%s", u.Scheme, u.Host, pathParts[0], pathParts[1])
			// Path = reste du chemin
			if len(pathParts) > 2 {
				path = strings.Join(pathParts[2:], "/")
			}
		} else {
			repoURL = fmt.Sprintf("%s://%s%s", u.Scheme, u.Host, u.Path)
		}
	}

	// Le reste du code demeure identique
	repoInfo, err := repository.DetectRepository(repoURL, token)
	if err != nil {
		return nil, fmt.Errorf("failed to detect repository type: %w", err)
	}

	if refOverride != "" {
		repoInfo.Ref = refOverride
	}

	return &KustomizeReference{
		Type:     ReferenceRemote,
		Original: ref,
		RepoInfo: repoInfo,
		Path:     strings.Trim(path, "/"),
	}, nil
}

// parseGitSSHReference parses Git SSH format
// Format: git@github.com:org/repo.git//path?ref=branch
func parseGitSSHReference(ref string, token string) (*KustomizeReference, error) {
	// Convert git@github.com:org/repo.git to https://github.com/org/repo
	ref = strings.TrimPrefix(ref, "git@")
	ref = strings.Replace(ref, ":", "/", 1)
	ref = "https://" + ref

	return parseHTTPReference(ref, token)
}

func (r *KustomizeReference) String() string {
	if r.Type == ReferenceRelative {
		return fmt.Sprintf("relative:%s", r.RelativePath)
	}
	return fmt.Sprintf("remote:%s/%s/%s@%s", r.RepoInfo.Type, r.RepoInfo.Owner, r.RepoInfo.Repo, r.RepoInfo.Ref)
}
