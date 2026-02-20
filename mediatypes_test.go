package oci

import (
	"testing"
)

func TestArtifactKinds(t *testing.T) {
	tests := []struct {
		name            string
		kind            ArtifactKind
		wantConfigType  string
		wantContentType string
	}{
		{
			name:            "PluginArtifact has correct media types",
			kind:            PluginArtifact,
			wantConfigType:  "application/vnd.giantswarm.klaus-plugin.config.v1+json",
			wantContentType: "application/vnd.giantswarm.klaus-plugin.content.v1.tar+gzip",
		},
		{
			name:            "PersonalityArtifact has correct media types",
			kind:            PersonalityArtifact,
			wantConfigType:  "application/vnd.giantswarm.klaus-personality.config.v1+json",
			wantContentType: "application/vnd.giantswarm.klaus-personality.content.v1.tar+gzip",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.kind.ConfigMediaType != tt.wantConfigType {
				t.Errorf("ConfigMediaType = %q, want %q", tt.kind.ConfigMediaType, tt.wantConfigType)
			}
			if tt.kind.ContentMediaType != tt.wantContentType {
				t.Errorf("ContentMediaType = %q, want %q", tt.kind.ContentMediaType, tt.wantContentType)
			}
		})
	}
}
