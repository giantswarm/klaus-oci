package oci

// Klaus-specific OCI manifest annotation keys.
//
// These keys are used across all Klaus artifact types (plugins, personalities,
// toolchains) for uniform identification and metadata extraction from OCI
// manifest annotations. Manifest-level annotations are preferred over Docker
// LABEL directives for remote registry operations because they can be read
// from the manifest without pulling config blobs.
const (
	// AnnotationKlausType identifies the Klaus artifact type.
	// Expected values are TypePlugin, TypePersonality, or TypeToolchain.
	AnnotationKlausType = "io.giantswarm.klaus.type"

	// AnnotationKlausName is the human-readable artifact name.
	AnnotationKlausName = "io.giantswarm.klaus.name"

	// AnnotationKlausVersion is the artifact version.
	AnnotationKlausVersion = "io.giantswarm.klaus.version"
)

// Annotation values for AnnotationKlausType.
const (
	TypePlugin      = "plugin"
	TypePersonality = "personality"
	TypeToolchain   = "toolchain"
)

// ArtifactInfo holds common metadata extracted from OCI manifest annotations
// using the Klaus-specific annotation keys.
type ArtifactInfo struct {
	// Type is the Klaus artifact type (plugin, personality, toolchain).
	Type string `json:"type"`
	// Name is the human-readable artifact name.
	Name string `json:"name"`
	// Version is the artifact version string.
	Version string `json:"version"`
}

// ArtifactInfoFromAnnotations extracts Klaus artifact metadata from an OCI
// manifest annotation map. It reads the Klaus-specific annotation keys and
// returns the values. Missing keys result in empty strings in the returned
// struct.
func ArtifactInfoFromAnnotations(annotations map[string]string) ArtifactInfo {
	return ArtifactInfo{
		Type:    annotations[AnnotationKlausType],
		Name:    annotations[AnnotationKlausName],
		Version: annotations[AnnotationKlausVersion],
	}
}
