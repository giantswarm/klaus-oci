package oci

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	godigest "github.com/opencontainers/go-digest"
	specs "github.com/opencontainers/image-spec/specs-go"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// testArtifactEntry describes a single artifact to serve from the test registry.
type testArtifactEntry struct {
	configJSON      []byte
	configMediaType string
	tags            []string
	annotations     map[string]string
}

// builtArtifact holds pre-computed manifest and blob data for serving.
type builtArtifact struct {
	manifestJSON   []byte
	manifestDigest godigest.Digest
	configJSON     []byte
	configDigest   godigest.Digest
	tags           []string
}

// newArtifactRegistry creates a test OCI registry that serves manifests,
// config blobs, and tag listings. The artifacts map is keyed by repository
// name (e.g. "giantswarm/klaus-plugins/gs-base").
func newArtifactRegistry(artifacts map[string]testArtifactEntry) *httptest.Server {
	built := make(map[string]*builtArtifact)
	for name, entry := range artifacts {
		configDigest := godigest.FromBytes(entry.configJSON)
		configDesc := ocispec.Descriptor{
			MediaType: entry.configMediaType,
			Digest:    configDigest,
			Size:      int64(len(entry.configJSON)),
		}

		manifest := ocispec.Manifest{
			Versioned:   specs.Versioned{SchemaVersion: 2},
			MediaType:   ocispec.MediaTypeImageManifest,
			Config:      configDesc,
			Layers:      []ocispec.Descriptor{},
			Annotations: entry.annotations,
		}

		manifestJSON, _ := json.Marshal(manifest)
		manifestDigest := godigest.FromBytes(manifestJSON)

		built[name] = &builtArtifact{
			manifestJSON:   manifestJSON,
			manifestDigest: manifestDigest,
			configJSON:     entry.configJSON,
			configDigest:   configDigest,
			tags:           entry.tags,
		}
	}

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		if path == "/v2/" || path == "/v2" {
			w.WriteHeader(http.StatusOK)
			return
		}

		if !strings.HasPrefix(path, "/v2/") {
			http.NotFound(w, r)
			return
		}

		rest := strings.TrimPrefix(path, "/v2/")

		if strings.HasSuffix(rest, "/tags/list") {
			repoName := strings.TrimSuffix(rest, "/tags/list")
			art, ok := built[repoName]
			if !ok {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{"name": repoName, "tags": art.tags})
			return
		}

		if idx := strings.LastIndex(rest, "/manifests/"); idx >= 0 {
			repoName := rest[:idx]
			reference := rest[idx+len("/manifests/"):]

			art, ok := built[repoName]
			if !ok {
				http.NotFound(w, r)
				return
			}

			validRef := reference == art.manifestDigest.String()
			if !validRef {
				for _, tag := range art.tags {
					if reference == tag {
						validRef = true
						break
					}
				}
			}
			if !validRef {
				http.NotFound(w, r)
				return
			}

			w.Header().Set("Content-Type", ocispec.MediaTypeImageManifest)
			w.Header().Set("Docker-Content-Digest", art.manifestDigest.String())
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(art.manifestJSON)))
			if r.Method == http.MethodHead {
				return
			}
			w.Write(art.manifestJSON)
			return
		}

		if idx := strings.LastIndex(rest, "/blobs/"); idx >= 0 {
			repoName := rest[:idx]
			blobDigest := rest[idx+len("/blobs/"):]

			art, ok := built[repoName]
			if !ok {
				http.NotFound(w, r)
				return
			}

			if blobDigest == art.configDigest.String() {
				w.Header().Set("Docker-Content-Digest", art.configDigest.String())
				w.Write(art.configJSON)
				return
			}

			http.NotFound(w, r)
			return
		}

		http.NotFound(w, r)
	}))
}

