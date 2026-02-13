package repository

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestDetectLocalRepository(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir: %v", err)
	}

	tmpDir, err := os.MkdirTemp(home, "kustomap-local-repo-*")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Initialize a proper git repo (required for git rev-parse --show-toplevel from subdirs)
	if out, err := exec.Command("git", "init", tmpDir).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v (%s)", err, string(out))
	}

	// Subdir: deploy/overlay
	overlayDir := filepath.Join(tmpDir, "deploy", "overlay")
	if err := os.MkdirAll(overlayDir, 0755); err != nil {
		t.Fatalf("MkdirAll overlay: %v", err)
	}

	t.Run("repo root", func(t *testing.T) {
		info, err := DetectLocalRepository(tmpDir)
		if err != nil {
			t.Fatalf("DetectLocalRepository: %v", err)
		}
		if info.Type != Local {
			t.Errorf("Type = %s, want local", info.Type)
		}
		if info.RootPath != tmpDir {
			t.Errorf("RootPath = %q, want %q", info.RootPath, tmpDir)
		}
		if info.Path != "" {
			t.Errorf("Path = %q, want empty", info.Path)
		}
		if info.Ref == "" {
			t.Error("Ref should be set (main or HEAD)")
		}
	})

	t.Run("subdir overlay", func(t *testing.T) {
		info, err := DetectLocalRepository(overlayDir)
		if err != nil {
			t.Fatalf("DetectLocalRepository: %v", err)
		}
		if info.Type != Local {
			t.Errorf("Type = %s, want local", info.Type)
		}
		if info.RootPath != tmpDir {
			t.Errorf("RootPath = %q, want %q", info.RootPath, tmpDir)
		}
		if info.Path != "deploy/overlay" {
			t.Errorf("Path = %q, want deploy/overlay", info.Path)
		}
	})

	t.Run("detached HEAD", func(t *testing.T) {
		// Create initial commit so we can detach
		if out, err := exec.Command("git", "-C", tmpDir, "commit", "--allow-empty", "-m", "init").CombinedOutput(); err != nil {
			t.Fatalf("git commit: %v (%s)", err, string(out))
		}
		hashOut, err := exec.Command("git", "-C", tmpDir, "rev-parse", "HEAD").Output()
		if err != nil {
			t.Fatalf("git rev-parse: %v", err)
		}
		fullHash := string(hashOut[:40])
		if out, err := exec.Command("git", "-C", tmpDir, "checkout", "--detach").CombinedOutput(); err != nil {
			t.Fatalf("git checkout --detach: %v (%s)", err, string(out))
		}

		info, err := DetectLocalRepository(tmpDir)
		if err != nil {
			t.Fatalf("DetectLocalRepository (detached): %v", err)
		}
		wantShort := fullHash[:7]
		if info.Ref != wantShort {
			t.Errorf("Ref = %q, want short hash %q (detached HEAD)", info.Ref, wantShort)
		}
	})

	t.Run("non-git dir", func(t *testing.T) {
		plainDir, err := os.MkdirTemp(home, "kustomap-plain-*")
		if err != nil {
			t.Fatalf("MkdirTemp: %v", err)
		}
		defer os.RemoveAll(plainDir)

		info, err := DetectLocalRepository(plainDir)
		if err != nil {
			t.Fatalf("DetectLocalRepository (non-git): %v", err)
		}
		if info.RootPath != plainDir {
			t.Errorf("RootPath = %q, want %q", info.RootPath, plainDir)
		}
		if info.Path != "" {
			t.Errorf("Path = %q, want empty for root", info.Path)
		}
		if info.Ref != defaultLocalRef {
			t.Errorf("Ref = %q, want %q (default for non-git)", info.Ref, defaultLocalRef)
		}
	})
}
