package parser

import (
	"errors"
	"testing"

	"github.com/cjeanner/kustomap/internal/fetcher"
	"github.com/cjeanner/kustomap/internal/repository"
	"github.com/cjeanner/kustomap/internal/types"
)

func TestIsYAMLFile(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"file.yaml", true},
		{"file.YAML", true},
		{"file.yml", true},
		{"file.YML", true},
		{"path/to/file.yaml", true},
		{"path/to/file.yml", true},
		{"file.yaml/extra", false},
		{"file.txt", false},
		{"noext", false},
		{"yaml", false},
	}
	for _, c := range cases {
		t.Run(c.path, func(t *testing.T) {
			got := isYAMLFile(c.path)
			if got != c.want {
				t.Errorf("isYAMLFile(%q) = %v, want %v", c.path, got, c.want)
			}
		})
	}
}

func TestResolvePath(t *testing.T) {
	cases := []struct {
		base     string
		relative string
		want     string
	}{
		{"overlay", "base", "overlay/base"},
		{"overlay/", "base", "overlay/base"},
		{"overlay/dev", "../base", "overlay/base"},
		{"a/b/c", "../../x", "a/x"},
		{"", "base", "base"},
		{"overlay", ".", "overlay"},
	}
	for _, c := range cases {
		t.Run(c.base+"_"+c.relative, func(t *testing.T) {
			got := resolvePath(c.base, c.relative)
			if got != c.want {
				t.Errorf("resolvePath(%q, %q) = %q, want %q", c.base, c.relative, got, c.want)
			}
		})
	}
}

func TestGetShortLabel(t *testing.T) {
	cases := []struct {
		name string
		path string
		want string
	}{
		{"empty", "", "unknown"},
		{"single segment", "base", "base"},
		{"two segments", "overlay/base", "overlay/base"},
		{"trailing slash", "/overlay/base/", "overlay/base"},
		{"long single filename", "openstackcontrolplane.yaml", "openstackcontrolplane.yaml"},
		{"many segments under limit", "a/b/c/d/e", "a/b/c/d/e"},
		{"single long segment", "this_is_a_very_long_filename_that_exceeds_multi_limit.yaml", "this_is_a_very_long_filename_that_exceeds_multi..."},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := getShortLabel(c.path)
			if got != c.want {
				t.Errorf("getShortLabel(%q) = %q, want %q", c.path, got, c.want)
			}
		})
	}
}

func TestSameRepoAsEntry(t *testing.T) {
	entryGitLab := &repository.RepositoryInfo{Type: repository.GitLab, Owner: "foo", Repo: "bar"}
	entryGitHub := &repository.RepositoryInfo{Type: repository.GitHub, Owner: "foo", Repo: "baz"}
	sameOwnerDiffRepo := &repository.RepositoryInfo{Type: repository.GitLab, Owner: "foo", Repo: "other"}
	diffOwnerSameRepo := &repository.RepositoryInfo{Type: repository.GitLab, Owner: "other", Repo: "bar"}
	sameRepo := &repository.RepositoryInfo{Type: repository.GitHub, Owner: "foo", Repo: "bar"} // same owner/repo as entryGitLab, type can differ

	entryLocal := &repository.RepositoryInfo{Type: repository.Local, RootPath: "/home/user/my-repo", Ref: "main"}
	localSameRoot := &repository.RepositoryInfo{Type: repository.Local, RootPath: "/home/user/my-repo", Ref: "main"}
	localOtherRoot := &repository.RepositoryInfo{Type: repository.Local, RootPath: "/home/user/other-repo", Ref: "main"}

	cases := []struct {
		name   string
		entry  *repository.RepositoryInfo
		current *repository.RepositoryInfo
		want   bool
	}{
		{"both nil", nil, nil, false},
		{"entry nil", nil, entryGitLab, false},
		{"current nil", entryGitLab, nil, false},
		{"same owner and repo", entryGitLab, sameRepo, true},
		{"same entry and current", entryGitLab, entryGitLab, true},
		{"different repo same owner", entryGitLab, sameOwnerDiffRepo, false},
		{"different owner same repo", entryGitLab, diffOwnerSameRepo, false},
		{"different repo and owner", entryGitLab, entryGitHub, false},
		{"local same root", entryLocal, localSameRoot, true},
		{"local different root", entryLocal, localOtherRoot, false},
		{"local entry empty root", &repository.RepositoryInfo{Type: repository.Local, RootPath: ""}, localSameRoot, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := sameRepoAsEntry(c.entry, c.current)
			if got != c.want {
				t.Errorf("sameRepoAsEntry(...) = %v, want %v", got, c.want)
			}
		})
	}
}

