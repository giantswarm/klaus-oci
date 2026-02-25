package oci

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadPluginFromDir(t *testing.T) {
	dir := t.TempDir()
	setupFullPlugin(t, dir)

	plugin, err := ReadPluginFromDir(dir)
	if err != nil {
		t.Fatalf("ReadPluginFromDir() error = %v", err)
	}

	if plugin.Name != "gs-base" {
		t.Errorf("Name = %q, want %q", plugin.Name, "gs-base")
	}
	if plugin.Description != "A general purpose plugin" {
		t.Errorf("Description = %q, want %q", plugin.Description, "A general purpose plugin")
	}
	if plugin.Author == nil || plugin.Author.Name != "Giant Swarm GmbH" {
		t.Errorf("Author = %+v, want name 'Giant Swarm GmbH'", plugin.Author)
	}
	if plugin.SourceRepo != "https://github.com/giantswarm/claude-code" {
		t.Errorf("SourceRepo = %q", plugin.SourceRepo)
	}
	if plugin.License != "Apache-2.0" {
		t.Errorf("License = %q", plugin.License)
	}
	if plugin.Homepage != "https://docs.giantswarm.io" {
		t.Errorf("Homepage = %q", plugin.Homepage)
	}

	wantKeywords := []string{"giantswarm", "platform"}
	if len(plugin.Keywords) != len(wantKeywords) {
		t.Errorf("Keywords = %v, want %v", plugin.Keywords, wantKeywords)
	}

	wantSkills := []string{"fluxcd", "kubernetes"}
	if len(plugin.Skills) != len(wantSkills) {
		t.Errorf("Skills = %v, want %v", plugin.Skills, wantSkills)
	} else {
		for i, s := range wantSkills {
			if plugin.Skills[i] != s {
				t.Errorf("Skills[%d] = %q, want %q", i, plugin.Skills[i], s)
			}
		}
	}

	wantCommands := []string{"hello", "init-kubernetes"}
	if len(plugin.Commands) != len(wantCommands) {
		t.Errorf("Commands = %v, want %v", plugin.Commands, wantCommands)
	} else {
		for i, c := range wantCommands {
			if plugin.Commands[i] != c {
				t.Errorf("Commands[%d] = %q, want %q", i, plugin.Commands[i], c)
			}
		}
	}

	wantAgents := []string{"code-reviewer", "security-reviewer"}
	if len(plugin.Agents) != len(wantAgents) {
		t.Errorf("Agents = %v, want %v", plugin.Agents, wantAgents)
	} else {
		for i, a := range wantAgents {
			if plugin.Agents[i] != a {
				t.Errorf("Agents[%d] = %q, want %q", i, plugin.Agents[i], a)
			}
		}
	}

	if !plugin.HasHooks {
		t.Error("HasHooks = false, want true")
	}

	wantMCP := []string{"github"}
	if len(plugin.MCPServers) != len(wantMCP) {
		t.Errorf("MCPServers = %v, want %v", plugin.MCPServers, wantMCP)
	} else if plugin.MCPServers[0] != "github" {
		t.Errorf("MCPServers[0] = %q, want %q", plugin.MCPServers[0], "github")
	}

	wantLSP := []string{"gopls"}
	if len(plugin.LSPServers) != len(wantLSP) {
		t.Errorf("LSPServers = %v, want %v", plugin.LSPServers, wantLSP)
	} else if plugin.LSPServers[0] != "gopls" {
		t.Errorf("LSPServers[0] = %q, want %q", plugin.LSPServers[0], "gopls")
	}

	if plugin.Version != "" {
		t.Errorf("Version = %q, want empty (not set by ReadPluginFromDir)", plugin.Version)
	}
}

func TestReadPluginFromDir_Minimal(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".claude-plugin", "plugin.json"),
		`{"name":"commit-commands","description":"Simple commands"}`)

	plugin, err := ReadPluginFromDir(dir)
	if err != nil {
		t.Fatalf("ReadPluginFromDir() error = %v", err)
	}

	if plugin.Name != "commit-commands" {
		t.Errorf("Name = %q, want %q", plugin.Name, "commit-commands")
	}
	if plugin.Description != "Simple commands" {
		t.Errorf("Description = %q", plugin.Description)
	}
	if plugin.Author != nil {
		t.Errorf("Author = %+v, want nil", plugin.Author)
	}
	if plugin.Skills != nil {
		t.Errorf("Skills = %v, want nil", plugin.Skills)
	}
	if plugin.Commands != nil {
		t.Errorf("Commands = %v, want nil", plugin.Commands)
	}
	if plugin.Agents != nil {
		t.Errorf("Agents = %v, want nil", plugin.Agents)
	}
	if plugin.HasHooks {
		t.Error("HasHooks = true, want false")
	}
	if plugin.MCPServers != nil {
		t.Errorf("MCPServers = %v, want nil", plugin.MCPServers)
	}
	if plugin.LSPServers != nil {
		t.Errorf("LSPServers = %v, want nil", plugin.LSPServers)
	}
}

