# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

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