func TestToolchainFromAnnotations(t *testing.T) {
	t.Run("full annotations", func(t *testing.T) {
		annotations := map[string]string{
			AnnotationName:        "go",
			AnnotationDescription: "Go toolchain for Klaus",
			AnnotationAuthorName:  "Giant Swarm GmbH",
			AnnotationHomepage:    "https://docs.giantswarm.io/klaus/",
			AnnotationRepository:  "https://github.com/giantswarm/klaus-images",
			AnnotationLicense:     "Apache-2.0",
			AnnotationKeywords:    "giantswarm,go,toolchain",
		}

		tc := toolchainFromAnnotations(annotations)

		if tc.Name != "go" {
			t.Errorf("Name = %q, want %q", tc.Name, "go")
		}
		if tc.Description != "Go toolchain for Klaus" {
			t.Errorf("Description = %q, want %q", tc.Description, "Go toolchain for Klaus")
		}
		if tc.Author == nil || tc.Author.Name != "Giant Swarm GmbH" {
			t.Errorf("Author = %+v, want name 'Giant Swarm GmbH'", tc.Author)
		}
		if tc.Homepage != "https://docs.giantswarm.io/klaus/" {
			t.Errorf("Homepage = %q", tc.Homepage)
		}
		if tc.SourceRepo != "https://github.com/giantswarm/klaus-images" {
			t.Errorf("SourceRepo = %q", tc.SourceRepo)
		}
		if tc.License != "Apache-2.0" {
			t.Errorf("License = %q", tc.License)
		}
		if len(tc.Keywords) != 3 || tc.Keywords[0] != "giantswarm" || tc.Keywords[1] != "go" || tc.Keywords[2] != "toolchain" {
			t.Errorf("Keywords = %v, want [giantswarm go toolchain]", tc.Keywords)
		}
		if tc.Version != "" {
			t.Errorf("Version = %q, want empty (set by caller, not annotations)", tc.Version)
		}
	})

	t.Run("minimal annotations", func(t *testing.T) {
		annotations := map[string]string{
			AnnotationName: "python",
		}

		tc := toolchainFromAnnotations(annotations)

		if tc.Name != "python" {
			t.Errorf("Name = %q, want %q", tc.Name, "python")
		}
		if tc.Author != nil {
			t.Errorf("Author = %+v, want nil", tc.Author)
		}
		if tc.Keywords != nil {
			t.Errorf("Keywords = %v, want nil", tc.Keywords)
		}
	})

	t.Run("nil annotations", func(t *testing.T) {
		tc := toolchainFromAnnotations(nil)

		if tc.Name != "" {
			t.Errorf("Name = %q, want empty", tc.Name)
		}
		if tc.Author != nil {
			t.Errorf("Author = %+v, want nil", tc.Author)
		}
	})

	t.Run("version annotation ignored", func(t *testing.T) {
		annotations := map[string]string{
			AnnotationName:                     "go",
			"org.opencontainers.image.version": "v1.2.0",
		}

		tc := toolchainFromAnnotations(annotations)

		if tc.Version != "" {
			t.Errorf("Version = %q, want empty (version comes from OCI tag)", tc.Version)
		}
	})
}

func TestDescribePlugin(t *testing.T) {
	blob := pluginConfigBlob{
		Skills:     []string{"kubernetes", "fluxcd"},
		Commands:   []string{"hello", "init-kubernetes"},
		Agents:     []string{"code-reviewer"},
		HasHooks:   true,
		MCPServers: []string{"github"},
	}
	configJSON, _ := json.Marshal(blob)
	annotations := buildKlausAnnotations(commonMetadata{
		Name:        "gs-base",
		Description: "A general purpose plugin",
		Author:      &Author{Name: "Giant Swarm GmbH"},
		SourceRepo:  "https://github.com/giantswarm/claude-code",
		License:     "Apache-2.0",
		Keywords:    []string{"giantswarm", "platform"},
	})

	ts := newArtifactRegistry(map[string]testArtifactEntry{
		"giantswarm/klaus-plugins/gs-base": {
			configJSON:      configJSON,
			configMediaType: MediaTypePluginConfig,
			tags:            []string{"v1.0.0"},
			annotations:     annotations,
		},
	})
	defer ts.Close()
	host := testRegistryHost(ts)

	client := NewClient(WithPlainHTTP(true))
	ref := host + "/giantswarm/klaus-plugins/gs-base:v1.0.0"

	described, err := client.DescribePlugin(t.Context(), ref)
	if err != nil {
		t.Fatalf("DescribePlugin() error = %v", err)
	}

	if described.Plugin.Name != "gs-base" {
		t.Errorf("Name = %q, want %q", described.Plugin.Name, "gs-base")
	}
	if described.Plugin.Version != "v1.0.0" {
		t.Errorf("Version = %q, want %q", described.Plugin.Version, "v1.0.0")
	}
	if described.Plugin.Description != "A general purpose plugin" {
		t.Errorf("Description = %q", described.Plugin.Description)
	}
	if described.Plugin.Author == nil || described.Plugin.Author.Name != "Giant Swarm GmbH" {
		t.Errorf("Author = %+v", described.Plugin.Author)
	}
	if described.Plugin.SourceRepo != "https://github.com/giantswarm/claude-code" {
		t.Errorf("SourceRepo = %q", described.Plugin.SourceRepo)
	}
	if described.Plugin.License != "Apache-2.0" {
		t.Errorf("License = %q", described.Plugin.License)
	}
	if len(described.Plugin.Skills) != 2 {
		t.Errorf("Skills = %v, want 2 items", described.Plugin.Skills)
	}
	if len(described.Plugin.Commands) != 2 {
		t.Errorf("Commands = %v, want 2 items", described.Plugin.Commands)
	}
	if len(described.Plugin.Agents) != 1 || described.Plugin.Agents[0] != "code-reviewer" {
		t.Errorf("Agents = %v, want [code-reviewer]", described.Plugin.Agents)
	}
	if !described.Plugin.HasHooks {
		t.Error("HasHooks = false, want true")
	}
	if len(described.Plugin.MCPServers) != 1 || described.Plugin.MCPServers[0] != "github" {
		t.Errorf("MCPServers = %v, want [github]", described.Plugin.MCPServers)
	}
	if described.ArtifactInfo.Tag != "v1.0.0" {
		t.Errorf("Tag = %q, want %q", described.ArtifactInfo.Tag, "v1.0.0")
	}
	if described.ArtifactInfo.Ref != ref {
		t.Errorf("Ref = %q, want %q", described.ArtifactInfo.Ref, ref)
	}
	if described.ArtifactInfo.Digest == "" {
		t.Error("Digest should not be empty")
	}
}

