package oci

import (
	"strings"

	"github.com/Masterminds/semver/v3"
)

// SplitRegistryBase splits a registry base path into the registry host and
// the repository name prefix (with trailing slash). For example,
// "gsoci.azurecr.io/giantswarm/klaus-plugins" returns
// ("gsoci.azurecr.io", "giantswarm/klaus-plugins/").
// If the base contains no slash, the prefix is empty (matches all repositories).
func SplitRegistryBase(base string) (host, prefix string) {
	idx := strings.Index(base, "/")
	if idx < 0 {
		return base, ""
	}
	return base[:idx], base[idx+1:] + "/"
}

// ShortName extracts the last segment of a repository path.
// For example, "gsoci.azurecr.io/giantswarm/klaus-plugins/gs-platform" returns "gs-platform".
func ShortName(repository string) string {
	parts := strings.Split(repository, "/")
	return parts[len(parts)-1]
}

// TruncateDigest shortens a digest string for human-readable display.
// For example, "sha256:abc123def456..." becomes "sha256:abc123def456".
func TruncateDigest(d string) string {
	if idx := strings.Index(d, ":"); idx >= 0 {
		suffix := d[idx+1:]
		if len(suffix) > 12 {
			return d[:idx+1] + suffix[:12]
		}
	}
	return d
}

// LatestSemverTag returns the highest semver tag from the given list.
// Tags that are not valid semver are silently ignored.
func LatestSemverTag(tags []string) string {
	var best *semver.Version
	var bestTag string

	for _, tag := range tags {
		v, err := semver.NewVersion(tag)
		if err != nil {
			continue
		}
		if best == nil || v.GreaterThan(best) {
			best = v
			bestTag = tag
		}
	}

	return bestTag
}

// SplitNameTag splits "name:tag" into name and tag. If no tag-position colon
// is present, tag is empty. Port-only colons (e.g. "localhost:5000/repo") are
// not treated as tag separators.
func SplitNameTag(ref string) (string, string) {
	nameStart := strings.LastIndex(ref, "/")
	if idx := strings.LastIndex(ref, ":"); idx > nameStart {
		return ref[:idx], ref[idx+1:]
	}
	return ref, ""
}

// RepositoryFromRef extracts the repository part from an OCI reference,
// stripping the tag or digest suffix. Handles both repo:tag and
// repo@sha256:digest formats. Port-only colons (e.g. localhost:5000/repo)
// are preserved. References without a path component (e.g. "localhost:5000")
// are returned unchanged.
func RepositoryFromRef(ref string) string {
	if idx := strings.Index(ref, "@"); idx > 0 {
		return ref[:idx]
	}
	nameStart := strings.LastIndex(ref, "/")
	if idx := strings.LastIndex(ref, ":"); idx > nameStart && nameStart >= 0 {
		return ref[:idx]
	}
	return ref
}

func hasTagOrDigest(ref string) bool {
	if hasDigest(ref) {
		return true
	}
	nameStart := strings.LastIndex(ref, "/")
	tagIdx := strings.LastIndex(ref, ":")
	return tagIdx > nameStart
}

func hasDigest(ref string) bool {
	return strings.Contains(ref, "@sha256:") || strings.Contains(ref, "@sha512:")
}

func extractTag(ref string) string {
	if hasDigest(ref) {
		return ""
	}
	_, tag := SplitNameTag(ref)
	return tag
}
