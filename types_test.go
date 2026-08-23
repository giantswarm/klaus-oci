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
			ref:  PluginReference{Repository: testRefExamplePluginTest, Tag: testTagV100},
			want: "registry.example.com/plugins/test:v1.0.0",
		},
		{
			name: "with digest",
			ref:  PluginReference{Repository: testRefExamplePluginTest, Digest: testDigestAbc123},
			want: "registry.example.com/plugins/test@sha256:abc123",
		},
		{
			name: "digest takes precedence over tag",
			ref:  PluginReference{Repository: testRefExamplePluginTest, Tag: testTagV100, Digest: testDigestAbc123},
			want: "registry.example.com/plugins/test@sha256:abc123",
		},
		{
			name: "bare repository",
			ref:  PluginReference{Repository: testRefExamplePluginTest},
			want: testRefExamplePluginTest,
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

func TestToolchainReference_Ref(t *testing.T) {
	tests := []struct {
		name string
		ref  ToolchainReference
		want string
	}{
		{
			name: "with tag",
			ref:  ToolchainReference{Repository: testRefExampleToolchainGo, Tag: testTagV120},
			want: "registry.example.com/toolchains/go:v1.2.0",
		},
		{
			name: "with digest",
			ref:  ToolchainReference{Repository: testRefExampleToolchainGo, Digest: testDigestDef456},
			want: "registry.example.com/toolchains/go@sha256:def456",
		},
		{
			name: "digest takes precedence over tag",
			ref:  ToolchainReference{Repository: testRefExampleToolchainGo, Tag: testTagV120, Digest: testDigestDef456},
			want: "registry.example.com/toolchains/go@sha256:def456",
		},
		{
			name: "bare repository",
			ref:  ToolchainReference{Repository: testRefExampleToolchainGo},
			want: testRefExampleToolchainGo,
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

func TestPlugin_JSON(t *testing.T) {
	plugin := Plugin{
		Name:        testNameGSBase,
		Version:     testSemver100,
		Description: testDescGeneralPlugin,
		Author:      &Author{Name: testAuthorGiantSwarmGmbH},
		SourceRepo:  testSourceClaudeCode,
		License:     testLicenseApache2,
		Keywords:    []string{testOrgGiantSwarm, testNamePlatform},
		Skills:      []string{testKeywordKubernetes, testNameFluxCD},
		Commands:    []string{testValueHello, testNameInitKubernetes},
		Agents:      []string{testNameCodeReviewer},
		HasHooks:    true,
		MCPServers:  []string{testNameGitHub},
	}

	data, err := json.Marshal(plugin)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded Plugin
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.Name != plugin.Name {
		t.Errorf("Name = %q, want %q", decoded.Name, plugin.Name)
	}
	if decoded.Version != "" {
		t.Errorf("Version should be excluded from JSON, got %q", decoded.Version)
	}
	if decoded.Description != plugin.Description {
		t.Errorf("Description = %q, want %q", decoded.Description, plugin.Description)
	}
	if decoded.Author == nil || decoded.Author.Name != testAuthorGiantSwarmGmbH {
		t.Errorf("Author = %+v, want name 'Giant Swarm GmbH'", decoded.Author)
	}
	if decoded.SourceRepo != plugin.SourceRepo {
		t.Errorf("SourceRepo = %q, want %q", decoded.SourceRepo, plugin.SourceRepo)
	}
	if decoded.License != plugin.License {
		t.Errorf("License = %q, want %q", decoded.License, plugin.License)
	}
	if len(decoded.Skills) != len(plugin.Skills) {
		t.Errorf("Skills length = %d, want %d", len(decoded.Skills), len(plugin.Skills))
	}
	if len(decoded.Commands) != len(plugin.Commands) {
		t.Errorf("Commands length = %d, want %d", len(decoded.Commands), len(plugin.Commands))
	}
	if len(decoded.Agents) != len(plugin.Agents) {
		t.Errorf("Agents length = %d, want %d", len(decoded.Agents), len(plugin.Agents))
	}
	if !decoded.HasHooks {
		t.Error("HasHooks = false, want true")
	}
	if len(decoded.MCPServers) != 1 || decoded.MCPServers[0] != testNameGitHub {
		t.Errorf("MCPServers = %v, want [github]", decoded.MCPServers)
	}
}

func TestPlugin_JSON_VersionExcluded(t *testing.T) {
	plugin := Plugin{
		Name:    "test-plugin",
		Version: testSemver100,
	}

	data, err := json.Marshal(plugin)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal raw: %v", err)
	}

	if _, ok := raw["version"]; ok {
		t.Error("version field should not be present in JSON output")
	}
}

func TestPlugin_JSON_Minimal(t *testing.T) {
	plugin := Plugin{
		Name:     testNameCommitCommands,
		Commands: []string{"commit", "push", "pr"},
	}

	data, err := json.Marshal(plugin)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	want := `{"name":"commit-commands","commands":["commit","push","pr"]}`
	if string(data) != want {
		t.Errorf("JSON = %s, want %s", data, want)
	}
}

func TestPersonality_JSON(t *testing.T) {
	personality := Personality{
		Name:        testNameSRE,
		Version:     testSemver100,
		Description: testDescGSSREPersonality,
		Author:      &Author{Name: testAuthorGiantSwarmGmbH},
		SourceRepo:  testSourcePersonalities,
		License:     testLicenseApache2,
		Keywords:    []string{testOrgGiantSwarm, testNameSRE, testKeywordKubernetes},
		Toolchain: ToolchainReference{
			Repository: testRefToolchainGo,
			Tag:        testTagLatest,
		},
		Plugins: []PluginReference{
			{Repository: testRefPluginGSBase, Tag: testTagLatest},
			{Repository: testRefPluginGSSRE, Tag: testTagLatest},
		},
	}

	data, err := json.Marshal(personality)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded Personality
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.Name != personality.Name {
		t.Errorf("Name = %q, want %q", decoded.Name, personality.Name)
	}
	if decoded.Version != "" {
		t.Errorf("Version should be excluded from JSON, got %q", decoded.Version)
	}
	if decoded.Description != personality.Description {
		t.Errorf("Description = %q, want %q", decoded.Description, personality.Description)
	}
	if decoded.Toolchain.Repository != testRefToolchainGo {
		t.Errorf("Toolchain.Repository = %q", decoded.Toolchain.Repository)
	}
	if len(decoded.Plugins) != 2 {
		t.Fatalf("Plugins length = %d, want 2", len(decoded.Plugins))
	}
}

func TestPersonality_JSON_VersionExcluded(t *testing.T) {
	personality := Personality{
		Name:    "go",
		Version: "0.3.0",
		Toolchain: ToolchainReference{
			Repository: testRefToolchainGo,
			Tag:        testTagLatest,
		},
	}

	data, err := json.Marshal(personality)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal raw: %v", err)
	}

	if _, ok := raw["version"]; ok {
		t.Error("version field should not be present in JSON output")
	}
}

func TestPersonality_YAML(t *testing.T) {
	input := `
name: sre
description: Giant Swarm SRE personality
author:
  name: Giant Swarm GmbH
repository: https://github.com/giantswarm/klaus-personalities
license: Apache-2.0
keywords:
  - giantswarm
  - sre
  - kubernetes
  - platform
toolchain:
  repository: gsoci.azurecr.io/giantswarm/klaus-toolchains/go
  tag: latest
plugins:
  - repository: gsoci.azurecr.io/giantswarm/klaus-plugins/gs-base
    tag: latest
  - repository: gsoci.azurecr.io/giantswarm/klaus-plugins/gs-sre
    tag: latest
`

	var p Personality
	if err := yaml.Unmarshal([]byte(input), &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if p.Name != testNameSRE {
		t.Errorf("Name = %q, want %q", p.Name, testNameSRE)
	}
	if p.Description != testDescGSSREPersonality {
		t.Errorf("Description = %q", p.Description)
	}
	if p.Author == nil || p.Author.Name != testAuthorGiantSwarmGmbH {
		t.Errorf("Author = %+v", p.Author)
	}
	if p.SourceRepo != testSourcePersonalities {
		t.Errorf("SourceRepo = %q", p.SourceRepo)
	}
	if p.License != testLicenseApache2 {
		t.Errorf("License = %q", p.License)
	}
	if len(p.Keywords) != 4 {
		t.Errorf("Keywords length = %d, want 4", len(p.Keywords))
	}
	if p.Toolchain.Repository != testRefToolchainGo {
		t.Errorf("Toolchain.Repository = %q", p.Toolchain.Repository)
	}
	if p.Toolchain.Tag != testTagLatest {
		t.Errorf("Toolchain.Tag = %q", p.Toolchain.Tag)
	}
	if len(p.Plugins) != 2 {
		t.Fatalf("Plugins length = %d, want 2", len(p.Plugins))
	}
	if p.Plugins[0].Repository != testRefPluginGSBase {
		t.Errorf("Plugins[0].Repository = %q", p.Plugins[0].Repository)
	}
	if p.Version != "" {
		t.Errorf("Version should not be set from YAML, got %q", p.Version)
	}
}

func TestPersonality_YAML_VersionIgnored(t *testing.T) {
	input := `
name: test
version: should-be-ignored
toolchain:
  repository: gsoci.azurecr.io/giantswarm/klaus-toolchains/go
  tag: latest
`

	var p Personality
	if err := yaml.Unmarshal([]byte(input), &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if p.Version != "" {
		t.Errorf("Version should be ignored by yaml:'-' tag, got %q", p.Version)
	}
}

func TestToolchain_JSON(t *testing.T) {
	toolchain := Toolchain{
		Name:        "go",
		Version:     "1.2.0",
		Description: testDescGoToolchainKlaus,
		Author:      &Author{Name: testAuthorGiantSwarmGmbH},
		Homepage:    testDocsURL,
		SourceRepo:  testSourceKlausImages,
		License:     testLicenseApache2,
		Keywords:    []string{testOrgGiantSwarm, "go", testKindToolchain},
	}

	data, err := json.Marshal(toolchain)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded Toolchain
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.Name != toolchain.Name {
		t.Errorf("Name = %q, want %q", decoded.Name, toolchain.Name)
	}
	if decoded.Version != "" {
		t.Errorf("Version should be excluded from JSON, got %q", decoded.Version)
	}
	if decoded.Description != toolchain.Description {
		t.Errorf("Description = %q, want %q", decoded.Description, toolchain.Description)
	}
	if decoded.Author == nil || decoded.Author.Name != testAuthorGiantSwarmGmbH {
		t.Errorf("Author = %+v", decoded.Author)
	}
	if decoded.Homepage != toolchain.Homepage {
		t.Errorf("Homepage = %q, want %q", decoded.Homepage, toolchain.Homepage)
	}
	if decoded.SourceRepo != toolchain.SourceRepo {
		t.Errorf("SourceRepo = %q, want %q", decoded.SourceRepo, toolchain.SourceRepo)
	}
	if decoded.License != toolchain.License {
		t.Errorf("License = %q, want %q", decoded.License, toolchain.License)
	}
}

func TestToolchain_JSON_OmitEmpty(t *testing.T) {
	toolchain := Toolchain{
		Name: "go",
	}

	data, err := json.Marshal(toolchain)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	want := `{"name":"go"}`
	if string(data) != want {
		t.Errorf("JSON = %s, want %s", data, want)
	}
}
