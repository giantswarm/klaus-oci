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

// ListArtifacts discovers all artifacts under a registry base path, resolves
// each to its latest semver version, and fetches manifest annotations for
// enriched metadata. This combines ListRepositories, ResolveLatestVersion,
// and FetchArtifactInfo into a single high-level operation.
//
// Repositories are resolved concurrently, bounded by the client's concurrency
// limit (default 10, configurable via WithConcurrency). Results are sorted
// alphabetically by repository name for deterministic output.
//
// Repositories that have no semver tags or whose manifests cannot be fetched
// are silently skipped. Use the lower-level methods directly if you need
// per-repository error handling.
func (c *Client) ListArtifacts(ctx context.Context, registryBase string) ([]ListedArtifact, error) {
	repos, err := c.ListRepositories(ctx, registryBase)
	if err != nil {
		return nil, err
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

			info, err := c.FetchArtifactInfo(ctx, ref)
			if err != nil {
				return nil
			}

			mu.Lock()
			artifacts = append(artifacts, ListedArtifact{
				Repository:   repo,
				Reference:    ref,
				ArtifactInfo: info,
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
