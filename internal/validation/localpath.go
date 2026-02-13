package validation

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// IsLocalPath returns true if raw looks like a local path (file://, ~/, or /).
// Used to decide whether to use ValidateLocalPath vs ValidateAnalyzeURL.
func IsLocalPath(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false
	}
	return strings.HasPrefix(raw, "file://") ||
		strings.HasPrefix(raw, "~/") ||
		raw == "~" ||
		filepath.IsAbs(raw)
}

// ValidateLocalPath validates and resolves a local path. It expands ~ to $HOME,
// resolves to an absolute path, and ensures the path is under $HOME.
// Returns the resolved absolute path, or an error.
// Only call when local mode is enabled (-enable-local).
func ValidateLocalPath(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("path is required")
	}
	if len(raw) > MaxAnalyzeURLLength {
		return "", fmt.Errorf("path exceeds maximum length of %d", MaxAnalyzeURLLength)
	}

	var path string
	if strings.HasPrefix(raw, "file://") {
		path = strings.TrimPrefix(raw, "file://")
		// file:/// on Unix leaves one leading slash; file:///C:/ on Windows
		path = strings.TrimPrefix(path, "/")
	} else if strings.HasPrefix(raw, "~/") || raw == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("cannot determine home directory: %w", err)
		}
		if raw == "~" {
			path = home
		} else {
			path = filepath.Join(home, strings.TrimPrefix(raw[1:], "/"))
		}
	} else if filepath.IsAbs(raw) {
		path = raw
	} else {
		return "", fmt.Errorf("path must be absolute or start with ~/")
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}

	// EvalSymlinks to resolve any .. or symlinks before checking $HOME
	resolved, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		// Path might not exist yet; use absPath and check existence below
		resolved = absPath
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	homeAbs, err := filepath.Abs(home)
	if err != nil {
		return "", fmt.Errorf("cannot resolve home directory: %w", err)
	}
	homeResolved, err := filepath.EvalSymlinks(homeAbs)
	if err != nil {
		homeResolved = homeAbs
	}

	// Ensure path is under $HOME (compare canonical paths; $HOME may be a symlink)
	homePrefix := filepath.Clean(homeResolved) + string(filepath.Separator)
	resolvedClean := filepath.Clean(resolved)
	if resolvedClean != homeResolved && !strings.HasPrefix(resolvedClean, homePrefix) {
		return "", fmt.Errorf("path must be under $HOME (%s)", homeAbs)
	}

	info, err := os.Stat(resolved)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("path does not exist: %s", resolved)
		}
		return "", fmt.Errorf("cannot access path: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("path is not a directory: %s", resolved)
	}

	return resolvedClean, nil
}
