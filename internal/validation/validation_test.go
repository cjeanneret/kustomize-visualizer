package validation

import (
	"strings"
	"testing"
)

func TestValidateAnalyzeURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"empty", "", true},
		{"valid github", "https://github.com/org/repo", false},
		{"valid github with path", "https://github.com/org/repo/tree/main/deploy", false},
		{"valid gitlab", "https://gitlab.com/group/project", false},
		{"self-hosted style allowed", "https://gitlab.mycompany.com/group/project", false},
		{"http rejected", "http://github.com/org/repo", true},
		{"file rejected", "file:///etc/passwd", true},
		{"localhost rejected", "https://localhost/org/repo", true},
		{"private IP rejected", "https://192.168.1.1/org/repo", true},
		{"loopback rejected", "https://127.0.0.1/org/repo", true},
		{"invalid URL", "://bad", true},
		{"missing host", "https:///path", true},
		{"too long", "https://github.com/" + strings.Repeat("a", MaxAnalyzeURLLength), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateAnalyzeURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateAnalyzeURL(%q) err = %v, wantErr %v", tt.url, err, tt.wantErr)
			}
		})
	}
}

func TestValidateFormat(t *testing.T) {
	if got := ValidateFormat(""); got != "json" {
		t.Errorf("ValidateFormat(\"\") = %q, want json", got)
	}
	if got := ValidateFormat("mermaid"); got != "mermaid" {
		t.Errorf("ValidateFormat(\"mermaid\") = %q, want mermaid", got)
	}
	if got := ValidateFormat("JSON"); got != "json" {
		t.Errorf("ValidateFormat(\"JSON\") = %q, want json", got)
	}
	if got := ValidateFormat("evil"); got != "json" {
		t.Errorf("ValidateFormat(\"evil\") = %q, want json (whitelist)", got)
	}
}

func TestValidateGraphID(t *testing.T) {
	validUUID := "550e8400-e29b-41d4-a716-446655440000"
	tests := []struct {
		name    string
		id      string
		wantErr bool
	}{
		{"empty", "", true},
		{"valid", validUUID, false},
		{"invalid format", "g1", true},
		{"not uuid", "not-a-uuid", true},
		{"too long", validUUID + "x", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateGraphID(tt.id)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateGraphID(%q) err = %v, wantErr %v", tt.id, err, tt.wantErr)
			}
		})
	}
}

func TestValidateHost(t *testing.T) {
	tests := []struct {
		name    string
		host    string
		wantErr bool
	}{
		{"empty", "", true},
		{"valid", "api.github.com", false},
		{"valid with port", "gitlab.example.com:443", false},
		{"localhost rejected", "localhost", true},
		{"private IP rejected", "192.168.1.1", true},
		{"loopback rejected", "127.0.0.1", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateHost(tt.host)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateHost(%q) err = %v, wantErr %v", tt.host, err, tt.wantErr)
			}
		})
	}
}

func TestValidateNodeID(t *testing.T) {
	tests := []struct {
		name    string
		nodeID  string
		wantErr bool
	}{
		{"empty", "", true},
		{"valid", "github:owner/repo/path@main", false},
		{"error node", "error:something", false},
		{"contains newline", "github:o\n/repo@main", true},
		{"contains CR", "github:o\r/repo@main", true},
		{"too long", strings.Repeat("a", MaxNodeIDLength+1), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateNodeID(tt.nodeID)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateNodeID(%q) err = %v, wantErr %v", tt.nodeID, err, tt.wantErr)
			}
		})
	}
}
