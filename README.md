# klaus-oci

Shared Go types and ORAS client for [Klaus](https://github.com/giantswarm/klausctl) OCI artifacts.

Klaus uses OCI artifacts with custom media types for distributing **plugins**, **personalities**, and **toolchains**. This module provides:

- Domain types: `Plugin`, `Personality`, `Toolchain` with OCI config blob serialization
- Describe operations: fetch metadata without downloading content layers
- Typed pull/push API for personalities and plugins
- List operations with version enumeration
- Reference resolution (short names to fully-qualified OCI references)
- Dependency resolution for personalities (toolchain + plugin references)
- Helpers for reading artifact metadata from source directories
- Digest-based caching to avoid redundant pulls
- Secure tar.gz archive extraction with path traversal protection

Both [klausctl](https://github.com/giantswarm/klausctl) and the [klaus-operator](https://github.com/giantswarm/klaus-operator) import this module to ensure artifact format consistency.

## Installation

```bash
go get github.com/giantswarm/klaus-oci
```

## Usage

### Listing available artifacts

```go
import oci "github.com/giantswarm/klaus-oci"

client := oci.NewClient(oci.WithRegistryAuthEnv("KLAUSCTL_REGISTRY_AUTH"))

// List available personalities (returns []ListEntry)
personalities, err := client.ListPersonalities(ctx)
for _, p := range personalities {
    fmt.Printf("%s (%s)\n", p.Name, p.Version) // "sre (v1.0.0)"
}

// List plugins from a custom registry
plugins, err := client.ListPlugins(ctx, oci.WithRegistry("custom.io/team/klaus-plugins"))

// List toolchains
toolchains, err := client.ListToolchains(ctx)
```

### Listing versions for a specific artifact

```go
// List all semver tags for a plugin, sorted descending
versions, err := client.ListPluginVersions(ctx, "gs-base")
// versions: ["v1.2.0", "v1.1.0", "v1.0.0"]

// Also works for personalities and toolchains
versions, err = client.ListPersonalityVersions(ctx, "sre")
versions, err = client.ListToolchainVersions(ctx, "go")
```

### Describing artifacts (metadata only, no download)

```go
// Describe a plugin -- fetches config blob, no content layer download
desc, err := client.DescribePlugin(ctx, "gs-base")
fmt.Println(desc.Plugin.Name)        // "gs-base"
fmt.Println(desc.Plugin.Version)     // "v1.0.0" (from OCI tag)
fmt.Println(desc.Plugin.Description) // "A general purpose plugin..."
fmt.Println(desc.Plugin.Skills)      // ["fluxcd", "kubernetes", ...]
fmt.Println(desc.Plugin.Commands)    // ["hello", "init-circleci", ...]
fmt.Println(desc.Digest)             // "sha256:abc..."

// Describe a personality -- metadata + composition, no soul text
desc, err := client.DescribePersonality(ctx, "sre:v1.0.0")
fmt.Println(desc.Personality.Name)      // "sre"
fmt.Println(desc.Personality.Toolchain) // {Repository: "gsoci.../go", Tag: "latest"}
fmt.Println(desc.Personality.Plugins)   // [{Repository: "gsoci.../gs-base", Tag: "latest"}, ...]

// Describe a toolchain -- metadata from OCI manifest annotations
desc, err := client.DescribeToolchain(ctx, "go")
fmt.Println(desc.Toolchain.Name)        // "go"
fmt.Println(desc.Toolchain.Version)     // "v1.2.0" (from OCI tag)
fmt.Println(desc.Toolchain.Description) // "Go toolchain for Klaus"
```

### Pulling artifacts

```go
// Pull a personality -- downloads config blob + content layer, extracts files
pulled, err := client.PullPersonality(ctx, "sre:v1.0.0", cacheDir)
fmt.Println(pulled.Personality.Name)      // "sre"
fmt.Println(pulled.Personality.Toolchain) // toolchain reference
fmt.Println(pulled.Soul)                  // SOUL.md contents (only available after pull)
fmt.Println(pulled.Dir)                   // local extraction directory
fmt.Println(pulled.Cached)               // true if skipped (cache hit)

// Pull a plugin
pulled, err := client.PullPlugin(ctx, "gs-base:v1.0.0", destDir)
fmt.Println(pulled.Plugin.Name)    // "gs-base"
fmt.Println(pulled.Plugin.Version) // "v1.0.0"
fmt.Println(pulled.Dir)            // local extraction directory
```

### Pushing artifacts

```go
// Read plugin metadata from source directory
plugin, err := oci.ReadPluginFromDir("./my-plugin")
// plugin.Name, plugin.Description, etc. from .claude-plugin/plugin.json
// plugin.Skills, plugin.Commands, etc. discovered by scanning the directory

// Push plugin -- version is conveyed via the OCI tag, not stored in config blob
result, err := client.PushPlugin(ctx, "./my-plugin",
    "gsoci.azurecr.io/giantswarm/klaus-plugins/my-plugin:v1.0.0", *plugin)
fmt.Println(result.Digest) // "sha256:abc..."

// Read personality metadata from source directory
personality, err := oci.ReadPersonalityFromDir("./my-personality")
// personality.Name, personality.Toolchain, personality.Plugins from personality.yaml

// Push personality -- version via OCI tag, SOUL.md included automatically
result, err := client.PushPersonality(ctx, "./my-personality",
    "gsoci.azurecr.io/giantswarm/klaus-personalities/my-personality:v1.0.0", *personality)
```

### Resolving references

```go
// Resolve short names to fully-qualified references with latest semver tag
ref, err := client.ResolvePersonalityRef(ctx, "sre")       // -> "gsoci.../sre:v1.0.0"
ref, err = client.ResolvePluginRef(ctx, "gs-base")          // -> "gsoci.../gs-base:v1.2.0"
ref, err = client.ResolveToolchainRef(ctx, "go")            // -> "gsoci.../go:v1.1.0"

// Refs with explicit tags are returned as-is (unless "latest")
ref, err = client.ResolvePluginRef(ctx, "gs-base:v0.5.0")   // -> "gsoci.../gs-base:v0.5.0"
```

### Resolving personality dependencies

```go
// Describe a personality, then resolve its toolchain and plugin references
desc, err := client.DescribePersonality(ctx, "sre:v1.0.0")

deps, err := client.ResolvePersonalityDeps(ctx, desc.Personality)
fmt.Println(deps.Toolchain.Name)       // "go"
fmt.Println(deps.Toolchain.Version)    // "v1.2.0"
for _, p := range deps.Plugins {
    fmt.Printf("  plugin: %s (%s)\n", p.Name, p.Version)
}
for _, w := range deps.Warnings {
    fmt.Println("  warning:", w) // e.g. "plugin gs-sre: not found in registry"
}
```

## Artifact Types

Klaus has three artifact types with different OCI representations:

| Artifact    | OCI Format          | Metadata Source              | Config Media Type                                             | Content Media Type                                                 |
|-------------|---------------------|------------------------------|---------------------------------------------------------------|--------------------------------------------------------------------|
| Plugin      | Custom OCI artifact | Config blob (JSON)           | `application/vnd.giantswarm.klaus-plugin.config.v1+json`      | `application/vnd.giantswarm.klaus-plugin.content.v1.tar+gzip`      |
| Personality | Custom OCI artifact | Config blob (JSON)           | `application/vnd.giantswarm.klaus-personality.config.v1+json`  | `application/vnd.giantswarm.klaus-personality.content.v1.tar+gzip`  |
| Toolchain   | Standard Docker image | Manifest annotations       | (standard Docker config)                                      | (standard Docker layers)                                           |

### Version handling

The version is **never** stored in the OCI config blob. For all three artifact types, the version is conveyed exclusively via the OCI tag. The `Version` field on domain types (`Plugin`, `Personality`, `Toolchain`) is populated from the resolved OCI tag during describe/pull operations.

## License

Apache 2.0 - see [LICENSE](LICENSE).
