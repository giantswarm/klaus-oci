package oci

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
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
		entry, _ := ReadCacheEntry(destDir)
		var configJSON []byte
		if entry != nil {
			configJSON = entry.ConfigJSON
		}
		return &pullResult{Digest: digest, Ref: ref, Cached: true, ConfigJSON: configJSON}, nil
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

	if err := WriteCacheEntry(destDir, CacheEntry{Digest: digest, Ref: ref, ConfigJSON: configJSON}); err != nil {
		return nil, fmt.Errorf("writing cache entry: %w", err)
	}

	return &pullResult{Digest: digest, Ref: ref, ConfigJSON: configJSON}, nil
}

// PullPersonality downloads a personality artifact from an OCI registry and
// returns a PulledPersonality with metadata, composition, and soul content.
// The config blob is persisted in the cache entry so that metadata is always
// populated, even on cache hits.
func (c *Client) PullPersonality(ctx context.Context, ref string, cacheDir string) (*PulledPersonality, error) {
	result, err := c.pull(ctx, ref, cacheDir, personalityArtifact)
	if err != nil {
		return nil, err
	}
	return parsePersonalityFromDir(cacheDir, ref, result)
}

// PullPlugin downloads a plugin artifact from an OCI registry and returns
// a PulledPlugin with metadata and the extraction directory. The config blob
// is persisted in the cache entry so that metadata is always populated.
func (c *Client) PullPlugin(ctx context.Context, ref string, destDir string) (*PulledPlugin, error) {
	result, err := c.pull(ctx, ref, destDir, pluginArtifact)
	if err != nil {
		return nil, err
	}
	_, tag := SplitNameTag(ref)
	p := &PulledPlugin{
		ArtifactInfo: ArtifactInfo{Ref: ref, Tag: tag, Digest: result.Digest},
		Dir:          destDir,
		Cached:       result.Cached,
	}
	if result.ConfigJSON != nil {
		if err := json.Unmarshal(result.ConfigJSON, &p.Plugin); err != nil {
			return nil, fmt.Errorf("parsing plugin config: %w", err)
		}
	}
	p.Plugin.Version = tag
	return p, nil
}

func parsePersonalityFromDir(dir, ref string, result *pullResult) (*PulledPersonality, error) {
	_, tag := SplitNameTag(ref)
	p := &PulledPersonality{
		ArtifactInfo: ArtifactInfo{
			Ref:    ref,
			Tag:    tag,
			Digest: result.Digest,
		},
		Dir:    dir,
		Cached: result.Cached,
	}

	if result.ConfigJSON != nil {
		if err := json.Unmarshal(result.ConfigJSON, &p.Personality); err != nil {
			return nil, fmt.Errorf("parsing personality config: %w", err)
		}
	}
	p.Personality.Version = tag

	soulData, err := os.ReadFile(filepath.Join(dir, "SOUL.md"))
	if err == nil {
		p.Soul = string(soulData)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("reading SOUL.md: %w", err)
	}

	return p, nil
}
