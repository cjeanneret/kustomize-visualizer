package repository

import (
	"testing"
)

func TestDetectRepository_GitHub(t *testing.T) {
	cases := []struct {
		name    string
		repoURL string
		owner   string
		repo    string
		path    string
		ref     string
		ambiguous string
	}{
		{
			name:    "simple",
			repoURL: "https://github.com/owner/repo",
			owner:   "owner",
			repo:    "repo",
			ref:     "main",
		},
		{
			name:    "with path (path not stored in detector; only tree/blob set AmbiguousPath)",
			repoURL: "https://github.com/owner/repo/deploy/base",
			owner:   "owner",
			repo:    "repo",
			ref:     "main",
		},
		{
			name:       "tree branch path",
			repoURL:    "https://github.com/owner/repo/tree/main/deploy/overlay",
			owner:      "owner",
			repo:       "repo",
			ambiguous:  "main/deploy/overlay",
			ref:        "main",
		},
		{
			name:       "blob branch path",
			repoURL:    "https://github.com/owner/repo/blob/develop/README.md",
			owner:      "owner",
			repo:       "repo",
			ambiguous:  "develop/README.md",
			ref:        "main",
		},
		{
			name:    "repo with .git",
			repoURL: "https://github.com/owner/repo.git",
			owner:   "owner",
			repo:    "repo",
			ref:     "main",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			info, err := DetectRepository(c.repoURL, "")
			if err != nil {
				t.Fatalf("DetectRepository error: %v", err)
			}
			if info.Type != GitHub {
				t.Errorf("Type = %s, want GitHub", info.Type)
			}
			if info.Owner != c.owner {
				t.Errorf("Owner = %q, want %q", info.Owner, c.owner)
			}
			if info.Repo != c.repo {
				t.Errorf("Repo = %q, want %q", info.Repo, c.repo)
			}
			if info.Path != c.path {
				t.Errorf("Path = %q, want %q", info.Path, c.path)
			}
			if info.Ref != c.ref {
				t.Errorf("Ref = %q, want %q", info.Ref, c.ref)
			}
			if info.AmbiguousPath != c.ambiguous {
				t.Errorf("AmbiguousPath = %q, want %q", info.AmbiguousPath, c.ambiguous)
			}
		})
	}
}

func TestDetectRepository_GitLab(t *testing.T) {
	cases := []struct {
		name       string
		repoURL    string
		owner      string
		repo       string
		path       string
		ambiguous  string
	}{
		{
			name:    "simple group/repo",
			repoURL: "https://gitlab.com/group/repo",
			owner:   "group",
			repo:    "repo",
		},
		{
			name:    "subgroup",
			repoURL: "https://gitlab.com/group/subgroup/project",
			owner:   "group/subgroup",
			repo:    "project",
		},
		{
			name:      "tree branch path",
			repoURL:   "https://gitlab.com/group/repo/-/tree/main/deploy/overlay",
			owner:     "group",
			repo:      "repo",
			ambiguous: "main/deploy/overlay",
		},
		{
			name:      "blob branch path",
			repoURL:   "https://gitlab.com/group/repo/-/blob/develop/README.md",
			owner:     "group",
			repo:      "repo",
			ambiguous: "develop/README.md",
		},
		{
			name:    "repo with .git",
			repoURL: "https://gitlab.com/group/repo.git",
			owner:   "group",
			repo:    "repo",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			info, err := DetectRepository(c.repoURL, "")
			if err != nil {
				t.Fatalf("DetectRepository error: %v", err)
			}
			if info.Type != GitLab {
				t.Errorf("Type = %s, want GitLab", info.Type)
			}
			if info.Owner != c.owner {
				t.Errorf("Owner = %q, want %q", info.Owner, c.owner)
			}
			if info.Repo != c.repo {
				t.Errorf("Repo = %q, want %q", info.Repo, c.repo)
			}
			if info.Path != c.path {
				t.Errorf("Path = %q, want %q", info.Path, c.path)
			}
			if info.AmbiguousPath != c.ambiguous {
				t.Errorf("AmbiguousPath = %q, want %q", info.AmbiguousPath, c.ambiguous)
			}
		})
	}
}

