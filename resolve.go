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

// ResolveArtifactRef resolves a short artifact name or OCI reference to a
// fully-qualified reference with its latest semver tag from the registry.
//
// If the ref already has a tag other than "latest" (or a digest), it is
// returned as-is. Short names (no "/") are expanded using registryBase and
// namePrefix (e.g. "go" with prefix "klaus-" becomes
// "gsoci.azurecr.io/giantswarm/klaus-go:v1.0.0").
//
// When no tag is provided or the tag is "latest", the registry is queried
// for all tags and the highest semver tag is selected.
func (c *Client) ResolveArtifactRef(ctx context.Context, ref, registryBase, namePrefix string) (string, error) {
	return resolveArtifactRef(ctx, c, ref, registryBase, namePrefix)
}

// ResolvePluginRefs resolves a slice of PluginReference entries, replacing
// "latest" or empty tags with the actual latest semver tag from the registry.
// Plugins with non-"latest" tags or digests are left unchanged.
func (c *Client) ResolvePluginRefs(ctx context.Context, plugins []PluginReference) ([]PluginReference, error) {
	return resolvePluginRefs(ctx, c, plugins)
}

func resolveArtifactRef(ctx context.Context, lister tagLister, ref, registryBase, namePrefix string) (string, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return ref, nil
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
	if namePrefix != "" && !strings.HasPrefix(name, namePrefix) {
		name = namePrefix + name
	}
	fullRepo := registryBase + "/" + name

	if tag != "" && tag != "latest" {
		return fullRepo + ":" + tag, nil
	}

	return resolveLatestSemver(ctx, lister, fullRepo)
}

func resolvePluginRefs(ctx context.Context, lister tagLister, plugins []PluginReference) ([]PluginReference, error) {
	resolved := make([]PluginReference, len(plugins))
	copy(resolved, plugins)

	for i := range resolved {
		if resolved[i].Digest != "" {
			continue
		}
		if resolved[i].Tag != "" && resolved[i].Tag != "latest" {
			continue
		}
		tag, err := resolveLatestTagForRepo(ctx, lister, resolved[i].Repository)
		if err != nil {
			return nil, fmt.Errorf("resolving plugin %s: %w", resolved[i].Repository, err)
		}
		resolved[i].Tag = tag
	}

	return resolved, nil
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
