package oci

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	godigest "github.com/opencontainers/go-digest"
	specs "github.com/opencontainers/image-spec/specs-go"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// push packages a directory and pushes it to an OCI registry as a Klaus artifact.
// The configJSON is the marshaled type-specific config blob (pluginConfigBlob or
// personalityConfigBlob). The annotations map carries common metadata and is set
// directly on the manifest.
func (c *Client) push(ctx context.Context, sourceDir string, ref string, configJSON []byte, annotations map[string]string, kind artifactKind) (*PushResult, error) {
	repo, tag, err := c.newRepository(ref)
	if err != nil {
		return nil, err
	}

	if tag == "" {
		return nil, fmt.Errorf("reference %q must include a tag", ref)
	}

	configDesc := ocispec.Descriptor{
		MediaType: kind.ConfigMediaType,
		Digest:    godigest.FromBytes(configJSON),
		Size:      int64(len(configJSON)),
	}

	if err := repo.Push(ctx, configDesc, bytes.NewReader(configJSON)); err != nil {
		return nil, fmt.Errorf("pushing config blob: %w", err)
	}

	layerData, err := createTarGz(sourceDir)
	if err != nil {
		return nil, fmt.Errorf("creating archive: %w", err)
	}
	layerDesc := ocispec.Descriptor{
		MediaType: kind.ContentMediaType,
		Digest:    godigest.FromBytes(layerData),
		Size:      int64(len(layerData)),
	}

	if err := repo.Push(ctx, layerDesc, bytes.NewReader(layerData)); err != nil {
		return nil, fmt.Errorf("pushing content layer: %w", err)
	}

	manifest := ocispec.Manifest{
		Versioned:   specs.Versioned{SchemaVersion: 2},
		MediaType:   ocispec.MediaTypeImageManifest,
		Config:      configDesc,
		Layers:      []ocispec.Descriptor{layerDesc},
		Annotations: annotations,
	}

	manifestJSON, err := json.Marshal(manifest)
	if err != nil {
		return nil, fmt.Errorf("marshaling manifest: %w", err)
	}
	manifestDesc := ocispec.Descriptor{
		MediaType: ocispec.MediaTypeImageManifest,
		Digest:    godigest.FromBytes(manifestJSON),
		Size:      int64(len(manifestJSON)),
	}

	if err := repo.Push(ctx, manifestDesc, bytes.NewReader(manifestJSON)); err != nil {
		return nil, fmt.Errorf("pushing manifest: %w", err)
	}

	if err := repo.Tag(ctx, manifestDesc, tag); err != nil {
		return nil, fmt.Errorf("tagging manifest as %s: %w", tag, err)
	}

	return &PushResult{Digest: manifestDesc.Digest.String()}, nil
}

// PushPersonality pushes a personality artifact to an OCI registry.
// Common metadata (name, description, author, etc.) is stored as Klaus
// annotations on the manifest. The config blob contains only composition
// data (toolchain + plugins). Version is conveyed through the OCI tag.
func (c *Client) PushPersonality(ctx context.Context, sourceDir, ref string, p Personality) (*PushResult, error) {
	blob := personalityConfigBlob{
		Toolchain: p.Toolchain,
		Plugins:   p.Plugins,
	}
	configJSON, err := json.Marshal(blob)
	if err != nil {
		return nil, fmt.Errorf("marshaling personality config: %w", err)
	}
	annotations := buildKlausAnnotations(p.Name, p.Description, p.Author, p.Homepage, p.SourceRepo, p.License, p.Keywords, "")
	return c.push(ctx, sourceDir, ref, configJSON, annotations, personalityArtifact)
}

// PushPlugin pushes a plugin artifact to an OCI registry.
// Common metadata (name, description, author, etc.) is stored as Klaus
// annotations on the manifest. The config blob contains only discovered
// components (skills, commands, etc.). Version is conveyed through the OCI tag.
func (c *Client) PushPlugin(ctx context.Context, sourceDir, ref string, p Plugin) (*PushResult, error) {
	blob := pluginConfigBlob{
		Skills:     p.Skills,
		Commands:   p.Commands,
		Agents:     p.Agents,
		HasHooks:   p.HasHooks,
		MCPServers: p.MCPServers,
		LSPServers: p.LSPServers,
	}
	configJSON, err := json.Marshal(blob)
	if err != nil {
		return nil, fmt.Errorf("marshaling plugin config: %w", err)
	}
	annotations := buildKlausAnnotations(p.Name, p.Description, p.Author, p.Homepage, p.SourceRepo, p.License, p.Keywords, "")
	return c.push(ctx, sourceDir, ref, configJSON, annotations, pluginArtifact)
}