// mockFetcher implements fetcher.Fetcher for tests. PathToContent maps path -> kustomization content;
// PathToError maps path -> error for FindKustomizationInPath. If path is in PathToError, that error is returned.
type mockFetcher struct {
	PathToContent map[string]string
	PathToError   map[string]error
	ListFilesErr  error
}

func (m *mockFetcher) FetchFile(path string) ([]byte, error) {
	return nil, errors.New("not implemented")
}

func (m *mockFetcher) ListFiles() ([]string, error) {
	if m.ListFilesErr != nil {
		return nil, m.ListFilesErr
	}
	return nil, nil
}

func (m *mockFetcher) FindKustomizationInPath(path string) (string, error) {
	if m.PathToError != nil {
		if err, ok := m.PathToError[path]; ok {
			return "", err
		}
	}
	if m.PathToContent != nil {
		if content, ok := m.PathToContent[path]; ok {
			return content, nil
		}
	}
	return "", errors.New("no kustomization file found in path: " + path)
}

// TestProcessReference_RelativeRefFromDifferentRepo ensures that when we process a relative
// reference (e.g. ./deployment) from a node that lives in a different repo than the entry point,
// we use a fetcher for the current repo (e.g. GitHub), not the entry fetcher (e.g. GitLab).
// Regression test for: entry GitLab, component points to GitHub dataplane, dataplane has ./deployment;
// deployment must be fetched from GitHub, not GitLab.
func TestProcessReference_RelativeRefFromDifferentRepo_UsesCurrentRepoFetcher(t *testing.T) {
	// Entry: GitLab repo (like rhos-gitops-examples)
	entryRepo := &repository.RepositoryInfo{
		Type: repository.GitLab, Owner: "entry-owner", Repo: "entry-repo", Ref: "components/new-base",
		BaseURL: "https://gitlab.example.com",
	}
	// Entry fetcher: only knows about entry paths; returns error for GitHub paths (simulates "path not in this repo")
	entryFetcher := &mockFetcher{
		PathToContent: map[string]string{
			"environments/demo/nodeset": `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
components:
  - https://github.com/github-owner/github-repo/components/rhoso/dataplane?ref=cjt/cleaning/test-nodeset-component
resources: []
`,
		},
		PathToError: map[string]error{
			"components/rhoso/dataplane/deployment": errors.New("no kustomization file found in path: components/rhoso/dataplane/deployment"),
		},
	}

	// GitHub fetcher: has dataplane and deployment (the path exists on GitHub)
	dataplaneKust := `apiVersion: kustomize.config.k8s.io/v1alpha1
kind: Component
components:
  - ./nodeset
  - ./deployment
resources: []
`
	githubFetcher := &mockFetcher{
		PathToContent: map[string]string{
			"components/rhoso/dataplane":         dataplaneKust,
			"components/rhoso/dataplane/nodeset": `apiVersion: kustomize.config.k8s.io/v1alpha1
kind: Component
resources: [nodeset.yaml]
`,
			"components/rhoso/dataplane/deployment": `apiVersion: kustomize.config.k8s.io/v1alpha1
kind: Component
resources: [dataplane-deployment.yaml]
`,
		},
	}

	// Factory: return githubFetcher when repo is GitHub (github-owner/github-repo), so relative refs use it
	var factoryCalls int
	factory := func(repo *repository.RepositoryInfo, _ string) (fetcher.Fetcher, error) {
		factoryCalls++
		if repo.Owner == "github-owner" && repo.Repo == "github-repo" {
			return githubFetcher, nil
		}
		return nil, errors.New("unexpected repo in factory")
	}

	p := NewParser(entryFetcher, entryRepo)
	p.FetcherFactory = factory
	p.SetToken(repository.GitHub, "")

	// Parse from entry path; entry fetcher returns kustomization that has remote component to GitHub
	graph, err := p.Parse("environments/demo/nodeset")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	// We must have used the factory to create a fetcher for GitHub (for the remote component and then for ./nodeset and ./deployment)
	if factoryCalls < 1 {
		t.Errorf("expected FetcherFactory to be called at least once (for remote component or relative refs), got %d", factoryCalls)
	}

	// The deployment node must NOT be an error node (we used GitHub fetcher, which has the path)
	var deploymentNode *types.Element
	for i := range graph.Elements {
		e := &graph.Elements[i]
		if e.Group != "nodes" {
			continue
		}
		if e.Data.Path == "components/rhoso/dataplane/deployment" {
			deploymentNode = e
			break
		}
	}
	if deploymentNode == nil {
		t.Fatal("expected to find node with path components/rhoso/dataplane/deployment")
	}
	if deploymentNode.Data.Type == "error" {
		t.Errorf("deployment node should not be an error (relative ref must use current-repo fetcher, not entry fetcher): content=%v", deploymentNode.Data.Content)
	}
}
