package server

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/cjeanner/kustomap/internal/storage"
	"github.com/cjeanner/kustomap/internal/types"
)

func TestNew(t *testing.T) {
	store := storage.NewMemoryStorage()
	webRoot := fstestMapFS{} // minimal fs.FS for router
	r := New(store, webRoot)
	if r == nil {
		t.Fatal("New() returned nil")
	}
}

func TestServer_GetGraph_NotFound(t *testing.T) {
	store := storage.NewMemoryStorage()
	webRoot := fstestMapFS{}
	r := New(store, webRoot)

	// Use a valid UUID that is not in the store → 404
	validMissingID := uuid.New().String()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/graph/"+validMissingID, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("GET /api/v1/graph/%s status = %d, want 404", validMissingID, rec.Code)
	}
}

func TestServer_GetGraph_InvalidID(t *testing.T) {
	store := storage.NewMemoryStorage()
	webRoot := fstestMapFS{}
	r := New(store, webRoot)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/graph/not-a-uuid", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("GET /api/v1/graph/not-a-uuid status = %d, want 400", rec.Code)
	}
}

func TestServer_GetGraph_Found(t *testing.T) {
	store := storage.NewMemoryStorage()
	graphID := uuid.New().String()
	g := &types.Graph{ID: graphID, Created: "2025-01-01", Elements: []types.Element{}}
	store.SaveGraph(g)
	webRoot := fstestMapFS{}
	r := New(store, webRoot)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/graph/"+graphID, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("GET /api/v1/graph/%s status = %d, want 200", graphID, rec.Code)
	}
	if rec.Header().Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", rec.Header().Get("Content-Type"))
	}
}

// fstestMapFS is a minimal fs.FS for tests (avoids importing testing/fstest in production).
type fstestMapFS struct{}

func (fstestMapFS) Open(name string) (fs.File, error) {
	return nil, fs.ErrNotExist
}
