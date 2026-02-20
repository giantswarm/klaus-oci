# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- `Client.ResolveLatestVersion` method for resolving a repository to its highest semver-tagged reference.
- `Client.ResolveArtifactRef` method for resolving short names and `:latest` tags to fully-qualified OCI references with semver resolution.
- `Client.ResolvePluginRefs` method for batch-resolving `[]PluginReference` entries to their latest semver tags.
- `Client.FetchManifestAnnotations` method for reading OCI manifest annotations without pulling content layers. Supports multi-arch (index) manifests by selecting the current runtime platform automatically.
- `LatestSemverTag` helper for selecting the highest semver tag from a tag list.
- `ShortToolchainName`, `SplitNameTag`, `RepositoryFromRef`, and `ToolchainRegistryRef` helper functions for OCI reference manipulation.
- `DefaultPluginRegistry`, `DefaultPersonalityRegistry`, and `DefaultToolchainRegistry` constants for standard Klaus registry base paths.
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
- Shared Klaus annotation key constants (`AnnotationKlausType`, `AnnotationKlausName`, `AnnotationKlausVersion`) for uniform cross-type identification on OCI manifests.
- Type value constants (`TypePlugin`, `TypePersonality`, `TypeToolchain`) for `AnnotationKlausType`.
- `ToolchainMeta` type for toolchain image metadata.
- `ArtifactInfo` type and `ArtifactInfoFromAnnotations` helper for extracting Klaus metadata from OCI manifest annotations.
