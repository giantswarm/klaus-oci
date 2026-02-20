package oci

import (
	"testing"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

func TestSelectPlatformForArch(t *testing.T) {
	tests := []struct {
		name       string
		manifests  []ocispec.Descriptor
		selectOS   string
		selectArch string
		wantOS     string
		wantArch   string
		wantErr    bool
	}{
		{
			name: "exact match for requested platform",
			manifests: []ocispec.Descriptor{
				{
					MediaType: ocispec.MediaTypeImageManifest,
					Platform:  &ocispec.Platform{OS: "linux", Architecture: "arm64"},
				},
				{
					MediaType: ocispec.MediaTypeImageManifest,
					Platform:  &ocispec.Platform{OS: "linux", Architecture: "amd64"},
				},
			},
			selectOS:   "linux",
			selectArch: "amd64",
			wantOS:     "linux",
			wantArch:   "amd64",
		},
		{
			name: "selects darwin/arm64 from mixed list",
			manifests: []ocispec.Descriptor{
				{
					MediaType: ocispec.MediaTypeImageManifest,
					Platform:  &ocispec.Platform{OS: "linux", Architecture: "amd64"},
				},
				{
					MediaType: ocispec.MediaTypeImageManifest,
					Platform:  &ocispec.Platform{OS: "darwin", Architecture: "arm64"},
				},
			},
			selectOS:   "darwin",
			selectArch: "arm64",
			wantOS:     "darwin",
			wantArch:   "arm64",
		},
		{
			name: "falls back to first image manifest when no platform match",
			manifests: []ocispec.Descriptor{
				{
					MediaType: "application/vnd.oci.image.manifest.attestation.v1+json",
					Platform:  &ocispec.Platform{OS: "unknown", Architecture: "unknown"},
				},
				{
					MediaType: ocispec.MediaTypeImageManifest,
					Platform:  &ocispec.Platform{OS: "other", Architecture: "other"},
				},
			},
			selectOS:   "nonexistent",
			selectArch: "nonexistent",
			wantOS:     "other",
			wantArch:   "other",
		},
		{
			name: "falls back to docker manifest when no platform match",
			manifests: []ocispec.Descriptor{
				{
					MediaType: "application/vnd.docker.distribution.manifest.v2+json",
					Platform:  &ocispec.Platform{OS: "linux", Architecture: "amd64"},
				},
			},
			selectOS:   "darwin",
			selectArch: "arm64",
			wantOS:     "linux",
			wantArch:   "amd64",
		},
		{
			name: "falls back to first entry when nothing matches",
			manifests: []ocispec.Descriptor{
				{
					MediaType: "application/unknown",
					Platform:  &ocispec.Platform{OS: "plan9", Architecture: "386"},
				},
			},
			selectOS:   "linux",
			selectArch: "amd64",
			wantOS:     "plan9",
			wantArch:   "386",
		},
		{
			name:       "empty manifests returns error",
			manifests:  []ocispec.Descriptor{},
			selectOS:   "linux",
			selectArch: "amd64",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := selectPlatformForArch(tt.manifests, tt.selectOS, tt.selectArch)
			if tt.wantErr {
				if err == nil {
					t.Fatal("selectPlatformForArch() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("selectPlatformForArch() error = %v", err)
			}
			if got.Platform == nil {
				t.Fatal("selectPlatformForArch() returned descriptor without platform")
			}
			if got.Platform.OS != tt.wantOS {
				t.Errorf("OS = %q, want %q", got.Platform.OS, tt.wantOS)
			}
			if got.Platform.Architecture != tt.wantArch {
				t.Errorf("Architecture = %q, want %q", got.Platform.Architecture, tt.wantArch)
			}
		})
	}
}
