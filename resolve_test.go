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
			"gsoci.azurecr.io/giantswarm/klaus-plugins/gs-ae":     {testTagV001, testTagV003, testTagV002},
			"gsoci.azurecr.io/giantswarm/klaus-personalities/sre": {testTagV010, testTagV020},
			"custom.registry.io/org/my-plugin":                    {testTagV200},
			"custom.registry.io/org/no-semver":                    {testTagLatest, testTagMain, testTagDev},
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
			registryBase: testRegistryPlugins,
			wantErr:      true,
		},
		{
			name:         "whitespace-only ref returns error",
			ref:          "   ",
			registryBase: testRegistryPlugins,
			wantErr:      true,
		},
		{
			name:         testCaseShortNameExplicitTag,
			ref:          "gs-ae:v0.0.2",
			registryBase: testRegistryPlugins,
			want:         testRefPluginGSAEV002,
		},
		{
			name:         "short name without tag resolves latest",
			ref:          testNameGSAE,
			registryBase: testRegistryPlugins,
			want:         testRefPluginGSAEV003,
		},
		{
			name:         "short name with latest tag resolves actual",
			ref:          "gs-ae:latest",
			registryBase: testRegistryPlugins,
			want:         testRefPluginGSAEV003,
		},
		{
			name:         "full ref with tag returned as-is",
			ref:          testRefCustomPluginV200,
			registryBase: testRegistryPlugins,
			want:         testRefCustomPluginV200,
		},
		{
			name:         "full ref with digest returned as-is",
			ref:          "custom.registry.io/org/my-plugin@sha256:abc123",
			registryBase: testRegistryPlugins,
			want:         "custom.registry.io/org/my-plugin@sha256:abc123",
		},
		{
			name:         "full ref without tag resolves latest",
			ref:          "custom.registry.io/org/my-plugin",
			registryBase: testRegistryPlugins,
			want:         testRefCustomPluginV200,
		},
		{
			name:         "full ref with latest tag resolves actual",
			ref:          "custom.registry.io/org/my-plugin:latest",
			registryBase: testRegistryPlugins,
			want:         testRefCustomPluginV200,
		},
		{
			name:         "whitespace trimmed",
			ref:          "  gs-ae:v0.0.2  ",
			registryBase: testRegistryPlugins,
			want:         testRefPluginGSAEV002,
		},
		{
			name:         "unknown short name returns error",
			ref:          testNameNonexistent,
			registryBase: testRegistryPlugins,
			wantErr:      true,
		},
		{
			name:         "full ref with no semver tags returns error",
			ref:          "custom.registry.io/org/no-semver",
			registryBase: testRegistryPlugins,
			wantErr:      true,
		},
		{
			name:         "full ref with latest tag and no semver tags returns error",
			ref:          "custom.registry.io/org/no-semver:latest",
			registryBase: testRegistryPlugins,
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
			testRefToolchainGo:     {testTagV100, testTagV110},
			testRefToolchainPython: {testTagV050},
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
			name: testCaseShortNameExplicitTag,
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
			"gsoci.azurecr.io/giantswarm/klaus-plugins/gs-ae": {testTagV001, testTagV003, testTagV002},
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
			ref:  testNameGSAE,
			want: testRefPluginGSAEV003,
		},
		{
			name: testCaseShortNameExplicitTag,
			ref:  "gs-ae:v0.0.2",
			want: testRefPluginGSAEV002,
		},
		{
			name:    "unknown plugin",
			ref:     testNameNonexistent,
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
			"gsoci.azurecr.io/giantswarm/klaus-personalities/sre": {testTagV010, testTagV020},
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
			ref:  testNameSRE,
			want: "gsoci.azurecr.io/giantswarm/klaus-personalities/sre:v0.2.0",
		},
		{
			name: testCaseShortNameExplicitTag,
			ref:  "sre:v0.1.0",
			want: "gsoci.azurecr.io/giantswarm/klaus-personalities/sre:v0.1.0",
		},
		{
			name:    "unknown personality",
			ref:     testNameNonexistent,
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
