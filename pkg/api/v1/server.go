package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// TODO:
// - GET /api/v1/pipelines/
// - GET /api/v1/pipelines/{pipeline}/phases/
// - GET /api/v1/pipelines/{pipeline}/phases/{phase}/instances/
// - GET /api/v1/pipelines/{pipeline}/phases/{phase}/instances/{resource}
// - POST /api/v1/pipelines/{pipeline}/phases/{phase}/instances/{resource}/reconcile
// - GET /api/v1/pipelines/{pipeline}/resources/ - give state of resource in pipeline across phases
// think of another name for app (ie: resource, etc)

// instance is a an resource in a phase

// this will hang off of the pipeline registry/workflow type
// Server represents the API server
type Server struct {
	router *chi.Mux
}

// NewServer creates a new instance of the API server
func NewServer() *Server {
	s := &Server{
		router: chi.NewRouter(),
	}
	s.setupRoutes()
	return s
}

func (s *Server) setupRoutes() {
	// Add some useful middleware
	s.router.Use(middleware.Logger)
	s.router.Use(middleware.Recoverer)

	// API routes
	s.router.Route("/api/v1", func(r chi.Router) {
		r.Get("/pipelines/{pipeline}/phases/{phase}/instances/{resource}", s.getPipelineInstance)
	})
}

// ServeHTTP implements the http.Handler interface
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

// getPipelineInstance handles the GET request for pipeline instances
func (s *Server) getPipelineInstance(w http.ResponseWriter, r *http.Request) {
	var (
		pipeline = chi.URLParam(r, "pipeline")
		phase    = chi.URLParam(r, "phase")
		resource = chi.URLParam(r, "resource")
	)

	response := map[string]string{
		"pipeline": pipeline,
		"phase":    phase,
		"resource": resource,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
