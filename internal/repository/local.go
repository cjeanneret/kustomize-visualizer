package repository

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-git/go-git/v5"
)

const defaultLocalRef = "main"

// DetectLocalRepository builds RepositoryInfo for a local path.
// resolvedPath must be an absolute path, already validated (e.g. by validation.ValidateLocalPath).
// It discovers the git root, current branch, and the relative path within the repo.
func DetectLocalRepository(resolvedPath string) (*RepositoryInfo, error) {
	resolvedPath = filepath.Clean(resolvedPath)
	info, err := os.Stat(resolvedPath)
	if err != nil {
		return nil, fmt.Errorf("cannot access path: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("path is not a directory: %s", resolvedPath)
	}

	var rootPath string
	var ref string

	// Detect git repo via go-git (DetectDotGit walks up to find .git from subdirs)
	_, err = git.PlainOpenWithOptions(resolvedPath, &git.PlainOpenOptions{DetectDotGit: true})
	if err == nil {
		// In a git repo: get branch/hash and root path
		ref, err = gitBranch(resolvedPath)
		if err != nil {
			ref = defaultLocalRef
		}
		rootPath, err = findGitRoot(resolvedPath)
		if err != nil {
			rootPath = resolvedPath
		}
	} else {
		// Not a git repo
		rootPath = resolvedPath
		ref = defaultLocalRef
	}

	// Path is relative to repo root
	relPath, err := filepath.Rel(rootPath, resolvedPath)
	if err != nil {
		return nil, fmt.Errorf("path outside repository: %w", err)
	}
	relPath = filepath.ToSlash(relPath)
	if relPath == "." {
		relPath = ""
	}

	return &RepositoryInfo{
		Type:     Local,
		Ref:      ref,
		Path:     relPath,
		RootPath: rootPath,
	}, nil
}

// findGitRoot walks up from dir to find the repository root (directory containing .git).
// It stops at $HOME and never returns a root above it, so we stay within the validated scope.
// Uses canonical paths (EvalSymlinks) so symlinks in $HOME or the path do not bypass the check.
func findGitRoot(dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	absResolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		absResolved = abs
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	homeAbs, err := filepath.Abs(home)
	if err != nil {
		return "", err
	}
	homeResolved, err := filepath.EvalSymlinks(homeAbs)
	if err != nil {
		homeResolved = homeAbs
	}
	homePrefix := filepath.Clean(homeResolved) + string(filepath.Separator)

	for p := absResolved; p != filepath.Dir(p); p = filepath.Dir(p) {
		// Never go above $HOME (use canonical paths for symlink-safe comparison)
		if p != homeResolved && !startsWithClean(p, homePrefix) {
			break
		}
		fi, err := os.Stat(filepath.Join(p, ".git"))
		if err == nil && fi.IsDir() {
			return p, nil
		}
	}
	return "", fmt.Errorf("not a git repository")
}

// startsWithClean returns true if path is under prefix (avoids false positives like /home vs /home2).
func startsWithClean(path, prefix string) bool {
	path = filepath.Clean(path) + string(filepath.Separator)
	return len(path) >= len(prefix) && path[:len(prefix)] == prefix
}

// gitBranch returns the current branch name using go-git.
// For detached HEAD, returns the short commit hash (7 chars).
func gitBranch(path string) (string, error) {
	repo, err := git.PlainOpenWithOptions(path, &git.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		return "", err
	}
	ref, err := repo.Head()
	if err != nil {
		return "", err
	}
	if ref.Name() == "HEAD" {
		// Detached HEAD: use short commit hash (like git rev-parse --short HEAD)
		h := ref.Hash().String()
		if len(h) > 7 {
			h = h[:7]
		}
		return h, nil
	}
	return ref.Name().Short(), nil
}
