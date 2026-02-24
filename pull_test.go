package oci

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestParsePersonalityFromDir(t *testing.T) {
	dir := t.TempDir()

	specYAML := `description: Giant Swarm SRE personality
image: gsoci.azurecr.io/giantswarm/klaus-toolchains/go:v1.0.0
plugins:
  - repository: gsoci.azurecr.io/giantswarm/klaus-plugins/gs-platform
    tag: v1.2.0
`
	if err := os.WriteFile(filepath.Join(dir, "personality.yaml"), []byte(specYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "soul.md"), []byte("You are an SRE."), 0o644); err != nil {
		t.Fatal(err)
	}

	meta := PersonalityMeta{Name: "sre", Version: "1.0.0", Description: "SRE personality"}
	configJSON, err := json.Marshal(meta)
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

	if p.Meta.Name != "sre" {
		t.Errorf("Meta.Name = %q, want %q", p.Meta.Name, "sre")
	}
	if p.Meta.Version != "1.0.0" {
		t.Errorf("Meta.Version = %q, want %q", p.Meta.Version, "1.0.0")
	}
	if p.Spec.Description != "Giant Swarm SRE personality" {
		t.Errorf("Spec.Description = %q", p.Spec.Description)
	}
	if p.Spec.Image != "gsoci.azurecr.io/giantswarm/klaus-toolchains/go:v1.0.0" {
		t.Errorf("Spec.Image = %q", p.Spec.Image)
	}
	if len(p.Spec.Plugins) != 1 {
		t.Fatalf("Spec.Plugins length = %d, want 1", len(p.Spec.Plugins))
	}
	if p.Spec.Plugins[0].Tag != "v1.2.0" {
		t.Errorf("Spec.Plugins[0].Tag = %q, want %q", p.Spec.Plugins[0].Tag, "v1.2.0")
	}
	if p.Soul != "You are an SRE." {
		t.Errorf("Soul = %q, want %q", p.Soul, "You are an SRE.")
	}
	if p.Dir != dir {
		t.Errorf("Dir = %q, want %q", p.Dir, dir)
	}
	if p.Digest != "sha256:abc123" {
		t.Errorf("Digest = %q, want %q", p.Digest, "sha256:abc123")
	}
	if p.Ref != result.Ref {
		t.Errorf("Ref = %q, want %q", p.Ref, result.Ref)
	}
}

func TestParsePersonalityFromDir_CachedNoConfig(t *testing.T) {
	dir := t.TempDir()

	specYAML := `description: cached personality
plugins: []
`
	if err := os.WriteFile(filepath.Join(dir, "personality.yaml"), []byte(specYAML), 0o644); err != nil {
		t.Fatal(err)
	}

	result := &pullResult{
		Digest: "sha256:def456",
		Ref:    "registry/personalities/cached:v1.0.0",
		Cached: true,
	}

	p, err := parsePersonalityFromDir(dir, result.Ref, result)
	if err != nil {
		t.Fatalf("parsePersonalityFromDir() error = %v", err)
	}

	if p.Meta.Name != "" {
		t.Errorf("Meta.Name = %q, want empty on cache hit", p.Meta.Name)
	}
	if p.Spec.Description != "cached personality" {
		t.Errorf("Spec.Description = %q", p.Spec.Description)
	}
	if !p.Cached {
		t.Error("expected Cached = true")
	}
	if p.Soul != "" {
		t.Errorf("Soul = %q, want empty when soul.md missing", p.Soul)
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

	if p.Spec.Description != "" {
		t.Errorf("Spec.Description = %q, want empty", p.Spec.Description)
	}
}

func TestPluginResult(t *testing.T) {
	meta := PluginMeta{
		Name:    "gs-platform",
		Version: "1.0.0",
		Skills:  []string{"kubernetes"},
	}
	configJSON, err := json.Marshal(meta)
	if err != nil {
		t.Fatal(err)
	}

	p := &Plugin{Dir: "/tmp/plugin", Digest: "sha256:abc", Ref: "reg/plugin:v1", Cached: false}
	if err := json.Unmarshal(configJSON, &p.Meta); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if p.Meta.Name != "gs-platform" {
		t.Errorf("Meta.Name = %q, want %q", p.Meta.Name, "gs-platform")
	}
	if len(p.Meta.Skills) != 1 || p.Meta.Skills[0] != "kubernetes" {
		t.Errorf("Meta.Skills = %v, want [kubernetes]", p.Meta.Skills)
	}
}
