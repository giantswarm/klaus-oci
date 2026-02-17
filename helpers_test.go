package oci

import "testing"

func TestShortName(t *testing.T) {
	tests := []struct {
		repository string
		want       string
	}{
		{"gsoci.azurecr.io/giantswarm/klaus-plugins/gs-platform", "gs-platform"},
		{"registry.example.com/repo", "repo"},
		{"simple", "simple"},
	}

	for _, tt := range tests {
		t.Run(tt.repository, func(t *testing.T) {
			if got := ShortName(tt.repository); got != tt.want {
				t.Errorf("ShortName(%q) = %q, want %q", tt.repository, got, tt.want)
			}
		})
	}
}

func TestTruncateDigest(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"sha256:abc123def456789abcdef", "sha256:abc123def456"},
		{"sha256:short", "sha256:short"},
		{"noprefix", "noprefix"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := TruncateDigest(tt.input); got != tt.want {
				t.Errorf("TruncateDigest(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
