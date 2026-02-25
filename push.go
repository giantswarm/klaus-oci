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
// The configJSON should be a marshaled Plugin or Personality (depending on kind).
// The ref must include a tag (e.g. "registry.example.com/repo:v1.0.0").
func (c *Client) push(ctx context.Context, sourceDir string, ref string, configJSON []byte, kind artifactKind) (*PushResult, error) {
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

	annotations, err := buildAnnotations(configJSON, tag)
	if err != nil {
		return nil, fmt.Errorf("building manifest annotations: %w", err)
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
// The Personality struct is marshaled to JSON as the config blob (Version
// is excluded via json:"-"). The version is conveyed through the OCI tag
// in the ref parameter.
func (c *Client) PushPersonality(ctx context.Context, sourceDir, ref string, p Personality) (*PushResult, error) {
	configJSON, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("marshaling personality config: %w", err)
	}
	return c.push(ctx, sourceDir, ref, configJSON, personalityArtifact)
}

// PushPlugin pushes a plugin artifact to an OCI registry.
// The Plugin struct is marshaled to JSON as the config blob (Version
// is excluded via json:"-"). The version is conveyed through the OCI tag
// in the ref parameter.
func (c *Client) PushPlugin(ctx context.Context, sourceDir, ref string, p Plugin) (*PushResult, error) {
	configJSON, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("marshaling plugin config: %w", err)
	}
	return c.push(ctx, sourceDir, ref, configJSON, pluginArtifact)
}

// buildAnnotations creates standard OCI manifest annotations from a config
// JSON blob and the OCI tag. Name and description are read from the config
// blob; version comes from the OCI tag (since Version is excluded from the
// config blob via json:"-").
func buildAnnotations(configJSON []byte, tag string) (map[string]string, error) {
	var fields struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(configJSON, &fields); err != nil {
		return nil, fmt.Errorf("parsing config for annotations: %w", err)
	}

	annotations := make(map[string]string)
	if fields.Name != "" {
		annotations[ocispec.AnnotationTitle] = fields.Name
	}
	if tag != "" {
		annotations[ocispec.AnnotationVersion] = tag
	}
	if fields.Description != "" {
		annotations[ocispec.AnnotationDescription] = fields.Description
	}
	if len(annotations) == 0 {
		return nil, nil
	}
	return annotations, nil
}
