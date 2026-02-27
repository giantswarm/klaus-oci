package oci

// Author represents the author of an artifact.
type Author struct {
	Name  string `json:"name,omitempty" yaml:"name,omitempty"`
	Email string `json:"email,omitempty" yaml:"email,omitempty"`
	URL   string `json:"url,omitempty" yaml:"url,omitempty"`
}

// Plugin represents a Klaus plugin.
// Most fields are serialized as JSON in the OCI config blob.
//
// The first group of fields comes directly from .claude-plugin/plugin.json
// and aligns with the Claude Code plugin manifest schema:
// https://code.claude.com/docs/en/plugins-reference#plugin-manifest-schema
//
// The second group (Skills, Commands, Agents, HasHooks, MCPServers,
// LSPServers) is computed at push time by scanning the plugin directory.
// These are NOT plugin.json fields -- in the upstream spec, "commands",
// "skills", "agents" etc. are path overrides (string|array) that tell
// Claude Code where to find components. Here we store the *discovered*
// component names so that Describe can report what the plugin provides
// without downloading the content layer.
type Plugin struct {
	// --- Manifest metadata (from plugin.json) ---

	Name string `json:"name"`
	// Version is NOT stored in the config blob. It is conveyed via the
	// OCI tag when pushing, and populated from the resolved OCI tag when
	// fetching (describe/pull).
	Version     string   `json:"-"`
	Description string   `json:"description,omitempty"`
	Author      *Author  `json:"author,omitempty"`
	Homepage    string   `json:"homepage,omitempty"`
	SourceRepo  string   `json:"repository,omitempty"`
	License     string   `json:"license,omitempty"`
	Keywords    []string `json:"keywords,omitempty"`

	// --- Discovered components (computed at push time) ---

	// Skills lists skill directory names found under skills/ (e.g. "kubernetes", "fluxcd").
	Skills []string `json:"skills,omitempty"`
	// Commands lists command file names found under commands/ (e.g. "hello", "init-kubernetes").
	Commands []string `json:"commands,omitempty"`
	// Agents lists agent file names found under agents/ (e.g. "code-reviewer", "security-reviewer").
	Agents []string `json:"agents,omitempty"`
	// HasHooks is true if the plugin contains hooks/ or a hooks configuration.
	HasHooks bool `json:"hasHooks,omitempty"`
	// MCPServers lists MCP server names (keys from .mcp.json).
	MCPServers []string `json:"mcpServers,omitempty"`
	// LSPServers lists LSP server names (keys from .lsp.json).
	LSPServers []string `json:"lspServers,omitempty"`
}

// Personality represents a Klaus personality.
// Most fields are serialized as JSON in the OCI config blob.
//
// Personalities are Giant Swarm's composition layer: they combine a
// toolchain (container image), a set of plugins, and a behavioral
// identity (soul) into a coherent agent configuration.
//
// Unlike plugins (where the manifest format is defined by upstream
// Claude Code), the personality definition format is our own. The
// on-disk source is personality.yaml (YAML) + SOUL.md (Markdown).
// At push time these are read and serialized as JSON into the OCI
// config blob (excluding Version, which is conveyed via the OCI tag).
//
// Fields are grouped by origin:
//   - Metadata: from personality.yaml
//   - Composition: from personality.yaml (toolchain + plugins)
//   - Version: from OCI tags (not in personality.yaml, not in config blob)
type Personality struct {
	// --- Metadata (from personality.yaml) ---

	Name        string   `yaml:"name" json:"name"`
	Description string   `yaml:"description,omitempty" json:"description,omitempty"`
	Author      *Author  `yaml:"author,omitempty" json:"author,omitempty"`
	Homepage    string   `yaml:"homepage,omitempty" json:"homepage,omitempty"`
	SourceRepo  string   `yaml:"repository,omitempty" json:"repository,omitempty"`
	License     string   `yaml:"license,omitempty" json:"license,omitempty"`
	Keywords    []string `yaml:"keywords,omitempty" json:"keywords,omitempty"`

	// --- Composition (from personality.yaml) ---

	// Toolchain is the container image that provides the runtime environment.
	Toolchain ToolchainReference `yaml:"toolchain,omitempty" json:"toolchain,omitempty"`
	// Plugins lists the plugin artifacts that compose this personality's capabilities.
	Plugins []PluginReference `yaml:"plugins,omitempty" json:"plugins,omitempty"`

	// --- External fields (not in personality.yaml, not in config blob) ---

	// Version is NOT stored in the config blob or personality.yaml. It is
	// conveyed via the OCI tag when pushing, and populated from the resolved
	// OCI tag when fetching (describe/pull).
	Version string `yaml:"-" json:"-"`
}