func TestDescribePlugin_Minimal(t *testing.T) {
	blob := pluginConfigBlob{
		Commands: []string{"commit", "push", "pr"},
	}
	configJSON, _ := json.Marshal(blob)
	annotations := buildKlausAnnotations(commonMetadata{Name: "commit-commands"})

	ts := newArtifactRegistry(map[string]testArtifactEntry{
		"giantswarm/klaus-plugins/commit-commands": {
			configJSON:      configJSON,
			configMediaType: MediaTypePluginConfig,
			tags:            []string{"v1.0.0"},
			annotations:     annotations,
		},
	})
	defer ts.Close()
	host := testRegistryHost(ts)

	client := NewClient(WithPlainHTTP(true))
	ref := host + "/giantswarm/klaus-plugins/commit-commands:v1.0.0"

	described, err := client.DescribePlugin(t.Context(), ref)
	if err != nil {
		t.Fatalf("DescribePlugin() error = %v", err)
	}

	if described.Plugin.Name != "commit-commands" {
		t.Errorf("Name = %q, want %q", described.Plugin.Name, "commit-commands")
	}
	if described.Plugin.Version != "v1.0.0" {
		t.Errorf("Version = %q, want %q", described.Plugin.Version, "v1.0.0")
	}
	if described.Plugin.Author != nil {
		t.Errorf("Author = %+v, want nil", described.Plugin.Author)
	}
	if len(described.Plugin.Commands) != 3 {
		t.Errorf("Commands = %v, want 3 items", described.Plugin.Commands)
	}
}

