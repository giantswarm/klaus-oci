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
// listing endpoints used by ListRepositories and ListArtifacts.
func newTestRegistry(repos map[string][]string) *httptest.Server {
	var sortedNames []string
	for name := range repos {
		sortedNames = append(sortedNames, name)
	}
	sort.Strings(sortedNames)

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/v2/" || r.URL.Path == "/v2":
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
			json.NewEncoder(w).Encode(map[string][]string{"repositories": result})

		case strings.HasSuffix(r.URL.Path, "/tags/list"):
			repoName := strings.TrimPrefix(r.URL.Path, "/v2/")
			repoName = strings.TrimSuffix(repoName, "/tags/list")
			tags, ok := repos[repoName]
			if !ok {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{"name": repoName, "tags": tags})

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
		"giantswarm/aaa":                       {},
		"giantswarm/klaus-plugins-extra/foo":   {"v1.0.0"},
		"giantswarm/klaus-plugins/gs-base":     {"v0.1.0"},
		"giantswarm/klaus-plugins/gs-platform": {"v0.2.0"},
		"giantswarm/zzz":                       {},
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
			registryBase: host + "/giantswarm/klaus-plugins",
			want: []string{
				host + "/giantswarm/klaus-plugins/gs-base",
				host + "/giantswarm/klaus-plugins/gs-platform",
			},
		},
		{
			name:         "no matches returns nil",
			registryBase: host + "/giantswarm/nonexistent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := client.ListRepositories(t.Context(), tt.registryBase)
			if err != nil {
				t.Fatalf("ListRepositories() error = %v", err)
			}
			sort.Strings(got)
			sort.Strings(tt.want)
			if !slices.Equal(got, tt.want) {
				t.Errorf("ListRepositories() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestListArtifacts(t *testing.T) {
	ts := newTestRegistry(map[string][]string{
		"giantswarm/klaus-plugins/gs-base":     {"v0.1.0", "v0.2.0"},
		"giantswarm/klaus-plugins/gs-platform": {"v1.0.0", "v1.1.0"},
		"giantswarm/klaus-plugins/no-semver":   {"latest", "dev"},
	})
	defer ts.Close()
	host := testRegistryHost(ts)
	base := host + "/giantswarm/klaus-plugins"

	client := NewClient(WithPlainHTTP(true))

	t.Run("discovers artifacts with latest semver", func(t *testing.T) {
		artifacts, err := client.ListArtifacts(t.Context(), base)
		if err != nil {
			t.Fatalf("ListArtifacts() error = %v", err)
		}
		if len(artifacts) != 2 {
			t.Fatalf("expected 2 artifacts, got %d: %v", len(artifacts), artifacts)
		}
		if !strings.HasSuffix(artifacts[0].Repository, "gs-base") {
			t.Errorf("artifacts[0].Repository = %q, want suffix gs-base", artifacts[0].Repository)
		}
		if !strings.HasSuffix(artifacts[0].Reference, ":v0.2.0") {
			t.Errorf("artifacts[0].Reference = %q, want suffix :v0.2.0", artifacts[0].Reference)
		}
		if !strings.HasSuffix(artifacts[1].Repository, "gs-platform") {
			t.Errorf("artifacts[1].Repository = %q, want suffix gs-platform", artifacts[1].Repository)
		}
		if !strings.HasSuffix(artifacts[1].Reference, ":v1.1.0") {
			t.Errorf("artifacts[1].Reference = %q, want suffix :v1.1.0", artifacts[1].Reference)
		}
	})

	t.Run("annotations not fetched by default", func(t *testing.T) {
		artifacts, err := client.ListArtifacts(t.Context(), base)
		if err != nil {
			t.Fatalf("ListArtifacts() error = %v", err)
		}
		for _, a := range artifacts {
			if a.ArtifactInfo.Name != "" {
				t.Errorf("expected empty ArtifactInfo.Name without WithAnnotations, got %q", a.ArtifactInfo.Name)
			}
		}
	})

	t.Run("WithFilter keeps only matching repos", func(t *testing.T) {
		artifacts, err := client.ListArtifacts(t.Context(), base,
			WithFilter(func(repo string) bool {
				return strings.HasSuffix(repo, "gs-base")
			}),
		)
		if err != nil {
			t.Fatalf("ListArtifacts() error = %v", err)
		}
		if len(artifacts) != 1 {
			t.Fatalf("expected 1 artifact, got %d", len(artifacts))
		}
		if !strings.HasSuffix(artifacts[0].Repository, "gs-base") {
			t.Errorf("artifact = %q, want suffix gs-base", artifacts[0].Repository)
		}
	})

	t.Run("WithFilter rejecting all returns empty", func(t *testing.T) {
		artifacts, err := client.ListArtifacts(t.Context(), base,
			WithFilter(func(string) bool { return false }),
		)
		if err != nil {
			t.Fatalf("ListArtifacts() error = %v", err)
		}
		if len(artifacts) != 0 {
			t.Errorf("expected 0 artifacts, got %d", len(artifacts))
		}
	})
}
