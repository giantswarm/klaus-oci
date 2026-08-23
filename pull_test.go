package oci

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestParsePersonalityFromDir(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "SOUL.md"), []byte("You are an SRE."), 0o600); err != nil {
		t.Fatal(err)
	}

	blob := personalityConfigBlob{
		Toolchain: ToolchainReference{
			Repository: testRefToolchainGo,
			Tag:        testTagV100,
		},
		Plugins: []PluginReference{
			{Repository: "gsoci.azurecr.io/giantswarm/klaus-plugins/gs-platform", Tag: testTagV120},
		},
	}
	configJSON, err := json.Marshal(blob)
	if err != nil {
		t.Fatal(err)
	}

	annotations := map[string]string{
		AnnotationName:        testNameSRE,
		AnnotationDescription: testDescSREPersonality,
	}

	result := &pullResult{
		Digest:      testDigestAbc123,
		Ref:         "registry/personalities/sre:v1.0.0",
		ConfigJSON:  configJSON,
		Annotations: annotations,
	}

	p, err := parsePersonalityFromDir(dir, result.Ref, result)
	if err != nil {
		t.Fatalf("parsePersonalityFromDir() error = %v", err)
	}

	if p.Name != testNameSRE {
		t.Errorf("Name = %q, want %q", p.Name, testNameSRE)
	}
	if p.Version != testTagV100 {
		t.Errorf("Version = %q, want %q", p.Version, testTagV100)
	}
	if p.Description != testDescSREPersonality {
		t.Errorf("Description = %q", p.Description)
	}
	if p.Toolchain.Repository != testRefToolchainGo {
		t.Errorf("Toolchain.Repository = %q", p.Toolchain.Repository)
	}
	if len(p.Plugins) != 1 {
		t.Fatalf("Plugins length = %d, want 1", len(p.Plugins))
	}
	if p.Personality.Plugins[0].Tag != testTagV120 {
		t.Errorf("Plugins[0].Tag = %q, want %q", p.Personality.Plugins[0].Tag, testTagV120)
	}
	if p.Soul != "You are an SRE." {
		t.Errorf("Soul = %q, want %q", p.Soul, "You are an SRE.")
	}
	if p.Dir != dir {
		t.Errorf("Dir = %q, want %q", p.Dir, dir)
	}
	if p.Digest != testDigestAbc123 {
		t.Errorf("Digest = %q, want %q", p.Digest, testDigestAbc123)
	}
	if p.Ref != result.Ref {
		t.Errorf("Ref = %q, want %q", p.Ref, result.Ref)
	}
	if p.Tag != testTagV100 {
		t.Errorf("Tag = %q, want %q", p.Tag, testTagV100)
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
		Digest:      testDigestDef456,
		Ref:         "registry/personalities/cached:v1.0.0",
		Cached:      true,
		ConfigJSON:  configJSON,
		Annotations: annotations,
	}

	p, err := parsePersonalityFromDir(dir, result.Ref, result)
	if err != nil {
		t.Fatalf("parsePersonalityFromDir() error = %v", err)
	}

	if p.Name != "cached" {
		t.Errorf("Name = %q, want %q", p.Name, "cached")
	}
	if p.Description != "cached personality" {
		t.Errorf("Description = %q", p.Description)
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

	if p.Description != "" {
		t.Errorf("Description = %q, want empty", p.Description)
	}
}

func TestPluginFromAnnotations(t *testing.T) {
	annotations := map[string]string{
		AnnotationName: testNameGSPlatform,
	}
	blob := pluginConfigBlob{
		Skills: []string{testKeywordKubernetes},
	}

	plugin := pluginFromAnnotations(annotations, "v1", blob)

	if plugin.Name != testNameGSPlatform {
		t.Errorf("Name = %q, want %q", plugin.Name, testNameGSPlatform)
	}
	if plugin.Version != "v1" {
		t.Errorf("Version = %q, want %q", plugin.Version, "v1")
	}
	if len(plugin.Skills) != 1 || plugin.Skills[0] != testKeywordKubernetes {
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
		AnnotationKeywords:    testNameTest,
	}
	blob := pluginConfigBlob{
		Skills:     []string{testTagAlpha, testTagBeta},
		Commands:   []string{"cmd-a", "cmd-b"},
		Agents:     []string{"agent-x"},
		HasHooks:   true,
		MCPServers: []string{"mcp-one"},
		LSPServers: []string{"lsp-one"},
	}

	plugin := pluginFromAnnotations(annotations, testTagV200, blob)

	if plugin.Name != "full-plugin" {
		t.Errorf("Name = %q", plugin.Name)
	}
	if plugin.Version != testTagV200 {
		t.Errorf("Version = %q, want %q", plugin.Version, testTagV200)
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

	if p.Name != "" {
		t.Errorf("Name = %q, want empty (nil ConfigJSON)", p.Name)
	}
	if p.Version != testTagV100 {
		t.Errorf("Version = %q, want %q (from ref tag)", p.Version, testTagV100)
	}
}

func TestParsePersonalityFromDir_WithFullMetadata(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "SOUL.md"), []byte("# SRE Personality\n\nYou are an SRE expert."), 0o600); err != nil {
		t.Fatal(err)
	}

	blob := personalityConfigBlob{
		Toolchain: ToolchainReference{
			Repository: testRefToolchainGo,
			Tag:        testTagV120,
		},
		Plugins: []PluginReference{
			{Repository: testRefPluginGSBase, Tag: testTagV010},
			{Repository: testRefPluginGSSRE, Tag: testTagV020},
		},
	}
	configJSON, err := json.Marshal(blob)
	if err != nil {
		t.Fatal(err)
	}

	annotations := map[string]string{
		AnnotationName:        testNameSRE,
		AnnotationDescription: testDescSREPersonality,
		AnnotationAuthorName:  testAuthorGiantSwarmGmbH,
		AnnotationRepository:  testSourcePersonalities,
		AnnotationLicense:     testLicenseApache2,
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

	if p.Name != testNameSRE {
		t.Errorf("Name = %q", p.Name)
	}
	if p.Version != testTagV200 {
		t.Errorf("Version = %q, want %q", p.Version, testTagV200)
	}
	if p.Author == nil || p.Author.Name != testAuthorGiantSwarmGmbH {
		t.Errorf("Author = %+v", p.Author)
	}
	if p.License != testLicenseApache2 {
		t.Errorf("License = %q", p.License)
	}
	if len(p.Keywords) != 2 {
		t.Errorf("Keywords = %v", p.Keywords)
	}
	if p.Toolchain.Repository != testRefToolchainGo {
		t.Errorf("Toolchain.Repository = %q", p.Toolchain.Repository)
	}
	if p.Toolchain.Tag != testTagV120 {
		t.Errorf("Toolchain.Tag = %q", p.Toolchain.Tag)
	}
	if len(p.Plugins) != 2 {
		t.Fatalf("Plugins length = %d, want 2", len(p.Plugins))
	}
	if p.Soul != "# SRE Personality\n\nYou are an SRE expert." {
		t.Errorf("Soul = %q", p.Soul)
	}
	if p.Dir != dir {
		t.Errorf("Dir = %q, want %q", p.Dir, dir)
	}
}
