package build

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cjeanner/kustomap/internal/repository"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/kyaml/filesys"
)

// Builder runs kustomize build for a node (overlay/base) by fetching the repo
// and running the kustomize API.
type Builder struct {
	githubToken string
	gitlabToken string
	client      *http.Client
}

// NewBuilder creates a Builder with optional tokens for private repos.
func NewBuilder(githubToken, gitlabToken string) *Builder {
	return &Builder{
		githubToken: githubToken,
		gitlabToken: gitlabToken,
		client: &http.Client{
			Timeout:   120 * time.Second,
			Transport: &http.Transport{MaxIdleConns: 2},
		},
	}
}

// Build fetches the repo for the given node ID, runs kustomize build at the node path,
// and returns the built YAML as a string. The node ID must be in format
// type:owner/repo/path@ref (e.g. github:foo/bar/deploy/overlay@main).
// baseURL is the repo base URL (e.g. https://gitlab.example.com) for self-hosted GitLab/GitHub; empty for public github.com/gitlab.com.
func (b *Builder) Build(nodeID, baseURL string) (yamlOut string, err error) {
	parts, err := ParseNodeID(nodeID)
	if err != nil {
		return "", fmt.Errorf("parse node ID: %w", err)
	}

	dir, err := os.MkdirTemp("", "kustomap-build-*")
	if err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}
	defer func() {
		if rerr := os.RemoveAll(dir); rerr != nil {
			log.Printf("warning: remove temp dir %s: %v", dir, rerr)
		}
	}()

	archivePath, err := b.downloadArchive(dir, parts, baseURL)
	if err != nil {
		return "", err
	}

	rootDir, err := extractTarGz(archivePath, dir)
	if err != nil {
		return "", fmt.Errorf("extract archive: %w", err)
	}

	buildPath := filepath.Join(dir, rootDir, parts.Path)
	buildPath = filepath.Clean(buildPath)
	if parts.Path == "" {
		buildPath = filepath.Join(dir, rootDir)
	}

	// Ensure path is under dir (no escape)
	absDir, _ := filepath.Abs(dir)
	absBuild, _ := filepath.Abs(buildPath)
	if !strings.HasPrefix(absBuild, absDir) {
		return "", fmt.Errorf("invalid build path")
	}

	fs := filesys.MakeFsOnDisk()
	k := krusty.MakeKustomizer(krusty.MakeDefaultOptions())
	resMap, err := k.Run(fs, buildPath)
	if err != nil {
		return "", fmt.Errorf("kustomize build: %w", err)
	}

	yamlBytes, err := resMap.AsYaml()
	if err != nil {
		return "", fmt.Errorf("encode YAML: %w", err)
	}

	return string(yamlBytes), nil
}

func (b *Builder) downloadArchive(dir string, parts *NodeIDParts, baseURL string) (string, error) {
	var archiveURL string
	var req *http.Request

	switch parts.Type {
	case repository.GitHub:
		apiBase := "https://api.github.com"
		if baseURL != "" {
			if baseURL == "https://github.com" {
				apiBase = "https://api.github.com"
			} else {
				apiBase = strings.TrimSuffix(baseURL, "/") + "/api/v3"
			}
		}
		archiveURL = fmt.Sprintf("%s/repos/%s/%s/tarball/%s", apiBase, parts.Owner, parts.Repo, parts.Ref)
		req, _ = http.NewRequest(http.MethodGet, archiveURL, nil)
		if b.githubToken != "" {
			req.Header.Set("Authorization", "Bearer "+b.githubToken)
		}
		req.Header.Set("Accept", "application/vnd.github.v3+json")
	case repository.GitLab:
		apiBase := "https://gitlab.com"
		if baseURL != "" {
			apiBase = strings.TrimSuffix(baseURL, "/")
		}
		projectID := parts.Owner + "%2F" + parts.Repo
		archiveURL = fmt.Sprintf("%s/api/v4/projects/%s/repository/archive.tar.gz?sha=%s", apiBase, projectID, url.QueryEscape(parts.Ref))
		req, _ = http.NewRequest(http.MethodGet, archiveURL, nil)
		if b.gitlabToken != "" {
			req.Header.Set("PRIVATE-TOKEN", b.gitlabToken)
		}
	default:
		return "", fmt.Errorf("unsupported repo type: %s", parts.Type)
	}

	resp, err := b.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("download archive: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return "", fmt.Errorf("download archive: %s: %s", resp.Status, string(body))
	}

	ext := ".tar.gz"
	if parts.Type == repository.GitLab && strings.Contains(resp.Header.Get("Content-Disposition"), "filename=") {
		// GitLab may return a different extension; we expect tar.gz
		ext = ".tar.gz"
	}
	archivePath := filepath.Join(dir, "repo"+ext)
	f, err := os.Create(archivePath)
	if err != nil {
		return "", fmt.Errorf("create archive file: %w", err)
	}
	defer f.Close()
	if _, err := io.Copy(f, resp.Body); err != nil {
		return "", fmt.Errorf("write archive: %w", err)
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	return archivePath, nil
}

// paxGlobalHeader is a tar metadata file (PAX extended header); not a real path. Skip it.
const paxGlobalHeader = "pax_global_header"

// extractTarGz extracts a .tar.gz file into dir and returns the single top-level directory name.
// Skips tar metadata like pax_global_header so we return the real repo root (e.g. owner-repo-sha).
func extractTarGz(archivePath, dir string) (string, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return "", err
	}
	defer f.Close()
	gr, err := gzip.NewReader(f)
	if err != nil {
		return "", err
	}
	defer gr.Close()
	tr := tar.NewReader(gr)
	var topDir string
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		name := filepath.Clean(h.Name)
		if name == "." || name == ".." || strings.Contains(name, "..") {
			continue
		}
		// Skip PAX global header and any path under it (not a real directory).
		if name == paxGlobalHeader || strings.HasPrefix(name, paxGlobalHeader+"/") {
			continue
		}
		if topDir == "" {
			if idx := strings.Index(name, "/"); idx > 0 {
				topDir = name[:idx]
			} else if name != "" && h.Typeflag == tar.TypeDir {
				topDir = name
			}
		}
		target := filepath.Join(dir, name)
		switch h.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return "", err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return "", err
			}
			w, err := os.Create(target)
			if err != nil {
				return "", err
			}
			if _, err := io.Copy(w, tr); err != nil {
				w.Close()
				return "", err
			}
			w.Close()
		}
	}
	if topDir == "" {
		return "", fmt.Errorf("archive has no top-level directory")
	}
	return topDir, nil
}