func TestDescribePersonality(t *testing.T) {
	blob := personalityConfigBlob{
		Toolchain: ToolchainReference{
			Repository: "gsoci.azurecr.io/giantswarm/klaus-toolchains/go",
			Tag:        "v1.0.0",
		},
		Plugins: []PluginReference{
			{Repository: "gsoci.azurecr.io/giantswarm/klaus-plugins/gs-base", Tag: "latest"},
			{Repository: "gsoci.azurecr.io/giantswarm/klaus-plugins/gs-sre", Tag: "v1.2.0"},
		},
	}
	configJSON, _ := json.Marshal(blob)
	annotations := buildKlausAnnotations(commonMetadata{
		Name:        "sre",
		Description: "SRE personality",
		Author:      &Author{Name: "Giant Swarm GmbH"},
		SourceRepo:  "https://github.com/giantswarm/klaus-personalities",
		License:     "Apache-2.0",
		Keywords:    []string{"giantswarm", "sre", "kubernetes"},
	})

	ts := newArtifactRegistry(map[string]testArtifactEntry{
		"giantswarm/klaus-personalities/sre": {
			configJSON:      configJSON,
			configMediaType: MediaTypePersonalityConfig,
			tags:            []string{"v1.0.0"},
			annotations:     annotations,
		},
	})
	defer ts.Close()
	host := testRegistryHost(ts)

	client := NewClient(WithPlainHTTP(true))
	ref := host + "/giantswarm/klaus-personalities/sre:v1.0.0"

	described, err := client.DescribePersonality(t.Context(), ref)
	if err != nil {
		t.Fatalf("DescribePersonality() error = %v", err)
	}

	if described.Personality.Name != "sre" {
		t.Errorf("Name = %q, want %q", described.Personality.Name, "sre")
	}
	if described.Personality.Version != "v1.0.0" {
		t.Errorf("Version = %q, want %q", described.Personality.Version, "v1.0.0")
	}
	if described.Personality.Description != "SRE personality" {
		t.Errorf("Description = %q", described.Personality.Description)
	}
	if described.Personality.Author == nil || described.Personality.Author.Name != "Giant Swarm GmbH" {
		t.Errorf("Author = %+v", described.Personality.Author)
	}
	if described.Personality.SourceRepo != "https://github.com/giantswarm/klaus-personalities" {
		t.Errorf("SourceRepo = %q", described.Personality.SourceRepo)
	}
	if described.Personality.License != "Apache-2.0" {
		t.Errorf("License = %q", described.Personality.License)
	}
	if len(described.Personality.Keywords) != 3 {
		t.Errorf("Keywords = %v, want 3 items", described.Personality.Keywords)
	}
	if described.Personality.Toolchain.Repository != "gsoci.azurecr.io/giantswarm/klaus-toolchains/go" {
		t.Errorf("Toolchain.Repository = %q", described.Personality.Toolchain.Repository)
	}
	if described.Personality.Toolchain.Tag != "v1.0.0" {
		t.Errorf("Toolchain.Tag = %q, want %q", described.Personality.Toolchain.Tag, "v1.0.0")
	}
	if len(described.Personality.Plugins) != 2 {
		t.Fatalf("Plugins length = %d, want 2", len(described.Personality.Plugins))
	}
	if described.Personality.Plugins[0].Repository != "gsoci.azurecr.io/giantswarm/klaus-plugins/gs-base" {
		t.Errorf("Plugins[0].Repository = %q", described.Personality.Plugins[0].Repository)
	}
	if described.Personality.Plugins[1].Tag != "v1.2.0" {
		t.Errorf("Plugins[1].Tag = %q, want %q", described.Personality.Plugins[1].Tag, "v1.2.0")
	}
	if described.ArtifactInfo.Tag != "v1.0.0" {
		t.Errorf("Tag = %q, want %q", described.ArtifactInfo.Tag, "v1.0.0")
	}
	if described.ArtifactInfo.Ref != ref {
		t.Errorf("Ref = %q, want %q", described.ArtifactInfo.Ref, ref)
	}
	if described.ArtifactInfo.Digest == "" {
		t.Error("Digest should not be empty")
	}
}

func TestDescribePersonality_Minimal(t *testing.T) {
	blob := personalityConfigBlob{
		Toolchain: ToolchainReference{
			Repository: "gsoci.azurecr.io/giantswarm/klaus-toolchains/go",
			Tag:        "latest",
		},
	}
	configJSON, _ := json.Marshal(blob)
	annotations := buildKlausAnnotations(commonMetadata{Name: "go"})

	ts := newArtifactRegistry(map[string]testArtifactEntry{
		"giantswarm/klaus-personalities/go": {
			configJSON:      configJSON,
			configMediaType: MediaTypePersonalityConfig,
			tags:            []string{"v0.3.0"},
			annotations:     annotations,
		},
	})
	defer ts.Close()
	host := testRegistryHost(ts)

	client := NewClient(WithPlainHTTP(true))
	ref := host + "/giantswarm/klaus-personalities/go:v0.3.0"

	described, err := client.DescribePersonality(t.Context(), ref)
	if err != nil {
		t.Fatalf("DescribePersonality() error = %v", err)
	}

	if described.Personality.Name != "go" {
		t.Errorf("Name = %q, want %q", described.Personality.Name, "go")
	}
	if described.Personality.Version != "v0.3.0" {
		t.Errorf("Version = %q, want %q", described.Personality.Version, "v0.3.0")
	}
	if described.Personality.Author != nil {
		t.Errorf("Author = %+v, want nil", described.Personality.Author)
	}
	if len(described.Personality.Plugins) != 0 {
		t.Errorf("Plugins = %v, want empty", described.Personality.Plugins)
	}
}

