package oci

import "strings"

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
