# klaus-oci

Shared Go types and ORAS client for [Klaus](https://github.com/giantswarm/klausctl) OCI artifacts.

Klaus uses OCI artifacts with custom media types for distributing **plugins** and **personalities**. This module provides:

- Media type constants for all Klaus artifact types
- Metadata structs (`PluginMeta`, `PersonalityMeta`, `PersonalitySpec`, `ToolchainMeta`)
- `Personality` type combining parsed spec, soul document, and OCI metadata
- `PullPersonality` method that pulls and parses a personality artifact in one call
- An ORAS-based registry client for pull, push, resolve, and list operations
- Digest-based caching to avoid redundant pulls
- Secure tar.gz archive extraction with path traversal protection

Both [klausctl](https://github.com/giantswarm/klausctl) and the [klaus-operator](https://github.com/giantswarm/klaus-operator) import this module to ensure artifact format consistency.

## Installation

```bash
go get github.com/giantswarm/klaus-oci
```

## Usage

### Pulling a personality

```go
import oci "github.com/giantswarm/klaus-oci"

client := oci.NewClient(
    oci.WithRegistryAuthEnv("KLAUSCTL_REGISTRY_AUTH"),
)

p, err := client.PullPersonality(ctx, "gsoci.azurecr.io/giantswarm/klaus-personalities/sre:v1.0.0", "/tmp/cache/sre")
if err != nil { ... }

fmt.Println(p.Meta.Name)           // "sre"
fmt.Println(p.Spec.Toolchain)      // "gsoci.azurecr.io/giantswarm/klaus-toolchains/go:v1.0.0"
fmt.Println(len(p.Spec.Plugins))   // 2
fmt.Println(p.Soul)                // "You are a senior SRE at Giant Swarm..."
```

### Pulling a plugin (generic)

```go
result, err := client.Pull(ctx, "gsoci.azurecr.io/giantswarm/klaus-plugins/gs-platform:v1.0.0", "/tmp/plugin", oci.PluginArtifact)
```

### Pushing a personality

```go
import oci "github.com/giantswarm/klaus-oci"

meta := oci.PersonalityMeta{
    Name:        "sre",
    Version:     "1.0.0",
    Description: "Giant Swarm SRE personality",
}
configJSON, _ := json.Marshal(meta)

client := oci.NewClient()
result, err := client.Push(ctx, "./personality-dir", "gsoci.azurecr.io/giantswarm/klaus-personalities/sre:v1.0.0", configJSON, oci.PersonalityArtifact)
```

### Listing tags

```go
tags, err := client.List(ctx, "gsoci.azurecr.io/giantswarm/klaus-plugins/gs-platform")
```

## Artifact Types

| Artifact    | Config Media Type                                              | Content Media Type                                                  |
|-------------|----------------------------------------------------------------|---------------------------------------------------------------------|
| Plugin      | `application/vnd.giantswarm.klaus-plugin.config.v1+json`      | `application/vnd.giantswarm.klaus-plugin.content.v1.tar+gzip`      |
| Personality | `application/vnd.giantswarm.klaus-personality.config.v1+json`  | `application/vnd.giantswarm.klaus-personality.content.v1.tar+gzip`  |

## License

Apache 2.0 - see [LICENSE](LICENSE).
