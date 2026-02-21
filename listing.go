package oci

import (
	"context"
	"slices"
	"strings"
	"sync"

	"golang.org/x/sync/errgroup"
)

// ListedArtifact holds metadata for an artifact discovered by ListArtifacts,
// combining the resolved OCI reference with annotation-based metadata.
type ListedArtifact struct {
	// Repository is the full OCI repository path (e.g. "gsoci.azurecr.io/giantswarm/klaus-go").
	Repository string
	// Reference is the resolved OCI reference including the latest semver tag
	// (e.g. "gsoci.azurecr.io/giantswarm/klaus-go:v1.0.0").
	Reference string
	ArtifactInfo
}

// ListOption configures the behaviour of ListArtifacts.
type ListOption func(*listConfig)

type listConfig struct {
	filter           func(repository string) bool
	fetchAnnotations bool
}

// WithFilter sets a predicate that is applied to each discovered repository
// before any network-intensive resolution. Only repositories for which fn
// returns true will be resolved. This is significantly faster than filtering
// after ListArtifacts returns when the registry base contains many unrelated
// repositories (e.g. using the broad "gsoci.azurecr.io/giantswarm" base for
// toolchains).
func WithFilter(fn func(repository string) bool) ListOption {
	return func(cfg *listConfig) { cfg.filter = fn }
}

// WithAnnotations enables manifest annotation fetching for each discovered
// artifact. When enabled, the ArtifactInfo fields (Name, Version,
// Description, Type) are populated from OCI manifest annotations. This
// requires additional HTTP round trips per artifact and is disabled by
// default.
func WithAnnotations() ListOption {
	return func(cfg *listConfig) { cfg.fetchAnnotations = true }
}

// ListArtifacts discovers all artifacts under a registry base path, resolves
// each to its latest semver version, and optionally fetches manifest
// annotations for enriched metadata. This combines ListRepositories,
// ResolveLatestVersion, and (optionally) FetchArtifactInfo into a single
// high-level operation.
//
// By default only repository discovery and version resolution are performed.
// Pass WithAnnotations() to also fetch manifest metadata. Pass WithFilter()
// to skip repositories that don't match a predicate before resolution.
//
// Repositories are resolved concurrently, bounded by the client's concurrency
// limit (default 10, configurable via WithConcurrency). Results are sorted
// alphabetically by repository name for deterministic output.
//
// Repositories that have no semver tags or whose manifests cannot be fetched
// are silently skipped. Use the lower-level methods directly if you need
// per-repository error handling.
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

			a := ListedArtifact{
				Repository: repo,
				Reference:  ref,
			}

			if cfg.fetchAnnotations {
				info, err := c.FetchArtifactInfo(ctx, ref)
				if err != nil {
					return nil
				}
				a.ArtifactInfo = info
			}

			mu.Lock()
			artifacts = append(artifacts, a)
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
