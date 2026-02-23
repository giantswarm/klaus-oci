# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed

- **Breaking**: Rename `PersonalitySpec.Image` to `PersonalitySpec.Toolchain` (YAML key `image` -> `toolchain`) to clearly represent the toolchain container image reference.
- Move toolchain images to `giantswarm/klaus-toolchains/<name>` sub-namespace (e.g. `gsoci.azurecr.io/giantswarm/klaus-toolchains/go`), matching the existing plugin and personality patterns. This narrows `ListRepositories` from scanning the entire `giantswarm/` catalog (921 repos, ~30s cold) to just the `klaus-toolchains/` sub-namespace.
- `DefaultToolchainRegistry` changed from `gsoci.azurecr.io/giantswarm` to `gsoci.azurecr.io/giantswarm/klaus-toolchains`.
- `ToolchainRegistryRef` no longer prepends `klaus-` to short names.
- `ResolveArtifactRef` signature simplified: removed the `namePrefix` parameter (no callers use it after the sub-namespace migration).
- `ResolveToolchainRef` no longer uses `"klaus-"` name prefix.

- `ListRepositories` now uses the catalog `last` parameter to seek past repositories that sort before the target prefix and stops early once past it, avoiding full catalog scans on large registries (921 repos down to 1-2 pages).
- `ListArtifacts` now accepts variadic `ListOption` arguments for filtering control.

### Removed

- `ArtifactInfo`, `ArtifactInfoFromAnnotations`, `AnnotationKlausType`, `AnnotationKlausName`, `AnnotationKlausVersion`, `TypePlugin`, `TypePersonality`, `TypeToolchain` -- annotation-based metadata is redundant now that each artifact type has its own sub-namespace. Type, name, and version are derivable from the repository path and tag.
- `FetchManifestAnnotations`, `FetchArtifactInfo` -- no longer needed without annotation-based identification.
- `WithAnnotations` option for `ListArtifacts` -- annotation fetching removed entirely.
- `WithPlatform` client option -- only used by the removed manifest annotation path.
- `ShortToolchainName` -- use `ShortName` instead.

### Added

- `Personality` type that combines `PersonalityMeta`, `PersonalitySpec`, and the soul document content into a single value.
- `Client.PullPersonality` method that pulls a personality artifact, parses `personality.yaml` and `SOUL.md`, and returns a fully populated `*Personality`. Consumers no longer need to navigate the extracted directory or hardcode filenames.

### Removed

- **Breaking**: `SoulFileName` public constant removed. The soul filename is now an internal detail of `PullPersonality`.
- `WithFilter` option for `ListArtifacts` to skip repositories before resolution, avoiding expensive tag listing and manifest fetches for non-matching repos.
- `Client.ListRepositories` method for discovering OCI repositories under a registry base path via the catalog API, enabling remote artifact discovery without local cache.
- `SplitRegistryBase` helper for parsing registry base paths into host and prefix components.
- Initial release of shared OCI types and ORAS client for Klaus artifacts.
- Media type constants for plugin and personality artifacts.
- `PluginMeta`, `PersonalityMeta`, and `PersonalitySpec` types.
- `PluginReference` type with `Ref()` method for building OCI reference strings.
- `ArtifactKind` type with predefined `PluginArtifact` and `PersonalityArtifact` values.
- ORAS-based `Client` with `Pull`, `Push`, `Resolve`, and `List` operations.
- Configurable credential resolution from Docker/Podman configs and environment variables.
- Digest-based caching to skip redundant pulls.
- Secure tar.gz extraction with path traversal protection and file size limits.
- `ToolchainMeta` type for toolchain image metadata.
