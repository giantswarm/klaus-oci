# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed

- **BREAKING**: Unified domain types -- `PluginMeta` renamed to `Plugin`, `PersonalityMeta`/`PersonalitySpec` merged into `Personality`, `ToolchainMeta` replaced by `Toolchain` with richer metadata fields (Author, Homepage, SourceRepo, License, Keywords derived from OCI manifest annotations).
- **BREAKING**: `PersonalitySpec.Image` (string) replaced by `Personality.Toolchain` (`ToolchainReference` with Repository/Tag/Digest).
- **BREAKING**: Pull return types changed from `*Personality`/`*Plugin` (which were pull result wrappers) to `*PulledPersonality`/`*PulledPlugin`, embedding the domain type plus OCI metadata and local file state.
- **BREAKING**: List return types unified from `[]ListedPersonality`/`[]ListedPlugin`/`[]ListedToolchain` to `[]ListEntry` across all three artifact types.
- **BREAKING**: Version excluded from OCI config blob for all artifact types (`json:"-"`). Version is conveyed exclusively via OCI tags and populated from the resolved tag during describe/pull. Existing artifacts need re-pushing.
- **BREAKING**: Personality config blob now contains the full `Personality` struct (metadata + composition: toolchain reference, plugin references) instead of the previous minimal `{name, version, description}`.
- **BREAKING**: `personality.yaml` format overhauled: `image: "..."` replaced by `toolchain: {repository: "...", tag: "..."}`, `name` field now required (previously implicit from directory name), new optional fields `author`, `repository`, `homepage`, `license`, `keywords`.
- **BREAKING**: `ReadPersonalityFromDir` signature changed from `ReadPersonalityFromDir(dir, name, version)` to `ReadPersonalityFromDir(dir)` -- name comes from `personality.yaml`, version from OCI tag.
- **BREAKING**: `PushPersonality` and `PushPlugin` now accept the domain type structs (`Personality`, `Plugin`) instead of the old `PersonalityMeta`/`PluginMeta`.
- Move toolchain images to `giantswarm/klaus-toolchains/<name>` sub-namespace (e.g. `gsoci.azurecr.io/giantswarm/klaus-toolchains/go`), matching the existing plugin and personality patterns. This narrows `ListRepositories` from scanning the entire `giantswarm/` catalog (921 repos, ~30s cold) to just the `klaus-toolchains/` sub-namespace.
- `DefaultToolchainRegistry` changed from `gsoci.azurecr.io/giantswarm` to `gsoci.azurecr.io/giantswarm/klaus-toolchains`.
- `ToolchainRegistryRef` no longer prepends `klaus-` to short names.
- `ResolveArtifactRef` signature simplified: removed the `namePrefix` parameter (no callers use it after the sub-namespace migration).
- `ResolveToolchainRef` no longer uses `"klaus-"` name prefix.
- `ListRepositories` now uses the catalog `last` parameter to seek past repositories that sort before the target prefix and stops early once past it, avoiding full catalog scans on large registries (921 repos down to 1-2 pages).
- Replace generic `Pull`/`Push`/`ListArtifacts` API with typed facade: consumers now use `PullPersonality`, `PullPlugin`, `PushPersonality`, `PushPlugin`, `ListPersonalities`, `ListPlugins`, `ListToolchains` instead of interacting with raw OCI concepts.
- **BREAKING**: Common metadata (name, description, author, homepage, repository, license, keywords) moved from OCI config blobs into `io.giantswarm.klaus.*` manifest annotations for plugins and personalities. Config blobs now contain only type-specific data: discovered components for plugins and composition (toolchain + plugin references) for personalities. Existing artifacts need re-pushing.
- **BREAKING**: Toolchain annotations switched from `org.opencontainers.image.*` to `io.giantswarm.klaus.*` namespace. Existing toolchain images need to be re-built with the new labels.
- **BREAKING**: Cache entries now include manifest annotations. Old cached artifacts without the `Annotations` field will lose common metadata until re-pulled.

### Removed