func TestDescribeToolchain(t *testing.T) {
	annotations := map[string]string{
		AnnotationName:        "go",
		AnnotationDescription: "Go toolchain for Klaus",
		AnnotationAuthorName:  "Giant Swarm GmbH",
		AnnotationHomepage:    "https://docs.giantswarm.io/klaus/",
		AnnotationRepository:  "https://github.com/giantswarm/klaus-images",
		AnnotationLicense:     "Apache-2.0",
		AnnotationKeywords:    "giantswarm,go,toolchain",
	}

	ts := newArtifactRegistry(map[string]testArtifactEntry{
		"giantswarm/klaus-toolchains/go": {
			configJSON:      []byte(`{"architecture":"amd64"}`),
			configMediaType: ocispec.MediaTypeImageConfig,
			tags:            []string{"v1.2.0"},
			annotations:     annotations,
		},
	})
	defer ts.Close()
	host := testRegistryHost(ts)

	client := NewClient(WithPlainHTTP(true))
	ref := host + "/giantswarm/klaus-toolchains/go:v1.2.0"

	described, err := client.DescribeToolchain(t.Context(), ref)
	if err != nil {
		t.Fatalf("DescribeToolchain() error = %v", err)
	}

	if described.Toolchain.Name != "go" {
		t.Errorf("Name = %q, want %q", described.Toolchain.Name, "go")
	}
	if described.Toolchain.Version != "v1.2.0" {
		t.Errorf("Version = %q, want %q", described.Toolchain.Version, "v1.2.0")
	}
	if described.Toolchain.Description != "Go toolchain for Klaus" {
		t.Errorf("Description = %q", described.Toolchain.Description)
	}
	if described.Toolchain.Author == nil || described.Toolchain.Author.Name != "Giant Swarm GmbH" {
		t.Errorf("Author = %+v", described.Toolchain.Author)
	}
	if described.Toolchain.Homepage != "https://docs.giantswarm.io/klaus/" {
		t.Errorf("Homepage = %q", described.Toolchain.Homepage)
	}
	if described.Toolchain.SourceRepo != "https://github.com/giantswarm/klaus-images" {
		t.Errorf("SourceRepo = %q", described.Toolchain.SourceRepo)
	}
	if described.Toolchain.License != "Apache-2.0" {
		t.Errorf("License = %q", described.Toolchain.License)
	}
	if len(described.Toolchain.Keywords) != 3 || described.Toolchain.Keywords[0] != "giantswarm" {
		t.Errorf("Keywords = %v, want [giantswarm go toolchain]", described.Toolchain.Keywords)
	}
	if described.ArtifactInfo.Tag != "v1.2.0" {
		t.Errorf("Tag = %q, want %q", described.ArtifactInfo.Tag, "v1.2.0")
	}
	if described.ArtifactInfo.Ref != ref {
		t.Errorf("Ref = %q, want %q", described.ArtifactInfo.Ref, ref)
	}
	if described.ArtifactInfo.Digest == "" {
		t.Error("Digest should not be empty")
	}
}

func TestDescribeToolchain_Minimal(t *testing.T) {
	annotations := map[string]string{
		AnnotationName:        "python",
		AnnotationDescription: "Python toolchain",
	}

	ts := newArtifactRegistry(map[string]testArtifactEntry{
		"giantswarm/klaus-toolchains/python": {
			configJSON:      []byte(`{}`),
			configMediaType: ocispec.MediaTypeImageConfig,
			tags:            []string{"v0.5.0"},
			annotations:     annotations,
		},
	})
	defer ts.Close()
	host := testRegistryHost(ts)

	client := NewClient(WithPlainHTTP(true))
	ref := host + "/giantswarm/klaus-toolchains/python:v0.5.0"

	described, err := client.DescribeToolchain(t.Context(), ref)
	if err != nil {
		t.Fatalf("DescribeToolchain() error = %v", err)
	}

	if described.Toolchain.Name != "python" {
		t.Errorf("Name = %q, want %q", described.Toolchain.Name, "python")
	}
	if described.Toolchain.Version != "v0.5.0" {
		t.Errorf("Version = %q, want %q", described.Toolchain.Version, "v0.5.0")
	}
	if described.Toolchain.Description != "Python toolchain" {
		t.Errorf("Description = %q", described.Toolchain.Description)
	}
	if described.Toolchain.Author != nil {
		t.Errorf("Author = %+v, want nil", described.Toolchain.Author)
	}
	if described.Toolchain.Keywords != nil {
		t.Errorf("Keywords = %v, want nil", described.Toolchain.Keywords)
	}
	if described.Toolchain.Homepage != "" {
		t.Errorf("Homepage = %q, want empty", described.Toolchain.Homepage)
	}
}

