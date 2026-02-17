// Package oci provides shared types and an ORAS client for Klaus OCI artifacts.
//
// Klaus uses OCI artifacts with custom media types for distributing plugins
// and personalities. This package defines the media types, metadata structs,
// and a registry client that both klausctl and the klaus-operator can use.
package oci

// Media types for Klaus plugin artifacts.
const (
	// MediaTypePluginConfig is the OCI media type for the plugin config blob.
	MediaTypePluginConfig = "application/vnd.giantswarm.klaus-plugin.config.v1+json"

	// MediaTypePluginContent is the OCI media type for the plugin content layer.
	MediaTypePluginContent = "application/vnd.giantswarm.klaus-plugin.content.v1.tar+gzip"
)

// Media types for Klaus personality artifacts.
const (
	// MediaTypePersonalityConfig is the OCI media type for the personality config blob.
	MediaTypePersonalityConfig = "application/vnd.giantswarm.klaus-personality.config.v1+json"

	// MediaTypePersonalityContent is the OCI media type for the personality content layer.
	MediaTypePersonalityContent = "application/vnd.giantswarm.klaus-personality.content.v1.tar+gzip"
)

// ArtifactKind bundles the media types for a specific Klaus artifact type.
// Use the predefined kinds (PluginArtifact, PersonalityArtifact) or define
// custom kinds for other artifact types.
type ArtifactKind struct {
	// ConfigMediaType is the media type for the OCI config blob.
	ConfigMediaType string
	// ContentMediaType is the media type for the OCI content layer.
	ContentMediaType string
}

// Predefined artifact kinds for Klaus artifacts.
var (
	// PluginArtifact is the artifact kind for Klaus plugins.
	PluginArtifact = ArtifactKind{
		ConfigMediaType:  MediaTypePluginConfig,
		ContentMediaType: MediaTypePluginContent,
	}

	// PersonalityArtifact is the artifact kind for Klaus personalities.
	PersonalityArtifact = ArtifactKind{
		ConfigMediaType:  MediaTypePersonalityConfig,
		ContentMediaType: MediaTypePersonalityContent,
	}
)
