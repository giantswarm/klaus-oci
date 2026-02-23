package oci

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"gopkg.in/yaml.v3"
)

// Pull downloads a Klaus artifact from an OCI registry and extracts it to destDir.
// The kind parameter determines which content media type to look for in the manifest.
// If the artifact is already cached with a matching digest, the pull is skipped
// and PullResult.Cached is set to true.
func (c *Client) Pull(ctx context.Context, ref string, destDir string, kind ArtifactKind) (*PullResult, error) {
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

// PullPersonality downloads a personality artifact, parses its contents, and
// returns a fully populated Personality. The cacheDir is used for digest-based
// caching of the extracted files; consumers never need to read from it directly.
func (c *Client) PullPersonality(ctx context.Context, ref string, cacheDir string) (*Personality, error) {
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

	digest := manifestDesc.Digest.String()

	if IsCached(cacheDir, digest) {
		return parsePersonalityFromDir(cacheDir, ref, digest, true)
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
		if manifest.Layers[i].MediaType == PersonalityArtifact.ContentMediaType {
			contentLayer = &manifest.Layers[i]
			break
		}
	}
	if contentLayer == nil {
		return nil, fmt.Errorf("no content layer found in %s (expected media type %s)", ref, PersonalityArtifact.ContentMediaType)
	}

	layerRC, err := repo.Fetch(ctx, *contentLayer)
	if err != nil {
		return nil, fmt.Errorf("fetching content layer for %s: %w", ref, err)
	}
	defer layerRC.Close()

	if err := cleanAndCreate(cacheDir); err != nil {
		return nil, err
	}

	if err := extractTarGz(layerRC, cacheDir); err != nil {
		return nil, fmt.Errorf("extracting content for %s: %w", ref, err)
	}

	if err := WriteCacheEntry(cacheDir, CacheEntry{Digest: digest, Ref: ref}); err != nil {
		return nil, fmt.Errorf("writing cache entry: %w", err)
	}

	p, err := parsePersonalityFromDir(cacheDir, ref, digest, false)
	if err != nil {
		return nil, err
	}

	// Populate Meta from the OCI config blob.
	if manifest.Config.MediaType == PersonalityArtifact.ConfigMediaType {
		configRC, err := repo.Fetch(ctx, manifest.Config)
		if err != nil {
			return nil, fmt.Errorf("fetching config blob for %s: %w", ref, err)
		}
		defer configRC.Close()

		if err := json.NewDecoder(configRC).Decode(&p.Meta); err != nil {
			return nil, fmt.Errorf("parsing config blob for %s: %w", ref, err)
		}
	}

	return p, nil
}

// parsePersonalityFromDir reads personality.yaml and SOUL.md from an
// extracted personality artifact directory and returns a Personality.
func parsePersonalityFromDir(dir, ref, digest string, cached bool) (*Personality, error) {
	specData, err := os.ReadFile(filepath.Join(dir, personalitySpecFileName))
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", personalitySpecFileName, err)
	}

	var spec PersonalitySpec
	if err := yaml.Unmarshal(specData, &spec); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", personalitySpecFileName, err)
	}

	soulData, err := os.ReadFile(filepath.Join(dir, soulFileName))
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("reading %s: %w", soulFileName, err)
	}

	return &Personality{
		Spec:   spec,
		Soul:   strings.TrimSpace(string(soulData)),
		Digest: digest,
		Ref:    ref,
		Cached: cached,
	}, nil
}
