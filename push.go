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

// Push packages a directory and pushes it to an OCI registry as a Klaus artifact.
// The configJSON should be a marshaled PluginMeta or PersonalityMeta (depending on kind).
// The ref must include a tag (e.g. "registry.example.com/repo:v1.0.0").
func (c *Client) Push(ctx context.Context, sourceDir string, ref string, configJSON []byte, kind ArtifactKind) (*PushResult, error) {
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

	annotations := annotationsFromConfig(configJSON)

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

// annotationsFromConfig extracts standard OCI annotations from a config JSON blob.
// It looks for "name", "version", and "description" fields.
func annotationsFromConfig(configJSON []byte) map[string]string {
	var fields struct {
		Name        string `json:"name"`
		Version     string `json:"version"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(configJSON, &fields); err != nil {
		return nil
	}

	annotations := make(map[string]string)
	if fields.Name != "" {
		annotations[ocispec.AnnotationTitle] = fields.Name
	}
	if fields.Version != "" {
		annotations[ocispec.AnnotationVersion] = fields.Version
	}
	if fields.Description != "" {
		annotations[ocispec.AnnotationDescription] = fields.Description
	}
	if len(annotations) == 0 {
		return nil
	}
	return annotations
}
