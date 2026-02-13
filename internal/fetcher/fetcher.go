package fetcher

import (
	"fmt"

	"github.com/cjeanner/kustomap/internal/repository"
)

// Fetcher interface for retrieving files from remote repositories
type Fetcher interface {
	// FetchFile retrieves a single file content
	FetchFile(path string) ([]byte, error)

	// ListFiles lists all files in the repository (recursive)
	ListFiles() ([]string, error)

	// FindKustomizationInPath finds kustomization.yaml in a specific path
	FindKustomizationInPath(path string) (string, error)
}

// NewFetcher creates the appropriate fetcher based on repository type
func NewFetcher(info *repository.RepositoryInfo, token string) (Fetcher, error) {
	switch info.Type {
	case repository.GitHub:
		return NewGitHubFetcher(info, token)
	case repository.GitLab:
		return NewGitLabFetcher(info, token)
	case repository.Local:
		return NewLocalFetcher(info, token)
	default:
		return nil, fmt.Errorf("unsupported repository type: %s", info.Type)
	}
}
