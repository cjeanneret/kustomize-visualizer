package repository

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type RepositoryType string

const (
	GitHub  RepositoryType = "github"
	GitLab  RepositoryType = "gitlab"
	Local   RepositoryType = "local"
	Unknown RepositoryType = "unknown"
)

type RepositoryInfo struct {
	Type          RepositoryType
	Owner         string
	Repo          string
	Ref           string
	BaseURL       string
	Path          string
	AmbiguousPath string

	// RootPath is the absolute path to the repository root. Used only when Type == Local.
	RootPath string
}

// DetectRepository parses the URL and determines the repository type
func DetectRepository(repoURL string, token string) (*RepositoryInfo, error) {
	parsedURL, err := url.Parse(repoURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	host := parsedURL.Host
	path := strings.Trim(parsedURL.Path, "/")
	baseURL := fmt.Sprintf("%s://%s", parsedURL.Scheme, host)

	// GitHub.com - direct detection
	if strings.Contains(host, "github.com") {
		return parseGitHubURL(path, baseURL)
	}

	// GitLab - detect by hostname or URL structure
	if strings.Contains(host, "gitlab") || strings.Contains(path, "/-/") {
		log.Printf("Detected GitLab from hostname or URL structure")
		return parseGitLabURL(path, baseURL)
	}

	// For ambiguous cases, try probing with token
	repoType := probeRepositoryType(baseURL, token)

	switch repoType {
	case GitLab:
		return parseGitLabURL(path, baseURL)
	case GitHub:
		return parseGitHubURL(path, baseURL)
	default:
		return nil, fmt.Errorf("unable to detect repository type for: %s", host)
	}
}

// isGitLabInstance checks if the URL is a GitLab instance
func isGitLabInstance(baseURL, token string) bool {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequest("GET", baseURL+"/api/v4/version", nil)
	if err != nil {
		return false
	}

	// Add token if provided
	if token != "" {
		req.Header.Set("PRIVATE-TOKEN", token)
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Failed to probe GitLab API: %v", err)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("GitLab API returned status %d", resp.StatusCode)
		return false
	}

	var version struct {
		Version  string `json:"version"`
		Revision string `json:"revision"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&version); err != nil {
		return false
	}

	return version.Version != ""
}

// isGitHubInstance checks if the URL is a GitHub instance
func isGitHubInstance(baseURL, token string) bool {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Try GitHub API
	req, err := http.NewRequest("GET", baseURL+"/api/v3", nil)
	if err != nil {
		return false
	}

	// Add token if provided
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

// probeRepositoryType attempts to detect the repository type by probing APIs
func probeRepositoryType(baseURL, token string) RepositoryType {
	// Try GitLab first
	if isGitLabInstance(baseURL, token) {
		return GitLab
	}

	// Try GitHub
	if isGitHubInstance(baseURL, token) {
		return GitHub
	}

	return Unknown
}

// parseGitHubURL extracts owner/repo/path from GitHub URL
func parseGitHubURL(path, baseURL string) (*RepositoryInfo, error) {
	parts := strings.Split(path, "/")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid GitHub repository path: %s", path)
	}

	info := &RepositoryInfo{
		Type:          GitHub,
		Owner:         parts[0],
		Repo:          strings.TrimSuffix(parts[1], ".git"),
		Ref:           "main",
		BaseURL:       baseURL,
		Path:          "",
		AmbiguousPath: "",
	}

	// Handle /tree/branch/path or /blob/branch/path URLs
	if len(parts) >= 4 && (parts[2] == "tree" || parts[2] == "blob") {
		// Everything after "tree" is ambiguous (branch + path mixed)
		info.AmbiguousPath = strings.Join(parts[3:], "/")
	}

	return info, nil
}

// parseGitLabURL extracts namespace/repo/path from GitLab URL
func parseGitLabURL(path, baseURL string) (*RepositoryInfo, error) {
	parts := strings.Split(path, "/")

	// Find the /-/ marker position
	markerIndex := -1
	for i, part := range parts {
		if part == "-" {
			markerIndex = i
			break
		}
	}

	var namespaceParts []string
	var ambiguousPath string // Path that contains branch + path mixed

	if markerIndex > 0 {
		// Extract namespace/repo before /-/
		namespaceParts = parts[:markerIndex]

		// After /-/ we have: tree/branch/path or blob/branch/path
		afterMarker := parts[markerIndex+1:]

		if len(afterMarker) >= 2 && (afterMarker[0] == "tree" || afterMarker[0] == "blob") {
			// Everything after "tree" is ambiguous (branch + path)
			ambiguousPath = strings.Join(afterMarker[1:], "/")
		}
	} else {
		// No /-/ marker, simpler case
		if len(parts) < 2 {
			return nil, fmt.Errorf("invalid GitLab repository path: %s", path)
		}
		namespaceParts = parts
	}

	if len(namespaceParts) < 2 {
		return nil, fmt.Errorf("invalid GitLab repository path: %s", path)
	}

	// Last part of namespace is the project, rest is the namespace
	var owner, repo string
	repo = strings.TrimSuffix(namespaceParts[len(namespaceParts)-1], ".git")
	if len(namespaceParts) > 2 {
		owner = strings.Join(namespaceParts[:len(namespaceParts)-1], "/")
	} else {
		owner = namespaceParts[0]
	}

	info := &RepositoryInfo{
		Type:          GitLab,
		Owner:         owner,
		Repo:          repo,
		Ref:           "main", // Will be resolved later
		BaseURL:       baseURL,
		Path:          "",
		AmbiguousPath: ambiguousPath, // Store for later resolution
	}

	return info, nil
}

func (r *RepositoryInfo) String() string {
	if r.Type == Local {
		return fmt.Sprintf("local:%s@%s", r.Path, r.Ref)
	}
	return fmt.Sprintf("%s:%s/%s@%s", r.Type, r.Owner, r.Repo, r.Ref)
}
