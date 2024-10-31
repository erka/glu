package oci

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/flipt-io/glu"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/credentials"
	"oras.land/oras-go/v2/registry/remote/retry"
)

const (
	LabelImageURL = "dev.getglue.sources.oci/v1/image-url"
)

type OCIDerivable interface {
	ReadFromOCIDescriptor(v1.Descriptor) error
}

type Source[A any, P interface {
	*A
	OCIDerivable
}] struct {
	remote *remote.Repository
	meta   glu.Metadata
	fn     func(glu.Metadata) P
}

func New[A any, P interface {
	*A
	OCIDerivable
}](meta glu.Metadata, fn func(glu.Metadata) P) (*Source[A, P], error) {
	repo, ok := meta.Labels[LabelImageURL]
	if !ok {
		return nil, fmt.Errorf("missing label %q on app %q metadata", LabelImageURL, meta.Name)
	}

	r, err := getRepository(repo)
	if err != nil {
		return nil, err
	}

	return &Source[A, P]{
		remote: r,
		meta:   meta,
		fn:     fn,
	}, nil
}

func (s *Source[A, P]) Reconcile(ctx context.Context) (P, error) {
	slog.Debug("Reconcile", "type", "oci", "name", s.meta.Name)

	desc, err := s.remote.Resolve(ctx, s.remote.Reference.Reference)
	if err != nil {
		return nil, err
	}

	p := s.fn(s.meta)
	if err := p.ReadFromOCIDescriptor(desc); err != nil {
		return nil, err
	}

	return p, nil
}

func getRepository(repo string) (*remote.Repository, error) {
	remote, err := remote.NewRepository(repo)
	if err != nil {
		return nil, err
	}

	creds, err := credentials.NewStoreFromDocker(credentials.StoreOptions{})
	if err != nil {
		return nil, err
	}

	remote.Client = &auth.Client{
		Client:     retry.DefaultClient,
		Cache:      auth.NewCache(),
		Credential: credentials.Credential(creds),
	}

	return remote, nil
}
