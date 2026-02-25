package oci

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/Masterminds/semver/v3"
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
// resolves each to its latest semver version. The defaultBase is used
// unless overridden by WithRegistry in opts.
//
// Repositories are resolved concurrently, bounded by the client's concurrency
// limit (default 10, configurable via WithConcurrency). Results are sorted
// alphabetically by repository name for deterministic output.
//
// Repositories that have no semver tags are silently skipped.
func (c *Client) listArtifacts(ctx context.Context, defaultBase string, opts ...ListOption) ([]listedArtifact, error) {
	cfg := &listConfig{}
	for _, o := range opts {
		o(cfg)
	}

	base := defaultBase
	if cfg.registryBase != "" {
		base = cfg.registryBase
	}

	repos, err := c.listRepositories(ctx, base)
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
// personality registry (or a custom one via WithRegistry) and returns
// ListEntry results with name and version extracted from the repository
// path and tag.
func (c *Client) ListPersonalities(ctx context.Context, opts ...ListOption) ([]ListEntry, error) {
	return c.listEntries(ctx, DefaultPersonalityRegistry, opts...)
}

// ListPlugins discovers all plugin artifacts under the default plugin
// registry (or a custom one via WithRegistry) and returns ListEntry results.
func (c *Client) ListPlugins(ctx context.Context, opts ...ListOption) ([]ListEntry, error) {
	return c.listEntries(ctx, DefaultPluginRegistry, opts...)
}

// ListToolchains discovers all toolchain images under the default toolchain
// registry (or a custom one via WithRegistry) and returns ListEntry results.
func (c *Client) ListToolchains(ctx context.Context, opts ...ListOption) ([]ListEntry, error) {
	return c.listEntries(ctx, DefaultToolchainRegistry, opts...)
}

func (c *Client) listEntries(ctx context.Context, defaultBase string, opts ...ListOption) ([]ListEntry, error) {
	artifacts, err := c.listArtifacts(ctx, defaultBase, opts...)
	if err != nil {
		return nil, err
	}

	result := make([]ListEntry, len(artifacts))
	for i, a := range artifacts {
		name, version := extractNameVersion(a)
		result[i] = ListEntry{
			Name:       name,
			Version:    version,
			Repository: a.Repository,
			Reference:  a.Reference,
		}
	}
	return result, nil
}

func extractNameVersion(a listedArtifact) (name, version string) {
	name = ShortName(a.Repository)
	_, version = SplitNameTag(a.Reference)
	return name, version
}

// ListPluginVersions returns all semver tags for a plugin, sorted descending.
// nameOrRef can be a short name (e.g. "gs-base") or a full OCI repository path.
func (c *Client) ListPluginVersions(ctx context.Context, nameOrRef string) ([]string, error) {
	return c.listVersions(ctx, nameOrRef, DefaultPluginRegistry)
}

// ListPersonalityVersions returns all semver tags for a personality, sorted descending.
// nameOrRef can be a short name (e.g. "sre") or a full OCI repository path.
func (c *Client) ListPersonalityVersions(ctx context.Context, nameOrRef string) ([]string, error) {
	return c.listVersions(ctx, nameOrRef, DefaultPersonalityRegistry)
}

// ListToolchainVersions returns all semver tags for a toolchain, sorted descending.
// nameOrRef can be a short name (e.g. "go") or a full OCI repository path.
func (c *Client) ListToolchainVersions(ctx context.Context, nameOrRef string) ([]string, error) {
	return c.listVersions(ctx, nameOrRef, DefaultToolchainRegistry)
}

// listVersions lists all semver tags for a single artifact, sorted descending.
// Short names (no "/") are expanded using the given registry base.
func (c *Client) listVersions(ctx context.Context, nameOrRef, registryBase string) ([]string, error) {
	nameOrRef = strings.TrimSpace(nameOrRef)
	if nameOrRef == "" {
		return nil, fmt.Errorf("empty artifact reference")
	}

	repo := nameOrRef
	if !strings.Contains(nameOrRef, "/") {
		repo = registryBase + "/" + nameOrRef
	}

	tags, err := c.List(ctx, repo)
	if err != nil {
		return nil, fmt.Errorf("listing versions for %s: %w", repo, err)
	}

	return sortedSemverTags(tags), nil
}

// sortedSemverTags filters tags to valid semver and sorts them descending.
func sortedSemverTags(tags []string) []string {
	type parsed struct {
		tag string
		ver *semver.Version
	}

	var versions []parsed
	for _, tag := range tags {
		v, err := semver.NewVersion(tag)
		if err != nil {
			continue
		}
		versions = append(versions, parsed{tag: tag, ver: v})
	}

	slices.SortFunc(versions, func(a, b parsed) int {
		return b.ver.Compare(a.ver)
	})

	result := make([]string, len(versions))
	for i, v := range versions {
		result[i] = v.tag
	}
	return result
}
