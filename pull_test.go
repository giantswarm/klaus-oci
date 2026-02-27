package oci

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestParsePersonalityFromDir(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "SOUL.md"), []byte("You are an SRE."), 0o644); err != nil {
		t.Fatal(err)
	}

	blob := personalityConfigBlob{
		Toolchain: ToolchainReference{
			Repository: "gsoci.azurecr.io/giantswarm/klaus-toolchains/go",
			Tag:        "v1.0.0",
		},
		Plugins: []PluginReference{
			{Repository: "gsoci.azurecr.io/giantswarm/klaus-plugins/gs-platform", Tag: "v1.2.0"},
		},
	}
	configJSON, err := json.Marshal(blob)
	if err != nil {
		t.Fatal(err)
	}

	annotations := map[string]string{
		AnnotationName:        "sre",
		AnnotationDescription: "SRE personality",
	}

	result := &pullResult{
		Digest:      "sha256:abc123",
		Ref:         "registry/personalities/sre:v1.0.0",
		ConfigJSON:  configJSON,
		Annotations: annotations,
	}

	p, err := parsePersonalityFromDir(dir, result.Ref, result)
	if err != nil {
		t.Fatalf("parsePersonalityFromDir() error = %v", err)
	}

	if p.Personality.Name != "sre" {
		t.Errorf("Name = %q, want %q", p.Personality.Name, "sre")
	}
	if p.Personality.Version != "v1.0.0" {
		t.Errorf("Version = %q, want %q", p.Personality.Version, "v1.0.0")
	}
	if p.Personality.Description != "SRE personality" {
		t.Errorf("Description = %q", p.Personality.Description)
	}
	if p.Personality.Toolchain.Repository != "gsoci.azurecr.io/giantswarm/klaus-toolchains/go" {
		t.Errorf("Toolchain.Repository = %q", p.Personality.Toolchain.Repository)
	}
	if len(p.Personality.Plugins) != 1 {
		t.Fatalf("Plugins length = %d, want 1", len(p.Personality.Plugins))
	}
	if p.Personality.Plugins[0].Tag != "v1.2.0" {
		t.Errorf("Plugins[0].Tag = %q, want %q", p.Personality.Plugins[0].Tag, "v1.2.0")
	}
	if p.Soul != "You are an SRE." {
		t.Errorf("Soul = %q, want %q", p.Soul, "You are an SRE.")
	}
	if p.Dir != dir {
		t.Errorf("Dir = %q, want %q", p.Dir, dir)
	}
	if p.ArtifactInfo.Digest != "sha256:abc123" {
		t.Errorf("Digest = %q, want %q", p.ArtifactInfo.Digest, "sha256:abc123")
	}
	if p.ArtifactInfo.Ref != result.Ref {
		t.Errorf("Ref = %q, want %q", p.ArtifactInfo.Ref, result.Ref)
	}
	if p.ArtifactInfo.Tag != "v1.0.0" {
		t.Errorf("Tag = %q, want %q", p.ArtifactInfo.Tag, "v1.0.0")
	}
}

func TestParsePersonalityFromDir_CachedWithConfig(t *testing.T) {
	dir := t.TempDir()

	blob := personalityConfigBlob{}
	configJSON, err := json.Marshal(blob)
	if err != nil {
		t.Fatal(err)
	}

	annotations := map[string]string{
		AnnotationName:        "cached",
		AnnotationDescription: "cached personality",
	}

	result := &pullResult{
		Digest:      "sha256:def456",
		Ref:         "registry/personalities/cached:v1.0.0",
		Cached:      true,
		ConfigJSON:  configJSON,
		Annotations: annotations,
	}

	p, err := parsePersonalityFromDir(dir, result.Ref, result)
	if err != nil {
		t.Fatalf("parsePersonalityFromDir() error = %v", err)
	}

	if p.Personality.Name != "cached" {
		t.Errorf("Name = %q, want %q", p.Personality.Name, "cached")
	}
	if p.Personality.Description != "cached personality" {
		t.Errorf("Description = %q", p.Personality.Description)
	}
	if !p.Cached {
		t.Error("expected Cached = true")
	}
	if p.Soul != "" {
		t.Errorf("Soul = %q, want empty when SOUL.md missing", p.Soul)
	}
}

func TestParsePersonalityFromDir_NoFiles(t *testing.T) {
	dir := t.TempDir()

	result := &pullResult{
		Digest: "sha256:empty",
		Ref:    "registry/personalities/empty:v1.0.0",
	}

	p, err := parsePersonalityFromDir(dir, result.Ref, result)
	if err != nil {
		t.Fatalf("parsePersonalityFromDir() error = %v", err)
	}

	if p.Personality.Description != "" {
		t.Errorf("Description = %q, want empty", p.Personality.Description)
	}
}

func TestPluginFromAnnotations(t *testing.T) {
	annotations := map[string]string{
		AnnotationName: "gs-platform",
	}
	blob := pluginConfigBlob{
		Skills: []string{"kubernetes"},
	}

	plugin := pluginFromAnnotations(annotations, "v1", blob)

	if plugin.Name != "gs-platform" {
		t.Errorf("Name = %q, want %q", plugin.Name, "gs-platform")
	}
	if plugin.Version != "v1" {
		t.Errorf("Version = %q, want %q", plugin.Version, "v1")
	}
	if len(plugin.Skills) != 1 || plugin.Skills[0] != "kubernetes" {
		t.Errorf("Skills = %v, want [kubernetes]", plugin.Skills)
	}
}

