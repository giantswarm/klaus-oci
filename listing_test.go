package oci

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"sort"
	"strings"
	"testing"
)

// newTestRegistry creates a minimal OCI distribution API server backed by the
// given repository map (repo name -> tags). It supports the catalog and tag
// listing endpoints used by listRepositories and listArtifacts.
func newTestRegistry(repos map[string][]string) *httptest.Server {
	var sortedNames []string
	for name := range repos {
		sortedNames = append(sortedNames, name)
	}
	sort.Strings(sortedNames)

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == testPathV2Slash || r.URL.Path == testPathV2:
			w.WriteHeader(http.StatusOK)

		case r.URL.Path == "/v2/_catalog":
			last := r.URL.Query().Get("last")
			var result []string
			for _, name := range sortedNames {
				if last == "" || name > last {
					result = append(result, name)
				}
			}
			if result == nil {
				result = []string{}
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string][]string{"repositories": result})

		case strings.HasSuffix(r.URL.Path, "/tags/list"):
			repoName := strings.TrimPrefix(r.URL.Path, testPathV2Slash)
			repoName = strings.TrimSuffix(repoName, "/tags/list")
			tags, ok := repos[repoName]
			if !ok {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{testKeyName: repoName, testKeyTags: tags})

		default:
			http.NotFound(w, r)
		}
	}))
}

func testRegistryHost(ts *httptest.Server) string {
	return strings.TrimPrefix(ts.URL, "http://")
}

