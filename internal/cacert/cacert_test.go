package cacert

import (
	"crypto/x509"
	"testing"

	"github.com/cjeanner/kustomap/internal/types"
)

func TestResolveTLSHost(t *testing.T) {
	tests := []struct {
		baseURL string
		want   string
	}{
		{"https://github.com", "api.github.com"},
		{"https://github.com/org/repo", "api.github.com"},
		{"https://gitlab.example.com", "gitlab.example.com"},
		{"https://gitlab.com", "gitlab.com"},
		{"https://ghe.example.com", "ghe.example.com"},
		{"https://custom.gitlab.io", "custom.gitlab.io"},
	}
	for _, tt := range tests {
		t.Run(tt.baseURL, func(t *testing.T) {
			got, err := resolveTLSHost(tt.baseURL)
			if err != nil {
				t.Fatalf("resolveTLSHost(%q): %v", tt.baseURL, err)
			}
			if got != tt.want {
				t.Errorf("resolveTLSHost(%q) = %q, want %q", tt.baseURL, got, tt.want)
			}
		})
	}
}

func TestUniqueHostsFromGraph(t *testing.T) {
	c := NewCollector(0)
	tests := []struct {
		name  string
		graph *types.Graph
		want  []string
	}{
		{
			name:  "nil",
			graph: nil,
			want:  nil,
		},
		{
			name:  "empty BaseURLs",
			graph: &types.Graph{BaseURLs: map[string]string{}},
			want:  nil,
		},
		{
			name: "single host",
			graph: &types.Graph{
				BaseURLs: map[string]string{"node1": "https://gitlab.example.com"},
			},
			want: []string{"gitlab.example.com"},
		},
		{
			name: "github resolves to api",
			graph: &types.Graph{
				BaseURLs: map[string]string{"node1": "https://github.com"},
			},
			want: []string{"api.github.com"},
		},
		{
			name: "deduplicated and sorted",
			graph: &types.Graph{
				BaseURLs: map[string]string{
					"node1": "https://gitlab.example.com",
					"node2": "https://github.com",
					"node3": "https://gitlab.example.com", // duplicate
				},
			},
			want: []string{"api.github.com", "gitlab.example.com"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := c.uniqueHostsFromGraph(tt.graph)
			if len(got) != len(tt.want) {
				t.Errorf("uniqueHostsFromGraph() = %v, want %v", got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("uniqueHostsFromGraph() = %v, want %v", got, tt.want)
					return
				}
			}
		})
	}
}

func TestCertFingerprint(t *testing.T) {
	// certFingerprint should be deterministic for the same cert
	fp := certFingerprint(&x509.Certificate{Raw: []byte{1, 2, 3}})
	if fp == "" {
		t.Error("certFingerprint returned empty")
	}
	// Same input → same output
	fp2 := certFingerprint(&x509.Certificate{Raw: []byte{1, 2, 3}})
	if fp != fp2 {
		t.Errorf("certFingerprint not deterministic: %q != %q", fp, fp2)
	}
}
