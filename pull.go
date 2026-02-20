package oci

import (
	"context"
	"encoding/json"
	"fmt"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/registry/remote/auth"
)

// PullOption configures a single Pull operation.
type PullOption func(*pullOptions)

type pullOptions struct {
	credentialFunc auth.CredentialFunc
}

// WithPullCredentials sets a credential resolution function for a single
// Pull request, overriding the client's default credentials. This is useful
// when different pull operations need different credentials, for example when
// a Kubernetes operator resolves per-instance imagePullSecrets.
func WithPullCredentials(f auth.CredentialFunc) PullOption {
	return func(o *pullOptions) {
		o.credentialFunc = f
	}
}

// Pull downloads a Klaus artifact from an OCI registry and extracts it to destDir.
// The kind parameter determines which content media type to look for in the manifest.
// If the artifact is already cached with a matching digest, the pull is skipped
// and PullResult.Cached is set to true.
//
// Optional PullOption values can override client-level settings for this
// request (e.g. WithPullCredentials for per-request authentication).
func (c *Client) Pull(ctx context.Context, ref string, destDir string, kind ArtifactKind, opts ...PullOption) (*PullResult, error) {
	var po pullOptions
	for _, o := range opts {
		o(&po)
	}

	repo, tag, err := c.newRepository(ref)
	if err != nil {
		return nil, err
	}

	if po.credentialFunc != nil {
		repo.Client = newAuthClientFromFunc(po.credentialFunc)
	}

	if tag == "" {
		return nil, fmt.Errorf("reference %q must include a tag or digest", ref)
	}

	manifestDesc, err := repo.Resolve(ctx, tag)
	if err != nil {
		return nil, fmt.Errorf("resolving %s: %w", ref, err)
	}

	digest := manifestDesc.Digest.String()

	if IsCached(destDir, digest) {
		return &PullResult{Digest: digest, Ref: ref, Cached: true}, nil
	}

	manifestRC, err := repo.Fetch(ctx, manifestDesc)
	if err != nil {
		return nil, fmt.Errorf("fetching manifest for %s: %w", ref, err)
	}
	defer manifestRC.Close()

	var manifest ocispec.Manifest
	if err := json.NewDecoder(manifestRC).Decode(&manifest); err != nil {
		return nil, fmt.Errorf("parsing manifest for %s: %w", ref, err)
	}

	var contentLayer *ocispec.Descriptor
	for i := range manifest.Layers {
		if manifest.Layers[i].MediaType == kind.ContentMediaType {
			contentLayer = &manifest.Layers[i]
			break
		}
	}
	if contentLayer == nil {
		return nil, fmt.Errorf("no content layer found in %s (expected media type %s)", ref, kind.ContentMediaType)
	}

	layerRC, err := repo.Fetch(ctx, *contentLayer)
	if err != nil {
		return nil, fmt.Errorf("fetching content layer for %s: %w", ref, err)
	}
	defer layerRC.Close()

	if err := cleanAndCreate(destDir); err != nil {
		return nil, err
	}

	if err := extractTarGz(layerRC, destDir); err != nil {
		return nil, fmt.Errorf("extracting content for %s: %w", ref, err)
	}

	if err := WriteCacheEntry(destDir, CacheEntry{Digest: digest, Ref: ref}); err != nil {
		return nil, fmt.Errorf("writing cache entry: %w", err)
	}

	return &PullResult{Digest: digest, Ref: ref}, nil
}
