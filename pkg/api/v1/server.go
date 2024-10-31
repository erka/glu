package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

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
		r.Get("/pipelines/{pipeline}/phases/{phase}/instances/{app}", s.getPipelineInstance)
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
		app      = chi.URLParam(r, "app")
	)

	response := map[string]string{
		"pipeline": pipeline,
		"phase":    phase,
		"app":      app,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
