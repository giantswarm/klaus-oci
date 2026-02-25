package oci

import (
	"encoding/json"
	"slices"
	"testing"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

func TestResolvePersonalityDeps(t *testing.T) {
	pluginBaseConfig := Plugin{
		Name:        "gs-base",
		Description: "Base plugin",
		Author:      &Author{Name: "Giant Swarm GmbH"},
		Skills:      []string{"kubernetes", "fluxcd"},
	}
	pluginBaseJSON, _ := json.Marshal(pluginBaseConfig)

	pluginSREConfig := Plugin{
		Name:        "gs-sre",
		Description: "SRE plugin",
		Commands:    []string{"check-cluster"},
	}
	pluginSREJSON, _ := json.Marshal(pluginSREConfig)

	toolchainAnnotations := map[string]string{
		ocispec.AnnotationTitle:       "go",
		ocispec.AnnotationDescription: "Go toolchain for Klaus",
		ocispec.AnnotationAuthors:     "Giant Swarm GmbH",
	}

	ts := newArtifactRegistry(map[string]testArtifactEntry{
		"giantswarm/klaus-plugins/gs-base": {
			configJSON:      pluginBaseJSON,
			configMediaType: MediaTypePluginConfig,
			tags:            []string{"v1.0.0"},
		},
		"giantswarm/klaus-plugins/gs-sre": {
			configJSON:      pluginSREJSON,
			configMediaType: MediaTypePluginConfig,
			tags:            []string{"v0.5.0"},
		},
		"giantswarm/klaus-toolchains/go": {
			configJSON:      []byte(`{"architecture":"amd64"}`),
			configMediaType: ocispec.MediaTypeImageConfig,
			tags:            []string{"v1.2.0"},
			annotations:     toolchainAnnotations,
		},
	})
	defer ts.Close()
	host := testRegistryHost(ts)

	client := NewClient(WithPlainHTTP(true))

	personality := Personality{
		Name: "sre",
		Toolchain: ToolchainReference{
			Repository: host + "/giantswarm/klaus-toolchains/go",
			Tag:        "v1.2.0",
		},
		Plugins: []PluginReference{
			{Repository: host + "/giantswarm/klaus-plugins/gs-base", Tag: "v1.0.0"},
			{Repository: host + "/giantswarm/klaus-plugins/gs-sre", Tag: "v0.5.0"},
		},
	}

	deps, err := client.ResolvePersonalityDeps(t.Context(), personality)
	if err != nil {
		t.Fatalf("ResolvePersonalityDeps() error = %v", err)
	}

	if len(deps.Warnings) != 0 {
		t.Errorf("Warnings = %v, want none", deps.Warnings)
	}

	if deps.Toolchain == nil {
		t.Fatal("Toolchain is nil, want non-nil")
	}
	if deps.Toolchain.Toolchain.Name != "go" {
		t.Errorf("Toolchain.Name = %q, want %q", deps.Toolchain.Toolchain.Name, "go")
	}
	if deps.Toolchain.Toolchain.Version != "v1.2.0" {
		t.Errorf("Toolchain.Version = %q, want %q", deps.Toolchain.Toolchain.Version, "v1.2.0")
	}
	if deps.Toolchain.Toolchain.Description != "Go toolchain for Klaus" {
		t.Errorf("Toolchain.Description = %q", deps.Toolchain.Toolchain.Description)
	}

	if len(deps.Plugins) != 2 {
		t.Fatalf("Plugins length = %d, want 2", len(deps.Plugins))
	}

	names := []string{deps.Plugins[0].Plugin.Name, deps.Plugins[1].Plugin.Name}
	slices.Sort(names)
	if names[0] != "gs-base" || names[1] != "gs-sre" {
		t.Errorf("Plugin names = %v, want [gs-base gs-sre]", names)
	}

	for _, dp := range deps.Plugins {
		if dp.ArtifactInfo.Digest == "" {
			t.Errorf("Plugin %q: Digest is empty", dp.Plugin.Name)
		}
		if dp.ArtifactInfo.Tag == "" {
			t.Errorf("Plugin %q: Tag is empty", dp.Plugin.Name)
		}
	}
}

func TestResolvePersonalityDeps_MissingPlugin(t *testing.T) {
	pluginBaseConfig := Plugin{
		Name:        "gs-base",
		Description: "Base plugin",
	}
	pluginBaseJSON, _ := json.Marshal(pluginBaseConfig)

	toolchainAnnotations := map[string]string{
		ocispec.AnnotationTitle:       "go",
		ocispec.AnnotationDescription: "Go toolchain",
	}

	ts := newArtifactRegistry(map[string]testArtifactEntry{
		"giantswarm/klaus-plugins/gs-base": {
			configJSON:      pluginBaseJSON,
			configMediaType: MediaTypePluginConfig,
			tags:            []string{"v1.0.0"},
		},
		"giantswarm/klaus-toolchains/go": {
			configJSON:      []byte(`{}`),
			configMediaType: ocispec.MediaTypeImageConfig,
			tags:            []string{"v1.0.0"},
			annotations:     toolchainAnnotations,
		},
	})
	defer ts.Close()
	host := testRegistryHost(ts)

	client := NewClient(WithPlainHTTP(true))

	personality := Personality{
		Name: "sre",
		Toolchain: ToolchainReference{
			Repository: host + "/giantswarm/klaus-toolchains/go",
			Tag:        "v1.0.0",
		},
		Plugins: []PluginReference{
			{Repository: host + "/giantswarm/klaus-plugins/gs-base", Tag: "v1.0.0"},
			{Repository: host + "/giantswarm/klaus-plugins/gs-missing", Tag: "v1.0.0"},
		},
	}

	deps, err := client.ResolvePersonalityDeps(t.Context(), personality)
	if err != nil {
		t.Fatalf("ResolvePersonalityDeps() error = %v", err)
	}

	if len(deps.Warnings) != 1 {
		t.Fatalf("Warnings length = %d, want 1: %v", len(deps.Warnings), deps.Warnings)
	}

	if len(deps.Plugins) != 1 {
		t.Fatalf("Plugins length = %d, want 1", len(deps.Plugins))
	}
	if deps.Plugins[0].Plugin.Name != "gs-base" {
		t.Errorf("Plugin.Name = %q, want %q", deps.Plugins[0].Plugin.Name, "gs-base")
	}

	if deps.Toolchain == nil {
		t.Error("Toolchain should not be nil")
	}
}

func TestResolvePersonalityDeps_MissingToolchain(t *testing.T) {
	pluginConfig := Plugin{
		Name: "gs-base",
	}
	pluginJSON, _ := json.Marshal(pluginConfig)

	ts := newArtifactRegistry(map[string]testArtifactEntry{
		"giantswarm/klaus-plugins/gs-base": {
			configJSON:      pluginJSON,
			configMediaType: MediaTypePluginConfig,
			tags:            []string{"v1.0.0"},
		},
	})
	defer ts.Close()
	host := testRegistryHost(ts)

	client := NewClient(WithPlainHTTP(true))

	personality := Personality{
		Name: "sre",
		Toolchain: ToolchainReference{
			Repository: host + "/giantswarm/klaus-toolchains/missing",
			Tag:        "v1.0.0",
		},
		Plugins: []PluginReference{
			{Repository: host + "/giantswarm/klaus-plugins/gs-base", Tag: "v1.0.0"},
		},
	}

	deps, err := client.ResolvePersonalityDeps(t.Context(), personality)
	if err != nil {
		t.Fatalf("ResolvePersonalityDeps() error = %v", err)
	}

	if deps.Toolchain != nil {
		t.Errorf("Toolchain = %+v, want nil (missing)", deps.Toolchain)
	}
	if len(deps.Warnings) != 1 {
		t.Fatalf("Warnings length = %d, want 1: %v", len(deps.Warnings), deps.Warnings)
	}

	if len(deps.Plugins) != 1 {
		t.Fatalf("Plugins length = %d, want 1", len(deps.Plugins))
	}
	if deps.Plugins[0].Plugin.Name != "gs-base" {
		t.Errorf("Plugin.Name = %q, want %q", deps.Plugins[0].Plugin.Name, "gs-base")
	}
}

func TestResolvePersonalityDeps_NoPlugins(t *testing.T) {
	toolchainAnnotations := map[string]string{
		ocispec.AnnotationTitle:       "go",
		ocispec.AnnotationDescription: "Go toolchain",
	}

	ts := newArtifactRegistry(map[string]testArtifactEntry{
		"giantswarm/klaus-toolchains/go": {
			configJSON:      []byte(`{}`),
			configMediaType: ocispec.MediaTypeImageConfig,
			tags:            []string{"v1.0.0"},
			annotations:     toolchainAnnotations,
		},
	})
	defer ts.Close()
	host := testRegistryHost(ts)

	client := NewClient(WithPlainHTTP(true))

	personality := Personality{
		Name: "minimal",
		Toolchain: ToolchainReference{
			Repository: host + "/giantswarm/klaus-toolchains/go",
			Tag:        "v1.0.0",
		},
	}

	deps, err := client.ResolvePersonalityDeps(t.Context(), personality)
	if err != nil {
		t.Fatalf("ResolvePersonalityDeps() error = %v", err)
	}

	if len(deps.Warnings) != 0 {
		t.Errorf("Warnings = %v, want none", deps.Warnings)
	}
	if deps.Toolchain == nil {
		t.Fatal("Toolchain is nil, want non-nil")
	}
	if deps.Toolchain.Toolchain.Name != "go" {
		t.Errorf("Toolchain.Name = %q, want %q", deps.Toolchain.Toolchain.Name, "go")
	}
	if len(deps.Plugins) != 0 {
		t.Errorf("Plugins = %v, want empty", deps.Plugins)
	}
}

func TestResolvePersonalityDeps_EmptyToolchain(t *testing.T) {
	pluginConfig := Plugin{
		Name: "gs-base",
	}
	pluginJSON, _ := json.Marshal(pluginConfig)

	ts := newArtifactRegistry(map[string]testArtifactEntry{
		"giantswarm/klaus-plugins/gs-base": {
			configJSON:      pluginJSON,
			configMediaType: MediaTypePluginConfig,
			tags:            []string{"v1.0.0"},
		},
	})
	defer ts.Close()
	host := testRegistryHost(ts)

	client := NewClient(WithPlainHTTP(true))

	personality := Personality{
		Name: "no-toolchain",
		Plugins: []PluginReference{
			{Repository: host + "/giantswarm/klaus-plugins/gs-base", Tag: "v1.0.0"},
		},
	}

	deps, err := client.ResolvePersonalityDeps(t.Context(), personality)
	if err != nil {
		t.Fatalf("ResolvePersonalityDeps() error = %v", err)
	}

	if deps.Toolchain != nil {
		t.Errorf("Toolchain = %+v, want nil", deps.Toolchain)
	}
	if len(deps.Warnings) != 0 {
		t.Errorf("Warnings = %v, want none", deps.Warnings)
	}
	if len(deps.Plugins) != 1 {
		t.Fatalf("Plugins length = %d, want 1", len(deps.Plugins))
	}
}