- `PluginMeta` -- replaced by `Plugin` (domain type with both manifest metadata and discovered components).
- `PersonalityMeta`, `PersonalitySpec` -- replaced by unified `Personality` type with dual `yaml`/`json` struct tags.
- `ToolchainMeta` -- replaced by `Toolchain` with richer metadata fields.
- `ArtifactResult` -- replaced by `ArtifactInfo` (embedded in `DescribedPlugin`, `PulledPlugin`, etc.).
- `ListedPersonality`, `ListedPlugin`, `ListedToolchain` -- replaced by unified `ListEntry` type.
- `ListedArtifactInfo` -- no longer needed.
- `ArtifactInfo`, `ArtifactInfoFromAnnotations`, `AnnotationKlausType`, `AnnotationKlausName`, `AnnotationKlausVersion`, `TypePlugin`, `TypePersonality`, `TypeToolchain` -- annotation-based metadata is redundant now that each artifact type has its own sub-namespace.
- `FetchManifestAnnotations`, `FetchArtifactInfo` -- no longer needed without annotation-based identification.
- `WithAnnotations` option for `ListArtifacts` -- annotation fetching removed entirely.
- `WithPlatform` client option -- only used by the removed manifest annotation path.
- `ShortToolchainName` -- use `ShortName` instead.
- `Pull`, `Push`, `ListArtifacts`, `ListRepositories` -- replaced by typed methods.
- `ArtifactKind`, `PluginArtifact`, `PersonalityArtifact` -- internalized; consumers no longer need to specify artifact kinds.
- `PullResult`, `ListedArtifact` -- replaced by typed result structs.

### Added

- `Plugin` domain type with manifest metadata (from `.claude-plugin/plugin.json`) and discovered components (Skills, Commands, Agents, HasHooks, MCPServers, LSPServers).
- `Personality` domain type with metadata, composition (Toolchain + Plugins references), and dual `yaml`/`json` struct tags for both on-disk and OCI config blob formats.
- `Toolchain` domain type derived from OCI manifest annotations (Name, Description, Author, Homepage, SourceRepo, License, Keywords).
- `DescribePlugin`, `DescribePersonality`, `DescribeToolchain` methods that fetch metadata without downloading content layers. `DescribeToolchain` maps OCI manifest annotations to the `Toolchain` struct.
- `DescribedPlugin`, `DescribedPersonality`, `DescribedToolchain` result types embedding `ArtifactInfo` and the domain type.
- `PulledPlugin`, `PulledPersonality` result types with OCI metadata, domain type, local file state, and (for personalities) the soul text from `SOUL.md`.
- `ArtifactInfo` base struct with Ref, Tag, and Digest fields for OCI-level metadata.
- `ListEntry` unified list result type with Name, Version, Repository, and Reference.
- `ToolchainReference` type (analogous to `PluginReference`) with `Ref()` method.
- `ReadPluginFromDir` helper that reads `.claude-plugin/plugin.json` and discovers components by scanning the directory tree.
- `ReadPersonalityFromDir` helper that reads `personality.yaml` into a `Personality` struct.
- `ListPluginVersions`, `ListPersonalityVersions`, `ListToolchainVersions` methods to list all semver tags for a specific artifact.
- `ResolvePersonalityDeps` method for resolving a personality's toolchain and plugin references concurrently, with warnings for missing artifacts.
- `ResolvedDependencies` result type with described toolchain, described plugins, and warnings.
- `PushResult` type with digest.
- `PullPersonality` and `PullPlugin` typed pull methods that return `*PulledPersonality` and `*PulledPlugin`.
- `PushPersonality` and `PushPlugin` typed push methods that accept domain type structs.
- `ListPersonalities`, `ListPlugins`, `ListToolchains` typed listing methods returning `[]ListEntry`.
- `WithRegistry` list option to override the default registry base path.
- `WithFilter` option for listing methods to skip repositories before resolution.
- OCI config blob persisted in cache entries so metadata is always populated on cache hits.
- `SplitRegistryBase` helper for parsing registry base paths into host and prefix components.
- `ResolvePluginRefs` batch resolution of `[]PluginReference`.
- Media type constants for plugin and personality artifacts.
- `PluginReference` type with `Ref()` method for building OCI reference strings.
- ORAS-based `Client` with typed pull, push, resolve, and list operations.
- Configurable credential resolution from Docker/Podman configs and environment variables.
- Digest-based caching to skip redundant pulls.
- Secure tar.gz extraction with path traversal protection and file size limits.
