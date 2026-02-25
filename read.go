package oci

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// ReadPluginFromDir reads a plugin's metadata from its source directory.
//
// It reads .claude-plugin/plugin.json for manifest metadata (name, description,
// author, etc.) and then scans the directory tree to discover components:
//
//   - skills/ subdirectories containing SKILL.md -> Skills
//   - commands/*.md files -> Commands
//   - agents/*.md files -> Agents
//   - hooks/ directory or hooks config in plugin.json -> HasHooks
//   - .mcp.json top-level keys -> MCPServers
//   - .lsp.json top-level keys -> LSPServers
//
// Version is NOT set -- it is conveyed via the OCI tag at push time.
func ReadPluginFromDir(dir string) (*Plugin, error) {
	manifestPath := filepath.Join(dir, ".claude-plugin", "plugin.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("reading plugin manifest: %w", err)
	}

	var plugin Plugin
	if err := json.Unmarshal(data, &plugin); err != nil {
		return nil, fmt.Errorf("parsing plugin manifest: %w", err)
	}

	plugin.Skills = discoverSkills(dir)
	plugin.Commands = discoverMarkdownNames(filepath.Join(dir, "commands"))
	plugin.Agents = discoverMarkdownNames(filepath.Join(dir, "agents"))
	plugin.HasHooks = detectHooks(dir)
	plugin.MCPServers = discoverJSONKeys(filepath.Join(dir, ".mcp.json"))
	plugin.LSPServers = discoverJSONKeys(filepath.Join(dir, ".lsp.json"))

	return &plugin, nil
}

// ReadPersonalityFromDir reads a personality's metadata from its source
// directory by parsing personality.yaml.
//
// Version is NOT set -- it is conveyed via the OCI tag at push time.
// SOUL.md is NOT read -- it lives in the content layer and is included
// automatically when PushPersonality tar.gz's the source directory.
func ReadPersonalityFromDir(dir string) (*Personality, error) {
	yamlPath := filepath.Join(dir, "personality.yaml")
	data, err := os.ReadFile(yamlPath)
	if err != nil {
		return nil, fmt.Errorf("reading personality.yaml: %w", err)
	}

	var p Personality
	if err := yaml.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("parsing personality.yaml: %w", err)
	}

	if p.Name == "" {
		return nil, fmt.Errorf("personality.yaml: name is required")
	}

	return &p, nil
}

// discoverSkills scans the skills/ directory for subdirectories that contain
// a SKILL.md file. Returns sorted skill names.
func discoverSkills(dir string) []string {
	skillsDir := filepath.Join(dir, "skills")
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		return nil
	}

	var names []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		skillFile := filepath.Join(skillsDir, entry.Name(), "SKILL.md")
		if _, err := os.Stat(skillFile); err == nil {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	return names
}

// discoverMarkdownNames scans a directory for .md files and returns their
// base names (without extension), sorted.
func discoverMarkdownNames(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var names []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".md") {
			names = append(names, strings.TrimSuffix(name, ".md"))
		}
	}
	sort.Strings(names)
	return names
}

// detectHooks returns true if the plugin directory contains a hooks/
// directory with content.
func detectHooks(dir string) bool {
	hooksDir := filepath.Join(dir, "hooks")
	entries, err := os.ReadDir(hooksDir)
	if err != nil {
		return false
	}
	return len(entries) > 0
}

// discoverJSONKeys reads a JSON file and returns its top-level object keys,
// sorted. Returns nil if the file doesn't exist or isn't a JSON object.
func discoverJSONKeys(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var obj map[string]json.RawMessage
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil
	}
	if len(obj) == 0 {
		return nil
	}

	names := make([]string, 0, len(obj))
	for key := range obj {
		names = append(names, key)
	}
	sort.Strings(names)
	return names
}
