package oci

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/registry/remote"
)

// DescribePlugin fetches the config blob for a plugin artifact and returns
// metadata without downloading the content layer. The ref parameter supports
// short names (e.g. "gs-base"), name:tag, or full OCI references.
func (c *Client) DescribePlugin(ctx context.Context, ref string) (*DescribedPlugin, error) {
	resolved, err := c.ResolvePluginRef(ctx, ref)
	if err != nil {
		return nil, fmt.Errorf("resolving plugin ref %q: %w", ref, err)
	}

	fm, err := c.fetchManifest(ctx, resolved)
	if err != nil {
		return nil, err
	}

	configJSON, err := fetchConfigBlob(ctx, fm.repo, resolved, fm.manifest.Config)
	if err != nil {
		return nil, err
	}

	var plugin Plugin
	if err := json.Unmarshal(configJSON, &plugin); err != nil {
		return nil, fmt.Errorf("parsing plugin config for %s: %w", resolved, err)
	}
	plugin.Version = fm.tag

	return &DescribedPlugin{
		ArtifactInfo: ArtifactInfo{Ref: resolved, Tag: fm.tag, Digest: fm.digest},
		Plugin:       plugin,
	}, nil
}

// DescribePersonality fetches the config blob for a personality artifact
// and returns metadata without downloading the content layer. The soul text
// is NOT available via describe -- use PullPersonality to get it.
func (c *Client) DescribePersonality(ctx context.Context, ref string) (*DescribedPersonality, error) {
	resolved, err := c.ResolvePersonalityRef(ctx, ref)
	if err != nil {
		return nil, fmt.Errorf("resolving personality ref %q: %w", ref, err)
	}

	fm, err := c.fetchManifest(ctx, resolved)
	if err != nil {
		return nil, err
	}

	configJSON, err := fetchConfigBlob(ctx, fm.repo, resolved, fm.manifest.Config)
	if err != nil {
		return nil, err
	}

	var personality Personality
	if err := json.Unmarshal(configJSON, &personality); err != nil {
		return nil, fmt.Errorf("parsing personality config for %s: %w", resolved, err)
	}
	personality.Version = fm.tag

	return &DescribedPersonality{
		ArtifactInfo: ArtifactInfo{Ref: resolved, Tag: fm.tag, Digest: fm.digest},
		Personality:  personality,
	}, nil
}

// DescribeToolchain fetches the manifest for a toolchain image and returns
// metadata derived from OCI manifest annotations. No config blob or layers
// are downloaded.
func (c *Client) DescribeToolchain(ctx context.Context, ref string) (*DescribedToolchain, error) {
	resolved, err := c.ResolveToolchainRef(ctx, ref)
	if err != nil {
		return nil, fmt.Errorf("resolving toolchain ref %q: %w", ref, err)
	}

	fm, err := c.fetchManifest(ctx, resolved)
	if err != nil {
		return nil, err
	}

	toolchain := toolchainFromAnnotations(fm.manifest.Annotations)
	toolchain.Version = fm.tag

	return &DescribedToolchain{
		ArtifactInfo: ArtifactInfo{Ref: resolved, Tag: fm.tag, Digest: fm.digest},
		Toolchain:    toolchain,
	}, nil
}

// fetchedManifest holds the intermediate result of fetching an OCI manifest.
type fetchedManifest struct {
	repo     *remote.Repository
	manifest ocispec.Manifest
	digest   string
	tag      string
}

// fetchManifest resolves a fully-qualified OCI reference, fetches its
// manifest, and returns the parsed manifest along with the repository
// client for subsequent blob fetches.
func (c *Client) fetchManifest(ctx context.Context, ref string) (*fetchedManifest, error) {
	repo, tag, err := c.newRepository(ref)
	if err != nil {
		return nil, err
	}
	if tag == "" {
		return nil, fmt.Errorf("reference %q must include a tag or digest", ref)
	}

	manifestDesc, err := repo.Resolve(ctx, tag)
	if err != nil {
		return nil, fmt.Errorf("resolving %s: %w", ref, err)
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

	return &fetchedManifest{
		repo:     repo,
		manifest: manifest,
		digest:   manifestDesc.Digest.String(),
		tag:      tag,
	}, nil
}

// fetchConfigBlob fetches a blob from the repository and returns its
// raw bytes. Used to retrieve the config blob after fetching the manifest.
func fetchConfigBlob(ctx context.Context, repo *remote.Repository, ref string, desc ocispec.Descriptor) ([]byte, error) {
	rc, err := repo.Fetch(ctx, desc)
	if err != nil {
		return nil, fmt.Errorf("fetching config for %s: %w", ref, err)
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		return nil, fmt.Errorf("reading config for %s: %w", ref, err)
	}
	return data, nil
}
