package oci

import (
	"context"
	"slices"
	"strings"
	"sync"

	"golang.org/x/sync/errgroup"
)

// listedArtifact holds metadata for an artifact discovered by listArtifacts.
type listedArtifact struct {
	// Repository is the full OCI repository path.
	Repository string
	// Reference is the resolved OCI reference including the latest semver tag.
	Reference string
}

// ListOption configures the behaviour of listing methods.
type ListOption func(*listConfig)

type listConfig struct {
	filter       func(repository string) bool
	registryBase string
}

// WithFilter sets a predicate that is applied to each discovered repository
// before any network-intensive resolution. Only repositories for which fn
// returns true will be resolved.
func WithFilter(fn func(repository string) bool) ListOption {
	return func(cfg *listConfig) { cfg.filter = fn }
}

// WithRegistry overrides the default registry base path for a listing
// operation. This supports multi-source registry configurations where the
// base path comes from user configuration rather than the default constants.
func WithRegistry(base string) ListOption {
	return func(cfg *listConfig) { cfg.registryBase = base }
}

// listArtifacts discovers all artifacts under a registry base path and
// resolves each to its latest semver version.
//
// Repositories are resolved concurrently, bounded by the client's concurrency
// limit (default 10, configurable via WithConcurrency). Results are sorted
// alphabetically by repository name for deterministic output.
//
// Repositories that have no semver tags are silently skipped.
func (c *Client) listArtifacts(ctx context.Context, registryBase string, opts ...ListOption) ([]listedArtifact, error) {
	cfg := &listConfig{}
	for _, o := range opts {
		o(cfg)
	}

	repos, err := c.listRepositories(ctx, registryBase)
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
		artifacts []listedArtifact
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
			artifacts = append(artifacts, listedArtifact{
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

	slices.SortFunc(artifacts, func(a, b listedArtifact) int {
		return strings.Compare(a.Repository, b.Repository)
	})

	return artifacts, nil
}

// ListPersonalities discovers all personality artifacts under the default
// personality registry (or a custom one via WithRegistry) and returns typed
// results with name and version extracted from the repository path and tag.
func (c *Client) ListPersonalities(ctx context.Context, opts ...ListOption) ([]ListedPersonality, error) {
	base, opts := extractRegistryBase(DefaultPersonalityRegistry, opts)

	artifacts, err := c.listArtifacts(ctx, base, opts...)
	if err != nil {
		return nil, err
	}

	result := make([]ListedPersonality, len(artifacts))
	for i, a := range artifacts {
		name, version := extractNameVersion(a)
		result[i] = ListedPersonality{
			Name:       name,
			Version:    version,
			Repository: a.Repository,
			Reference:  a.Reference,
		}
	}
	return result, nil
}

// ListPlugins discovers all plugin artifacts under the default plugin
// registry (or a custom one via WithRegistry) and returns typed results.
func (c *Client) ListPlugins(ctx context.Context, opts ...ListOption) ([]ListedPlugin, error) {
	base, opts := extractRegistryBase(DefaultPluginRegistry, opts)

	artifacts, err := c.listArtifacts(ctx, base, opts...)
	if err != nil {
		return nil, err
	}

	result := make([]ListedPlugin, len(artifacts))
	for i, a := range artifacts {
		name, version := extractNameVersion(a)
		result[i] = ListedPlugin{
			Name:       name,
			Version:    version,
			Repository: a.Repository,
			Reference:  a.Reference,
		}
	}
	return result, nil
}

// ListToolchains discovers all toolchain images under the default toolchain
// registry (or a custom one via WithRegistry) and returns typed results.
func (c *Client) ListToolchains(ctx context.Context, opts ...ListOption) ([]ListedToolchain, error) {
	base, opts := extractRegistryBase(DefaultToolchainRegistry, opts)

	artifacts, err := c.listArtifacts(ctx, base, opts...)
	if err != nil {
		return nil, err
	}

	result := make([]ListedToolchain, len(artifacts))
	for i, a := range artifacts {
		name, version := extractNameVersion(a)
		result[i] = ListedToolchain{
			Name:       name,
			Version:    version,
			Repository: a.Repository,
			Reference:  a.Reference,
		}
	}
	return result, nil
}

// extractRegistryBase applies options to find a WithRegistry override and
// returns the effective base plus the remaining options (without the registry
// override so it isn't applied twice by listArtifacts).
func extractRegistryBase(defaultBase string, opts []ListOption) (string, []ListOption) {
	cfg := &listConfig{}
	for _, o := range opts {
		o(cfg)
	}
	if cfg.registryBase != "" {
		return cfg.registryBase, opts
	}
	return defaultBase, opts
}

func extractNameVersion(a listedArtifact) (name, version string) {
	name = ShortName(a.Repository)
	_, version = SplitNameTag(a.Reference)
	return name, version
}
