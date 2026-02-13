package types

// Graph represents the complete graph
type Graph struct {
	ID       string            `json:"id"`
	Elements []Element         `json:"elements"`
	Created  string            `json:"created"`
	// BaseURLs maps node ID -> repo base URL (e.g. https://gitlab.example.com) for build
	BaseURLs map[string]string `json:"base_urls,omitempty"`

	// CABundle is the concatenated PEM of CA certs from all hosts in the overlay stack.
	// Used for Argo CD when repos use self-signed or corporate CA certificates.
	CABundle string `json:"ca_bundle,omitempty"`
	// CABundleExpires is when the CA bundle is considered stale (RFC3339).
	CABundleExpires string `json:"ca_bundle_expires,omitempty"`
}

// Element can be a node or an edge
type Element struct {
	Group string      `json:"group"` // "nodes" ou "edges"
	Data  ElementData `json:"data"`
}

type ElementData struct {
	// Common
	ID string `json:"id"`

	// For nodes
	Label   string                 `json:"label,omitempty"`
	Type    string                 `json:"type,omitempty"` // "resource", "overlay", "component"
	Path    string                 `json:"path,omitempty"`
	Content map[string]interface{} `json:"content,omitempty"` // kustomization.yaml content

	// For edges
	Source   string `json:"source,omitempty"`
	Target   string `json:"target,omitempty"`
	EdgeType string `json:"edgeType,omitempty"` // "base", "resource", "patch"
}

// NodeDetails for details endpoint
type NodeDetails struct {
	ID      string                 `json:"id"`
	Label   string                 `json:"label"`
	Type    string                 `json:"type"`
	Path    string                 `json:"path"`
	Content map[string]interface{} `json:"content"`

	// Relations
	Parents  []string `json:"parents"`  // Nodes pointing to current node
	Children []string `json:"children"` // Nodes pointed by current node
}
