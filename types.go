package oci

// Well-known filenames inside a personality artifact.
const (
	personalitySpecFileName = "personality.yaml"
	soulFileName            = "SOUL.md"
)

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
// inside a personality artifact. It defines the three pillars of an agent
// personality: a soul document (SOUL.md, by convention), a toolchain
// container image, and a set of plugin artifacts.
type PersonalitySpec struct {
	// Description is a human-readable description of the personality.
	Description string `yaml:"description,omitempty"`
	// Toolchain is the optional container image reference for the execution
	// environment (e.g. "gsoci.azurecr.io/giantswarm/klaus-toolchains/go:v1.0.0").
	Toolchain string `yaml:"toolchain,omitempty"`
	// Plugins lists the plugin artifacts that make up this personality.
	Plugins []PluginReference `yaml:"plugins,omitempty"`
}

// Personality is the fully resolved result of pulling a personality artifact.
// It combines the OCI metadata, the parsed personality.yaml spec, and the
// soul document content into a single value that consumers can use directly
// without navigating the extracted directory.
type Personality struct {
	// Meta holds the OCI config metadata (name, version, description).
	Meta PersonalityMeta
	// Spec holds the parsed personality.yaml (toolchain, plugins).
	Spec PersonalitySpec
	// Soul is the content of SOUL.md -- the agent's identity document.
	Soul string

	// Digest is the resolved OCI manifest digest.
	Digest string
	// Ref is the original OCI reference that was pulled.
	Ref string
	// Cached is true when the pull was skipped because the local cache was fresh.
	Cached bool
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

// PullResult holds the result of a successful pull operation.
type PullResult struct {
	// Digest is the resolved manifest digest.
	Digest string
	// Ref is the original reference string.
	Ref string
	// Cached is true if the pull was skipped because the local cache was fresh.
	Cached bool
}

// PushResult holds the result of a successful push operation.
type PushResult struct {
	// Digest is the manifest digest of the pushed artifact.
	Digest string
}
