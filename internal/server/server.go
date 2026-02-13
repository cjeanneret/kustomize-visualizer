package server

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"

	"github.com/cjeanner/kustomap/internal/build"
	"github.com/cjeanner/kustomap/internal/export"
	"github.com/cjeanner/kustomap/internal/fetcher"
	"github.com/cjeanner/kustomap/internal/parser"
	"github.com/cjeanner/kustomap/internal/repository"
	"github.com/cjeanner/kustomap/internal/storage"
	"github.com/cjeanner/kustomap/internal/validation"
)

// Max request body sizes for JSON endpoints (OWASP API4: Unrestricted Resource Consumption).
const (
	maxAnalyzeBodyBytes = 64 * 1024  // 64 KB for analyze (URL + tokens)
	maxBuildBodyBytes   = 32 * 1024  // 32 KB for build (tokens only)
)

// AnalyzeRequest is the JSON body for POST /api/v1/analyze.
type AnalyzeRequest struct {
	URL         string `json:"url"`
	GitHubToken string `json:"github_token"`
	GitLabToken string `json:"gitlab_token"`
}

// AnalyzeResponse is the JSON response for analyze and error responses.
type AnalyzeResponse struct {
	ID      string `json:"id"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

// New builds a chi router with API and static file routes.
// webRoot is the embedded web filesystem (e.g. fs.Sub(embedFS, "web")).
func New(store storage.Storage, webRoot fs.FS) *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(setContentType)

	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/analyze", handleAnalyze(store))
		r.Get("/graph/{id}", handleGetGraph(store))
		r.Get("/node/{graphID}/{nodeID}", handleGetNode(store))
		r.Post("/node/{graphID}/{nodeID}/build", handleBuildNode(store))
	})

	r.Get("/*", func(w http.ResponseWriter, r *http.Request) {
		http.FileServer(http.FS(webRoot)).ServeHTTP(w, r)
	})

	return r
}

func setContentType(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}
		ext := strings.ToLower(filepath.Ext(r.URL.Path))
		var contentType string
		switch ext {
		case ".js":
			contentType = "application/javascript; charset=utf-8"
		case ".css":
			contentType = "text/css; charset=utf-8"
		case ".html":
			contentType = "text/html; charset=utf-8"
		case ".json":
			contentType = "application/json; charset=utf-8"
		case ".png":
			contentType = "image/png"
		case ".jpg", ".jpeg":
			contentType = "image/jpeg"
		case ".svg":
			contentType = "image/svg+xml"
		}
		if contentType != "" {
			w.Header().Set("Content-Type", contentType)
		}
		next.ServeHTTP(w, r)
	})
}

func handleAnalyze(store storage.Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxAnalyzeBodyBytes)
		var req AnalyzeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		if err := validation.ValidateAnalyzeURL(req.URL); err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}

		log.Printf("Analyzing repository: %s", truncateForLog(req.URL, 256))

		repoInfo, err := repository.DetectRepository(req.URL, "")
		if err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}
		log.Printf("✅ Detected: %s", repoInfo.String())

		var token string
		switch repoInfo.Type {
		case repository.GitHub:
			token = req.GitHubToken
		case repository.GitLab:
			token = req.GitLabToken
		}

		if repoInfo.AmbiguousPath != "" {
			log.Printf("Resolving ambiguous path: %s", repoInfo.AmbiguousPath)
			branch, path, err := repository.ResolveBranchAndPath(repoInfo, repoInfo.AmbiguousPath, token)
			if err != nil {
				respondError(w, http.StatusBadRequest, fmt.Sprintf("failed to resolve branch: %v", err))
				return
			}
			repoInfo.Ref = branch
			repoInfo.Path = path
			log.Printf("✅ Resolved: branch=%s, path=%s", branch, path)
		}

		searchPath := repoInfo.Path

		f, err := fetcher.NewFetcher(repoInfo, token)
		if err != nil {
			log.Printf("NewFetcher error: %v", err)
			respondError(w, http.StatusInternalServerError, "Failed to create fetcher")
			return
		}
		log.Printf("✅ Created fetcher")

		p := parser.NewParser(f, repoInfo)
		p.SetToken(repository.GitHub, req.GitHubToken)
		p.SetToken(repository.GitLab, req.GitLabToken)

		graph, err := p.Parse(searchPath)
		if err != nil {
			log.Printf("Parse error: %v", err)
			respondError(w, http.StatusInternalServerError, "Failed to analyze repository")
			return
		}

		graph.ID = uuid.New().String()
		graph.Created = time.Now().Format(time.RFC3339)

		if err := store.SaveGraph(graph); err != nil {
			log.Printf("SaveGraph error: %v", err)
			respondError(w, http.StatusInternalServerError, "Failed to save graph")
			return
		}
		log.Printf("✅ Graph saved with ID: %s (%d elements)", graph.ID, len(graph.Elements))

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(AnalyzeResponse{ID: graph.ID, Status: "success"})
	}
}

func handleGetGraph(store storage.Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		graphID := chi.URLParam(r, "id")
		if err := validation.ValidateGraphID(graphID); err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}
		format := validation.ValidateFormat(r.URL.Query().Get("format"))
		log.Printf("Retrieving graph: %s (format: %s)", graphID, format)

		graph, err := store.GetGraph(graphID)
		if err != nil {
			respondError(w, http.StatusNotFound, "Graph not found")
			return
		}

		switch format {
		case "mermaid":
			mermaidCode := export.ToMermaid(graph)
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			// graphID is already validated as UUID, safe for header
			w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=graph-%s.mmd", graphID))
			w.Write([]byte(mermaidCode))
		default:
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(graph)
		}
	}
}

func handleGetNode(store storage.Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		graphID := chi.URLParam(r, "graphID")
		nodeID := chi.URLParam(r, "nodeID")
		if err := validation.ValidateGraphID(graphID); err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}
		decodedNodeID, err := url.QueryUnescape(nodeID)
		if err != nil {
			respondError(w, http.StatusBadRequest, "Invalid node ID")
			return
		}
		if err := validation.ValidateNodeID(decodedNodeID); err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}
		log.Printf("Retrieving node: %s from graph: %s", truncateForLog(nodeID, 128), graphID)

		nodeDetails, err := store.GetNode(graphID, decodedNodeID)
		if err != nil {
			respondError(w, http.StatusNotFound, err.Error())
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(nodeDetails)
	}
}

// BuildRequest is the optional JSON body for POST /api/v1/node/{graphID}/{nodeID}/build.
type BuildRequest struct {
	GitHubToken string `json:"github_token"`
	GitLabToken string `json:"gitlab_token"`
}

// BuildResponse is the JSON response for a successful build.
type BuildResponse struct {
	YAML string `json:"yaml"`
}

func handleBuildNode(store storage.Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		graphID := chi.URLParam(r, "graphID")
		nodeID := chi.URLParam(r, "nodeID")
		if err := validation.ValidateGraphID(graphID); err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}
		decodedNodeID, err := url.QueryUnescape(nodeID)
		if err != nil {
			respondError(w, http.StatusBadRequest, "Invalid node ID")
			return
		}
		if _, err := build.ParseNodeID(decodedNodeID); err != nil {
			respondError(w, http.StatusBadRequest, "Invalid node ID format")
			return
		}

		nodeDetails, err := store.GetNode(graphID, decodedNodeID)
		if err != nil {
			respondError(w, http.StatusNotFound, "Node not found")
			return
		}

		if nodeDetails.Type == "component" {
			respondError(w, http.StatusBadRequest, "Build is not available for component nodes; use an overlay or resource node")
			return
		}
		if nodeDetails.Type == "error" {
			respondError(w, http.StatusBadRequest, "Build is not available for error nodes")
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, maxBuildBodyBytes)
		var req BuildRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		graph, err := store.GetGraph(graphID)
		if err != nil {
			respondError(w, http.StatusNotFound, "Graph not found")
			return
		}
		baseURL := ""
		if graph.BaseURLs != nil {
			baseURL = graph.BaseURLs[decodedNodeID]
		}

		b := build.NewBuilder(req.GitHubToken, req.GitLabToken)
		yamlOut, err := b.Build(decodedNodeID, baseURL)
		if err != nil {
			log.Printf("Build failed for node %s: %v", decodedNodeID, err)
			respondError(w, http.StatusUnprocessableEntity, "Build failed")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(BuildResponse{YAML: yamlOut})
	}
}

func respondError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(AnalyzeResponse{Status: "error", Message: message})
}

// truncateForLog truncates s to maxLen for safe logging (avoids huge or sensitive data in logs).
func truncateForLog(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
