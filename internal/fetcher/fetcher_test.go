package fetcher

import (
	"testing"

	"github.com/cjeanner/kustomap/internal/repository"
)

func TestNewFetcher_GitHub(t *testing.T) {
	info := &repository.RepositoryInfo{
		Type: repository.GitHub, Owner: "o", Repo: "r", Ref: "main", BaseURL: "https://github.com",
	}
	f, err := NewFetcher(info, "")
	if err != nil {
		t.Fatalf("NewFetcher(GitHub): %v", err)
	}
	if f == nil {
		t.Fatal("NewFetcher(GitHub) returned nil")
	}
}

func TestNewFetcher_GitLab(t *testing.T) {
	info := &repository.RepositoryInfo{
		Type: repository.GitLab, Owner: "g", Repo: "p", Ref: "main", BaseURL: "https://gitlab.com",
	}
	f, err := NewFetcher(info, "")
	if err != nil {
		t.Fatalf("NewFetcher(GitLab): %v", err)
	}
	if f == nil {
		t.Fatal("NewFetcher(GitLab) returned nil")
	}
}

func TestNewFetcher_Unsupported(t *testing.T) {
	info := &repository.RepositoryInfo{Type: repository.Unknown, Owner: "o", Repo: "r"}
	_, err := NewFetcher(info, "")
	if err == nil {
		t.Fatal("NewFetcher(Unknown) should error")
	}
}
