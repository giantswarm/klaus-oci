package oci

import (
	"context"
	"encoding/json"
	"fmt"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// FetchManifestAnnotations fetches the OCI manifest for the given reference
// and returns its annotation map. This is lighter than Pull -- no content
// layers are fetched, only the manifest blob.
//
// For multi-architecture images (index/manifest list), a platform manifest
// matching the current runtime (GOOS/GOARCH) is selected automatically.
func (c *Client) FetchManifestAnnotations(ctx context.Context, ref string) (map[string]string, error) {
	manifest, _, err := c.fetchManifest(ctx, ref)
	if err != nil {
		return nil, err
	}

	if manifest.Annotations == nil {
		return map[string]string{}, nil
	}

	return manifest.Annotations, nil
}

// FetchArtifactInfo fetches the OCI manifest for the given reference and
// returns the Klaus artifact metadata extracted from its annotations.
// This is a convenience method combining FetchManifestAnnotations and
// ArtifactInfoFromAnnotations.
func (c *Client) FetchArtifactInfo(ctx context.Context, ref string) (ArtifactInfo, error) {
	annotations, err := c.FetchManifestAnnotations(ctx, ref)
	if err != nil {
		return ArtifactInfo{}, err
	}
	return ArtifactInfoFromAnnotations(annotations), nil
}

// fetchManifest resolves the reference, fetches the manifest, and returns
// the parsed OCI manifest along with the resolved descriptor (which carries
// the digest). If the top-level object is an OCI index (multi-arch), a
// platform manifest for the current runtime is selected first.
func (c *Client) fetchManifest(ctx context.Context, ref string) (*ocispec.Manifest, ocispec.Descriptor, error) {
	repo, tag, err := c.newRepository(ref)
	if err != nil {
		return nil, ocispec.Descriptor{}, err
	}

	if tag == "" {
		return nil, ocispec.Descriptor{}, fmt.Errorf("reference %q must include a tag or digest", ref)
	}

	desc, err := repo.Resolve(ctx, tag)
	if err != nil {
		return nil, ocispec.Descriptor{}, fmt.Errorf("resolving %s: %w", ref, err)
	}

	rc, err := repo.Fetch(ctx, desc)
	if err != nil {
		return nil, ocispec.Descriptor{}, fmt.Errorf("fetching manifest for %s: %w", ref, err)
	}
	defer rc.Close()

	switch desc.MediaType {
	case ocispec.MediaTypeImageIndex, "application/vnd.docker.distribution.manifest.list.v2+json":
		var index ocispec.Index
		if err := json.NewDecoder(rc).Decode(&index); err != nil {
			return nil, ocispec.Descriptor{}, fmt.Errorf("parsing index for %s: %w", ref, err)
		}

		platformDesc, err := selectPlatformForArch(index.Manifests, c.platformOS, c.platformArch)
		if err != nil {
			return nil, ocispec.Descriptor{}, fmt.Errorf("selecting platform manifest for %s: %w", ref, err)
		}

		platformRC, err := repo.Fetch(ctx, platformDesc)
		if err != nil {
			return nil, ocispec.Descriptor{}, fmt.Errorf("fetching platform manifest for %s: %w", ref, err)
		}
		defer platformRC.Close()

		var manifest ocispec.Manifest
		if err := json.NewDecoder(platformRC).Decode(&manifest); err != nil {
			return nil, ocispec.Descriptor{}, fmt.Errorf("parsing platform manifest for %s: %w", ref, err)
		}
		return &manifest, platformDesc, nil

	default:
		var manifest ocispec.Manifest
		if err := json.NewDecoder(rc).Decode(&manifest); err != nil {
			return nil, ocispec.Descriptor{}, fmt.Errorf("parsing manifest for %s: %w", ref, err)
		}
		return &manifest, desc, nil
	}
}

// selectPlatformForArch picks the manifest descriptor matching the requested
// OS/architecture from a list of index manifests.
func selectPlatformForArch(manifests []ocispec.Descriptor, wantOS, wantArch string) (ocispec.Descriptor, error) {
	for _, m := range manifests {
		if m.Platform != nil && m.Platform.OS == wantOS && m.Platform.Architecture == wantArch {
			return m, nil
		}
	}

	// Fall back to first manifest with a non-attestation media type.
	for _, m := range manifests {
		if m.MediaType == ocispec.MediaTypeImageManifest ||
			m.MediaType == "application/vnd.docker.distribution.manifest.v2+json" {
			return m, nil
		}
	}

	if len(manifests) > 0 {
		return manifests[0], nil
	}

	return ocispec.Descriptor{}, fmt.Errorf("no manifests in index")
}
