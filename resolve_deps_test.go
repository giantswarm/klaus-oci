package oci

import (
	"encoding/json"
	"slices"
	"testing"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

func TestResolvePersonalityDeps(t *testing.T) {
	pluginBaseBlob := pluginConfigBlob{Skills: []string{"kubernetes", "fluxcd"}}
	pluginBaseJSON, _ := json.Marshal(pluginBaseBlob)
	pluginBaseAnnotations := buildKlausAnnotations("gs-base", "Base plugin", &Author{Name: "Giant Swarm GmbH"}, "", "", "", nil)

	pluginSREBlob := pluginConfigBlob{Commands: []string{"check-cluster"}}
	pluginSREJSON, _ := json.Marshal(pluginSREBlob)
	pluginSREAnnotations := buildKlausAnnotations("gs-sre", "SRE plugin", nil, "", "", "", nil)

	toolchainAnnotations := map[string]string{
		AnnotationName:        "go",
		AnnotationDescription: "Go toolchain for Klaus",
		AnnotationAuthorName:  "Giant Swarm GmbH",
	}

	ts := newArtifactRegistry(map[string]testArtifactEntry{
		"giantswarm/klaus-plugins/gs-base": {
			configJSON:      pluginBaseJSON,
			configMediaType: MediaTypePluginConfig,
			tags:            []string{"v1.0.0"},
			annotations:     pluginBaseAnnotations,
		},
		"giantswarm/klaus-plugins/gs-sre": {
			configJSON:      pluginSREJSON,
			configMediaType: MediaTypePluginConfig,
			tags:            []string{"v0.5.0"},
			annotations:     pluginSREAnnotations,
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
	pluginBaseBlob := pluginConfigBlob{}
	pluginBaseJSON, _ := json.Marshal(pluginBaseBlob)
	pluginBaseAnnotations := buildKlausAnnotations("gs-base", "Base plugin", nil, "", "", "", nil)

	toolchainAnnotations := map[string]string{
		AnnotationName:        "go",
		AnnotationDescription: "Go toolchain",
	}

	ts := newArtifactRegistry(map[string]testArtifactEntry{
		"giantswarm/klaus-plugins/gs-base": {
			configJSON:      pluginBaseJSON,
			configMediaType: MediaTypePluginConfig,
			tags:            []string{"v1.0.0"},
			annotations:     pluginBaseAnnotations,
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
	pluginBlob := pluginConfigBlob{}
	pluginJSON, _ := json.Marshal(pluginBlob)
	pluginAnnotations := buildKlausAnnotations("gs-base", "", nil, "", "", "", nil)

	ts := newArtifactRegistry(map[string]testArtifactEntry{
		"giantswarm/klaus-plugins/gs-base": {
			configJSON:      pluginJSON,
			configMediaType: MediaTypePluginConfig,
			tags:            []string{"v1.0.0"},
			annotations:     pluginAnnotations,
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
		AnnotationName:        "go",
		AnnotationDescription: "Go toolchain",
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
	pluginBlob := pluginConfigBlob{}
	pluginJSON, _ := json.Marshal(pluginBlob)
	pluginAnnotations := buildKlausAnnotations("gs-base", "", nil, "", "", "", nil)

	ts := newArtifactRegistry(map[string]testArtifactEntry{
		"giantswarm/klaus-plugins/gs-base": {
			configJSON:      pluginJSON,
			configMediaType: MediaTypePluginConfig,
			tags:            []string{"v1.0.0"},
			annotations:     pluginAnnotations,
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

func TestResolvePersonalityDeps_AllPluginsMissing(t *testing.T) {
	toolchainAnnotations := map[string]string{
		AnnotationName: "go",
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
		Name: "all-missing",
		Toolchain: ToolchainReference{
			Repository: host + "/giantswarm/klaus-toolchains/go",
			Tag:        "v1.0.0",
		},
		Plugins: []PluginReference{
			{Repository: host + "/giantswarm/klaus-plugins/missing-a", Tag: "v1.0.0"},
			{Repository: host + "/giantswarm/klaus-plugins/missing-b", Tag: "v2.0.0"},
		},
	}

	deps, err := client.ResolvePersonalityDeps(t.Context(), personality)
	if err != nil {
		t.Fatalf("ResolvePersonalityDeps() error = %v", err)
	}

	if deps.Toolchain == nil {
		t.Fatal("Toolchain should not be nil")
	}
	if len(deps.Plugins) != 0 {
		t.Errorf("Plugins = %v, want empty (all missing)", deps.Plugins)
	}
	if len(deps.Warnings) != 2 {
		t.Errorf("Warnings length = %d, want 2: %v", len(deps.Warnings), deps.Warnings)
	}
}

func TestResolvePersonalityDeps_Empty(t *testing.T) {
	ts := newArtifactRegistry(map[string]testArtifactEntry{})
	defer ts.Close()

	client := NewClient(WithPlainHTTP(true))

	personality := Personality{Name: "empty"}

	deps, err := client.ResolvePersonalityDeps(t.Context(), personality)
	if err != nil {
		t.Fatalf("ResolvePersonalityDeps() error = %v", err)
	}

	if deps.Toolchain != nil {
		t.Errorf("Toolchain = %+v, want nil", deps.Toolchain)
	}
	if len(deps.Plugins) != 0 {
		t.Errorf("Plugins = %v, want empty", deps.Plugins)
	}
	if len(deps.Warnings) != 0 {
		t.Errorf("Warnings = %v, want none", deps.Warnings)
	}
}

func TestResolvePersonalityDeps_VersionFromTag(t *testing.T) {
	pluginBlob := pluginConfigBlob{}
	pluginJSON, _ := json.Marshal(pluginBlob)
	pluginAnnotations := buildKlausAnnotations("gs-base", "Base plugin", nil, "", "", "", nil)

	ts := newArtifactRegistry(map[string]testArtifactEntry{
		"giantswarm/klaus-plugins/gs-base": {
			configJSON:      pluginJSON,
			configMediaType: MediaTypePluginConfig,
			tags:            []string{"v2.3.0"},
			annotations:     pluginAnnotations,
		},
	})
	defer ts.Close()
	host := testRegistryHost(ts)

	client := NewClient(WithPlainHTTP(true))

	personality := Personality{
		Name: "version-check",
		Plugins: []PluginReference{
			{Repository: host + "/giantswarm/klaus-plugins/gs-base", Tag: "v2.3.0"},
		},
	}

	deps, err := client.ResolvePersonalityDeps(t.Context(), personality)
	if err != nil {
		t.Fatalf("ResolvePersonalityDeps() error = %v", err)
	}

	if len(deps.Plugins) != 1 {
		t.Fatalf("Plugins length = %d, want 1", len(deps.Plugins))
	}
	if deps.Plugins[0].Plugin.Version != "v2.3.0" {
		t.Errorf("Plugin.Version = %q, want %q (from OCI tag)", deps.Plugins[0].Plugin.Version, "v2.3.0")
	}
	if deps.Plugins[0].ArtifactInfo.Tag != "v2.3.0" {
		t.Errorf("ArtifactInfo.Tag = %q, want %q", deps.Plugins[0].ArtifactInfo.Tag, "v2.3.0")
	}
}

func TestResolvePersonalityDeps_ToolchainVersionFromTag(t *testing.T) {
	toolchainAnnotations := map[string]string{
		AnnotationName:        "go",
		AnnotationDescription: "Go toolchain",
	}

	ts := newArtifactRegistry(map[string]testArtifactEntry{
		"giantswarm/klaus-toolchains/go": {
			configJSON:      []byte(`{}`),
			configMediaType: ocispec.MediaTypeImageConfig,
			tags:            []string{"v1.5.0"},
			annotations:     toolchainAnnotations,
		},
	})
	defer ts.Close()
	host := testRegistryHost(ts)

	client := NewClient(WithPlainHTTP(true))

	personality := Personality{
		Name: "tc-version",
		Toolchain: ToolchainReference{
			Repository: host + "/giantswarm/klaus-toolchains/go",
			Tag:        "v1.5.0",
		},
	}

	deps, err := client.ResolvePersonalityDeps(t.Context(), personality)
	if err != nil {
		t.Fatalf("ResolvePersonalityDeps() error = %v", err)
	}

	if deps.Toolchain == nil {
		t.Fatal("Toolchain is nil, want non-nil")
	}
	if deps.Toolchain.Toolchain.Version != "v1.5.0" {
		t.Errorf("Toolchain.Version = %q, want %q (from OCI tag)", deps.Toolchain.Toolchain.Version, "v1.5.0")
	}
	if deps.Toolchain.Toolchain.Name != "go" {
		t.Errorf("Toolchain.Name = %q, want %q", deps.Toolchain.Toolchain.Name, "go")
	}
}
