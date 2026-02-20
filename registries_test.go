package oci

import "testing"

func TestShortToolchainName(t *testing.T) {
	tests := []struct {
		repository string
		want       string
	}{
		{"gsoci.azurecr.io/giantswarm/klaus-go", "go"},
		{"gsoci.azurecr.io/giantswarm/klaus-python", "python"},
		{"gsoci.azurecr.io/giantswarm/klaus-git", "git"},
		{"registry.example.com/other-image", "other-image"},
		{"klaus-go", "go"},
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
		{"go", "gsoci.azurecr.io/giantswarm/klaus-go"},
		{"python", "gsoci.azurecr.io/giantswarm/klaus-python"},
		{"gsoci.azurecr.io/giantswarm/klaus-go", "gsoci.azurecr.io/giantswarm/klaus-go"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ToolchainRegistryRef(tt.name); got != tt.want {
				t.Errorf("ToolchainRegistryRef(%q) = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}