func TestListRepositories(t *testing.T) {
	ts := newTestRegistry(map[string][]string{
		"giantswarm/aaa":                     {},
		"giantswarm/klaus-plugins-extra/foo": {testTagV100},
		testRepoPluginGSBase:                 {testTagV010},
		testRepoPluginGSPlatform:             {testTagV020},
		"giantswarm/zzz":                     {},
	})
	defer ts.Close()
	host := testRegistryHost(ts)

	client := NewClient(WithPlainHTTP(true))

	tests := []struct {
		name         string
		registryBase string
		want         []string
	}{
		{
			name:         "matches only repos under prefix",
			registryBase: host + "/giantswarm/klaus-plugins",
			want: []string{
				host + "/giantswarm/klaus-plugins/gs-base",
				host + "/giantswarm/klaus-plugins/gs-platform",
			},
		},
		{
			name:         "similar prefix not included",
			registryBase: host + "/giantswarm/klaus-plugins-extra",
			want: []string{
				host + "/giantswarm/klaus-plugins-extra/foo",
			},
		},
		{
			name:         "no matches returns nil",
			registryBase: host + "/giantswarm/nonexistent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := client.listRepositories(t.Context(), tt.registryBase)
			if err != nil {
				t.Fatalf("listRepositories() error = %v", err)
			}
			sort.Strings(got)
			sort.Strings(tt.want)
			if !slices.Equal(got, tt.want) {
				t.Errorf("listRepositories() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestListArtifacts(t *testing.T) {
	ts := newTestRegistry(map[string][]string{
		testRepoPluginGSBase:                 {testTagV010, testTagV020},
		testRepoPluginGSPlatform:             {testTagV100, testTagV110},
		"giantswarm/klaus-plugins/no-semver": {testTagLatest, testTagDev},
	})
	defer ts.Close()
	host := testRegistryHost(ts)
	base := host + "/giantswarm/klaus-plugins"

	client := NewClient(WithPlainHTTP(true))

	t.Run("discovers artifacts with latest semver", func(t *testing.T) {
		artifacts, err := client.listArtifacts(t.Context(), base)
		if err != nil {
			t.Fatalf("listArtifacts() error = %v", err)
		}
		if len(artifacts) != 2 {
			t.Fatalf("expected 2 artifacts, got %d: %v", len(artifacts), artifacts)
		}
		if !strings.HasSuffix(artifacts[0].Repository, testNameGSBase) {
			t.Errorf("artifacts[0].Repository = %q, want suffix gs-base", artifacts[0].Repository)
		}
		if !strings.HasSuffix(artifacts[0].Reference, ":v0.2.0") {
			t.Errorf("artifacts[0].Reference = %q, want suffix :v0.2.0", artifacts[0].Reference)
		}
		if !strings.HasSuffix(artifacts[1].Repository, testNameGSPlatform) {
			t.Errorf("artifacts[1].Repository = %q, want suffix gs-platform", artifacts[1].Repository)
		}
		if !strings.HasSuffix(artifacts[1].Reference, ":v1.1.0") {
			t.Errorf("artifacts[1].Reference = %q, want suffix :v1.1.0", artifacts[1].Reference)
		}
	})

	t.Run("WithFilter keeps only matching repos", func(t *testing.T) {
		artifacts, err := client.listArtifacts(t.Context(), base,
			WithFilter(func(repo string) bool {
				return strings.HasSuffix(repo, testNameGSBase)
			}),
		)
		if err != nil {
			t.Fatalf("listArtifacts() error = %v", err)
		}
		if len(artifacts) != 1 {
			t.Fatalf("expected 1 artifact, got %d", len(artifacts))
		}
		if !strings.HasSuffix(artifacts[0].Repository, testNameGSBase) {
			t.Errorf("artifact = %q, want suffix gs-base", artifacts[0].Repository)
		}
	})

	t.Run("WithFilter rejecting all returns empty", func(t *testing.T) {
		artifacts, err := client.listArtifacts(t.Context(), base,
			WithFilter(func(string) bool { return false }),
		)
		if err != nil {
			t.Fatalf("listArtifacts() error = %v", err)
		}
		if len(artifacts) != 0 {
			t.Errorf("expected 0 artifacts, got %d", len(artifacts))
		}
	})
}

func TestListPersonalities(t *testing.T) {
	ts := newTestRegistry(map[string][]string{
		testRepoPersonalitySRE:                     {testTagV010, testTagV020},
		"giantswarm/klaus-personalities/engineer":  {testTagV100},
		"giantswarm/klaus-personalities/no-semver": {testTagLatest},
	})
	defer ts.Close()
	host := testRegistryHost(ts)

	client := NewClient(WithPlainHTTP(true))

	t.Run("discovers personalities with name and version", func(t *testing.T) {
		personalities, err := client.ListPersonalities(t.Context(),
			WithRegistry(host+"/giantswarm/klaus-personalities"))
		if err != nil {
			t.Fatalf("ListPersonalities() error = %v", err)
		}
		if len(personalities) != 2 {
			t.Fatalf("expected 2 personalities, got %d", len(personalities))
		}

		if personalities[0].Name != "engineer" {
			t.Errorf("personalities[0].Name = %q, want %q", personalities[0].Name, "engineer")
		}
		if personalities[0].Version != testTagV100 {
			t.Errorf("personalities[0].Version = %q, want %q", personalities[0].Version, testTagV100)
		}
		if personalities[1].Name != testNameSRE {
			t.Errorf("personalities[1].Name = %q, want %q", personalities[1].Name, testNameSRE)
		}
		if personalities[1].Version != testTagV020 {
			t.Errorf("personalities[1].Version = %q, want %q", personalities[1].Version, testTagV020)
		}
	})

	t.Run("WithFilter narrows results", func(t *testing.T) {
		personalities, err := client.ListPersonalities(t.Context(),
			WithRegistry(host+"/giantswarm/klaus-personalities"),
			WithFilter(func(repo string) bool {
				return strings.HasSuffix(repo, testNameSRE)
			}),
		)
		if err != nil {
			t.Fatalf("ListPersonalities() error = %v", err)
		}
		if len(personalities) != 1 {
			t.Fatalf("expected 1 personality, got %d", len(personalities))
		}
		if personalities[0].Name != testNameSRE {
			t.Errorf("Name = %q, want %q", personalities[0].Name, testNameSRE)
		}
	})
}

func TestListPlugins(t *testing.T) {
	ts := newTestRegistry(map[string][]string{
		testRepoPluginGSBase:     {testTagV010, testTagV020},
		testRepoPluginGSPlatform: {testTagV100},
	})
	defer ts.Close()
	host := testRegistryHost(ts)

	client := NewClient(WithPlainHTTP(true))

	plugins, err := client.ListPlugins(t.Context(),
		WithRegistry(host+"/giantswarm/klaus-plugins"))
	if err != nil {
		t.Fatalf("ListPlugins() error = %v", err)
	}
	if len(plugins) != 2 {
		t.Fatalf("expected 2 plugins, got %d", len(plugins))
	}
	if plugins[0].Name != testNameGSBase {
		t.Errorf("plugins[0].Name = %q, want %q", plugins[0].Name, testNameGSBase)
	}
	if plugins[0].Version != testTagV020 {
		t.Errorf("plugins[0].Version = %q, want %q", plugins[0].Version, testTagV020)
	}
	if plugins[1].Name != testNameGSPlatform {
		t.Errorf("plugins[1].Name = %q, want %q", plugins[1].Name, testNameGSPlatform)
	}
}

func TestListToolchains(t *testing.T) {
	ts := newTestRegistry(map[string][]string{
		testRepoToolchainGo:                  {testTagV100, testTagV110},
		"giantswarm/klaus-toolchains/python": {testTagV050},
	})
	defer ts.Close()
	host := testRegistryHost(ts)

	client := NewClient(WithPlainHTTP(true))

	toolchains, err := client.ListToolchains(t.Context(),
		WithRegistry(host+"/giantswarm/klaus-toolchains"))
	if err != nil {
		t.Fatalf("ListToolchains() error = %v", err)
	}
	if len(toolchains) != 2 {
		t.Fatalf("expected 2 toolchains, got %d", len(toolchains))
	}
	if toolchains[0].Name != "go" {
		t.Errorf("toolchains[0].Name = %q, want %q", toolchains[0].Name, "go")
	}
	if toolchains[0].Version != testTagV110 {
		t.Errorf("toolchains[0].Version = %q, want %q", toolchains[0].Version, testTagV110)
	}
	if toolchains[1].Name != testNamePython {
		t.Errorf("toolchains[1].Name = %q, want %q", toolchains[1].Name, testNamePython)
	}
}

func TestWithRegistry(t *testing.T) {
	ts := newTestRegistry(map[string][]string{
		"custom/team/plugins/alpha": {testTagV200},
		"custom/team/plugins/beta":  {"v3.0.0"},
	})
	defer ts.Close()
	host := testRegistryHost(ts)

	client := NewClient(WithPlainHTTP(true))

	plugins, err := client.ListPlugins(t.Context(),
		WithRegistry(host+"/custom/team/plugins"))
	if err != nil {
		t.Fatalf("ListPlugins() error = %v", err)
	}
	if len(plugins) != 2 {
		t.Fatalf("expected 2 plugins from custom registry, got %d", len(plugins))
	}
	if plugins[0].Name != testTagAlpha {
		t.Errorf("plugins[0].Name = %q, want %q", plugins[0].Name, testTagAlpha)
	}
	if plugins[1].Name != testTagBeta {
		t.Errorf("plugins[1].Name = %q, want %q", plugins[1].Name, testTagBeta)
	}
}

func TestListPluginVersions(t *testing.T) {
	ts := newTestRegistry(map[string][]string{
		testRepoPluginGSBase: {testTagV010, testTagV030, testTagV020, testTagLatest, testTagDev},
	})
	defer ts.Close()
	host := testRegistryHost(ts)

	client := NewClient(WithPlainHTTP(true))

	t.Run("short name with default registry fails for test server", func(t *testing.T) {
		_, err := client.ListPluginVersions(t.Context(), testNameGSBase)
		if err == nil {
			t.Fatal("expected error when short name resolves to unreachable default registry")
		}
	})

	t.Run("full repo returns semver tags descending", func(t *testing.T) {
		versions, err := client.ListPluginVersions(t.Context(),
			host+"/giantswarm/klaus-plugins/gs-base",
		)
		if err != nil {
			t.Fatalf("ListPluginVersions() error = %v", err)
		}
		want := []string{testTagV030, testTagV020, testTagV010}
		if !slices.Equal(versions, want) {
			t.Errorf("ListPluginVersions() = %v, want %v", versions, want)
		}
	})

	t.Run("empty ref returns error", func(t *testing.T) {
		_, err := client.ListPluginVersions(t.Context(), "")
		if err == nil {
			t.Fatal("expected error for empty ref")
		}
	})

	t.Run("whitespace-only ref returns error", func(t *testing.T) {
		_, err := client.ListPluginVersions(t.Context(), "   ")
		if err == nil {
			t.Fatal("expected error for whitespace-only ref")
		}
	})
}

func TestListPersonalityVersions(t *testing.T) {
	ts := newTestRegistry(map[string][]string{
		testRepoPersonalitySRE: {testTagV100, testTagV020, testTagV010, testTagLatest},
	})
	defer ts.Close()
	host := testRegistryHost(ts)

	client := NewClient(WithPlainHTTP(true))

	versions, err := client.ListPersonalityVersions(t.Context(),
		host+"/giantswarm/klaus-personalities/sre",
	)
	if err != nil {
		t.Fatalf("ListPersonalityVersions() error = %v", err)
	}
	want := []string{testTagV100, testTagV020, testTagV010}
	if !slices.Equal(versions, want) {
		t.Errorf("ListPersonalityVersions() = %v, want %v", versions, want)
	}
}

func TestListToolchainVersions(t *testing.T) {
	ts := newTestRegistry(map[string][]string{
		testRepoToolchainGo: {testTagV110, testTagV100, testTagV090, "nightly"},
	})
	defer ts.Close()
	host := testRegistryHost(ts)

	client := NewClient(WithPlainHTTP(true))

	versions, err := client.ListToolchainVersions(t.Context(),
		host+"/giantswarm/klaus-toolchains/go",
	)
	if err != nil {
		t.Fatalf("ListToolchainVersions() error = %v", err)
	}
	want := []string{testTagV110, testTagV100, testTagV090}
	if !slices.Equal(versions, want) {
		t.Errorf("ListToolchainVersions() = %v, want %v", versions, want)
	}
}

func TestListVersionsNoSemverTags(t *testing.T) {
	ts := newTestRegistry(map[string][]string{
		"giantswarm/klaus-plugins/dev-only": {testTagLatest, testTagDev, testTagMain},
	})
	defer ts.Close()
	host := testRegistryHost(ts)

	client := NewClient(WithPlainHTTP(true))

	versions, err := client.ListPluginVersions(t.Context(),
		host+"/giantswarm/klaus-plugins/dev-only",
	)
	if err != nil {
		t.Fatalf("ListPluginVersions() error = %v", err)
	}
	if len(versions) != 0 {
		t.Errorf("expected empty slice, got %v", versions)
	}
}

func TestExtractNameVersion(t *testing.T) {
	tests := []struct {
		artifact    listedArtifact
		wantName    string
		wantVersion string
	}{
		{
			artifact:    listedArtifact{Repository: testRefPluginGSBase, Reference: "gsoci.azurecr.io/giantswarm/klaus-plugins/gs-base:v0.2.0"},
			wantName:    testNameGSBase,
			wantVersion: testTagV020,
		},
		{
			artifact:    listedArtifact{Repository: testRefOrgRepo, Reference: "registry.io/org/repo:v1.0.0"},
			wantName:    "repo",
			wantVersion: testTagV100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.wantName, func(t *testing.T) {
			name, version := extractNameVersion(tt.artifact)
			if name != tt.wantName {
				t.Errorf("name = %q, want %q", name, tt.wantName)
			}
			if version != tt.wantVersion {
				t.Errorf("version = %q, want %q", version, tt.wantVersion)
			}
		})
	}
}

func TestSortedSemverTags(t *testing.T) {
	tests := []struct {
		name string
		tags []string
		want []string
	}{
		{
			name: "multiple versions sorted descending",
			tags: []string{testTagV010, testTagV100, testTagV050},
			want: []string{testTagV100, testTagV050, testTagV010},
		},
		{
			name: "non-semver tags filtered out",
			tags: []string{testTagV100, testTagLatest, testTagMain, testTagV050, testTagDev},
			want: []string{testTagV100, testTagV050},
		},
		{
			name: "pre-release before release",
			tags: []string{testTagV100, "v1.1.0-rc.1", testTagV090},
			want: []string{"v1.1.0-rc.1", testTagV100, testTagV090},
		},
		{
			name: "single tag",
			tags: []string{testTagV100},
			want: []string{testTagV100},
		},
		{
			name: "no semver tags",
			tags: []string{testTagLatest, testTagMain},
			want: []string{},
		},
		{
			name: "empty input",
			tags: nil,
			want: []string{},
		},
		{
			name: "patch versions sorted correctly",
			tags: []string{"v1.0.2", "v1.0.10", "v1.0.1"},
			want: []string{"v1.0.10", "v1.0.2", "v1.0.1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sortedSemverTags(tt.tags)
			if len(got) != len(tt.want) {
				t.Fatalf("sortedSemverTags() returned %d tags, want %d: %v", len(got), len(tt.want), got)
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Errorf("sortedSemverTags()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestListVersionsSingleTag(t *testing.T) {
	ts := newTestRegistry(map[string][]string{
		"giantswarm/klaus-plugins/single": {testTagV100},
	})
	defer ts.Close()
	host := testRegistryHost(ts)

	client := NewClient(WithPlainHTTP(true))

	versions, err := client.ListPluginVersions(t.Context(),
		host+"/giantswarm/klaus-plugins/single",
	)
	if err != nil {
		t.Fatalf("ListPluginVersions() error = %v", err)
	}
	if len(versions) != 1 || versions[0] != testTagV100 {
		t.Errorf("ListPluginVersions() = %v, want [v1.0.0]", versions)
	}
}

func TestListVersionsPreRelease(t *testing.T) {
	ts := newTestRegistry(map[string][]string{
		"giantswarm/klaus-plugins/prerel": {"v1.0.0-alpha.1", "v1.0.0-beta.1", testTagV100, testTagV090},
	})
	defer ts.Close()
	host := testRegistryHost(ts)

	client := NewClient(WithPlainHTTP(true))

	versions, err := client.ListPluginVersions(t.Context(),
		host+"/giantswarm/klaus-plugins/prerel",
	)
	if err != nil {
		t.Fatalf("ListPluginVersions() error = %v", err)
	}

	want := []string{testTagV100, "v1.0.0-beta.1", "v1.0.0-alpha.1", testTagV090}
	if !slices.Equal(versions, want) {
		t.Errorf("ListPluginVersions() = %v, want %v", versions, want)
	}
}

func TestListPersonalities_Empty(t *testing.T) {
	ts := newTestRegistry(map[string][]string{})
	defer ts.Close()
	host := testRegistryHost(ts)

	client := NewClient(WithPlainHTTP(true))

	personalities, err := client.ListPersonalities(t.Context(),
		WithRegistry(host+"/giantswarm/klaus-personalities"))
	if err != nil {
		t.Fatalf("ListPersonalities() error = %v", err)
	}
	if len(personalities) != 0 {
		t.Errorf("expected empty result, got %d", len(personalities))
	}
}

func TestListEntry_Fields(t *testing.T) {
	ts := newTestRegistry(map[string][]string{
		testRepoPluginGSBase: {testTagV100},
	})
	defer ts.Close()
	host := testRegistryHost(ts)

	client := NewClient(WithPlainHTTP(true))

	entries, err := client.ListPlugins(t.Context(),
		WithRegistry(host+"/giantswarm/klaus-plugins"))
	if err != nil {
		t.Fatalf("ListPlugins() error = %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	entry := entries[0]
	if entry.Name != testNameGSBase {
		t.Errorf("Name = %q, want %q", entry.Name, testNameGSBase)
	}
	if entry.Version != testTagV100 {
		t.Errorf("Version = %q, want %q", entry.Version, testTagV100)
	}
	if !strings.HasSuffix(entry.Repository, testRepoPluginGSBase) {
		t.Errorf("Repository = %q, want suffix giantswarm/klaus-plugins/gs-base", entry.Repository)
	}
	if !strings.HasSuffix(entry.Reference, ":v1.0.0") {
		t.Errorf("Reference = %q, want suffix :v1.0.0", entry.Reference)
	}
}
