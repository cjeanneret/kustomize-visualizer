package fetcher

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/cjeanner/kustomap/internal/repository"
)

type LocalFetcher struct {
	info *repository.RepositoryInfo
}

func NewLocalFetcher(info *repository.RepositoryInfo, _ string) (*LocalFetcher, error) {
	if info.RootPath == "" {
		return nil, fmt.Errorf("LocalFetcher requires RootPath")
	}
	abs, err := filepath.Abs(info.RootPath)
	if err != nil {
		return nil, fmt.Errorf("invalid root path: %w", err)
	}
	return &LocalFetcher{
		info: &repository.RepositoryInfo{
			Type:     info.Type,
			RootPath: abs,
			Path:     info.Path,
			Ref:      info.Ref,
		},
	}, nil
}

// joinPath joins rootPath with path and ensures the result is under rootPath (no escape).
func (f *LocalFetcher) joinPath(path string) (string, error) {
	path = strings.Trim(path, "/")
	full := filepath.Join(f.info.RootPath, path)
	full = filepath.Clean(full)
	root := filepath.Clean(f.info.RootPath)
	rootPrefix := root + string(filepath.Separator)
	if full != root && !strings.HasPrefix(full, rootPrefix) {
		return "", fmt.Errorf("path escapes repository root: %s", path)
	}
	return full, nil
}

// FetchFile retrieves a single file content from the local filesystem.
func (f *LocalFetcher) FetchFile(path string) ([]byte, error) {
	full, err := f.joinPath(path)
	if err != nil {
		return nil, err
	}
	log.Printf("Fetching file from local: %s", full)
	return os.ReadFile(full)
}

// ListFiles lists all files recursively under the repository root.
func (f *LocalFetcher) ListFiles() ([]string, error) {
	root := filepath.Clean(f.info.RootPath)
	var files []string
	err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if info.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("list files: %w", err)
	}
	log.Printf("Found %d files in local repository", len(files))
	return files, nil
}

// FindKustomizationInPath finds kustomization.yaml in a specific path.
func (f *LocalFetcher) FindKustomizationInPath(path string) (string, error) {
	path = strings.Trim(path, "/")

	// Try path as a file first (path could be kustomization.yaml)
	full, err := f.joinPath(path)
	if err != nil {
		return "", err
	}
	if info, err := os.Stat(full); err == nil && !info.IsDir() {
		content, err := os.ReadFile(full)
		if err != nil {
			return "", err
		}
		return string(content), nil
	}

	// Try common kustomization file names
	for _, name := range []string{"kustomization.yaml", "kustomization.yml", "Kustomization"} {
		var p string
		if path == "" {
			p = name
		} else {
			p = path + "/" + name
		}
		full, err := f.joinPath(p)
		if err != nil {
			continue
		}
		content, err := os.ReadFile(full)
		if err == nil {
			log.Printf("✅ Found kustomization file: %s", full)
			return string(content), nil
		}
	}

	return "", fmt.Errorf("no kustomization file found in path: %s", path)
}
