package glu

import (
	"context"
	"fmt"

	"github.com/flipt-io/glu/pkg/config"
	"github.com/flipt-io/glu/pkg/core"
	"github.com/flipt-io/glu/pkg/credentials"
	"github.com/flipt-io/glu/pkg/repository"
)

type Metadata = core.Metadata

type Phase = core.Phase

type Pipeline struct {
	*core.Pipeline

	ctx   context.Context
	conf  *config.Config
	creds *credentials.CredentialSource
}

func NewPipeline(ctx context.Context, name string) (*Pipeline, error) {
	conf, err := config.ReadFromPath("glu.yaml")
	if err != nil {
		return nil, err
	}

	return &Pipeline{
		Pipeline: core.NewPipeline(ctx, name),
		ctx:      ctx,
		conf:     conf,
		creds:    credentials.New(conf.Credentials),
	}, nil
}

func (p *Pipeline) NewRepository(name string) (core.Repository, error) {
	conf, ok := p.conf.Repositories[name]
	if !ok {
		return nil, fmt.Errorf("repository %q: configuration not found", name)
	}

	return repository.NewGitRepository(p.ctx, conf, p.creds, name)
}

func NewInstance[A any, P interface {
	*A
	core.App
}](phase *Phase, meta Metadata, fn func(Metadata) P, opts ...core.InstanceOption[A, P]) *core.Instance[A, P] {
	return core.NewInstance(phase, meta, fn, opts...)
}

func DependsOn[A any, P interface {
	*A
	core.App
}](src core.Reconciler[P]) core.InstanceOption[A, P] {
	return core.DependsOn(src)
}