func TestDescribePlugin_VersionFromTag(t *testing.T) {
	blob := pluginConfigBlob{}
	configJSON, _ := json.Marshal(blob)
	annotations := buildKlausAnnotations(commonMetadata{Name: "versioned-plugin"})

	ts := newArtifactRegistry(map[string]testArtifactEntry{
		"giantswarm/klaus-plugins/versioned-plugin": {
			configJSON:      configJSON,
			configMediaType: MediaTypePluginConfig,
			tags:            []string{"v2.5.0"},
			annotations:     annotations,
		},
	})
	defer ts.Close()
	host := testRegistryHost(ts)

	client := NewClient(WithPlainHTTP(true))
	ref := host + "/giantswarm/klaus-plugins/versioned-plugin:v2.5.0"

	described, err := client.DescribePlugin(t.Context(), ref)
	if err != nil {
		t.Fatalf("DescribePlugin() error = %v", err)
	}

	if described.Plugin.Version != "v2.5.0" {
		t.Errorf("Version = %q, want %q (from OCI tag)", described.Plugin.Version, "v2.5.0")
	}
}

func TestDescribePlugin_NotFound(t *testing.T) {
	ts := newArtifactRegistry(map[string]testArtifactEntry{})
	defer ts.Close()
	host := testRegistryHost(ts)

	client := NewClient(WithPlainHTTP(true))
	ref := host + "/giantswarm/klaus-plugins/nonexistent:v1.0.0"

	_, err := client.DescribePlugin(t.Context(), ref)
	if err == nil {
		t.Fatal("expected error for non-existent plugin")
	}
}

func TestDescribePlugin_InvalidConfigJSON(t *testing.T) {
	ts := newArtifactRegistry(map[string]testArtifactEntry{
		"giantswarm/klaus-plugins/bad-config": {
			configJSON:      []byte(`not valid json`),
			configMediaType: MediaTypePluginConfig,
			tags:            []string{"v1.0.0"},
		},
	})
	defer ts.Close()
	host := testRegistryHost(ts)

	client := NewClient(WithPlainHTTP(true))
	ref := host + "/giantswarm/klaus-plugins/bad-config:v1.0.0"

	_, err := client.DescribePlugin(t.Context(), ref)
	if err == nil {
		t.Fatal("expected error for invalid config JSON")
	}
}

func TestDescribePlugin_WithAllComponents(t *testing.T) {
	blob := pluginConfigBlob{
		Skills:     []string{"alpha", "beta"},
		Commands:   []string{"cmd-one", "cmd-two", "cmd-three"},
		Agents:     []string{"agent-a"},
		HasHooks:   true,
		MCPServers: []string{"server-x", "server-y"},
		LSPServers: []string{"lsp-z"},
	}
	configJSON, _ := json.Marshal(blob)
	annotations := buildKlausAnnotations(commonMetadata{
		Name:        "full-featured",
		Description: "A plugin with every component type",
		Author:      &Author{Name: "Test Author", Email: "test@example.com", URL: "https://example.com"},
		Homepage:    "https://docs.example.com",
		SourceRepo:  "https://github.com/example/repo",
		License:     "MIT",
		Keywords:    []string{"test", "full", "featured"},
	})

	ts := newArtifactRegistry(map[string]testArtifactEntry{
		"giantswarm/klaus-plugins/full-featured": {
			configJSON:      configJSON,
			configMediaType: MediaTypePluginConfig,
			tags:            []string{"v3.0.0"},
			annotations:     annotations,
		},
	})
	defer ts.Close()
	host := testRegistryHost(ts)

	client := NewClient(WithPlainHTTP(true))
	ref := host + "/giantswarm/klaus-plugins/full-featured:v3.0.0"

	described, err := client.DescribePlugin(t.Context(), ref)
	if err != nil {
		t.Fatalf("DescribePlugin() error = %v", err)
	}

	p := described.Plugin
	if p.Name != "full-featured" {
		t.Errorf("Name = %q", p.Name)
	}
	if p.Author.Email != "test@example.com" {
		t.Errorf("Author.Email = %q", p.Author.Email)
	}
	if p.Author.URL != "https://example.com" {
		t.Errorf("Author.URL = %q", p.Author.URL)
	}
	if p.Homepage != "https://docs.example.com" {
		t.Errorf("Homepage = %q", p.Homepage)
	}
	if len(p.Keywords) != 3 {
		t.Errorf("Keywords = %v, want 3 items", p.Keywords)
	}
	if len(p.Commands) != 3 {
		t.Errorf("Commands = %v, want 3 items", p.Commands)
	}
	if len(p.MCPServers) != 2 {
		t.Errorf("MCPServers = %v, want 2 items", p.MCPServers)
	}
	if len(p.LSPServers) != 1 || p.LSPServers[0] != "lsp-z" {
		t.Errorf("LSPServers = %v, want [lsp-z]", p.LSPServers)
	}
}

