package server

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/cjeanner/kustomap/internal/storage"
	"github.com/cjeanner/kustomap/internal/types"
)

func TestNew(t *testing.T) {
	store := storage.NewMemoryStorage()
	webRoot := fstestMapFS{} // minimal fs.FS for router
	r := New(store, webRoot, nil)
	if r == nil {
		t.Fatal("New() returned nil")
	}
}

func TestServer_GetGraph_NotFound(t *testing.T) {
	store := storage.NewMemoryStorage()
	webRoot := fstestMapFS{}
	r := New(store, webRoot, nil)

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
	r := New(store, webRoot, nil)

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
	r := New(store, webRoot, nil)

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

func TestServer_GetCABundle_NotFound(t *testing.T) {
	store := storage.NewMemoryStorage()
	webRoot := fstestMapFS{}
	r := New(store, webRoot, nil)

	validMissingID := uuid.New().String()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/graph/"+validMissingID+"/ca-bundle", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("GET /api/v1/graph/%s/ca-bundle status = %d, want 404", validMissingID, rec.Code)
	}
}

func TestServer_GetCABundle_NoBundle(t *testing.T) {
	store := storage.NewMemoryStorage()
	graphID := uuid.New().String()
	g := &types.Graph{ID: graphID, Created: "2025-01-01", Elements: []types.Element{}}
	store.SaveGraph(g)
	webRoot := fstestMapFS{}
	r := New(store, webRoot, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/graph/"+graphID+"/ca-bundle", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("GET ca-bundle (no bundle) status = %d, want 404", rec.Code)
	}
}

func TestServer_GetCABundle_Found(t *testing.T) {
	store := storage.NewMemoryStorage()
	graphID := uuid.New().String()
	bundle := "-----BEGIN CERTIFICATE-----\ntest\n-----END CERTIFICATE-----\n"
	g := &types.Graph{
		ID:              graphID,
		Created:         "2025-01-01",
		Elements:        []types.Element{},
		CABundle:        bundle,
		CABundleExpires: "2025-12-31T00:00:00Z",
	}
	store.SaveGraph(g)
	webRoot := fstestMapFS{}
	r := New(store, webRoot, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/graph/"+graphID+"/ca-bundle", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("GET ca-bundle status = %d, want 200", rec.Code)
	}
	if rec.Header().Get("Content-Type") != "application/x-pem-file" {
		t.Errorf("Content-Type = %q, want application/x-pem-file", rec.Header().Get("Content-Type"))
	}
	if !strings.Contains(rec.Header().Get("Content-Disposition"), "ca-bundle-"+graphID+".pem") {
		t.Errorf("Content-Disposition missing filename: %q", rec.Header().Get("Content-Disposition"))
	}
	if rec.Body.String() != bundle {
		t.Errorf("body = %q, want %q", rec.Body.String(), bundle)
	}
}

// fstestMapFS is a minimal fs.FS for tests (avoids importing testing/fstest in production).
type fstestMapFS struct{}

func (fstestMapFS) Open(name string) (fs.File, error) {
	return nil, fs.ErrNotExist
}