func TestPluginFromAnnotations_Full(t *testing.T) {
	annotations := map[string]string{
		AnnotationName:        "full-plugin",
		AnnotationDescription: "A full-featured plugin",
		AnnotationAuthorName:  "Test",
		AnnotationAuthorEmail: "test@test.com",
		AnnotationRepository:  "https://github.com/test/repo",
		AnnotationLicense:     "MIT",
		AnnotationKeywords:    "test",
	}
	blob := pluginConfigBlob{
		Skills:     []string{"alpha", "beta"},
		Commands:   []string{"cmd-a", "cmd-b"},
		Agents:     []string{"agent-x"},
		HasHooks:   true,
		MCPServers: []string{"mcp-one"},
		LSPServers: []string{"lsp-one"},
	}

	plugin := pluginFromAnnotations(annotations, "v2.0.0", blob)

	if plugin.Name != "full-plugin" {
		t.Errorf("Name = %q", plugin.Name)
	}
	if plugin.Version != "v2.0.0" {
		t.Errorf("Version = %q, want %q", plugin.Version, "v2.0.0")
	}
	if plugin.Author == nil || plugin.Author.Email != "test@test.com" {
		t.Errorf("Author = %+v", plugin.Author)
	}
	if !plugin.HasHooks {
		t.Error("HasHooks = false, want true")
	}
	if len(plugin.MCPServers) != 1 {
		t.Errorf("MCPServers = %v", plugin.MCPServers)
	}
	if len(plugin.LSPServers) != 1 {
		t.Errorf("LSPServers = %v", plugin.LSPServers)
	}
	if len(plugin.Agents) != 1 {
		t.Errorf("Agents = %v", plugin.Agents)
	}
}

func TestParsePersonalityFromDir_NilConfigJSON(t *testing.T) {
	dir := t.TempDir()

	result := &pullResult{
		Digest:     "sha256:nil-config",
		Ref:        "registry/personalities/no-config:v1.0.0",
		ConfigJSON: nil,
	}

	p, err := parsePersonalityFromDir(dir, result.Ref, result)
	if err != nil {
		t.Fatalf("parsePersonalityFromDir() error = %v", err)
	}

	if p.Personality.Name != "" {
		t.Errorf("Name = %q, want empty (nil ConfigJSON)", p.Personality.Name)
	}
	if p.Personality.Version != "v1.0.0" {
		t.Errorf("Version = %q, want %q (from ref tag)", p.Personality.Version, "v1.0.0")
	}
}

func TestParsePersonalityFromDir_WithFullMetadata(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "SOUL.md"), []byte("# SRE Personality\n\nYou are an SRE expert."), 0o644); err != nil {
		t.Fatal(err)
	}

	blob := personalityConfigBlob{
		Toolchain: ToolchainReference{
			Repository: "gsoci.azurecr.io/giantswarm/klaus-toolchains/go",
			Tag:        "v1.2.0",
		},
		Plugins: []PluginReference{
			{Repository: "gsoci.azurecr.io/giantswarm/klaus-plugins/gs-base", Tag: "v0.1.0"},
			{Repository: "gsoci.azurecr.io/giantswarm/klaus-plugins/gs-sre", Tag: "v0.2.0"},
		},
	}
	configJSON, err := json.Marshal(blob)
	if err != nil {
		t.Fatal(err)
	}

	annotations := map[string]string{
		AnnotationName:        "sre",
		AnnotationDescription: "SRE personality",
		AnnotationAuthorName:  "Giant Swarm GmbH",
		AnnotationRepository:  "https://github.com/giantswarm/klaus-personalities",
		AnnotationLicense:     "Apache-2.0",
		AnnotationKeywords:    "giantswarm,sre",
	}

	result := &pullResult{
		Digest:      "sha256:full-meta",
		Ref:         "registry/personalities/sre:v2.0.0",
		ConfigJSON:  configJSON,
		Annotations: annotations,
	}

	p, err := parsePersonalityFromDir(dir, result.Ref, result)
	if err != nil {
		t.Fatalf("parsePersonalityFromDir() error = %v", err)
	}

	if p.Personality.Name != "sre" {
		t.Errorf("Name = %q", p.Personality.Name)
	}
	if p.Personality.Version != "v2.0.0" {
		t.Errorf("Version = %q, want %q", p.Personality.Version, "v2.0.0")
	}
	if p.Personality.Author == nil || p.Personality.Author.Name != "Giant Swarm GmbH" {
		t.Errorf("Author = %+v", p.Personality.Author)
	}
	if p.Personality.License != "Apache-2.0" {
		t.Errorf("License = %q", p.Personality.License)
	}
	if len(p.Personality.Keywords) != 2 {
		t.Errorf("Keywords = %v", p.Personality.Keywords)
	}
	if p.Personality.Toolchain.Repository != "gsoci.azurecr.io/giantswarm/klaus-toolchains/go" {
		t.Errorf("Toolchain.Repository = %q", p.Personality.Toolchain.Repository)
	}
	if p.Personality.Toolchain.Tag != "v1.2.0" {
		t.Errorf("Toolchain.Tag = %q", p.Personality.Toolchain.Tag)
	}
	if len(p.Personality.Plugins) != 2 {
		t.Fatalf("Plugins length = %d, want 2", len(p.Personality.Plugins))
	}
	if p.Soul != "# SRE Personality\n\nYou are an SRE expert." {
		t.Errorf("Soul = %q", p.Soul)
	}
	if p.Dir != dir {
		t.Errorf("Dir = %q, want %q", p.Dir, dir)
	}
}
