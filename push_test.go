package oci

import (
	"encoding/json"
	"testing"
)

func TestPluginConfigBlob_ExcludesCommonMetadata(t *testing.T) {
	p := Plugin{
		Name:        testNameGSBase,
		Version:     testTagV100,
		Description: "A base plugin",
		Author:      &Author{Name: testAuthorGiantSwarm, Email: testEmailDev},
		Homepage:    testHomeURL,
		SourceRepo:  testSourceGSBase,
		License:     testLicenseApache2,
		Keywords:    []string{testNamePlatform, "base"},
		Skills:      []string{testKeywordKubernetes, testNameFluxCD},
		Commands:    []string{"init", "deploy"},
		Agents:      []string{testNameCodeReviewer},
		HasHooks:    true,
		MCPServers:  []string{testNameGitHub},
		LSPServers:  []string{testNameGopls},
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

	for _, forbidden := range []string{testKeyName, testKeyDescription, testKeyAuthor, testKeyHomepage, testKeyRepository, testKeyLicense, testKeyKeywords} {
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
		Name:        testNameSRE,
		Description: testDescSREPersonality,
		Author:      &Author{Name: testAuthorGiantSwarm},
		Homepage:    testHomeURL,
		SourceRepo:  "https://github.com/giantswarm/sre",
		License:     testLicenseApache2,
		Keywords:    []string{testNameSRE, "ops"},
		Toolchain: ToolchainReference{
			Repository: testRefToolchainGo,
			Tag:        testTagV100,
		},
		Plugins: []PluginReference{
			{Repository: testRefPluginGSBase, Tag: testTagV100},
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

	for _, forbidden := range []string{testKeyName, testKeyDescription, testKeyAuthor, testKeyHomepage, testKeyRepository, testKeyLicense, testKeyKeywords} {
		if _, ok := raw[forbidden]; ok {
			t.Errorf("config blob should not contain %q, but it does", forbidden)
		}
	}

	if _, ok := raw[testKindToolchain]; !ok {
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

	for _, forbidden := range []string{testKeyName, testKeyDescription, testKeyAuthor, testKeyHomepage, testKeyLicense, testKeyKeywords} {
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
		Name:        testNameGSBase,
		Description: "Giant Swarm base plugin",
		Author:      &Author{Name: testAuthorGiantSwarm, Email: testEmailDev, URL: testHomeURL},
		Homepage:    "https://giantswarm.io/plugins/gs-base",
		SourceRepo:  testSourceGSBase,
		License:     testLicenseApache2,
		Keywords:    []string{testNamePlatform, "base"},
		Skills:      []string{testKeywordKubernetes},
	}

	annotations := buildKlausAnnotations(p.klausMetadata())

	expected := map[string]string{
		AnnotationName:        testNameGSBase,
		AnnotationDescription: "Giant Swarm base plugin",
		AnnotationAuthorName:  testAuthorGiantSwarm,
		AnnotationAuthorEmail: testEmailDev,
		AnnotationAuthorURL:   testHomeURL,
		AnnotationHomepage:    "https://giantswarm.io/plugins/gs-base",
		AnnotationRepository:  testSourceGSBase,
		AnnotationLicense:     testLicenseApache2,
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
		Name:        testNameSRE,
		Description: testDescSREPersonality,
		Author:      &Author{Name: testAuthorGiantSwarm},
		Toolchain: ToolchainReference{
			Repository: testRefToolchainGo,
			Tag:        testTagV100,
		},
	}

	annotations := buildKlausAnnotations(p.klausMetadata())

	if annotations[AnnotationName] != testNameSRE {
		t.Errorf("name = %q, want %q", annotations[AnnotationName], testNameSRE)
	}
	if annotations[AnnotationDescription] != testDescSREPersonality {
		t.Errorf("description = %q, want %q", annotations[AnnotationDescription], testDescSREPersonality)
	}
	if annotations[AnnotationAuthorName] != testAuthorGiantSwarm {
		t.Errorf("author.name = %q, want %q", annotations[AnnotationAuthorName], testAuthorGiantSwarm)
	}

	for _, absent := range []string{AnnotationHomepage, AnnotationRepository, AnnotationLicense, AnnotationKeywords, AnnotationAuthorEmail, AnnotationAuthorURL} {
		if _, ok := annotations[absent]; ok {
			t.Errorf("annotation %s should not be present for empty field", absent)
		}
	}
}

func TestPushPlugin_MinimalMetadata(t *testing.T) {
	p := Plugin{Name: testNameMinimal}

	annotations := buildKlausAnnotations(p.klausMetadata())

	if annotations[AnnotationName] != testNameMinimal {
		t.Errorf("name = %q, want %q", annotations[AnnotationName], testNameMinimal)
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
