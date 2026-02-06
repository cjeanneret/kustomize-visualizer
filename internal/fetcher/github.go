package fetcher

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/cjeanner/kustomap/internal/repository"
	"github.com/google/go-github/v82/github"
)

type GitHubFetcher struct {
	client *github.Client
	info   *repository.RepositoryInfo
	ctx    context.Context
}

func NewGitHubFetcher(info *repository.RepositoryInfo, token string) (*GitHubFetcher, error) {
	ctx := context.Background()

	var client *github.Client
	if token != "" {
		client = github.NewClient(nil).WithAuthToken(token)
	} else {
		client = github.NewClient(nil)
	}

	return &GitHubFetcher{
		client: client,
		info:   info,
		ctx:    ctx,
	}, nil
}

// FetchFile retrieves a single file content
func (f *GitHubFetcher) FetchFile(path string) ([]byte, error) {
	log.Printf("Fetching file from GitHub: %s/%s/%s @ %s",
		f.info.Owner, f.info.Repo, path, f.info.Ref)

	fileContent, _, _, err := f.client.Repositories.GetContents(
		f.ctx,
		f.info.Owner,
		f.info.Repo,
		path,
		&github.RepositoryContentGetOptions{Ref: f.info.Ref},
	)

	if err != nil {
		return nil, fmt.Errorf("failed to fetch file %s: %w", path, err)
	}

	if fileContent == nil {
		return nil, fmt.Errorf("file not found: %s", path)
	}

	content, err := fileContent.GetContent()
	if err != nil {
		return nil, fmt.Errorf("failed to decode file content: %w", err)
	}

	return []byte(content), nil
}

// ListFiles lists all files recursively in the repository
func (f *GitHubFetcher) ListFiles() ([]string, error) {
	log.Printf("Listing files from GitHub: %s/%s @ %s",
		f.info.Owner, f.info.Repo, f.info.Ref)

	tree, _, err := f.client.Git.GetTree(
		f.ctx,
		f.info.Owner,
		f.info.Repo,
		f.info.Ref,
		true, // recursive
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get repository tree: %w", err)
	}

	var files []string
	for _, entry := range tree.Entries {
		if entry.GetType() == "blob" {
			files = append(files, entry.GetPath())
		}
	}

	log.Printf("Found %d files in repository", len(files))
	return files, nil
}

// FindKustomizationInPath finds kustomization.yaml in a specific path
func (f *GitHubFetcher) FindKustomizationInPath(path string) (string, error) {
	// Normalize path
	path = strings.Trim(path, "/")

	log.Printf("Trying to fetch path as-is: %s", path)
	content, err := f.FetchFile(path)
	if err == nil {
		return string(content), nil
	}

	// Try common kustomization file names
	kustomizationFiles := []string{
		"kustomization.yaml",
		"kustomization.yml",
		"Kustomization",
	}

	for _, filename := range kustomizationFiles {
		var fullPath string
		if path == "" {
			fullPath = filename
		} else {
			fullPath = path + "/" + filename
		}

		log.Printf("Trying to fetch: %s", fullPath)

		content, err := f.FetchFile(fullPath)
		if err == nil {
			log.Printf("✅ Found kustomization file: %s", fullPath)
			return string(content), nil
		}
	}

	return "", fmt.Errorf("no kustomization file found in path: %s", strings.Clone(path))
}
