package oci

import (
	"context"
	"slices"
	"strings"
	"sync"

	"golang.org/x/sync/errgroup"
)

// ListedArtifact holds metadata for an artifact discovered by ListArtifacts.
type ListedArtifact struct {
	// Repository is the full OCI repository path (e.g. "gsoci.azurecr.io/giantswarm/klaus-toolchains/go").
	Repository string
	// Reference is the resolved OCI reference including the latest semver tag
	// (e.g. "gsoci.azurecr.io/giantswarm/klaus-toolchains/go:v1.0.0").
	Reference string
}

// ListOption configures the behaviour of ListArtifacts.
type ListOption func(*listConfig)

type listConfig struct {
	filter func(repository string) bool
}

// WithFilter sets a predicate that is applied to each discovered repository
// before any network-intensive resolution. Only repositories for which fn
// returns true will be resolved.
func WithFilter(fn func(repository string) bool) ListOption {
	return func(cfg *listConfig) { cfg.filter = fn }
}

// ListArtifacts discovers all artifacts under a registry base path and
// resolves each to its latest semver version. This combines
// ListRepositories and ResolveLatestVersion into a single high-level
// operation.
//
// Pass WithFilter() to skip repositories that don't match a predicate
// before resolution.
//
// Repositories are resolved concurrently, bounded by the client's concurrency
// limit (default 10, configurable via WithConcurrency). Results are sorted
// alphabetically by repository name for deterministic output.
//
// Repositories that have no semver tags are silently skipped. Use the
// lower-level methods directly if you need per-repository error handling.
func (c *Client) ListArtifacts(ctx context.Context, registryBase string, opts ...ListOption) ([]ListedArtifact, error) {
	cfg := &listConfig{}
	for _, o := range opts {
		o(cfg)
	}

	repos, err := c.ListRepositories(ctx, registryBase)
	if err != nil {
		return nil, err
	}

	if cfg.filter != nil {
		filtered := repos[:0]
		for _, r := range repos {
			if cfg.filter(r) {
				filtered = append(filtered, r)
			}
		}
		repos = filtered
	}

	var (
		mu        sync.Mutex
		artifacts []ListedArtifact
	)

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(c.concurrency)

	for _, repo := range repos {
		g.Go(func() error {
			ref, err := c.ResolveLatestVersion(ctx, repo)
			if err != nil {
				return nil
			}

			mu.Lock()
			artifacts = append(artifacts, ListedArtifact{
				Repository: repo,
				Reference:  ref,
			})
			mu.Unlock()
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	slices.SortFunc(artifacts, func(a, b ListedArtifact) int {
		return strings.Compare(a.Repository, b.Repository)
	})

	return artifacts, nil
}
