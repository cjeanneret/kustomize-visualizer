package storage

import (
	"testing"

	"github.com/cjeanner/kustomap/internal/types"
)

func TestNewMemoryStorage(t *testing.T) {
	s := NewMemoryStorage()
	if s == nil {
		t.Fatal("NewMemoryStorage() returned nil")
	}
}

func TestMemoryStorage_SaveAndGetGraph(t *testing.T) {
	s := NewMemoryStorage()
	g := &types.Graph{ID: "g1", Created: "2025-01-01", Elements: []types.Element{}}

	if err := s.SaveGraph(g); err != nil {
		t.Fatalf("SaveGraph: %v", err)
	}
	got, err := s.GetGraph("g1")
	if err != nil {
		t.Fatalf("GetGraph: %v", err)
	}
	if got.ID != "g1" {
		t.Errorf("GetGraph ID = %q, want g1", got.ID)
	}
}

func TestMemoryStorage_SaveGraph_EmptyID(t *testing.T) {
	s := NewMemoryStorage()
	err := s.SaveGraph(&types.Graph{ID: ""})
	if err == nil {
		t.Fatal("SaveGraph with empty ID should error")
	}
}

func TestMemoryStorage_GetGraph_NotFound(t *testing.T) {
	s := NewMemoryStorage()
	_, err := s.GetGraph("missing")
	if err == nil {
		t.Fatal("GetGraph(missing) should error")
	}
}

func TestMemoryStorage_GetNode(t *testing.T) {
	s := NewMemoryStorage()
	g := &types.Graph{
		ID:      "g1",
		Created: "2025-01-01",
		Elements: []types.Element{
			{Group: "nodes", Data: types.ElementData{ID: "n1", Label: "overlay", Type: "overlay", Path: "overlay"}},
			{Group: "nodes", Data: types.ElementData{ID: "n2", Label: "base", Type: "resource", Path: "base"}},
			{Group: "edges", Data: types.ElementData{Source: "n1", Target: "n2", EdgeType: "resource"}},
		},
	}
	if err := s.SaveGraph(g); err != nil {
		t.Fatalf("SaveGraph: %v", err)
	}

	details, err := s.GetNode("g1", "n1")
	if err != nil {
		t.Fatalf("GetNode: %v", err)
	}
	if details.ID != "n1" || details.Label != "overlay" {
		t.Errorf("GetNode = ID %q Label %q, want n1 overlay", details.ID, details.Label)
	}
	if len(details.Children) != 1 || details.Children[0] != "n2" {
		t.Errorf("Children = %v, want [n2]", details.Children)
	}
	if len(details.Parents) != 0 {
		t.Errorf("Parents = %v, want []", details.Parents)
	}
}

func TestMemoryStorage_GetNode_NotFound(t *testing.T) {
	s := NewMemoryStorage()
	g := &types.Graph{ID: "g1", Elements: []types.Element{}}
	s.SaveGraph(g)

	_, err := s.GetNode("g1", "missing-node")
	if err == nil {
		t.Fatal("GetNode(missing-node) should error")
	}
	_, err = s.GetNode("missing-graph", "n1")
	if err == nil {
		t.Fatal("GetNode(missing-graph) should error")
	}
}
