package oci

import (
	"encoding/json"
	"testing"
)

func TestPluginConfigBlob_ExcludesCommonMetadata(t *testing.T) {
	p := Plugin{
		Name:        "gs-base",
		Version:     "v1.0.0",
		Description: "A base plugin",
		Author:      &Author{Name: "Giant Swarm", Email: "dev@giantswarm.io"},
		Homepage:    "https://giantswarm.io",
		SourceRepo:  "https://github.com/giantswarm/gs-base",
		License:     "Apache-2.0",
		Keywords:    []string{"platform", "base"},
		Skills:      []string{"kubernetes", "fluxcd"},
		Commands:    []string{"init", "deploy"},
		Agents:      []string{"code-reviewer"},
		HasHooks:    true,
		MCPServers:  []string{"github"},
		LSPServers:  []string{"gopls"},
	}

	blob := pluginConfigBlob{
		Skills:     p.Skills,
		Commands:   p.Commands,
		Agents:     p.Agents,
		HasHooks:   p.HasHooks,
		MCPServers: p.MCPServers,
		LSPServers: p.LSPServers,
	}

	data, err := json.Marshal(blob)
	if err != nil {
		t.Fatalf("json.Marshal(pluginConfigBlob) error = %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("json.Unmarshal error = %v", err)
	}

	for _, forbidden := range []string{"name", "description", "author", "homepage", "repository", "license", "keywords"} {
		if _, ok := raw[forbidden]; ok {
			t.Errorf("config blob should not contain %q, but it does", forbidden)
		}
	}

	for _, expected := range []string{"skills", "commands", "agents", "hasHooks", "mcpServers", "lspServers"} {
		if _, ok := raw[expected]; !ok {
			t.Errorf("config blob should contain %q, but it does not", expected)
		}
	}
}

func TestPluginConfigBlob_EmptyComponents(t *testing.T) {
	blob := pluginConfigBlob{}
	data, err := json.Marshal(blob)
	if err != nil {
		t.Fatalf("json.Marshal error = %v", err)
	}
	if string(data) != "{}" {
		t.Errorf("empty pluginConfigBlob = %s, want {}", data)
	}
}

func TestPersonalityConfigBlob_ExcludesCommonMetadata(t *testing.T) {
	p := Personality{
		Name:        "sre",
		Description: "SRE personality",
		Author:      &Author{Name: "Giant Swarm"},
		Homepage:    "https://giantswarm.io",
		SourceRepo:  "https://github.com/giantswarm/sre",
		License:     "Apache-2.0",
		Keywords:    []string{"sre", "ops"},
		Toolchain: ToolchainReference{
			Repository: "gsoci.azurecr.io/giantswarm/klaus-toolchains/go",
			Tag:        "v1.0.0",
		},
		Plugins: []PluginReference{
			{Repository: "gsoci.azurecr.io/giantswarm/klaus-plugins/gs-base", Tag: "v1.0.0"},
		},
	}

	blob := personalityConfigBlob{
		Toolchain: p.Toolchain,
		Plugins:   p.Plugins,
	}

	data, err := json.Marshal(blob)
	if err != nil {
		t.Fatalf("json.Marshal(personalityConfigBlob) error = %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("json.Unmarshal error = %v", err)
	}

	for _, forbidden := range []string{"name", "description", "author", "homepage", "repository", "license", "keywords"} {
		if _, ok := raw[forbidden]; ok {
			t.Errorf("config blob should not contain %q, but it does", forbidden)
		}
	}

	if _, ok := raw["toolchain"]; !ok {
		t.Error("config blob should contain toolchain")
	}
	if _, ok := raw["plugins"]; !ok {
		t.Error("config blob should contain plugins")
	}
}

func TestPersonalityConfigBlob_EmptyComposition(t *testing.T) {
	blob := personalityConfigBlob{}
	data, err := json.Marshal(blob)
	if err != nil {
		t.Fatalf("json.Marshal error = %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("json.Unmarshal error = %v", err)
	}

	for _, forbidden := range []string{"name", "description", "author", "homepage", "license", "keywords"} {
		if _, ok := raw[forbidden]; ok {
			t.Errorf("empty config blob should not contain %q", forbidden)
		}
	}

	if _, ok := raw["plugins"]; ok {
		t.Error("empty config blob should omit nil plugins")
	}
}

func TestPushPlugin_AnnotationsFromMetadata(t *testing.T) {
	p := Plugin{
		Name:        "gs-base",
		Description: "Giant Swarm base plugin",
		Author:      &Author{Name: "Giant Swarm", Email: "dev@giantswarm.io", URL: "https://giantswarm.io"},
		Homepage:    "https://giantswarm.io/plugins/gs-base",
		SourceRepo:  "https://github.com/giantswarm/gs-base",
		License:     "Apache-2.0",
		Keywords:    []string{"platform", "base"},
		Skills:      []string{"kubernetes"},
	}

	annotations := buildKlausAnnotations(p.klausMetadata())

	expected := map[string]string{
		AnnotationName:        "gs-base",
		AnnotationDescription: "Giant Swarm base plugin",
		AnnotationAuthorName:  "Giant Swarm",
		AnnotationAuthorEmail: "dev@giantswarm.io",
		AnnotationAuthorURL:   "https://giantswarm.io",
		AnnotationHomepage:    "https://giantswarm.io/plugins/gs-base",
		AnnotationRepository:  "https://github.com/giantswarm/gs-base",
		AnnotationLicense:     "Apache-2.0",
		AnnotationKeywords:    "platform,base",
	}

	for k, want := range expected {
		if got := annotations[k]; got != want {
			t.Errorf("annotation %s = %q, want %q", k, got, want)
		}
	}

	if len(annotations) != len(expected) {
		t.Errorf("got %d annotations, want %d", len(annotations), len(expected))
	}
}

func TestPushPersonality_AnnotationsFromMetadata(t *testing.T) {
	p := Personality{
		Name:        "sre",
		Description: "SRE personality",
		Author:      &Author{Name: "Giant Swarm"},
		Toolchain: ToolchainReference{
			Repository: "gsoci.azurecr.io/giantswarm/klaus-toolchains/go",
			Tag:        "v1.0.0",
		},
	}

	annotations := buildKlausAnnotations(p.klausMetadata())

	if annotations[AnnotationName] != "sre" {
		t.Errorf("name = %q, want %q", annotations[AnnotationName], "sre")
	}
	if annotations[AnnotationDescription] != "SRE personality" {
		t.Errorf("description = %q, want %q", annotations[AnnotationDescription], "SRE personality")
	}
	if annotations[AnnotationAuthorName] != "Giant Swarm" {
		t.Errorf("author.name = %q, want %q", annotations[AnnotationAuthorName], "Giant Swarm")
	}

	for _, absent := range []string{AnnotationHomepage, AnnotationRepository, AnnotationLicense, AnnotationKeywords, AnnotationAuthorEmail, AnnotationAuthorURL} {
		if _, ok := annotations[absent]; ok {
			t.Errorf("annotation %s should not be present for empty field", absent)
		}
	}
}

func TestPushPlugin_MinimalMetadata(t *testing.T) {
	p := Plugin{Name: "minimal"}

	annotations := buildKlausAnnotations(p.klausMetadata())

	if annotations[AnnotationName] != "minimal" {
		t.Errorf("name = %q, want %q", annotations[AnnotationName], "minimal")
	}
	if len(annotations) != 1 {
		t.Errorf("got %d annotations, want 1 (name only)", len(annotations))
	}
}

func TestPushPlugin_NoMetadata(t *testing.T) {
	annotations := buildKlausAnnotations(commonMetadata{})
	if annotations != nil {
		t.Errorf("expected nil annotations for empty metadata, got %v", annotations)
	}
}
