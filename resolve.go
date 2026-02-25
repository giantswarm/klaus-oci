package oci

import (
	"context"
	"fmt"
	"strings"
)

// tagLister can list tags for an OCI repository. Declared as an interface to
// allow unit testing without network access. *Client satisfies this interface.
type tagLister interface {
	List(ctx context.Context, repository string) ([]string, error)
}

// ResolveLatestVersion lists tags for a repository and returns the full
// reference with the highest semver tag (e.g. "repo:v1.2.3").
func (c *Client) ResolveLatestVersion(ctx context.Context, repository string) (string, error) {
	return resolveLatestSemver(ctx, c, repository)
}

// ResolveToolchainRef resolves a toolchain short name or OCI reference to a
// fully-qualified reference with its latest semver tag.
// Short names (e.g. "go") are expanded using the default toolchain registry
// (e.g. "gsoci.azurecr.io/giantswarm/klaus-toolchains/go:v1.0.0").
func (c *Client) ResolveToolchainRef(ctx context.Context, ref string) (string, error) {
	return resolveArtifactRef(ctx, c, ref, DefaultToolchainRegistry)
}

// ResolvePluginRef resolves a plugin short name or OCI reference to a
// fully-qualified reference with its latest semver tag.
// Short names (e.g. "gs-ae") are expanded using the default plugin registry
// (e.g. "gsoci.azurecr.io/giantswarm/klaus-plugins/gs-ae:v0.0.3").
func (c *Client) ResolvePluginRef(ctx context.Context, ref string) (string, error) {
	return resolveArtifactRef(ctx, c, ref, DefaultPluginRegistry)
}

// ResolvePersonalityRef resolves a personality short name or OCI reference to a
// fully-qualified reference with its latest semver tag.
// Short names (e.g. "sre") are expanded using the default personality registry
// (e.g. "gsoci.azurecr.io/giantswarm/klaus-personalities/sre:v0.2.0").
func (c *Client) ResolvePersonalityRef(ctx context.Context, ref string) (string, error) {
	return resolveArtifactRef(ctx, c, ref, DefaultPersonalityRegistry)
}

func resolveArtifactRef(ctx context.Context, lister tagLister, ref, registryBase string) (string, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", fmt.Errorf("empty artifact reference")
	}

	if strings.Contains(ref, "/") {
		if !hasTagOrDigest(ref) {
			return resolveLatestSemver(ctx, lister, ref)
		}
		if hasDigest(ref) {
			return ref, nil
		}
		tag := extractTag(ref)
		if tag != "latest" {
			return ref, nil
		}
		repo := RepositoryFromRef(ref)
		return resolveLatestSemver(ctx, lister, repo)
	}

	name, tag := SplitNameTag(ref)
	fullRepo := registryBase + "/" + name

	if tag != "" && tag != "latest" {
		return fullRepo + ":" + tag, nil
	}

	return resolveLatestSemver(ctx, lister, fullRepo)
}

func resolveLatestSemver(ctx context.Context, lister tagLister, repo string) (string, error) {
	tag, err := resolveLatestTagForRepo(ctx, lister, repo)
	if err != nil {
		return "", err
	}
	return repo + ":" + tag, nil
}

func resolveLatestTagForRepo(ctx context.Context, lister tagLister, repo string) (string, error) {
	tags, err := lister.List(ctx, repo)
	if err != nil {
		return "", fmt.Errorf("listing tags for %s: %w", repo, err)
	}

	latest := LatestSemverTag(tags)
	if latest == "" {
		return "", fmt.Errorf("no semver tags found for %s", repo)
	}

	return latest, nil
}
