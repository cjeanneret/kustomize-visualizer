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

	"github.com/cjeanner/kustomap/internal/export"
	"github.com/cjeanner/kustomap/internal/fetcher"
	"github.com/cjeanner/kustomap/internal/parser"
	"github.com/cjeanner/kustomap/internal/repository"
	"github.com/cjeanner/kustomap/internal/storage"
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
		var req AnalyzeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		if req.URL == "" {
			http.Error(w, "URL is required", http.StatusBadRequest)
			return
		}

		log.Printf("Analyzing repository: %s", req.URL)

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
			respondError(w, http.StatusInternalServerError, err.Error())
			return
		}
		log.Printf("✅ Created fetcher")

		p := parser.NewParser(f, repoInfo)
		p.SetToken(repository.GitHub, req.GitHubToken)
		p.SetToken(repository.GitLab, req.GitLabToken)

		graph, err := p.Parse(searchPath)
		if err != nil {
			respondError(w, http.StatusInternalServerError, err.Error())
			return
		}

		graph.ID = uuid.New().String()
		graph.Created = time.Now().Format(time.RFC3339)

		if err := store.SaveGraph(graph); err != nil {
			respondError(w, http.StatusInternalServerError, fmt.Sprintf("failed to save graph: %v", err))
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
		format := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("format")))
		if format == "" {
			format = "json"
		}
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
			w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=graph-%s.mmd", graphID))
			w.Write([]byte(mermaidCode))
		case "json":
			fallthrough
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

		decodedNodeID, err := url.QueryUnescape(nodeID)
		if err != nil {
			respondError(w, http.StatusBadRequest, "Invalid node ID")
			return
		}
		log.Printf("Retrieving node: %s from graph: %s", nodeID, graphID)

		nodeDetails, err := store.GetNode(graphID, decodedNodeID)
		if err != nil {
			respondError(w, http.StatusNotFound, err.Error())
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(nodeDetails)
	}
}

func respondError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(AnalyzeResponse{Status: "error", Message: message})
}
