package oci

import (
	"context"
	"fmt"
	"testing"
)

// mockTagLister returns preconfigured tag lists keyed by repository.
type mockTagLister struct {
	tags map[string][]string
}

func (m *mockTagLister) List(_ context.Context, repository string) ([]string, error) {
	tags, ok := m.tags[repository]
	if !ok {
		return nil, fmt.Errorf("repository not found: %s", repository)
	}
	return tags, nil
}

func TestResolveArtifactRef(t *testing.T) {
	lister := &mockTagLister{
		tags: map[string][]string{
			"gsoci.azurecr.io/giantswarm/klaus-plugins/gs-ae":     {"v0.0.1", "v0.0.3", "v0.0.2"},
			"gsoci.azurecr.io/giantswarm/klaus-personalities/sre": {"v0.1.0", "v0.2.0"},
			"custom.registry.io/org/my-plugin":                    {"v2.0.0"},
			"custom.registry.io/org/no-semver":                    {"latest", "main", "dev"},
		},
	}
	tests := []struct {
		name         string
		ref          string
		registryBase string
		want         string
		wantErr      bool
	}{
		{
			name:         "empty ref returns error",
			ref:          "",
			registryBase: "gsoci.azurecr.io/giantswarm/klaus-plugins",
			wantErr:      true,
		},
		{
			name:         "whitespace-only ref returns error",
			ref:          "   ",
			registryBase: "gsoci.azurecr.io/giantswarm/klaus-plugins",
			wantErr:      true,
		},
		{
			name:         "short name with explicit tag",
			ref:          "gs-ae:v0.0.2",
			registryBase: "gsoci.azurecr.io/giantswarm/klaus-plugins",
			want:         "gsoci.azurecr.io/giantswarm/klaus-plugins/gs-ae:v0.0.2",
		},
		{
			name:         "short name without tag resolves latest",
			ref:          "gs-ae",
			registryBase: "gsoci.azurecr.io/giantswarm/klaus-plugins",
			want:         "gsoci.azurecr.io/giantswarm/klaus-plugins/gs-ae:v0.0.3",
		},
		{
			name:         "short name with latest tag resolves actual",
			ref:          "gs-ae:latest",
			registryBase: "gsoci.azurecr.io/giantswarm/klaus-plugins",
			want:         "gsoci.azurecr.io/giantswarm/klaus-plugins/gs-ae:v0.0.3",
		},
		{
			name:         "full ref with tag returned as-is",
			ref:          "custom.registry.io/org/my-plugin:v2.0.0",
			registryBase: "gsoci.azurecr.io/giantswarm/klaus-plugins",
			want:         "custom.registry.io/org/my-plugin:v2.0.0",
		},
		{
			name:         "full ref with digest returned as-is",
			ref:          "custom.registry.io/org/my-plugin@sha256:abc123",
			registryBase: "gsoci.azurecr.io/giantswarm/klaus-plugins",
			want:         "custom.registry.io/org/my-plugin@sha256:abc123",
		},
		{
			name:         "full ref without tag resolves latest",
			ref:          "custom.registry.io/org/my-plugin",
			registryBase: "gsoci.azurecr.io/giantswarm/klaus-plugins",
			want:         "custom.registry.io/org/my-plugin:v2.0.0",
		},
		{
			name:         "full ref with latest tag resolves actual",
			ref:          "custom.registry.io/org/my-plugin:latest",
			registryBase: "gsoci.azurecr.io/giantswarm/klaus-plugins",
			want:         "custom.registry.io/org/my-plugin:v2.0.0",
		},
		{
			name:         "whitespace trimmed",
			ref:          "  gs-ae:v0.0.2  ",
			registryBase: "gsoci.azurecr.io/giantswarm/klaus-plugins",
			want:         "gsoci.azurecr.io/giantswarm/klaus-plugins/gs-ae:v0.0.2",
		},
		{
			name:         "unknown short name returns error",
			ref:          "nonexistent",
			registryBase: "gsoci.azurecr.io/giantswarm/klaus-plugins",
			wantErr:      true,
		},
		{
			name:         "full ref with no semver tags returns error",
			ref:          "custom.registry.io/org/no-semver",
			registryBase: "gsoci.azurecr.io/giantswarm/klaus-plugins",
			wantErr:      true,
		},
		{
			name:         "full ref with latest tag and no semver tags returns error",
			ref:          "custom.registry.io/org/no-semver:latest",
			registryBase: "gsoci.azurecr.io/giantswarm/klaus-plugins",
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveArtifactRef(t.Context(), lister, tt.ref, tt.registryBase)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("resolveArtifactRef() = %q, want error", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveArtifactRef() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("resolveArtifactRef() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveToolchainRef(t *testing.T) {
	lister := &mockTagLister{
		tags: map[string][]string{
			"gsoci.azurecr.io/giantswarm/klaus-toolchains/go":     {"v1.0.0", "v1.1.0"},
			"gsoci.azurecr.io/giantswarm/klaus-toolchains/python": {"v0.5.0"},
		},
	}

	tests := []struct {
		name    string
		ref     string
		want    string
		wantErr bool
	}{
		{
			name: "short name",
			ref:  "go",
			want: "gsoci.azurecr.io/giantswarm/klaus-toolchains/go:v1.1.0",
		},
		{
			name: "short name with explicit tag",
			ref:  "go:v1.0.0",
			want: "gsoci.azurecr.io/giantswarm/klaus-toolchains/go:v1.0.0",
		},
		{
			name:    "unknown short name",
			ref:     "rust",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveArtifactRef(t.Context(), lister, tt.ref, DefaultToolchainRegistry)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ResolveToolchainRef() = %q, want error", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ResolveToolchainRef() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("ResolveToolchainRef() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolvePluginRef(t *testing.T) {
	lister := &mockTagLister{
		tags: map[string][]string{
			"gsoci.azurecr.io/giantswarm/klaus-plugins/gs-ae": {"v0.0.1", "v0.0.3", "v0.0.2"},
		},
	}

	tests := []struct {
		name    string
		ref     string
		want    string
		wantErr bool
	}{
		{
			name: "short name resolves latest",
			ref:  "gs-ae",
			want: "gsoci.azurecr.io/giantswarm/klaus-plugins/gs-ae:v0.0.3",
		},
		{
			name: "short name with explicit tag",
			ref:  "gs-ae:v0.0.2",
			want: "gsoci.azurecr.io/giantswarm/klaus-plugins/gs-ae:v0.0.2",
		},
		{
			name:    "unknown plugin",
			ref:     "nonexistent",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveArtifactRef(t.Context(), lister, tt.ref, DefaultPluginRegistry)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ResolvePluginRef() = %q, want error", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ResolvePluginRef() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("ResolvePluginRef() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolvePersonalityRef(t *testing.T) {
	lister := &mockTagLister{
		tags: map[string][]string{
			"gsoci.azurecr.io/giantswarm/klaus-personalities/sre": {"v0.1.0", "v0.2.0"},
		},
	}

	tests := []struct {
		name    string
		ref     string
		want    string
		wantErr bool
	}{
		{
			name: "short name resolves latest",
			ref:  "sre",
			want: "gsoci.azurecr.io/giantswarm/klaus-personalities/sre:v0.2.0",
		},
		{
			name: "short name with explicit tag",
			ref:  "sre:v0.1.0",
			want: "gsoci.azurecr.io/giantswarm/klaus-personalities/sre:v0.1.0",
		},
		{
			name:    "unknown personality",
			ref:     "nonexistent",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveArtifactRef(t.Context(), lister, tt.ref, DefaultPersonalityRegistry)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ResolvePersonalityRef() = %q, want error", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ResolvePersonalityRef() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("ResolvePersonalityRef() = %q, want %q", got, tt.want)
			}
		})
	}
}
