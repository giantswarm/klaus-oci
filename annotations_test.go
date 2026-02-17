package oci

import (
	"testing"
)

func TestArtifactInfoFromAnnotations(t *testing.T) {
	t.Run("all keys present", func(t *testing.T) {
		annotations := map[string]string{
			AnnotationKlausType:    TypePlugin,
			AnnotationKlausName:    "gs-platform",
			AnnotationKlausVersion: "1.2.0",
		}

		info := ArtifactInfoFromAnnotations(annotations)

		if info.Type != TypePlugin {
			t.Errorf("Type = %q, want %q", info.Type, TypePlugin)
		}
		if info.Name != "gs-platform" {
			t.Errorf("Name = %q, want %q", info.Name, "gs-platform")
		}
		if info.Version != "1.2.0" {
			t.Errorf("Version = %q, want %q", info.Version, "1.2.0")
		}
	})

	t.Run("missing keys return empty strings", func(t *testing.T) {
		annotations := map[string]string{
			AnnotationKlausName: "some-artifact",
		}

		info := ArtifactInfoFromAnnotations(annotations)

		if info.Type != "" {
			t.Errorf("Type = %q, want empty", info.Type)
		}
		if info.Name != "some-artifact" {
			t.Errorf("Name = %q, want %q", info.Name, "some-artifact")
		}
		if info.Version != "" {
			t.Errorf("Version = %q, want empty", info.Version)
		}
	})

	t.Run("nil map returns zero struct", func(t *testing.T) {
		info := ArtifactInfoFromAnnotations(nil)

		if info.Type != "" || info.Name != "" || info.Version != "" {
			t.Errorf("expected zero ArtifactInfo, got %+v", info)
		}
	})

	t.Run("extra keys are ignored", func(t *testing.T) {
		annotations := map[string]string{
			AnnotationKlausType:    TypeToolchain,
			AnnotationKlausName:    "klaus-go",
			AnnotationKlausVersion: "1.0.0",
			"unrelated.key":        "ignored",
		}

		info := ArtifactInfoFromAnnotations(annotations)

		if info.Type != TypeToolchain {
			t.Errorf("Type = %q, want %q", info.Type, TypeToolchain)
		}
		if info.Name != "klaus-go" {
			t.Errorf("Name = %q, want %q", info.Name, "klaus-go")
		}
		if info.Version != "1.0.0" {
			t.Errorf("Version = %q, want %q", info.Version, "1.0.0")
		}
	})
}
