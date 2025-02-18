package glu

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/get-glu/glu/pkg/core"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

type Server struct {
	system *System
	router *chi.Mux
}

func newServer(system *System) *Server {
	s := &Server{
		system: system,
		router: chi.NewRouter(),
	}

	s.setupRoutes()
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

func (s *Server) setupRoutes() {
	s.router.Use(middleware.Logger)
	s.router.Use(middleware.Recoverer)
	s.router.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://*", "https://*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"*"},
		AllowCredentials: false,
		MaxAge:           300,
	}))
	s.router.Use(middleware.SetHeader("Content-Type", "application/json"))
	s.router.Use(middleware.StripSlashes)

	s.router.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// API routes
	s.router.Route("/api/v1", func(r chi.Router) {
		r.Get("/", s.getRoot)
		r.Get("/pipelines", s.listPipelines)
		r.Get("/pipelines/{pipeline}", s.getPipeline)
		r.Get("/pipelines/{pipeline}/phases/{phase}", s.getPhase)
		r.Post("/pipelines/{pipeline}/phases/{phase}/promote", s.promotePhase)
	})
}

func (s *Server) getRoot(w http.ResponseWriter, r *http.Request) {
	if err := json.NewEncoder(w).Encode(s.system.meta); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

type listPipelinesResponse struct {
	// TODO: does a system have metadata?
	//	Metadata  Metadata           `json:"metadata"`
	Pipelines []pipelineResponse `json:"pipelines"`
}

type pipelineResponse struct {
	Name   string            `json:"name"`
	Labels map[string]string `json:"labels,omitempty"`
	Phases []phaseResponse   `json:"phases,omitempty"`
}

type phaseResponse struct {
	Name       string            `json:"name"`
	DependsOn  string            `json:"depends_on,omitempty"`
	SourceType string            `json:"source_type,omitempty"`
	Labels     map[string]string `json:"labels,omitempty"`
	Value      interface{}       `json:"value,omitempty"`
}

func (s *Server) createPhaseResponse(phase core.Phase, dependencies map[core.Phase]core.Phase) phaseResponse {
	var (
		dependsOn string
		labels    map[string]string
	)

	if dependencies != nil {
		if d, ok := dependencies[phase]; ok && d != nil {
			dependsOn = d.Metadata().Name
		}
	}

	if phase.Metadata().Labels != nil {
		labels = phase.Metadata().Labels
	}

	return phaseResponse{
		Name:       phase.Metadata().Name,
		DependsOn:  dependsOn,
		Labels:     labels,
		SourceType: phase.SourceType(),
	}
}

func (s *Server) createPipelineResponse(ctx context.Context, pipeline core.Pipeline) (pipelineResponse, error) {
	dependencies := pipeline.Dependencies()
	phases := make([]phaseResponse, 0)

	for phase := range pipeline.Phases() {
		response := s.createPhaseResponse(phase, dependencies)

		v, err := phase.Get(ctx)
		if err != nil {
			return pipelineResponse{}, err
		}
		response.Value = v

		phases = append(phases, response)
	}

	var labels map[string]string
	if pipeline.Metadata().Labels != nil {
		labels = pipeline.Metadata().Labels
	}

	return pipelineResponse{
		Name:   pipeline.Metadata().Name,
		Labels: labels,
		Phases: phases,
	}, nil
}

// Handler methods
func (s *Server) listPipelines(w http.ResponseWriter, r *http.Request) {
	var (
		ctx               = r.Context()
		pipelineResponses = make([]pipelineResponse, 0, len(s.system.pipelines))
	)

	for _, pipeline := range s.system.pipelines {
		response, err := s.createPipelineResponse(ctx, pipeline)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		pipelineResponses = append(pipelineResponses, response)
	}

	// TODO: handle pagination
	if err := json.NewEncoder(w).Encode(listPipelinesResponse{
		Pipelines: pipelineResponses,
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (s *Server) getPipeline(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	pipeline, err := s.system.GetPipeline(chi.URLParam(r, "pipeline"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	response, err := s.createPipelineResponse(ctx, pipeline)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(response)
}

func (s *Server) getPhase(w http.ResponseWriter, r *http.Request) {
	pipeline, err := s.system.GetPipeline(chi.URLParam(r, "pipeline"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	phaseName := chi.URLParam(r, "phase")
	phase, err := pipeline.PhaseByName(phaseName)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, core.ErrNotFound) {
			status = http.StatusNotFound
		}

		http.Error(w, err.Error(), status)
		return
	}

	v, err := phase.Get(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := s.createPhaseResponse(phase, pipeline.Dependencies())
	response.Value = v

	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (s *Server) promotePhase(w http.ResponseWriter, r *http.Request) {
	pipeline, err := s.system.GetPipeline(chi.URLParam(r, "pipeline"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	phaseName := chi.URLParam(r, "phase")
	phase, err := pipeline.PhaseByName(phaseName)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, core.ErrNotFound) {
			status = http.StatusNotFound
		}

		http.Error(w, err.Error(), status)
		return
	}

	if err := phase.Promote(r.Context()); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