func TestDescribePersonality_VersionFromTag(t *testing.T) {
	blob := personalityConfigBlob{
		Toolchain: ToolchainReference{
			Repository: "gsoci.azurecr.io/giantswarm/klaus-toolchains/go",
			Tag:        "latest",
		},
	}
	configJSON, _ := json.Marshal(blob)
	annotations := buildKlausAnnotations(commonMetadata{Name: "versioned"})

	ts := newArtifactRegistry(map[string]testArtifactEntry{
		"giantswarm/klaus-personalities/versioned": {
			configJSON:      configJSON,
			configMediaType: MediaTypePersonalityConfig,
			tags:            []string{"v3.1.0"},
			annotations:     annotations,
		},
	})
	defer ts.Close()
	host := testRegistryHost(ts)

	client := NewClient(WithPlainHTTP(true))
	ref := host + "/giantswarm/klaus-personalities/versioned:v3.1.0"

	described, err := client.DescribePersonality(t.Context(), ref)
	if err != nil {
		t.Fatalf("DescribePersonality() error = %v", err)
	}

	if described.Personality.Version != "v3.1.0" {
		t.Errorf("Version = %q, want %q (from OCI tag)", described.Personality.Version, "v3.1.0")
	}
}

func TestDescribePersonality_NotFound(t *testing.T) {
	ts := newArtifactRegistry(map[string]testArtifactEntry{})
	defer ts.Close()
	host := testRegistryHost(ts)

	client := NewClient(WithPlainHTTP(true))
	ref := host + "/giantswarm/klaus-personalities/nonexistent:v1.0.0"

	_, err := client.DescribePersonality(t.Context(), ref)
	if err == nil {
		t.Fatal("expected error for non-existent personality")
	}
}

func TestDescribePersonality_InvalidConfigJSON(t *testing.T) {
	ts := newArtifactRegistry(map[string]testArtifactEntry{
		"giantswarm/klaus-personalities/bad": {
			configJSON:      []byte(`{invalid json}`),
			configMediaType: MediaTypePersonalityConfig,
			tags:            []string{"v1.0.0"},
		},
	})
	defer ts.Close()
	host := testRegistryHost(ts)

	client := NewClient(WithPlainHTTP(true))
	ref := host + "/giantswarm/klaus-personalities/bad:v1.0.0"

	_, err := client.DescribePersonality(t.Context(), ref)
	if err == nil {
		t.Fatal("expected error for invalid config JSON")
	}
}

func TestDescribePersonality_WithPinnedDeps(t *testing.T) {
	blob := personalityConfigBlob{
		Toolchain: ToolchainReference{
			Repository: "gsoci.azurecr.io/giantswarm/klaus-toolchains/go",
			Tag:        "v1.2.0",
		},
		Plugins: []PluginReference{
			{Repository: "gsoci.azurecr.io/giantswarm/klaus-plugins/gs-base", Tag: "v0.1.0"},
			{Repository: "gsoci.azurecr.io/giantswarm/klaus-plugins/gs-godev", Tag: "v0.1.0"},
			{Repository: "gsoci.azurecr.io/giantswarm/klaus-plugins/gs-product", Tag: "v0.1.0"},
		},
	}
	configJSON, _ := json.Marshal(blob)
	annotations := buildKlausAnnotations(commonMetadata{
		Name:        "program-manager",
		Description: "Program manager personality",
		Author:      &Author{Name: "Giant Swarm GmbH"},
		SourceRepo:  "https://github.com/giantswarm/klaus-personalities",
		License:     "Apache-2.0",
		Keywords:    []string{"giantswarm", "management"},
	})

	ts := newArtifactRegistry(map[string]testArtifactEntry{
		"giantswarm/klaus-personalities/program-manager": {
			configJSON:      configJSON,
			configMediaType: MediaTypePersonalityConfig,
			tags:            []string{"v2.0.0"},
			annotations:     annotations,
		},
	})
	defer ts.Close()
	host := testRegistryHost(ts)

	client := NewClient(WithPlainHTTP(true))
	ref := host + "/giantswarm/klaus-personalities/program-manager:v2.0.0"

	described, err := client.DescribePersonality(t.Context(), ref)
	if err != nil {
		t.Fatalf("DescribePersonality() error = %v", err)
	}

	if described.Personality.Version != "v2.0.0" {
		t.Errorf("Version = %q, want %q", described.Personality.Version, "v2.0.0")
	}
	if len(described.Personality.Plugins) != 3 {
		t.Fatalf("Plugins length = %d, want 3", len(described.Personality.Plugins))
	}
	for i, p := range described.Personality.Plugins {
		if p.Tag != "v0.1.0" {
			t.Errorf("Plugins[%d].Tag = %q, want %q", i, p.Tag, "v0.1.0")
		}
	}
	if described.Personality.Toolchain.Tag != "v1.2.0" {
		t.Errorf("Toolchain.Tag = %q, want %q", described.Personality.Toolchain.Tag, "v1.2.0")
	}
}