func TestReadPluginFromDir_MissingManifest(t *testing.T) {
	dir := t.TempDir()

	_, err := ReadPluginFromDir(dir)
	if err == nil {
		t.Fatal("expected error for missing plugin.json")
	}
}

func TestReadPluginFromDir_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".claude-plugin", "plugin.json"), "not json")

	_, err := ReadPluginFromDir(dir)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestReadPluginFromDir_SkillsWithoutSKILLMD(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".claude-plugin", "plugin.json"), `{"name":"test"}`)

	// Create a skills directory with a subdirectory that lacks SKILL.md.
	mkdirAll(t, filepath.Join(dir, "skills", "incomplete"))
	writeFile(t, filepath.Join(dir, "skills", "incomplete", "README.md"), "not a skill")

	// Create a valid skill directory.
	mkdirAll(t, filepath.Join(dir, "skills", "valid"))
	writeFile(t, filepath.Join(dir, "skills", "valid", "SKILL.md"), "# Valid Skill")

	plugin, err := ReadPluginFromDir(dir)
	if err != nil {
		t.Fatalf("ReadPluginFromDir() error = %v", err)
	}

	if len(plugin.Skills) != 1 || plugin.Skills[0] != "valid" {
		t.Errorf("Skills = %v, want [valid]", plugin.Skills)
	}
}

func TestReadPluginFromDir_EmptyHooksDir(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".claude-plugin", "plugin.json"), `{"name":"test"}`)
	mkdirAll(t, filepath.Join(dir, "hooks"))

	plugin, err := ReadPluginFromDir(dir)
	if err != nil {
		t.Fatalf("ReadPluginFromDir() error = %v", err)
	}

	if plugin.HasHooks {
		t.Error("HasHooks = true, want false (empty hooks directory)")
	}
}

func TestReadPluginFromDir_VersionInManifestIgnored(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".claude-plugin", "plugin.json"),
		`{"name":"test","version":"1.0.0"}`)

	plugin, err := ReadPluginFromDir(dir)
	if err != nil {
		t.Fatalf("ReadPluginFromDir() error = %v", err)
	}

	if plugin.Version != "" {
		t.Errorf("Version = %q, want empty (json:\"-\" excludes it)", plugin.Version)
	}
}

func TestReadPersonalityFromDir(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "personality.yaml"), `
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
    tag: v1.2.0
`)

	p, err := ReadPersonalityFromDir(dir)
	if err != nil {
		t.Fatalf("ReadPersonalityFromDir() error = %v", err)
	}

	if p.Name != "sre" {
		t.Errorf("Name = %q, want %q", p.Name, "sre")
	}
	if p.Description != "Giant Swarm SRE personality" {
		t.Errorf("Description = %q", p.Description)
	}
	if p.Author == nil || p.Author.Name != "Giant Swarm GmbH" {
		t.Errorf("Author = %+v", p.Author)
	}
	if p.SourceRepo != "https://github.com/giantswarm/klaus-personalities" {
		t.Errorf("SourceRepo = %q", p.SourceRepo)
	}
	if p.License != "Apache-2.0" {
		t.Errorf("License = %q", p.License)
	}
	if len(p.Keywords) != 4 {
		t.Errorf("Keywords length = %d, want 4", len(p.Keywords))
	}
	if p.Toolchain.Repository != "gsoci.azurecr.io/giantswarm/klaus-toolchains/go" {
		t.Errorf("Toolchain.Repository = %q", p.Toolchain.Repository)
	}
	if p.Toolchain.Tag != "latest" {
		t.Errorf("Toolchain.Tag = %q", p.Toolchain.Tag)
	}
	if len(p.Plugins) != 2 {
		t.Fatalf("Plugins length = %d, want 2", len(p.Plugins))
	}
	if p.Plugins[0].Repository != "gsoci.azurecr.io/giantswarm/klaus-plugins/gs-base" {
		t.Errorf("Plugins[0].Repository = %q", p.Plugins[0].Repository)
	}
	if p.Plugins[0].Tag != "latest" {
		t.Errorf("Plugins[0].Tag = %q", p.Plugins[0].Tag)
	}
	if p.Plugins[1].Repository != "gsoci.azurecr.io/giantswarm/klaus-plugins/gs-sre" {
		t.Errorf("Plugins[1].Repository = %q", p.Plugins[1].Repository)
	}
	if p.Plugins[1].Tag != "v1.2.0" {
		t.Errorf("Plugins[1].Tag = %q", p.Plugins[1].Tag)
	}
	if p.Version != "" {
		t.Errorf("Version = %q, want empty (not set by ReadPersonalityFromDir)", p.Version)
	}
}

