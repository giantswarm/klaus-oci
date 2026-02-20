package oci

import "strings"

// Default OCI registry base paths for each Klaus artifact type.
const (
	DefaultPluginRegistry      = "gsoci.azurecr.io/giantswarm/klaus-plugins"
	DefaultPersonalityRegistry = "gsoci.azurecr.io/giantswarm/klaus-personalities"
	DefaultToolchainRegistry   = "gsoci.azurecr.io/giantswarm"
)

// ShortToolchainName extracts the toolchain name from a full repository path,
// stripping both the registry prefix and the "klaus-" convention.
// For example, "gsoci.azurecr.io/giantswarm/klaus-go" returns "go".
func ShortToolchainName(repository string) string {
	name := ShortName(repository)
	return strings.TrimPrefix(name, "klaus-")
}

// ToolchainRegistryRef returns the full registry reference for a toolchain
// image name. Toolchains use the pattern gsoci.azurecr.io/giantswarm/klaus-<name>.
// If the name already starts with the toolchain registry base, it is returned as-is.
func ToolchainRegistryRef(name string) string {
	if strings.HasPrefix(name, DefaultToolchainRegistry) {
		return name
	}
	if !strings.HasPrefix(name, "klaus-") {
		name = "klaus-" + name
	}
	return DefaultToolchainRegistry + "/" + name
}