func TestParseGitHubURL(t *testing.T) {
	cases := []struct {
		path      string
		baseURL   string
		wantOwner string
		wantRepo  string
		wantAmbiguous string
		wantErr   bool
	}{
		{"owner/repo", "https://github.com", "owner", "repo", "", false},
		{"owner/repo.git", "https://github.com", "owner", "repo", "", false},
		{"owner/repo/tree/main/path", "https://github.com", "owner", "repo", "main/path", false},
		{"owner/repo/blob/develop/file", "https://github.com", "owner", "repo", "develop/file", false},
		{"owner", "https://github.com", "", "", "", true},
	}
	for _, c := range cases {
		t.Run(c.path, func(t *testing.T) {
			info, err := parseGitHubURL(c.path, c.baseURL)
			if c.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("parseGitHubURL error: %v", err)
			}
			if info.Owner != c.wantOwner || info.Repo != c.wantRepo {
				t.Errorf("Owner/Repo = %s/%s, want %s/%s", info.Owner, info.Repo, c.wantOwner, c.wantRepo)
			}
			if info.AmbiguousPath != c.wantAmbiguous {
				t.Errorf("AmbiguousPath = %q, want %q", info.AmbiguousPath, c.wantAmbiguous)
			}
		})
	}
}

func TestParseGitLabURL(t *testing.T) {
	cases := []struct {
		path         string
		baseURL      string
		wantOwner    string
		wantRepo     string
		wantAmbiguous string
		wantErr      bool
	}{
		{"group/repo", "https://gitlab.com", "group", "repo", "", false},
		{"group/subgroup/project", "https://gitlab.com", "group/subgroup", "project", "", false},
		{"group/repo/-/tree/main/deploy", "https://gitlab.com", "group", "repo", "main/deploy", false},
		{"group/repo/-/blob/develop/README", "https://gitlab.com", "group", "repo", "develop/README", false},
		{"group", "https://gitlab.com", "", "", "", true},
	}
	for _, c := range cases {
		t.Run(c.path, func(t *testing.T) {
			info, err := parseGitLabURL(c.path, c.baseURL)
			if c.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("parseGitLabURL error: %v", err)
			}
			if info.Owner != c.wantOwner || info.Repo != c.wantRepo {
				t.Errorf("Owner/Repo = %s/%s, want %s/%s", info.Owner, info.Repo, c.wantOwner, c.wantRepo)
			}
			if info.AmbiguousPath != c.wantAmbiguous {
				t.Errorf("AmbiguousPath = %q, want %q", info.AmbiguousPath, c.wantAmbiguous)
			}
		})
	}
}

func TestDetectRepository_InvalidURL(t *testing.T) {
	_, err := DetectRepository("://invalid", "")
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
}

// TestDetectRepository_RealURLs validates parsing of real GitHub/GitLab URLs.
func TestDetectRepository_RealURLs(t *testing.T) {
	cases := []struct {
		name        string
		repoURL     string
		wantType    RepositoryType
		wantOwner   string
		wantRepo    string
		wantAmbiguous string
	}{
		{
			name:        "openstack-k8s architecture tree main",
			repoURL:     "https://github.com/openstack-k8s-operators/architecture/tree/main/examples/va/hci/control-plane",
			wantType:    GitHub,
			wantOwner:   "openstack-k8s-operators",
			wantRepo:    "architecture",
			wantAmbiguous: "main/examples/va/hci/control-plane",
		},
		{
			name:        "GitLab self-hosted tree with branch path",
			repoURL:     "https://gitlab.example.com/group/gitops-examples/-/tree/components/base/environments/demo/scale-out/deployment?ref_type=heads",
			wantType:    GitLab,
			wantOwner:   "group",
			wantRepo:    "gitops-examples",
			wantAmbiguous: "components/base/environments/demo/scale-out/deployment",
		},
		{
			name:        "rhoso-gitops tree branch with slashes",
			repoURL:     "https://github.com/cjeanner/rhoso-gitops/tree/cjt/cleaning/test-nodeset-component/example/controlplane",
			wantType:    GitHub,
			wantOwner:   "cjeanner",
			wantRepo:    "rhoso-gitops",
			wantAmbiguous: "cjt/cleaning/test-nodeset-component/example/controlplane",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			info, err := DetectRepository(c.repoURL, "")
			if err != nil {
				t.Fatalf("DetectRepository error: %v", err)
			}
			if info.Type != c.wantType {
				t.Errorf("Type = %s, want %s", info.Type, c.wantType)
			}
			if info.Owner != c.wantOwner {
				t.Errorf("Owner = %q, want %q", info.Owner, c.wantOwner)
			}
			if info.Repo != c.wantRepo {
				t.Errorf("Repo = %q, want %q", info.Repo, c.wantRepo)
			}
			if info.AmbiguousPath != c.wantAmbiguous {
				t.Errorf("AmbiguousPath = %q, want %q", info.AmbiguousPath, c.wantAmbiguous)
			}
		})
	}
}