func TestReadPersonalityFromDir_Minimal(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "personality.yaml"), `
name: python
description: Python personality
toolchain:
  repository: gsoci.azurecr.io/giantswarm/klaus-toolchains/python
  tag: latest
`)

	p, err := ReadPersonalityFromDir(dir)
	if err != nil {
		t.Fatalf("ReadPersonalityFromDir() error = %v", err)
	}

	if p.Name != "python" {
		t.Errorf("Name = %q, want %q", p.Name, "python")
	}
	if p.Description != "Python personality" {
		t.Errorf("Description = %q", p.Description)
	}
	if p.Author != nil {
		t.Errorf("Author = %+v, want nil", p.Author)
	}
	if p.SourceRepo != "" {
		t.Errorf("SourceRepo = %q, want empty", p.SourceRepo)
	}
	if p.Toolchain.Repository != "gsoci.azurecr.io/giantswarm/klaus-toolchains/python" {
		t.Errorf("Toolchain.Repository = %q", p.Toolchain.Repository)
	}
	if len(p.Plugins) != 0 {
		t.Errorf("Plugins = %v, want empty", p.Plugins)
	}
}

func TestReadPersonalityFromDir_MissingFile(t *testing.T) {
	dir := t.TempDir()

	_, err := ReadPersonalityFromDir(dir)
	if err == nil {
		t.Fatal("expected error for missing personality.yaml")
	}
}

func TestReadPersonalityFromDir_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "personality.yaml"), ":\n  :\n    :\n  bad: [")

	_, err := ReadPersonalityFromDir(dir)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestReadPersonalityFromDir_MissingName(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "personality.yaml"), `
description: A personality without a name
toolchain:
  repository: gsoci.azurecr.io/giantswarm/klaus-toolchains/go
  tag: latest
`)

	_, err := ReadPersonalityFromDir(dir)
	if err == nil {
		t.Fatal("expected error for missing name")
	}
}

func TestReadPersonalityFromDir_VersionInYAMLIgnored(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "personality.yaml"), `
name: test
version: should-be-ignored
toolchain:
  repository: gsoci.azurecr.io/giantswarm/klaus-toolchains/go
  tag: latest
`)

	p, err := ReadPersonalityFromDir(dir)
	if err != nil {
		t.Fatalf("ReadPersonalityFromDir() error = %v", err)
	}

	if p.Version != "" {
		t.Errorf("Version = %q, want empty (yaml:\"-\" excludes it)", p.Version)
	}
}

func TestReadPersonalityFromDir_SoulNotRead(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "personality.yaml"), `
name: sre
toolchain:
  repository: gsoci.azurecr.io/giantswarm/klaus-toolchains/go
  tag: latest
`)
	writeFile(t, filepath.Join(dir, "SOUL.md"), "# SRE Soul\nI am the SRE personality.")

	p, err := ReadPersonalityFromDir(dir)
	if err != nil {
		t.Fatalf("ReadPersonalityFromDir() error = %v", err)
	}

	if p.Name != "sre" {
		t.Errorf("Name = %q, want %q", p.Name, "sre")
	}
}

// setupFullPlugin creates a complete plugin directory structure with all
// component types for testing ReadPluginFromDir.
func setupFullPlugin(t *testing.T, dir string) {
	t.Helper()

	writeFile(t, filepath.Join(dir, ".claude-plugin", "plugin.json"), `{
  "name": "gs-base",
  "description": "A general purpose plugin",
  "author": {"name": "Giant Swarm GmbH"},
  "repository": "https://github.com/giantswarm/claude-code",
  "homepage": "https://docs.giantswarm.io",
  "license": "Apache-2.0",
  "keywords": ["giantswarm", "platform"]
}`)

	writeFile(t, filepath.Join(dir, "skills", "kubernetes", "SKILL.md"), "# Kubernetes")
	writeFile(t, filepath.Join(dir, "skills", "fluxcd", "SKILL.md"), "# FluxCD")

	writeFile(t, filepath.Join(dir, "commands", "hello.md"), "# Hello command")
	writeFile(t, filepath.Join(dir, "commands", "init-kubernetes.md"), "# Init Kubernetes")

	writeFile(t, filepath.Join(dir, "agents", "code-reviewer.md"), "# Code Reviewer")
	writeFile(t, filepath.Join(dir, "agents", "security-reviewer.md"), "# Security Reviewer")

	writeFile(t, filepath.Join(dir, "hooks", "hooks.json"), `{"PreToolUse": []}`)

	writeFile(t, filepath.Join(dir, ".mcp.json"), `{"github": {"command": "gh-mcp"}}`)
	writeFile(t, filepath.Join(dir, ".lsp.json"), `{"gopls": {"command": "gopls"}}`)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	mkdirAll(t, filepath.Dir(path))
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}

func mkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("creating directory %s: %v", path, err)
	}
}