// Toolchain represents a Klaus toolchain (container image).
// Derived from OCI manifest annotations since toolchains are
// standard Docker images, not custom OCI artifacts.
//
// Fields mirror the metadata fields of Plugin and Personality for
// consistency. Each field maps to a Klaus annotation (io.giantswarm.klaus.*),
// parsed via metadataFromAnnotations.
//
// Version is populated from the OCI tag, same as Plugin and Personality.
type Toolchain struct {
	// --- Metadata (from OCI manifest annotations) ---

	Name string `json:"name"`
	// Version is populated from the OCI tag (not from annotations or
	// any config blob). Same convention as Plugin and Personality.
	Version     string   `json:"-"`
	Description string   `json:"description,omitempty"`
	Author      *Author  `json:"author,omitempty"`
	Homepage    string   `json:"homepage,omitempty"`
	SourceRepo  string   `json:"repository,omitempty"`
	License     string   `json:"license,omitempty"`
	Keywords    []string `json:"keywords,omitempty"`
}

// PluginReference points to a plugin OCI artifact.
type PluginReference struct {
	Repository string `yaml:"repository" json:"repository"`
	Tag        string `yaml:"tag,omitempty" json:"tag,omitempty"`
	Digest     string `yaml:"digest,omitempty" json:"digest,omitempty"`
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

// ToolchainReference points to a toolchain container image.
// Same shape as PluginReference but a distinct type for clarity.
type ToolchainReference struct {
	Repository string `yaml:"repository" json:"repository"`
	Tag        string `yaml:"tag,omitempty" json:"tag,omitempty"`
	Digest     string `yaml:"digest,omitempty" json:"digest,omitempty"`
}

// Ref returns the full OCI reference string for this toolchain.
// If Digest is set, it is used (repo@digest). Otherwise Tag is used (repo:tag).
// If neither is set, the bare repository is returned.
func (t ToolchainReference) Ref() string {
	if t.Digest != "" {
		return t.Repository + "@" + t.Digest
	}
	if t.Tag != "" {
		return t.Repository + ":" + t.Tag
	}
	return t.Repository
}

// ArtifactInfo holds OCI-level metadata returned by all operations
// that contact the registry (describe, pull).
type ArtifactInfo struct {
	Ref    string // Fully-qualified OCI reference (includes tag)
	Tag    string // Resolved OCI tag (e.g. "v1.0.0") -- source of truth for Version
	Digest string // Manifest digest
}

// ListEntry holds metadata for an artifact discovered by list operations.
// Populated from the registry catalog + tag resolution (no config fetch).
type ListEntry struct {
	Name       string // Short name (e.g. "sre", "gs-base")
	Version    string // Latest semver tag (e.g. "v1.0.0")
	Repository string // Full OCI repository path
	Reference  string // Full OCI reference with tag
}

// DescribedPlugin is a Plugin with its OCI metadata.
// Returned by DescribePlugin (config blob fetch only, no layer download).
type DescribedPlugin struct {
	ArtifactInfo
	Plugin
}

// DescribedPersonality is a Personality with its OCI metadata.
type DescribedPersonality struct {
	ArtifactInfo
	Personality
}

// DescribedToolchain is a Toolchain with its OCI metadata.
type DescribedToolchain struct {
	ArtifactInfo
	Toolchain
}

// PulledPlugin is a Plugin with OCI metadata and local file state.
type PulledPlugin struct {
	ArtifactInfo
	Plugin
	Dir    string // Local directory where files were extracted
	Cached bool   // True if pull was skipped (cache hit)
}

// PulledPersonality is a Personality with OCI metadata, local file state,
// and the soul text (which is only available after pulling the content layer).
type PulledPersonality struct {
	ArtifactInfo
	Personality
	Soul   string // Behavioral identity text from SOUL.md (content layer only)
	Dir    string
	Cached bool
}

// ResolvedDependencies holds the result of resolving a personality's
// toolchain and plugin references.
type ResolvedDependencies struct {
	Toolchain *DescribedToolchain
	Plugins   []DescribedPlugin
	Warnings  []string // e.g. "plugin gs-sre: not found in registry"
}

// PushResult holds the outcome of a push operation.
type PushResult struct {
	Digest string
}

// pullResult holds the result of a successful internal pull operation.
type pullResult struct {
	Digest     string
	Ref        string
	Cached     bool
	ConfigJSON []byte // Raw OCI config blob (read from cache entry on cache hit).
}
