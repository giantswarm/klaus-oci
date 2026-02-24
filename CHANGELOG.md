# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed

- Move toolchain images to `giantswarm/klaus-toolchains/<name>` sub-namespace (e.g. `gsoci.azurecr.io/giantswarm/klaus-toolchains/go`), matching the existing plugin and personality patterns. This narrows `ListRepositories` from scanning the entire `giantswarm/` catalog (921 repos, ~30s cold) to just the `klaus-toolchains/` sub-namespace.
- `DefaultToolchainRegistry` changed from `gsoci.azurecr.io/giantswarm` to `gsoci.azurecr.io/giantswarm/klaus-toolchains`.
- `ToolchainRegistryRef` no longer prepends `klaus-` to short names.
- `ResolveArtifactRef` signature simplified: removed the `namePrefix` parameter (no callers use it after the sub-namespace migration).
- `ResolveToolchainRef` no longer uses `"klaus-"` name prefix.

- `ListRepositories` now uses the catalog `last` parameter to seek past repositories that sort before the target prefix and stops early once past it, avoiding full catalog scans on large registries (921 repos down to 1-2 pages).
- Replace generic `Pull`/`Push`/`ListArtifacts` API with typed facade: consumers now use `PullPersonality`, `PullPlugin`, `PushPersonality`, `PushPlugin`, `ListPersonalities`, `ListPlugins`, `ListToolchains` instead of interacting with raw OCI concepts.

### Removed

- `ArtifactInfo`, `ArtifactInfoFromAnnotations`, `AnnotationKlausType`, `AnnotationKlausName`, `AnnotationKlausVersion`, `TypePlugin`, `TypePersonality`, `TypeToolchain` -- annotation-based metadata is redundant now that each artifact type has its own sub-namespace. Type, name, and version are derivable from the repository path and tag.
- `FetchManifestAnnotations`, `FetchArtifactInfo` -- no longer needed without annotation-based identification.
- `WithAnnotations` option for `ListArtifacts` -- annotation fetching removed entirely.
- `WithPlatform` client option -- only used by the removed manifest annotation path.
- `ShortToolchainName` -- use `ShortName` instead.
- `Pull`, `Push`, `ListArtifacts`, `ListRepositories` -- replaced by typed methods (`PullPersonality`, `PullPlugin`, `PushPersonality`, `PushPlugin`, `ListPersonalities`, `ListPlugins`, `ListToolchains`).
- `ArtifactKind`, `PluginArtifact`, `PersonalityArtifact` -- internalized; consumers no longer need to specify artifact kinds.
- `PullResult`, `ListedArtifact` -- replaced by typed result structs (`Personality`, `Plugin`, `ListedPersonality`, `ListedPlugin`, `ListedToolchain`).

### Added

- `PullPersonality` and `PullPlugin` typed pull methods that return parsed `*Personality` and `*Plugin` structs respectively, hiding OCI transport details from consumers.
- `PushPersonality` and `PushPlugin` typed push methods that accept `PersonalityMeta`/`PluginMeta` structs directly instead of raw JSON.
- `ListPersonalities`, `ListPlugins`, `ListToolchains` typed listing methods that return `[]ListedPersonality`, `[]ListedPlugin`, `[]ListedToolchain` with extracted name and version.
- `WithRegistry` list option to override the default registry base path for multi-source configurations.
- `Personality` struct as the pull result for personality artifacts, containing parsed `Meta`, `Spec`, `Soul`, and extraction directory.
- `Plugin` struct as the pull result for plugin artifacts, containing parsed `Meta` and extraction directory.
- `ListedPersonality`, `ListedPlugin`, `ListedToolchain` lightweight discovery result types.
- `WithFilter` option for listing methods to skip repositories before resolution.
- `SplitRegistryBase` helper for parsing registry base paths into host and prefix components.
- Initial release of shared OCI types and ORAS client for Klaus artifacts.
- Media type constants for plugin and personality artifacts.
- `PluginMeta`, `PersonalityMeta`, and `PersonalitySpec` types.
- `PluginReference` type with `Ref()` method for building OCI reference strings.
- ORAS-based `Client` with typed pull, push, resolve, and list operations.
- Configurable credential resolution from Docker/Podman configs and environment variables.
- Digest-based caching to skip redundant pulls.
- Secure tar.gz extraction with path traversal protection and file size limits.
- `ToolchainMeta` type for toolchain image metadata.
