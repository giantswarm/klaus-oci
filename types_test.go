package oci

import (
	"encoding/json"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestPluginReference_Ref(t *testing.T) {
	tests := []struct {
		name string
		ref  PluginReference
		want string
	}{
		{
			name: "with tag",
			ref:  PluginReference{Repository: "registry.example.com/plugins/test", Tag: "v1.0.0"},
			want: "registry.example.com/plugins/test:v1.0.0",
		},
		{
			name: "with digest",
			ref:  PluginReference{Repository: "registry.example.com/plugins/test", Digest: "sha256:abc123"},
			want: "registry.example.com/plugins/test@sha256:abc123",
		},
		{
			name: "digest takes precedence over tag",
			ref:  PluginReference{Repository: "registry.example.com/plugins/test", Tag: "v1.0.0", Digest: "sha256:abc123"},
			want: "registry.example.com/plugins/test@sha256:abc123",
		},
		{
			name: "bare repository",
			ref:  PluginReference{Repository: "registry.example.com/plugins/test"},
			want: "registry.example.com/plugins/test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.ref.Ref(); got != tt.want {
				t.Errorf("Ref() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPluginMeta_JSON(t *testing.T) {
	meta := PluginMeta{
		Name:        "test-plugin",
		Version:     "1.0.0",
		Description: "A test plugin",
		Skills:      []string{"kubernetes", "helm"},
		Commands:    []string{"deploy"},
	}

	data, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded PluginMeta
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.Name != meta.Name {
		t.Errorf("Name = %q, want %q", decoded.Name, meta.Name)
	}
	if decoded.Version != meta.Version {
		t.Errorf("Version = %q, want %q", decoded.Version, meta.Version)
	}
	if len(decoded.Skills) != len(meta.Skills) {
		t.Errorf("Skills length = %d, want %d", len(decoded.Skills), len(meta.Skills))
	}
}

func TestPersonalityMeta_JSON(t *testing.T) {
	meta := PersonalityMeta{
		Name:        "sre",
		Version:     "1.0.0",
		Description: "Giant Swarm SRE personality",
	}

	data, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded PersonalityMeta
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.Name != meta.Name {
		t.Errorf("Name = %q, want %q", decoded.Name, meta.Name)
	}
}

func TestPersonalitySpec_YAML(t *testing.T) {
	input := `
description: Giant Swarm SRE personality
image: gsoci.azurecr.io/giantswarm/klaus-go:1.0.0
plugins:
  - repository: gsoci.azurecr.io/giantswarm/klaus-plugins/gs-platform
    tag: v1.2.0
  - repository: gsoci.azurecr.io/giantswarm/klaus-plugins/kubernetes
    tag: v1.0.0
`

	var spec PersonalitySpec
	if err := yaml.Unmarshal([]byte(input), &spec); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if spec.Description != "Giant Swarm SRE personality" {
		t.Errorf("Description = %q, want %q", spec.Description, "Giant Swarm SRE personality")
	}
	if spec.Image != "gsoci.azurecr.io/giantswarm/klaus-go:1.0.0" {
		t.Errorf("Image = %q", spec.Image)
	}
	if len(spec.Plugins) != 2 {
		t.Fatalf("Plugins length = %d, want 2", len(spec.Plugins))
	}
	if spec.Plugins[0].Repository != "gsoci.azurecr.io/giantswarm/klaus-plugins/gs-platform" {
		t.Errorf("Plugins[0].Repository = %q", spec.Plugins[0].Repository)
	}
	if spec.Plugins[0].Tag != "v1.2.0" {
		t.Errorf("Plugins[0].Tag = %q", spec.Plugins[0].Tag)
	}
}
