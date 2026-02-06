package export

import (
	"fmt"
	"strings"

	"github.com/cjeanner/kustomap/internal/types"
)

// ToMermaid converts a kustomize dependency graph to Mermaid flowchart syntax.
// Node types and edge types are preserved as labels where possible.
// Safe for use in documentation (e.g. README, MkDocs, Docusaurus).
func ToMermaid(graph *types.Graph) string {
	if graph == nil || len(graph.Elements) == 0 {
		return "flowchart TD\n  empty[\"empty graph\"]\n"
	}

	nodeIDToSafe := make(map[string]string)
	var nodeOrder []string
	safeIndex := 0
	for i := range graph.Elements {
		e := &graph.Elements[i]
		if e.Group != "nodes" {
			continue
		}
		id := e.Data.ID
		if _, ok := nodeIDToSafe[id]; ok {
			continue
		}
		safeID := fmt.Sprintf("n%d", safeIndex)
		safeIndex++
		nodeIDToSafe[id] = safeID
		nodeOrder = append(nodeOrder, id)
	}

	var b strings.Builder
	b.WriteString("flowchart TD\n")

	// Output nodes: safeId["label"] with optional type hint
	for _, id := range nodeOrder {
		var label string
		for i := range graph.Elements {
			e := &graph.Elements[i]
			if e.Group == "nodes" && e.Data.ID == id {
				label = e.Data.Label
				if label == "" {
					label = id
				}
				break
			}
		}
		safeID := nodeIDToSafe[id]
		escaped := escapeMermaidLabel(label)
		b.WriteString(fmt.Sprintf("  %s[\"%s\"]\n", safeID, escaped))
	}

	// Output edges: source --> target or source -->|edgeType| target
	for i := range graph.Elements {
		e := &graph.Elements[i]
		if e.Group != "edges" {
			continue
		}
		src := nodeIDToSafe[e.Data.Source]
		tgt := nodeIDToSafe[e.Data.Target]
		if src == "" || tgt == "" {
			continue
		}
		if e.Data.EdgeType != "" {
			edgeLabel := escapeMermaidLabel(e.Data.EdgeType)
			b.WriteString(fmt.Sprintf("  %s -->|\"%s\"| %s\n", src, edgeLabel, tgt))
		} else {
			b.WriteString(fmt.Sprintf("  %s --> %s\n", src, tgt))
		}
	}

	return strings.TrimSuffix(b.String(), "\n")
}

// escapeMermaidLabel escapes double quotes and backslashes for use inside "..." in Mermaid.
func escapeMermaidLabel(s string) string {
	return strings.NewReplacer(
		`\`, `\\`,
		`"`, `\"`,
	).Replace(s)
}
