package oci

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"gopkg.in/yaml.v3"
)

// pull downloads a Klaus artifact from an OCI registry and extracts it to destDir.
// The kind parameter determines which content media type to look for in the manifest.
// If the artifact is already cached with a matching digest, the pull is skipped
// and pullResult.Cached is set to true.
func (c *Client) pull(ctx context.Context, ref string, destDir string, kind artifactKind) (*pullResult, error) {
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
		return &pullResult{Digest: digest, Ref: ref, Cached: true}, nil
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

	configRC, err := repo.Fetch(ctx, manifest.Config)
	if err != nil {
		return nil, fmt.Errorf("fetching config for %s: %w", ref, err)
	}
	defer configRC.Close()
	configJSON, err := io.ReadAll(configRC)
	if err != nil {
		return nil, fmt.Errorf("reading config for %s: %w", ref, err)
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

	return &pullResult{Digest: digest, Ref: ref, ConfigJSON: configJSON}, nil
}

// PullPersonality downloads a personality artifact from an OCI registry and
// returns a fully parsed Personality with metadata, spec, and soul content.
// On a cache hit (Cached == true) the Meta field will be zero-valued because
// the OCI config blob is not re-fetched; Spec and Soul are always parsed
// from the extracted files on disk.
func (c *Client) PullPersonality(ctx context.Context, ref string, cacheDir string) (*Personality, error) {
	result, err := c.pull(ctx, ref, cacheDir, personalityArtifact)
	if err != nil {
		return nil, err
	}
	return parsePersonalityFromDir(cacheDir, ref, result)
}

// PullPlugin downloads a plugin artifact from an OCI registry and returns
// a Plugin with metadata and the extraction directory.
// On a cache hit (Cached == true) the Meta field will be zero-valued because
// the OCI config blob is not re-fetched.
func (c *Client) PullPlugin(ctx context.Context, ref string, destDir string) (*Plugin, error) {
	result, err := c.pull(ctx, ref, destDir, pluginArtifact)
	if err != nil {
		return nil, err
	}
	p := &Plugin{Dir: destDir, Digest: result.Digest, Ref: ref, Cached: result.Cached}
	if result.ConfigJSON != nil {
		if err := json.Unmarshal(result.ConfigJSON, &p.Meta); err != nil {
			return nil, fmt.Errorf("parsing plugin config: %w", err)
		}
	}
	return p, nil
}

func parsePersonalityFromDir(dir, ref string, result *pullResult) (*Personality, error) {
	p := &Personality{
		Dir:    dir,
		Digest: result.Digest,
		Ref:    ref,
		Cached: result.Cached,
	}

	if result.ConfigJSON != nil {
		if err := json.Unmarshal(result.ConfigJSON, &p.Meta); err != nil {
			return nil, fmt.Errorf("parsing personality config: %w", err)
		}
	}

	specData, err := os.ReadFile(filepath.Join(dir, "personality.yaml"))
	if err == nil {
		if err := yaml.Unmarshal(specData, &p.Spec); err != nil {
			return nil, fmt.Errorf("parsing personality.yaml: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("reading personality.yaml: %w", err)
	}

	soulData, err := os.ReadFile(filepath.Join(dir, "soul.md"))
	if err == nil {
		p.Soul = string(soulData)
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("reading soul.md: %w", err)
	}

	return p, nil
}
