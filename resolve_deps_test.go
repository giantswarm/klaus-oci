package oci

import (
	"encoding/json"
	"slices"
	"testing"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

func TestResolvePersonalityDeps(t *testing.T) {
	pluginBaseBlob := pluginConfigBlob{Skills: []string{testKeywordKubernetes, testNameFluxCD}}
	pluginBaseJSON, _ := json.Marshal(pluginBaseBlob)
	pluginBaseAnnotations := buildKlausAnnotations(commonMetadata{
		Name:        testNameGSBase,
		Description: testDescBasePlugin,
		Author:      &Author{Name: testAuthorGiantSwarmGmbH},
	})

	pluginSREBlob := pluginConfigBlob{Commands: []string{"check-cluster"}}
	pluginSREJSON, _ := json.Marshal(pluginSREBlob)
	pluginSREAnnotations := buildKlausAnnotations(commonMetadata{
		Name:        "gs-sre",
		Description: "SRE plugin",
	})

	toolchainAnnotations := map[string]string{
		AnnotationName:        "go",
		AnnotationDescription: testDescGoToolchainKlaus,
		AnnotationAuthorName:  testAuthorGiantSwarmGmbH,
	}

	ts := newArtifactRegistry(map[string]testArtifactEntry{
		testRepoPluginGSBase: {
			configJSON:      pluginBaseJSON,
			configMediaType: MediaTypePluginConfig,
			tags:            []string{testTagV100},
			annotations:     pluginBaseAnnotations,
		},
		"giantswarm/klaus-plugins/gs-sre": {
			configJSON:      pluginSREJSON,
			configMediaType: MediaTypePluginConfig,
			tags:            []string{testTagV050},
			annotations:     pluginSREAnnotations,
		},
		testRepoToolchainGo: {
			configJSON:      []byte(`{"architecture":"amd64"}`),
			configMediaType: ocispec.MediaTypeImageConfig,
			tags:            []string{testTagV120},
			annotations:     toolchainAnnotations,
		},
	})
	defer ts.Close()
	host := testRegistryHost(ts)

	client := NewClient(WithPlainHTTP(true))

	personality := Personality{
		Name: testNameSRE,
		Toolchain: ToolchainReference{
			Repository: host + "/giantswarm/klaus-toolchains/go",
			Tag:        testTagV120,
		},
		Plugins: []PluginReference{
			{Repository: host + "/giantswarm/klaus-plugins/gs-base", Tag: testTagV100},
			{Repository: host + "/giantswarm/klaus-plugins/gs-sre", Tag: testTagV050},
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
	if deps.Toolchain.Name != "go" {
		t.Errorf("Toolchain.Name = %q, want %q", deps.Toolchain.Name, "go")
	}
	if deps.Toolchain.Version != testTagV120 {
		t.Errorf("Toolchain.Version = %q, want %q", deps.Toolchain.Version, testTagV120)
	}
	if deps.Toolchain.Description != testDescGoToolchainKlaus {
		t.Errorf("Toolchain.Description = %q", deps.Toolchain.Description)
	}

	if len(deps.Plugins) != 2 {
		t.Fatalf("Plugins length = %d, want 2", len(deps.Plugins))
	}

	names := []string{deps.Plugins[0].Name, deps.Plugins[1].Name}
	slices.Sort(names)
	if names[0] != testNameGSBase || names[1] != "gs-sre" {
		t.Errorf("Plugin names = %v, want [gs-base gs-sre]", names)
	}

	for _, dp := range deps.Plugins {
		if dp.Digest == "" {
			t.Errorf("Plugin %q: Digest is empty", dp.Name)
		}
		if dp.Tag == "" {
			t.Errorf("Plugin %q: Tag is empty", dp.Name)
		}
	}
}

func TestResolvePersonalityDeps_MissingPlugin(t *testing.T) {
	pluginBaseBlob := pluginConfigBlob{}
	pluginBaseJSON, _ := json.Marshal(pluginBaseBlob)
	pluginBaseAnnotations := buildKlausAnnotations(commonMetadata{
		Name:        testNameGSBase,
		Description: testDescBasePlugin,
	})

	toolchainAnnotations := map[string]string{
		AnnotationName:        "go",
		AnnotationDescription: testDescGoToolchain,
	}

	ts := newArtifactRegistry(map[string]testArtifactEntry{
		testRepoPluginGSBase: {
			configJSON:      pluginBaseJSON,
			configMediaType: MediaTypePluginConfig,
			tags:            []string{testTagV100},
			annotations:     pluginBaseAnnotations,
		},
		testRepoToolchainGo: {
			configJSON:      []byte(`{}`),
			configMediaType: ocispec.MediaTypeImageConfig,
			tags:            []string{testTagV100},
			annotations:     toolchainAnnotations,
		},
	})
	defer ts.Close()
	host := testRegistryHost(ts)

	client := NewClient(WithPlainHTTP(true))

	personality := Personality{
		Name: testNameSRE,
		Toolchain: ToolchainReference{
			Repository: host + "/giantswarm/klaus-toolchains/go",
			Tag:        testTagV100,
		},
		Plugins: []PluginReference{
			{Repository: host + "/giantswarm/klaus-plugins/gs-base", Tag: testTagV100},
			{Repository: host + "/giantswarm/klaus-plugins/gs-missing", Tag: testTagV100},
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
	if deps.Plugins[0].Name != testNameGSBase {
		t.Errorf("Plugin.Name = %q, want %q", deps.Plugins[0].Name, testNameGSBase)
	}

	if deps.Toolchain == nil {
		t.Error("Toolchain should not be nil")
	}
}

func TestResolvePersonalityDeps_MissingToolchain(t *testing.T) {
	pluginBlob := pluginConfigBlob{}
	pluginJSON, _ := json.Marshal(pluginBlob)
	pluginAnnotations := buildKlausAnnotations(commonMetadata{Name: testNameGSBase})

	ts := newArtifactRegistry(map[string]testArtifactEntry{
		testRepoPluginGSBase: {
			configJSON:      pluginJSON,
			configMediaType: MediaTypePluginConfig,
			tags:            []string{testTagV100},
			annotations:     pluginAnnotations,
		},
	})
	defer ts.Close()
	host := testRegistryHost(ts)

	client := NewClient(WithPlainHTTP(true))

	personality := Personality{
		Name: testNameSRE,
		Toolchain: ToolchainReference{
			Repository: host + "/giantswarm/klaus-toolchains/missing",
			Tag:        testTagV100,
		},
		Plugins: []PluginReference{
			{Repository: host + "/giantswarm/klaus-plugins/gs-base", Tag: testTagV100},
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
	if deps.Plugins[0].Name != testNameGSBase {
		t.Errorf("Plugin.Name = %q, want %q", deps.Plugins[0].Name, testNameGSBase)
	}
}

func TestResolvePersonalityDeps_NoPlugins(t *testing.T) {
	toolchainAnnotations := map[string]string{
		AnnotationName:        "go",
		AnnotationDescription: testDescGoToolchain,
	}

	ts := newArtifactRegistry(map[string]testArtifactEntry{
		testRepoToolchainGo: {
			configJSON:      []byte(`{}`),
			configMediaType: ocispec.MediaTypeImageConfig,
			tags:            []string{testTagV100},
			annotations:     toolchainAnnotations,
		},
	})
	defer ts.Close()
	host := testRegistryHost(ts)

	client := NewClient(WithPlainHTTP(true))

	personality := Personality{
		Name: testNameMinimal,
		Toolchain: ToolchainReference{
			Repository: host + "/giantswarm/klaus-toolchains/go",
			Tag:        testTagV100,
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
	if deps.Toolchain.Name != "go" {
		t.Errorf("Toolchain.Name = %q, want %q", deps.Toolchain.Name, "go")
	}
	if len(deps.Plugins) != 0 {
		t.Errorf("Plugins = %v, want empty", deps.Plugins)
	}
}

func TestResolvePersonalityDeps_EmptyToolchain(t *testing.T) {
	pluginBlob := pluginConfigBlob{}
	pluginJSON, _ := json.Marshal(pluginBlob)
	pluginAnnotations := buildKlausAnnotations(commonMetadata{Name: testNameGSBase})

	ts := newArtifactRegistry(map[string]testArtifactEntry{
		testRepoPluginGSBase: {
			configJSON:      pluginJSON,
			configMediaType: MediaTypePluginConfig,
			tags:            []string{testTagV100},
			annotations:     pluginAnnotations,
		},
	})
	defer ts.Close()
	host := testRegistryHost(ts)

	client := NewClient(WithPlainHTTP(true))

	personality := Personality{
		Name: "no-toolchain",
		Plugins: []PluginReference{
			{Repository: host + "/giantswarm/klaus-plugins/gs-base", Tag: testTagV100},
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
		testRepoToolchainGo: {
			configJSON:      []byte(`{}`),
			configMediaType: ocispec.MediaTypeImageConfig,
			tags:            []string{testTagV100},
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
			Tag:        testTagV100,
		},
		Plugins: []PluginReference{
			{Repository: host + "/giantswarm/klaus-plugins/missing-a", Tag: testTagV100},
			{Repository: host + "/giantswarm/klaus-plugins/missing-b", Tag: testTagV200},
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
	pluginAnnotations := buildKlausAnnotations(commonMetadata{
		Name:        testNameGSBase,
		Description: testDescBasePlugin,
	})

	ts := newArtifactRegistry(map[string]testArtifactEntry{
		testRepoPluginGSBase: {
			configJSON:      pluginJSON,
			configMediaType: MediaTypePluginConfig,
			tags:            []string{testTagV230},
			annotations:     pluginAnnotations,
		},
	})
	defer ts.Close()
	host := testRegistryHost(ts)

	client := NewClient(WithPlainHTTP(true))

	personality := Personality{
		Name: "version-check",
		Plugins: []PluginReference{
			{Repository: host + "/giantswarm/klaus-plugins/gs-base", Tag: testTagV230},
		},
	}

	deps, err := client.ResolvePersonalityDeps(t.Context(), personality)
	if err != nil {
		t.Fatalf("ResolvePersonalityDeps() error = %v", err)
	}

	if len(deps.Plugins) != 1 {
		t.Fatalf("Plugins length = %d, want 1", len(deps.Plugins))
	}
	if deps.Plugins[0].Version != testTagV230 {
		t.Errorf("Plugin.Version = %q, want %q (from OCI tag)", deps.Plugins[0].Version, testTagV230)
	}
	if deps.Plugins[0].Tag != testTagV230 {
		t.Errorf("ArtifactInfo.Tag = %q, want %q", deps.Plugins[0].Tag, testTagV230)
	}
}

func TestResolvePersonalityDeps_ToolchainVersionFromTag(t *testing.T) {
	toolchainAnnotations := map[string]string{
		AnnotationName:        "go",
		AnnotationDescription: testDescGoToolchain,
	}

	ts := newArtifactRegistry(map[string]testArtifactEntry{
		testRepoToolchainGo: {
			configJSON:      []byte(`{}`),
			configMediaType: ocispec.MediaTypeImageConfig,
			tags:            []string{testTagV150},
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
			Tag:        testTagV150,
		},
	}

	deps, err := client.ResolvePersonalityDeps(t.Context(), personality)
	if err != nil {
		t.Fatalf("ResolvePersonalityDeps() error = %v", err)
	}

	if deps.Toolchain == nil {
		t.Fatal("Toolchain is nil, want non-nil")
	}
	if deps.Toolchain.Version != testTagV150 {
		t.Errorf("Toolchain.Version = %q, want %q (from OCI tag)", deps.Toolchain.Version, testTagV150)
	}
	if deps.Toolchain.Name != "go" {
		t.Errorf("Toolchain.Name = %q, want %q", deps.Toolchain.Name, "go")
	}
}
