package oci

import "testing"

func TestToolchainRegistryRef(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"go", "gsoci.azurecr.io/giantswarm/klaus-toolchains/go"},
		{"python", "gsoci.azurecr.io/giantswarm/klaus-toolchains/python"},
		{"git-debian", "gsoci.azurecr.io/giantswarm/klaus-toolchains/git-debian"},
		{"gsoci.azurecr.io/giantswarm/klaus-toolchains/go", "gsoci.azurecr.io/giantswarm/klaus-toolchains/go"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ToolchainRegistryRef(tt.name); got != tt.want {
				t.Errorf("ToolchainRegistryRef(%q) = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}
