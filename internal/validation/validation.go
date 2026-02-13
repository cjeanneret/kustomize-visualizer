// Package validation provides input validation for user-provided data to mitigate
// injection, SSRF, and ensure expected formats (OWASP API Security, Go best practices).
package validation

import (
	"fmt"
	"net"
	"net/url"
	"strings"

	"github.com/google/uuid"
)

// MaxAnalyzeURLLength is the maximum allowed length for the analyze request URL.
const MaxAnalyzeURLLength = 4096

// Allowed URL schemes for repository analysis (SSRF prevention).
var allowedSchemes = map[string]bool{
	"https": true,
	"http":  false, // disabled by default; enable only for localhost in dev if needed
}

// ValidateAnalyzeURL ensures the URL is safe for server-side fetch (SSRF prevention).
// It restricts scheme to https and rejects URLs that target internal services
// (private IPs, loopback, .local, localhost). Any other public host is allowed,
// including self-hosted GitLab/GitHub, without using an allow list.
func ValidateAnalyzeURL(rawURL string) error {
	if rawURL == "" {
		return fmt.Errorf("URL is required")
	}
	if len(rawURL) > MaxAnalyzeURLLength {
		return fmt.Errorf("URL exceeds maximum length of %d", MaxAnalyzeURLLength)
	}

	u, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	if !allowedSchemes[u.Scheme] {
		return fmt.Errorf("URL scheme must be https")
	}

	host := strings.ToLower(strings.TrimSpace(u.Hostname()))
	if host == "" {
		return fmt.Errorf("invalid URL: missing host")
	}

	// Reject SSRF targets: private/loopback IPs and internal hostnames only.
	// No allow list: any public host is accepted (e.g. github.com, gitlab.com,
	// self-hosted instances) so we never leak internal hostnames via config.
	if err := rejectPrivateOrReservedHost(host); err != nil {
		return err
	}

	return nil
}

// ValidateHost ensures a hostname is safe for outbound connections (SSRF prevention).
// Used when connecting to hosts derived from the graph (e.g. for CA cert collection).
// Rejects private/loopback IPs and internal hostnames.
func ValidateHost(host string) error {
	h := strings.ToLower(strings.TrimSpace(host))
	if h == "" {
		return fmt.Errorf("host is required")
	}
	if idx := strings.Index(h, ":"); idx > 0 {
		h = h[:idx]
	}
	return rejectPrivateOrReservedHost(h)
}

// rejectPrivateOrReservedHost prevents SSRF to internal/reserved addresses.
func rejectPrivateOrReservedHost(host string) error {
	// Strip port for resolution
	if idx := strings.Index(host, ":"); idx > 0 {
		host = host[:idx]
	}
	ip := net.ParseIP(host)
	if ip != nil {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
			return fmt.Errorf("URL host must not be a private or loopback address")
		}
		return nil
	}
	// Host is a name; reject common internal names
	lower := strings.ToLower(host)
	if lower == "localhost" || strings.HasSuffix(lower, ".localhost") ||
		strings.HasSuffix(lower, ".local") || lower == "internal" {
		return fmt.Errorf("URL host must not be a private or loopback hostname")
	}
	return nil
}

// Format for graph export (whitelist to prevent injection).
var validFormats = map[string]bool{
	"json":    true,
	"mermaid": true,
}

// ValidateFormat returns the format if it is allowed, or "json" as default.
// Used for the ?format= query parameter.
func ValidateFormat(format string) string {
	f := strings.ToLower(strings.TrimSpace(format))
	if validFormats[f] {
		return f
	}
	return "json"
}

// ValidateGraphID returns an error if id is not a valid UUID format.
// Prevents header injection (e.g. CRLF in Content-Disposition) and ensures
// consistent identifier format.
func ValidateGraphID(id string) error {
	if id == "" {
		return fmt.Errorf("graph ID is required")
	}
	if len(id) > 64 {
		return fmt.Errorf("graph ID too long")
	}
	if _, err := uuid.Parse(id); err != nil {
		return fmt.Errorf("invalid graph ID format")
	}
	return nil
}

// MaxNodeIDLength is the maximum allowed length for a node ID (URL path segment).
const MaxNodeIDLength = 2048

// ValidateNodeID ensures the node ID has a safe length and contains no control
// characters (prevents log/header injection). Does not require a specific format
// so that both "type:owner/repo/path@ref" and "error:..." IDs are accepted.
func ValidateNodeID(nodeID string) error {
	if nodeID == "" {
		return fmt.Errorf("node ID is required")
	}
	if len(nodeID) > MaxNodeIDLength {
		return fmt.Errorf("node ID too long")
	}
	for _, r := range nodeID {
		if r == '\r' || r == '\n' || r == '\x00' {
			return fmt.Errorf("invalid node ID: control characters not allowed")
		}
	}
	return nil
}