func TestDescribeToolchain_NotFound(t *testing.T) {
	ts := newArtifactRegistry(map[string]testArtifactEntry{})
	defer ts.Close()
	host := testRegistryHost(ts)

	client := NewClient(WithPlainHTTP(true))
	ref := host + "/giantswarm/klaus-toolchains/nonexistent:v1.0.0"

	_, err := client.DescribeToolchain(t.Context(), ref)
	if err == nil {
		t.Fatal("expected error for non-existent toolchain")
	}
}

func TestDescribeToolchain_NoAnnotations(t *testing.T) {
	ts := newArtifactRegistry(map[string]testArtifactEntry{
		"giantswarm/klaus-toolchains/bare": {
			configJSON:      []byte(`{}`),
			configMediaType: ocispec.MediaTypeImageConfig,
			tags:            []string{"v0.1.0"},
		},
	})
	defer ts.Close()
	host := testRegistryHost(ts)

	client := NewClient(WithPlainHTTP(true))
	ref := host + "/giantswarm/klaus-toolchains/bare:v0.1.0"

	described, err := client.DescribeToolchain(t.Context(), ref)
	if err != nil {
		t.Fatalf("DescribeToolchain() error = %v", err)
	}

	if described.Toolchain.Name != "" {
		t.Errorf("Name = %q, want empty (no annotations)", described.Toolchain.Name)
	}
	if described.Toolchain.Description != "" {
		t.Errorf("Description = %q, want empty", described.Toolchain.Description)
	}
	if described.Toolchain.Author != nil {
		t.Errorf("Author = %+v, want nil", described.Toolchain.Author)
	}
	if described.Toolchain.Version != "v0.1.0" {
		t.Errorf("Version = %q, want %q (from OCI tag)", described.Toolchain.Version, "v0.1.0")
	}
}

func TestDescribeToolchain_VersionFromTag(t *testing.T) {
	annotations := map[string]string{
		AnnotationName:                     "go",
		"org.opencontainers.image.version": "v999.0.0",
	}

	ts := newArtifactRegistry(map[string]testArtifactEntry{
		"giantswarm/klaus-toolchains/go": {
			configJSON:      []byte(`{}`),
			configMediaType: ocispec.MediaTypeImageConfig,
			tags:            []string{"v1.2.0"},
			annotations:     annotations,
		},
	})
	defer ts.Close()
	host := testRegistryHost(ts)

	client := NewClient(WithPlainHTTP(true))
	ref := host + "/giantswarm/klaus-toolchains/go:v1.2.0"

	described, err := client.DescribeToolchain(t.Context(), ref)
	if err != nil {
		t.Fatalf("DescribeToolchain() error = %v", err)
	}

	if described.Toolchain.Version != "v1.2.0" {
		t.Errorf("Version = %q, want %q (from OCI tag, not annotation)", described.Toolchain.Version, "v1.2.0")
	}
}

func TestToolchainFromAnnotations_SingleKeyword(t *testing.T) {
	annotations := map[string]string{
		AnnotationName:     "test",
		AnnotationKeywords: "single",
	}

	tc := toolchainFromAnnotations(annotations)

	if len(tc.Keywords) != 1 || tc.Keywords[0] != "single" {
		t.Errorf("Keywords = %v, want [single]", tc.Keywords)
	}
}
