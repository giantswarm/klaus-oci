package oci

import "context"

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
// Repositories are resolved sequentially. For large registries this may be
// slow; use the lower-level methods with concurrent fetching if performance
// is critical.
//
// Repositories that have no semver tags or whose manifests cannot be fetched
// are silently skipped. Use the lower-level methods directly if you need
// per-repository error handling.
func (c *Client) ListArtifacts(ctx context.Context, registryBase string) ([]ListedArtifact, error) {
	repos, err := c.ListRepositories(ctx, registryBase)
	if err != nil {
		return nil, err
	}

	var artifacts []ListedArtifact
	for _, repo := range repos {
		ref, err := c.ResolveLatestVersion(ctx, repo)
		if err != nil {
			continue
		}

		info, err := c.FetchArtifactInfo(ctx, ref)
		if err != nil {
			continue
		}

		artifacts = append(artifacts, ListedArtifact{
			Repository:   repo,
			Reference:    ref,
			ArtifactInfo: info,
		})
	}

	return artifacts, nil
}
