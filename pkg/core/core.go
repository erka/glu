package core

import (
	"context"
	"fmt"
	"log/slog"
	"reflect"
	"sync"

	"github.com/flipt-io/glu/pkg/fs"
)

var (
	defaultRegistry              = NewRegistry()
	DefaultRegisterer Registerer = defaultRegistry
)

type Registerer interface {
	Register(*Pipeline) error
}

type Registry struct {
	mu        sync.Mutex
	pipelines map[string]*Pipeline
}

func NewRegistry() *Registry {
	return &Registry{
		pipelines: make(map[string]*Pipeline),
	}
}

func (r *Registry) Register(p *Pipeline) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.pipelines[p.name]; ok {
		return fmt.Errorf("pipeline %q already registered", p.name)
	}

	r.pipelines[p.name] = p
	return nil
}

type Proposal struct {
	BaseRevision string
	BaseBranch   string
	Branch       string
	Title        string
	Body         string

	ExternalMetadata map[string]any
}

type Repository interface {
	View(context.Context, *Phase, func(fs.Filesystem) error) error
	Update(context.Context, *Phase, *Metadata, func(fs.Filesystem) (string, error)) error
}

type Pipeline struct {
	mu     sync.Mutex
	ctx    context.Context
	name   string
	phases map[string]*Phase
}

func NewPipeline(ctx context.Context, name string) *Pipeline {
	p := &Pipeline{
		ctx:    ctx,
		name:   name,
		phases: make(map[string]*Phase),
	}

	// TODO(mark): make this configurable
	DefaultRegisterer.Register(p)

	return p
}

func (p *Pipeline) Run(ctx context.Context) error {
	<-ctx.Done()
	return ctx.Err()
}

type Phase struct {
	name string
	// TODO(georgemac): make optionally configurable
	branch    string
	repo      Repository
	instances map[string]any
	mu        sync.Mutex
}

func (p *Phase) Name() string {
	return p.name
}

func (p *Phase) Branch() string {
	return p.branch
}

func (p *Phase) Repository() Repository {
	return p.repo
}

func (p *Pipeline) NewPhase(name string, repo Repository) *Phase {
	pp := &Phase{
		name:   name,
		branch: "main",
		repo:   repo,
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	p.phases[name] = pp
	return pp
}

type Metadata struct {
	Name   string
	Labels map[string]string
}

type Instance[A any, P interface {
	*A
	App
}] struct {
	phase *Phase
	meta  Metadata
	fn    func(Metadata) P
	src   Reconciler[P]
}

func (i *Instance[A, P]) Reconcile(ctx context.Context) (P, error) {
	slog.Debug("reconcile started", "type", "instance", "phase", i.phase.Name(), "name", i.meta.Name)

	repo := i.phase.Repository()

	a := i.fn(i.meta)
	if err := repo.View(ctx, i.phase, func(f fs.Filesystem) error {
		return a.ReadFrom(ctx, i.phase, f)
	}); err != nil {
		return nil, err
	}

	if i.src == nil {
		return a, nil
	}

	b, err := i.src.Reconcile(ctx)
	if err != nil {
		return nil, err
	}

	if reflect.DeepEqual(a, b) {
		slog.Debug("skipping reconcile", "reason", "UpToDate")
		return a, nil
	}

	if err := repo.Update(ctx, i.phase, &i.meta, func(f fs.Filesystem) (string, error) {
		return fmt.Sprintf("Update %s in %s", i.meta.Name, i.phase.Name()), b.WriteTo(ctx, i.phase, f)
	}); err != nil {
		return nil, err
	}

	return b, nil
}

type Reconciler[A any] interface {
	Reconcile(context.Context) (A, error)
}

type InstanceOption[A any, P interface {
	*A
	App
}] func(*Instance[A, P])

func DependsOn[A any, P interface {
	*A
	App
}](src Reconciler[P]) InstanceOption[A, P] {
	return func(i *Instance[A, P]) {
		i.src = src
	}
}

func NewInstance[A any, P interface {
	*A
	App
}](phase *Phase, meta Metadata, fn func(Metadata) P, opts ...InstanceOption[A, P]) *Instance[A, P] {
	inst := &Instance[A, P]{phase: phase, meta: meta, fn: fn}
	for _, opt := range opts {
		opt(inst)
	}

	phase.mu.Lock()
	defer phase.mu.Unlock()

	phase.instances[meta.Name] = inst
	return inst
}

type App interface {
	ReadFrom(context.Context, *Phase, fs.Filesystem) error
	WriteTo(_ context.Context, _ *Phase, _ fs.Filesystem) error
}
