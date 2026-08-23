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

		if path == testPathV2Slash || path == testPathV2 {
			w.WriteHeader(http.StatusOK)
			return
		}

		if !strings.HasPrefix(path, testPathV2Slash) {
			http.NotFound(w, r)
			return
		}

		rest := strings.TrimPrefix(path, testPathV2Slash)

		if strings.HasSuffix(rest, "/tags/list") {
			repoName := strings.TrimSuffix(rest, "/tags/list")
			art, ok := built[repoName]
			if !ok {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{testKeyName: repoName, testKeyTags: art.tags})
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
			_, _ = w.Write(art.manifestJSON)
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
				_, _ = w.Write(art.configJSON)
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
			AnnotationDescription: testDescGoToolchainKlaus,
			AnnotationAuthorName:  testAuthorGiantSwarmGmbH,
			AnnotationHomepage:    testDocsURL,
			AnnotationRepository:  testSourceKlausImages,
			AnnotationLicense:     testLicenseApache2,
			AnnotationKeywords:    "giantswarm,go,toolchain",
		}

		tc := toolchainFromAnnotations(annotations)

		if tc.Name != "go" {
			t.Errorf("Name = %q, want %q", tc.Name, "go")
		}
		if tc.Description != testDescGoToolchainKlaus {
			t.Errorf("Description = %q, want %q", tc.Description, testDescGoToolchainKlaus)
		}
		if tc.Author == nil || tc.Author.Name != testAuthorGiantSwarmGmbH {
			t.Errorf("Author = %+v, want name 'Giant Swarm GmbH'", tc.Author)
		}
		if tc.Homepage != testDocsURL {
			t.Errorf("Homepage = %q", tc.Homepage)
		}
		if tc.SourceRepo != testSourceKlausImages {
			t.Errorf("SourceRepo = %q", tc.SourceRepo)
		}
		if tc.License != testLicenseApache2 {
			t.Errorf("License = %q", tc.License)
		}
		if len(tc.Keywords) != 3 || tc.Keywords[0] != testOrgGiantSwarm || tc.Keywords[1] != "go" || tc.Keywords[2] != testKindToolchain {
			t.Errorf("Keywords = %v, want [giantswarm go toolchain]", tc.Keywords)
		}
		if tc.Version != "" {
			t.Errorf("Version = %q, want empty (set by caller, not annotations)", tc.Version)
		}
	})

	t.Run("minimal annotations", func(t *testing.T) {
		annotations := map[string]string{
			AnnotationName: testNamePython,
		}

		tc := toolchainFromAnnotations(annotations)

		if tc.Name != testNamePython {
			t.Errorf("Name = %q, want %q", tc.Name, testNamePython)
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
			"org.opencontainers.image.version": testTagV120,
		}

		tc := toolchainFromAnnotations(annotations)

		if tc.Version != "" {
			t.Errorf("Version = %q, want empty (version comes from OCI tag)", tc.Version)
		}
	})

	t.Run("keywords whitespace trimmed", func(t *testing.T) {
		annotations := map[string]string{
			AnnotationName:     "go",
			AnnotationKeywords: "giantswarm, go , toolchain",
		}

		tc := toolchainFromAnnotations(annotations)

		if len(tc.Keywords) != 3 {
			t.Fatalf("Keywords length = %d, want 3", len(tc.Keywords))
		}
		for i, want := range []string{testOrgGiantSwarm, "go", testKindToolchain} {
			if tc.Keywords[i] != want {
				t.Errorf("Keywords[%d] = %q, want %q", i, tc.Keywords[i], want)
			}
		}
	})
}

