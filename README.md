# klaus-oci

Shared Go types and ORAS client for [Klaus](https://github.com/giantswarm/klausctl) OCI artifacts.

Klaus uses OCI artifacts with custom media types for distributing **plugins** and **personalities**. This module provides:

- Typed pull/push/list API for personalities, plugins, and toolchains
- Metadata structs (`PersonalityMeta`, `PluginMeta`, `PersonalitySpec`, `ToolchainMeta`)
- Result types (`Personality`, `Plugin`, `ListedPersonality`, `ListedPlugin`, `ListedToolchain`)
- Reference resolution (`ResolvePersonalityRef`, `ResolvePluginRef`, `ResolveToolchainRef`)
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

// List available personalities
personalities, err := client.ListPersonalities(ctx)
for _, p := range personalities {
    fmt.Printf("%s (%s)\n", p.Name, p.Version) // "sre (v1.0.0)"
}

// List plugins from a custom registry
plugins, err := client.ListPlugins(ctx, oci.WithRegistry("custom.io/team/klaus-plugins"))

// List toolchains
toolchains, err := client.ListToolchains(ctx)
```

### Pulling a personality

```go
personality, err := client.PullPersonality(ctx, "gsoci.azurecr.io/giantswarm/klaus-personalities/sre:v1.0.0", cacheDir)
fmt.Println(personality.Meta.Name)
fmt.Println(personality.Spec.Image)
fmt.Println(personality.Soul)
```

### Pulling a plugin

```go
plugin, err := client.PullPlugin(ctx, "gsoci.azurecr.io/giantswarm/klaus-plugins/gs-platform:v1.0.0", destDir)
fmt.Printf("Plugin %s extracted to %s\n", plugin.Meta.Name, plugin.Dir)
```

### Pushing a personality

```go
result, err := client.PushPersonality(ctx, "./my-personality",
    "gsoci.azurecr.io/giantswarm/klaus-personalities/custom:v1.0.0",
    oci.PersonalityMeta{Name: "custom", Version: "1.0.0", Description: "My personality"})
```

### Pushing a plugin

```go
result, err := client.PushPlugin(ctx, "./my-plugin",
    "gsoci.azurecr.io/giantswarm/klaus-plugins/my-plugin:v1.0.0",
    oci.PluginMeta{Name: "my-plugin", Version: "1.0.0", Skills: []string{"kubernetes"}})
```

### Resolving references

```go
// Resolve short names to fully-qualified references with latest semver tag
ref, err := client.ResolvePersonalityRef(ctx, "sre")       // -> "gsoci.../sre:v1.0.0"
ref, err := client.ResolvePluginRef(ctx, "gs-platform")     // -> "gsoci.../gs-platform:v1.2.0"
ref, err := client.ResolveToolchainRef(ctx, "go")           // -> "gsoci.../go:v1.1.0"
```

## Artifact Types

| Artifact    | Config Media Type                                              | Content Media Type                                                  |
|-------------|----------------------------------------------------------------|---------------------------------------------------------------------|
| Plugin      | `application/vnd.giantswarm.klaus-plugin.config.v1+json`      | `application/vnd.giantswarm.klaus-plugin.content.v1.tar+gzip`      |
| Personality | `application/vnd.giantswarm.klaus-personality.config.v1+json`  | `application/vnd.giantswarm.klaus-personality.content.v1.tar+gzip`  |

## License

Apache 2.0 - see [LICENSE](LICENSE).
