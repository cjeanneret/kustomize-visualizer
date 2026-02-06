package storage

import (
	"fmt"
	"sync"

	"github.com/cjeanner/kustomap/internal/types"
)

// Storage interface for graph persistence
type Storage interface {
	SaveGraph(graph *types.Graph) error
	GetGraph(id string) (*types.Graph, error)
	GetNode(graphID, nodeID string) (*types.NodeDetails, error)
}

// MemoryStorage stores graphs in memory
type MemoryStorage struct {
	graphs map[string]*types.Graph
	mu     sync.RWMutex
}

// NewMemoryStorage creates a new in-memory storage
func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{
		graphs: make(map[string]*types.Graph),
	}
}

// SaveGraph saves a graph to memory
func (s *MemoryStorage) SaveGraph(graph *types.Graph) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if graph.ID == "" {
		return fmt.Errorf("graph ID is required")
	}

	s.graphs[graph.ID] = graph
	return nil
}

// GetGraph retrieves a graph by ID
func (s *MemoryStorage) GetGraph(id string) (*types.Graph, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	graph, exists := s.graphs[id]
	if !exists {
		return nil, fmt.Errorf("graph not found: %s", id)
	}

	return graph, nil
}

// GetNode retrieves detailed information about a specific node
func (s *MemoryStorage) GetNode(graphID, nodeID string) (*types.NodeDetails, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	graph, exists := s.graphs[graphID]
	if !exists {
		return nil, fmt.Errorf("graph not found: %s", graphID)
	}

	// Find the node
	var nodeData *types.ElementData
	for _, elem := range graph.Elements {
		if elem.Group == "nodes" && elem.Data.ID == nodeID {
			nodeData = &elem.Data
			break
		}
	}

	if nodeData == nil {
		return nil, fmt.Errorf("node not found: %s", nodeID)
	}

	// Build NodeDetails with relationships
	details := &types.NodeDetails{
		ID:       nodeData.ID,
		Label:    nodeData.Label,
		Type:     nodeData.Type,
		Path:     nodeData.Path,
		Content:  nodeData.Content,
		Parents:  []string{},
		Children: []string{},
	}

	// Find parent and child nodes
	for _, elem := range graph.Elements {
		if elem.Group == "edges" {
			if elem.Data.Target == nodeID {
				// This edge points TO our node (parent relationship)
				details.Parents = append(details.Parents, elem.Data.Source)
			}
			if elem.Data.Source == nodeID {
				// This edge points FROM our node (child relationship)
				details.Children = append(details.Children, elem.Data.Target)
			}
		}
	}

	return details, nil
}