func TestDescribePlugin(t *testing.T) {
	blob := pluginConfigBlob{
		Skills:     []string{testKeywordKubernetes, testNameFluxCD},
		Commands:   []string{testValueHello, testNameInitKubernetes},
		Agents:     []string{testNameCodeReviewer},
		HasHooks:   true,
		MCPServers: []string{testNameGitHub},
	}
	configJSON, _ := json.Marshal(blob)
	annotations := buildKlausAnnotations(commonMetadata{
		Name:        testNameGSBase,
		Description: testDescGeneralPlugin,
		Author:      &Author{Name: testAuthorGiantSwarmGmbH},
		SourceRepo:  testSourceClaudeCode,
		License:     testLicenseApache2,
		Keywords:    []string{testOrgGiantSwarm, testNamePlatform},
	})

	ts := newArtifactRegistry(map[string]testArtifactEntry{
		testRepoPluginGSBase: {
			configJSON:      configJSON,
			configMediaType: MediaTypePluginConfig,
			tags:            []string{testTagV100},
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

	if described.Name != testNameGSBase {
		t.Errorf("Name = %q, want %q", described.Name, testNameGSBase)
	}
	if described.Version != testTagV100 {
		t.Errorf("Version = %q, want %q", described.Version, testTagV100)
	}
	if described.Description != testDescGeneralPlugin {
		t.Errorf("Description = %q", described.Description)
	}
	if described.Author == nil || described.Author.Name != testAuthorGiantSwarmGmbH {
		t.Errorf("Author = %+v", described.Author)
	}
	if described.SourceRepo != testSourceClaudeCode {
		t.Errorf("SourceRepo = %q", described.SourceRepo)
	}
	if described.License != testLicenseApache2 {
		t.Errorf("License = %q", described.License)
	}
	if len(described.Skills) != 2 {
		t.Errorf("Skills = %v, want 2 items", described.Skills)
	}
	if len(described.Commands) != 2 {
		t.Errorf("Commands = %v, want 2 items", described.Commands)
	}
	if len(described.Agents) != 1 || described.Agents[0] != testNameCodeReviewer {
		t.Errorf("Agents = %v, want [code-reviewer]", described.Agents)
	}
	if !described.HasHooks {
		t.Error("HasHooks = false, want true")
	}
	if len(described.MCPServers) != 1 || described.MCPServers[0] != testNameGitHub {
		t.Errorf("MCPServers = %v, want [github]", described.MCPServers)
	}
	if described.Tag != testTagV100 {
		t.Errorf("Tag = %q, want %q", described.Tag, testTagV100)
	}
	if described.Ref != ref {
		t.Errorf("Ref = %q, want %q", described.Ref, ref)
	}
	if described.Digest == "" {
		t.Error("Digest should not be empty")
	}
}

func TestDescribePlugin_Minimal(t *testing.T) {
	blob := pluginConfigBlob{
		Commands: []string{"commit", "push", "pr"},
	}
	configJSON, _ := json.Marshal(blob)
	annotations := buildKlausAnnotations(commonMetadata{Name: testNameCommitCommands})

	ts := newArtifactRegistry(map[string]testArtifactEntry{
		"giantswarm/klaus-plugins/commit-commands": {
			configJSON:      configJSON,
			configMediaType: MediaTypePluginConfig,
			tags:            []string{testTagV100},
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

	if described.Name != testNameCommitCommands {
		t.Errorf("Name = %q, want %q", described.Name, testNameCommitCommands)
	}
	if described.Version != testTagV100 {
		t.Errorf("Version = %q, want %q", described.Version, testTagV100)
	}
	if described.Author != nil {
		t.Errorf("Author = %+v, want nil", described.Author)
	}
	if len(described.Commands) != 3 {
		t.Errorf("Commands = %v, want 3 items", described.Commands)
	}
}

func TestDescribePersonality(t *testing.T) {
	blob := personalityConfigBlob{
		Toolchain: ToolchainReference{
			Repository: testRefToolchainGo,
			Tag:        testTagV100,
		},
		Plugins: []PluginReference{
			{Repository: testRefPluginGSBase, Tag: testTagLatest},
			{Repository: testRefPluginGSSRE, Tag: testTagV120},
		},
	}
	configJSON, _ := json.Marshal(blob)
	annotations := buildKlausAnnotations(commonMetadata{
		Name:        testNameSRE,
		Description: testDescSREPersonality,
		Author:      &Author{Name: testAuthorGiantSwarmGmbH},
		SourceRepo:  testSourcePersonalities,
		License:     testLicenseApache2,
		Keywords:    []string{testOrgGiantSwarm, testNameSRE, testKeywordKubernetes},
	})

	ts := newArtifactRegistry(map[string]testArtifactEntry{
		testRepoPersonalitySRE: {
			configJSON:      configJSON,
			configMediaType: MediaTypePersonalityConfig,
			tags:            []string{testTagV100},
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

	if described.Name != testNameSRE {
		t.Errorf("Name = %q, want %q", described.Name, testNameSRE)
	}
	if described.Version != testTagV100 {
		t.Errorf("Version = %q, want %q", described.Version, testTagV100)
	}
	if described.Description != testDescSREPersonality {
		t.Errorf("Description = %q", described.Description)
	}
	if described.Author == nil || described.Author.Name != testAuthorGiantSwarmGmbH {
		t.Errorf("Author = %+v", described.Author)
	}
	if described.SourceRepo != testSourcePersonalities {
		t.Errorf("SourceRepo = %q", described.SourceRepo)
	}
	if described.License != testLicenseApache2 {
		t.Errorf("License = %q", described.License)
	}
	if len(described.Keywords) != 3 {
		t.Errorf("Keywords = %v, want 3 items", described.Keywords)
	}
	if described.Toolchain.Repository != testRefToolchainGo {
		t.Errorf("Toolchain.Repository = %q", described.Toolchain.Repository)
	}
	if described.Toolchain.Tag != testTagV100 {
		t.Errorf("Toolchain.Tag = %q, want %q", described.Toolchain.Tag, testTagV100)
	}
	if len(described.Plugins) != 2 {
		t.Fatalf("Plugins length = %d, want 2", len(described.Plugins))
	}
	if described.Personality.Plugins[0].Repository != testRefPluginGSBase {
		t.Errorf("Plugins[0].Repository = %q", described.Personality.Plugins[0].Repository)
	}
	if described.Personality.Plugins[1].Tag != testTagV120 {
		t.Errorf("Plugins[1].Tag = %q, want %q", described.Personality.Plugins[1].Tag, testTagV120)
	}
	if described.Tag != testTagV100 {
		t.Errorf("Tag = %q, want %q", described.Tag, testTagV100)
	}
	if described.Ref != ref {
		t.Errorf("Ref = %q, want %q", described.Ref, ref)
	}
	if described.Digest == "" {
		t.Error("Digest should not be empty")
	}
}

func TestDescribePersonality_Minimal(t *testing.T) {
	blob := personalityConfigBlob{
		Toolchain: ToolchainReference{
			Repository: testRefToolchainGo,
			Tag:        testTagLatest,
		},
	}
	configJSON, _ := json.Marshal(blob)
	annotations := buildKlausAnnotations(commonMetadata{Name: "go"})

	ts := newArtifactRegistry(map[string]testArtifactEntry{
		"giantswarm/klaus-personalities/go": {
			configJSON:      configJSON,
			configMediaType: MediaTypePersonalityConfig,
			tags:            []string{testTagV030},
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

	if described.Name != "go" {
		t.Errorf("Name = %q, want %q", described.Name, "go")
	}
	if described.Version != testTagV030 {
		t.Errorf("Version = %q, want %q", described.Version, testTagV030)
	}
	if described.Author != nil {
		t.Errorf("Author = %+v, want nil", described.Author)
	}
	if len(described.Plugins) != 0 {
		t.Errorf("Plugins = %v, want empty", described.Plugins)
	}
}

func TestDescribeToolchain(t *testing.T) {
	annotations := map[string]string{
		AnnotationName:        "go",
		AnnotationDescription: testDescGoToolchainKlaus,
		AnnotationAuthorName:  testAuthorGiantSwarmGmbH,
		AnnotationHomepage:    testDocsURL,
		AnnotationRepository:  testSourceKlausImages,
		AnnotationLicense:     testLicenseApache2,
		AnnotationKeywords:    "giantswarm,go,toolchain",
	}

	ts := newArtifactRegistry(map[string]testArtifactEntry{
		testRepoToolchainGo: {
			configJSON:      []byte(`{"architecture":"amd64"}`),
			configMediaType: ocispec.MediaTypeImageConfig,
			tags:            []string{testTagV120},
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

	if described.Name != "go" {
		t.Errorf("Name = %q, want %q", described.Name, "go")
	}
	if described.Version != testTagV120 {
		t.Errorf("Version = %q, want %q", described.Version, testTagV120)
	}
	if described.Description != testDescGoToolchainKlaus {
		t.Errorf("Description = %q", described.Description)
	}
	if described.Author == nil || described.Author.Name != testAuthorGiantSwarmGmbH {
		t.Errorf("Author = %+v", described.Author)
	}
	if described.Homepage != testDocsURL {
		t.Errorf("Homepage = %q", described.Homepage)
	}
	if described.SourceRepo != testSourceKlausImages {
		t.Errorf("SourceRepo = %q", described.SourceRepo)
	}
	if described.License != testLicenseApache2 {
		t.Errorf("License = %q", described.License)
	}
	if len(described.Keywords) != 3 || described.Keywords[0] != testOrgGiantSwarm {
		t.Errorf("Keywords = %v, want [giantswarm go toolchain]", described.Keywords)
	}
	if described.Tag != testTagV120 {
		t.Errorf("Tag = %q, want %q", described.Tag, testTagV120)
	}
	if described.Ref != ref {
		t.Errorf("Ref = %q, want %q", described.Ref, ref)
	}
	if described.Digest == "" {
		t.Error("Digest should not be empty")
	}
}

func TestDescribeToolchain_Minimal(t *testing.T) {
	annotations := map[string]string{
		AnnotationName:        testNamePython,
		AnnotationDescription: "Python toolchain",
	}

	ts := newArtifactRegistry(map[string]testArtifactEntry{
		"giantswarm/klaus-toolchains/python": {
			configJSON:      []byte(`{}`),
			configMediaType: ocispec.MediaTypeImageConfig,
			tags:            []string{testTagV050},
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

	if described.Name != testNamePython {
		t.Errorf("Name = %q, want %q", described.Name, testNamePython)
	}
	if described.Version != testTagV050 {
		t.Errorf("Version = %q, want %q", described.Version, testTagV050)
	}
	if described.Description != "Python toolchain" {
		t.Errorf("Description = %q", described.Description)
	}
	if described.Author != nil {
		t.Errorf("Author = %+v, want nil", described.Author)
	}
	if described.Keywords != nil {
		t.Errorf("Keywords = %v, want nil", described.Keywords)
	}
	if described.Homepage != "" {
		t.Errorf("Homepage = %q, want empty", described.Homepage)
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

	if described.Version != "v2.5.0" {
		t.Errorf("Version = %q, want %q (from OCI tag)", described.Version, "v2.5.0")
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
			tags:            []string{testTagV100},
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
		Skills:     []string{testTagAlpha, testTagBeta},
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
		Author:      &Author{Name: "Test Author", Email: testEmail, URL: testURLExample},
		Homepage:    "https://docs.example.com",
		SourceRepo:  "https://github.com/example/repo",
		License:     "MIT",
		Keywords:    []string{testNameTest, "full", "featured"},
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
	if p.Author.Email != testEmail {
		t.Errorf("Author.Email = %q", p.Author.Email)
	}
	if p.Author.URL != testURLExample {
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
			Repository: testRefToolchainGo,
			Tag:        testTagLatest,
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

	if described.Version != "v3.1.0" {
		t.Errorf("Version = %q, want %q (from OCI tag)", described.Version, "v3.1.0")
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
			tags:            []string{testTagV100},
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
			Repository: testRefToolchainGo,
			Tag:        testTagV120,
		},
		Plugins: []PluginReference{
			{Repository: testRefPluginGSBase, Tag: testTagV010},
			{Repository: "gsoci.azurecr.io/giantswarm/klaus-plugins/gs-godev", Tag: testTagV010},
			{Repository: "gsoci.azurecr.io/giantswarm/klaus-plugins/gs-product", Tag: testTagV010},
		},
	}
	configJSON, _ := json.Marshal(blob)
	annotations := buildKlausAnnotations(commonMetadata{
		Name:        "program-manager",
		Description: "Program manager personality",
		Author:      &Author{Name: testAuthorGiantSwarmGmbH},
		SourceRepo:  testSourcePersonalities,
		License:     testLicenseApache2,
		Keywords:    []string{testOrgGiantSwarm, "management"},
	})

	ts := newArtifactRegistry(map[string]testArtifactEntry{
		"giantswarm/klaus-personalities/program-manager": {
			configJSON:      configJSON,
			configMediaType: MediaTypePersonalityConfig,
			tags:            []string{testTagV200},
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

	if described.Version != testTagV200 {
		t.Errorf("Version = %q, want %q", described.Version, testTagV200)
	}
	if len(described.Plugins) != 3 {
		t.Fatalf("Plugins length = %d, want 3", len(described.Plugins))
	}
	for i, p := range described.Plugins {
		if p.Tag != testTagV010 {
			t.Errorf("Plugins[%d].Tag = %q, want %q", i, p.Tag, testTagV010)
		}
	}
	if described.Toolchain.Tag != testTagV120 {
		t.Errorf("Toolchain.Tag = %q, want %q", described.Toolchain.Tag, testTagV120)
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
			tags:            []string{testTagV010},
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

	if described.Name != "" {
		t.Errorf("Name = %q, want empty (no annotations)", described.Name)
	}
	if described.Description != "" {
		t.Errorf("Description = %q, want empty", described.Description)
	}
	if described.Author != nil {
		t.Errorf("Author = %+v, want nil", described.Author)
	}
	if described.Version != testTagV010 {
		t.Errorf("Version = %q, want %q (from OCI tag)", described.Version, testTagV010)
	}
}

func TestDescribeToolchain_VersionFromTag(t *testing.T) {
	annotations := map[string]string{
		AnnotationName:                     "go",
		"org.opencontainers.image.version": "v999.0.0",
	}

	ts := newArtifactRegistry(map[string]testArtifactEntry{
		testRepoToolchainGo: {
			configJSON:      []byte(`{}`),
			configMediaType: ocispec.MediaTypeImageConfig,
			tags:            []string{testTagV120},
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

	if described.Version != testTagV120 {
		t.Errorf("Version = %q, want %q (from OCI tag, not annotation)", described.Version, testTagV120)
	}
}

func TestToolchainFromAnnotations_SingleKeyword(t *testing.T) {
	annotations := map[string]string{
		AnnotationName:     testNameTest,
		AnnotationKeywords: "single",
	}

	tc := toolchainFromAnnotations(annotations)

	if len(tc.Keywords) != 1 || tc.Keywords[0] != "single" {
		t.Errorf("Keywords = %v, want [single]", tc.Keywords)
	}
}
