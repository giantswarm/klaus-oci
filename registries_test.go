package oci

import "testing"

func TestShortToolchainName(t *testing.T) {
	tests := []struct {
		repository string
		want       string
	}{
		{"gsoci.azurecr.io/giantswarm/klaus-toolchains/go", "go"},
		{"gsoci.azurecr.io/giantswarm/klaus-toolchains/python", "python"},
		{"gsoci.azurecr.io/giantswarm/klaus-toolchains/git", "git"},
		{"gsoci.azurecr.io/giantswarm/klaus-toolchains/git-debian", "git-debian"},
		{"registry.example.com/other-image", "other-image"},
	}

	for _, tt := range tests {
		t.Run(tt.repository, func(t *testing.T) {
			if got := ShortToolchainName(tt.repository); got != tt.want {
				t.Errorf("ShortToolchainName(%q) = %q, want %q", tt.repository, got, tt.want)
			}
		})
	}
}

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
