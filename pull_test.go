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

	personality := Personality{
		Name:        "sre",
		Description: "SRE personality",
		Toolchain: ToolchainReference{
			Repository: "gsoci.azurecr.io/giantswarm/klaus-toolchains/go",
			Tag:        "v1.0.0",
		},
		Plugins: []PluginReference{
			{Repository: "gsoci.azurecr.io/giantswarm/klaus-plugins/gs-platform", Tag: "v1.2.0"},
		},
	}
	configJSON, err := json.Marshal(personality)
	if err != nil {
		t.Fatal(err)
	}

	result := &pullResult{
		Digest:     "sha256:abc123",
		Ref:        "registry/personalities/sre:v1.0.0",
		ConfigJSON: configJSON,
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

	personality := Personality{
		Name:        "cached",
		Description: "cached personality",
	}
	configJSON, err := json.Marshal(personality)
	if err != nil {
		t.Fatal(err)
	}

	result := &pullResult{
		Digest:     "sha256:def456",
		Ref:        "registry/personalities/cached:v1.0.0",
		Cached:     true,
		ConfigJSON: configJSON,
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

func TestPluginUnmarshal(t *testing.T) {
	plugin := Plugin{
		Name:   "gs-platform",
		Skills: []string{"kubernetes"},
	}
	configJSON, err := json.Marshal(plugin)
	if err != nil {
		t.Fatal(err)
	}

	p := &PulledPlugin{
		ArtifactInfo: ArtifactInfo{Ref: "reg/plugin:v1", Tag: "v1", Digest: "sha256:abc"},
		Dir:          "/tmp/plugin",
	}
	if err := json.Unmarshal(configJSON, &p.Plugin); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if p.Plugin.Name != "gs-platform" {
		t.Errorf("Name = %q, want %q", p.Plugin.Name, "gs-platform")
	}
	if len(p.Plugin.Skills) != 1 || p.Plugin.Skills[0] != "kubernetes" {
		t.Errorf("Skills = %v, want [kubernetes]", p.Plugin.Skills)
	}
}
