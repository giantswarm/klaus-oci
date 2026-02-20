package oci

import "testing"

func TestSplitRegistryBase(t *testing.T) {
	tests := []struct {
		base       string
		wantHost   string
		wantPrefix string
	}{
		{"gsoci.azurecr.io/giantswarm/klaus-plugins", "gsoci.azurecr.io", "giantswarm/klaus-plugins/"},
		{"gsoci.azurecr.io/giantswarm/klaus-personalities", "gsoci.azurecr.io", "giantswarm/klaus-personalities/"},
		{"gsoci.azurecr.io", "gsoci.azurecr.io", ""},
		{"localhost:5000/plugins", "localhost:5000", "plugins/"},
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
		{"gsoci.azurecr.io/giantswarm/klaus-plugins/gs-platform", "gs-platform"},
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
		{"sha256:abc123def456789abcdef", "sha256:abc123def456"},
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
			tags: []string{"v0.0.1", "v0.0.3", "v0.0.2"},
			want: "v0.0.3",
		},
		{
			name: "single version",
			tags: []string{"v1.0.0"},
			want: "v1.0.0",
		},
		{
			name: "mixed valid and invalid",
			tags: []string{"latest", "v0.0.6", "main", "v0.0.7"},
			want: "v0.0.7",
		},
		{
			name: "no valid semver",
			tags: []string{"latest", "main", "dev"},
			want: "",
		},
		{
			name: "empty",
			tags: nil,
			want: "",
		},
		{
			name: "prerelease lower than release",
			tags: []string{"v1.0.0-rc.1", "v0.9.0"},
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
		{"gs-ae", "gs-ae", ""},
		{"gs-ae:v0.0.7", "gs-ae", "v0.0.7"},
		{"my-plugin:latest", "my-plugin", "latest"},
		{"localhost:5000/repo", "localhost:5000/repo", ""},
		{"localhost:5000/repo:v1.0.0", "localhost:5000/repo", "v1.0.0"},
		{"registry.io/org/repo:tag", "registry.io/org/repo", "tag"},
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
		{"example.com/repo:v1.0.0", "example.com/repo"},
		{"example.com/repo@sha256:abc123", "example.com/repo"},
		{"example.com/repo", "example.com/repo"},
		{"localhost:5000/repo", "localhost:5000/repo"},
		{"localhost:5000/repo:v1.0.0", "localhost:5000/repo"},
		{"localhost:5000", "localhost:5000"},
		{"registry.io/org/repo:tag", "registry.io/org/repo"},
		{"registry.io/org/repo@sha256:deadbeef", "registry.io/org/repo"},
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
		{"example.com/repo:v1.0.0", true},
		{"example.com/repo@sha256:abc123", true},
		{"example.com/repo", false},
		{"localhost:5000/repo", false},
		{"localhost:5000/repo:v1.0.0", true},
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
		{"example.com/repo:v1.0.0", "v1.0.0"},
		{"example.com/repo:latest", "latest"},
		{"example.com/repo@sha256:abc123", ""},
		{"example.com/repo", ""},
		{"localhost:5000/repo:v1.0.0", "v1.0.0"},
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
