package oci

// PluginMeta holds metadata stored in the OCI config blob of a plugin artifact.
type PluginMeta struct {
	Name        string   `json:"name"`
	Version     string   `json:"version"`
	Description string   `json:"description,omitempty"`
	Skills      []string `json:"skills,omitempty"`
	Commands    []string `json:"commands,omitempty"`
}

// PersonalityMeta holds metadata stored in the OCI config blob of a personality artifact.
type PersonalityMeta struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description,omitempty"`
}

// PersonalitySpec is the deserialized content of a personality.yaml file
// inside a personality artifact. It defines which plugins to compose and
// optionally specifies a toolchain image.
type PersonalitySpec struct {
	// Description is a human-readable description of the personality.
	Description string `yaml:"description,omitempty"`
	// Image is the optional toolchain container image reference.
	Image string `yaml:"image,omitempty"`
	// Plugins lists the plugin artifacts that make up this personality.
	Plugins []PluginReference `yaml:"plugins,omitempty"`
}

// PluginReference is a reference to a plugin OCI artifact.
// Either Tag or Digest must be set.
type PluginReference struct {
	// Repository is the OCI repository path (e.g., "gsoci.azurecr.io/giantswarm/klaus-plugins/gs-platform").
	Repository string `yaml:"repository" json:"repository"`
	// Tag is the OCI tag (e.g., "v1.0.0"). Mutually preferred with Digest.
	Tag string `yaml:"tag,omitempty" json:"tag,omitempty"`
	// Digest is the OCI manifest digest (e.g., "sha256:abc123..."). Takes precedence over Tag.
	Digest string `yaml:"digest,omitempty" json:"digest,omitempty"`
}

// Ref returns the full OCI reference string for this plugin.
// If Digest is set, it is used (repo@digest). Otherwise Tag is used (repo:tag).
// If neither is set, the bare repository is returned.
func (p PluginReference) Ref() string {
	if p.Digest != "" {
		return p.Repository + "@" + p.Digest
	}
	if p.Tag != "" {
		return p.Repository + ":" + p.Tag
	}
	return p.Repository
}

// ToolchainMeta holds metadata stored in the OCI config blob of a toolchain artifact.
// Toolchain images are standard Docker/OCI container images used as the
// execution environment for Klaus personalities.
//
// The struct is intentionally separate from PersonalityMeta to allow the two
// artifact types to diverge independently (e.g. architecture, base-image fields).
type ToolchainMeta struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description,omitempty"`
}

// Personality holds the result of pulling a personality artifact.
// It contains the parsed metadata, spec, and soul content.
type Personality struct {
	// Meta is the personality metadata from the OCI config blob.
	Meta PersonalityMeta
	// Spec is the deserialized personality.yaml from the artifact.
	Spec PersonalitySpec
	// Soul is the raw content of soul.md, if present.
	Soul string
	// Dir is the path where personality files were extracted.
	Dir string
	// Digest is the resolved manifest digest.
	Digest string
	// Ref is the original OCI reference string.
	Ref string
	// Cached is true if the pull was skipped because the local cache was fresh.
	Cached bool
}

// Plugin holds the result of pulling a plugin artifact.
// Plugins contain skill files and command definitions that are mounted into
// the agent container, so the consumer gets the directory path rather than
// individually parsed files.
type Plugin struct {
	// Meta is the plugin metadata from the OCI config blob.
	Meta PluginMeta
	// Dir is the path where plugin files were extracted.
	Dir string
	// Digest is the resolved manifest digest.
	Digest string
	// Ref is the original OCI reference string.
	Ref string
	// Cached is true if the pull was skipped because the local cache was fresh.
	Cached bool
}

// ListedPersonality holds metadata for a personality discovered by ListPersonalities.
type ListedPersonality struct {
	// Name is the short name derived from the repository path (e.g. "sre").
	Name string
	// Version is the semver tag (e.g. "v1.0.0").
	Version string
	// Repository is the full OCI repository path.
	Repository string
	// Reference is the fully-qualified OCI reference including tag.
	Reference string
}

// ListedPlugin holds metadata for a plugin discovered by ListPlugins.
type ListedPlugin struct {
	// Name is the short name derived from the repository path (e.g. "gs-platform").
	Name string
	// Version is the semver tag (e.g. "v1.0.0").
	Version string
	// Repository is the full OCI repository path.
	Repository string
	// Reference is the fully-qualified OCI reference including tag.
	Reference string
}

// ListedToolchain holds metadata for a toolchain discovered by ListToolchains.
type ListedToolchain struct {
	// Name is the short name derived from the repository path (e.g. "go").
	Name string
	// Version is the semver tag (e.g. "v1.0.0").
	Version string
	// Repository is the full OCI repository path.
	Repository string
	// Reference is the fully-qualified OCI reference including tag.
	Reference string
}

// pullResult holds the result of a successful internal pull operation.
type pullResult struct {
	// Digest is the resolved manifest digest.
	Digest string
	// Ref is the original reference string.
	Ref string
	// Cached is true if the pull was skipped because the local cache was fresh.
	Cached bool
	// ConfigJSON is the raw OCI config blob (nil on cache hit).
	ConfigJSON []byte
}

// PushResult holds the result of a successful push operation.
type PushResult struct {
	// Digest is the manifest digest of the pushed artifact.
	Digest string
}
