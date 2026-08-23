package oci

import "testing"

func TestSplitRegistryBase(t *testing.T) {
	tests := []struct {
		base       string
		wantHost   string
		wantPrefix string
	}{
		{testRegistryPlugins, testRegistryGSOCI, "giantswarm/klaus-plugins/"},
		{"gsoci.azurecr.io/giantswarm/klaus-personalities", testRegistryGSOCI, "giantswarm/klaus-personalities/"},
		{testRegistryGSOCI, testRegistryGSOCI, ""},
		{"localhost:5000/plugins", testRegistryLocal, "plugins/"},
		{"example.com/org/team/artifacts", "example.com", "org/team/artifacts/"},
	}

	for _, tt := range tests {
		t.Run(tt.base, func(t *testing.T) {
			gotHost, gotPrefix := SplitRegistryBase(tt.base)
			if gotHost != tt.wantHost {
				t.Errorf("host = %q, want %q", gotHost, tt.wantHost)
			}
			if gotPrefix != tt.wantPrefix {
				t.Errorf("prefix = %q, want %q", gotPrefix, tt.wantPrefix)
			}
		})
	}
}

func TestShortName(t *testing.T) {
	tests := []struct {
		repository string
		want       string
	}{
		{"gsoci.azurecr.io/giantswarm/klaus-plugins/gs-platform", testNameGSPlatform},
		{"registry.example.com/repo", "repo"},
		{"simple", "simple"},
	}

	for _, tt := range tests {
		t.Run(tt.repository, func(t *testing.T) {
			if got := ShortName(tt.repository); got != tt.want {
				t.Errorf("ShortName(%q) = %q, want %q", tt.repository, got, tt.want)
			}
		})
	}
}

func TestTruncateDigest(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"sha256:abc123def456789abcdef", testDigestAbc123Def456},
		{"sha256:short", "sha256:short"},
		{"noprefix", "noprefix"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := TruncateDigest(tt.input); got != tt.want {
				t.Errorf("TruncateDigest(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestLatestSemverTag(t *testing.T) {
	tests := []struct {
		name string
		tags []string
		want string
	}{
		{
			name: "multiple versions",
			tags: []string{testTagV001, testTagV003, testTagV002},
			want: testTagV003,
		},
		{
			name: "single version",
			tags: []string{testTagV100},
			want: testTagV100,
		},
		{
			name: "mixed valid and invalid",
			tags: []string{testTagLatest, "v0.0.6", testTagMain, testTagV007},
			want: testTagV007,
		},
		{
			name: "no valid semver",
			tags: []string{testTagLatest, testTagMain, testTagDev},
			want: "",
		},
		{
			name: "empty",
			tags: nil,
			want: "",
		},
		{
			name: "prerelease lower than release",
			tags: []string{"v1.0.0-rc.1", testTagV090},
			want: "v1.0.0-rc.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := LatestSemverTag(tt.tags)
			if got != tt.want {
				t.Errorf("LatestSemverTag(%v) = %q, want %q", tt.tags, got, tt.want)
			}
		})
	}
}

func TestSplitNameTag(t *testing.T) {
	tests := []struct {
		ref      string
		wantName string
		wantTag  string
	}{
		{testNameGSAE, testNameGSAE, ""},
		{"gs-ae:v0.0.7", testNameGSAE, testTagV007},
		{"my-plugin:latest", "my-plugin", testTagLatest},
		{testRefLocalRepo, testRefLocalRepo, ""},
		{testRefLocalRepoV100, testRefLocalRepo, testTagV100},
		{"registry.io/org/repo:tag", testRefOrgRepo, "tag"},
	}

	for _, tt := range tests {
		t.Run(tt.ref, func(t *testing.T) {
			name, tag := SplitNameTag(tt.ref)
			if name != tt.wantName {
				t.Errorf("name = %q, want %q", name, tt.wantName)
			}
			if tag != tt.wantTag {
				t.Errorf("tag = %q, want %q", tag, tt.wantTag)
			}
		})
	}
}

func TestRepositoryFromRef(t *testing.T) {
	tests := []struct {
		ref  string
		want string
	}{
		{testRefExampleRepoV100, testRefExampleRepo},
		{testRefExampleRepoDigest, testRefExampleRepo},
		{testRefExampleRepo, testRefExampleRepo},
		{testRefLocalRepo, testRefLocalRepo},
		{testRefLocalRepoV100, testRefLocalRepo},
		{testRegistryLocal, testRegistryLocal},
		{"registry.io/org/repo:tag", testRefOrgRepo},
		{"registry.io/org/repo@sha256:deadbeef", testRefOrgRepo},
	}

	for _, tt := range tests {
		t.Run(tt.ref, func(t *testing.T) {
			if got := RepositoryFromRef(tt.ref); got != tt.want {
				t.Errorf("RepositoryFromRef(%q) = %q, want %q", tt.ref, got, tt.want)
			}
		})
	}
}

func TestHasTagOrDigest(t *testing.T) {
	tests := []struct {
		ref  string
		want bool
	}{
		{testRefExampleRepoV100, true},
		{testRefExampleRepoDigest, true},
		{testRefExampleRepo, false},
		{testRefLocalRepo, false},
		{testRefLocalRepoV100, true},
	}

	for _, tt := range tests {
		t.Run(tt.ref, func(t *testing.T) {
			got := hasTagOrDigest(tt.ref)
			if got != tt.want {
				t.Errorf("hasTagOrDigest(%q) = %v, want %v", tt.ref, got, tt.want)
			}
		})
	}
}

func TestExtractTag(t *testing.T) {
	tests := []struct {
		ref  string
		want string
	}{
		{testRefExampleRepoV100, testTagV100},
		{"example.com/repo:latest", testTagLatest},
		{testRefExampleRepoDigest, ""},
		{testRefExampleRepo, ""},
		{testRefLocalRepoV100, testTagV100},
	}

	for _, tt := range tests {
		t.Run(tt.ref, func(t *testing.T) {
			got := extractTag(tt.ref)
			if got != tt.want {
				t.Errorf("extractTag(%q) = %q, want %q", tt.ref, got, tt.want)
			}
		})
	}
}
