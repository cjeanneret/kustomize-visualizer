package export

import (
	"strings"
	"testing"

	"github.com/cjeanner/kustomap/internal/types"
)

func TestToMermaid_NilOrEmpty(t *testing.T) {
	if got := ToMermaid(nil); !strings.Contains(got, "empty graph") {
		t.Errorf("ToMermaid(nil) = %q, want substring 'empty graph'", got)
	}
	empty := &types.Graph{Elements: []types.Element{}}
	if got := ToMermaid(empty); !strings.Contains(got, "empty graph") {
		t.Errorf("ToMermaid(empty) = %q, want substring 'empty graph'", got)
	}
}

func TestToMermaid_OneNodeOneEdge(t *testing.T) {
	g := &types.Graph{
		Elements: []types.Element{
			{Group: "nodes", Data: types.ElementData{ID: "a", Label: "overlay", Type: "overlay", Path: "overlay"}},
			{Group: "nodes", Data: types.ElementData{ID: "b", Label: "base", Type: "resource", Path: "base"}},
			{Group: "edges", Data: types.ElementData{ID: "a->b", Source: "a", Target: "b", EdgeType: "resource"}},
		},
	}
	got := ToMermaid(g)
	if !strings.HasPrefix(got, "flowchart TD") {
		t.Errorf("expected 'flowchart TD' prefix, got %q", got)
	}
	if !strings.Contains(got, "overlay") || !strings.Contains(got, "base") {
		t.Errorf("expected node labels in output: %s", got)
	}
	if !strings.Contains(got, "-->") {
		t.Errorf("expected edge in output: %s", got)
	}
}

func TestToMermaid_EdgeTypeLabel(t *testing.T) {
	g := &types.Graph{
		Elements: []types.Element{
			{Group: "nodes", Data: types.ElementData{ID: "x", Label: "x"}},
			{Group: "nodes", Data: types.ElementData{ID: "y", Label: "y"}},
			{Group: "edges", Data: types.ElementData{Source: "x", Target: "y", EdgeType: "component"}},
		},
	}
	got := ToMermaid(g)
	if !strings.Contains(got, "component") {
		t.Errorf("expected edge type 'component' in output: %s", got)
	}
}
