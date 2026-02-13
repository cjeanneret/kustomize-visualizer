package validation

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsLocalPath(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{"empty", "", false},
		{"file scheme", "file:///home/user/repo", true},
		{"tilde", "~/repo", true},
		{"tilde only", "~", true},
		{"absolute unix", "/home/user/repo", true},
		{"https url", "https://github.com/org/repo", false},
		{"relative", "some/path", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsLocalPath(tt.in)
			if got != tt.want {
				t.Errorf("IsLocalPath(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestValidateLocalPath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir: %v", err)
	}

	// Create a temp dir under $HOME for valid path tests
	tmpDir, err := os.MkdirTemp(home, "kustomap-validate-local-*")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"empty", "", true},
		{"valid under home", tmpDir, false},
		{"valid tilde", "~" + tmpDir[len(home):], false},
		{"outside home", "/etc", true},
		{"does not exist", home + "/nonexistent-path-12345", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValidateLocalPath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateLocalPath(%q) err = %v, wantErr %v", tt.path, err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if got == "" {
					t.Error("ValidateLocalPath returned empty path")
				}
				absGot, _ := filepath.Abs(got)
				absHome, _ := filepath.Abs(home)
				if absGot != filepath.Clean(absHome) && len(absGot) < len(absHome) {
					t.Errorf("path %q not under home %q", got, home)
				}
			}
		})
	}
}
