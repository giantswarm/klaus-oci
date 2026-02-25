package oci

import (
	"testing"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

func TestBuildAnnotations(t *testing.T) {
	t.Run("full metadata", func(t *testing.T) {
		configJSON := []byte(`{"name":"test","description":"A test"}`)
		annotations := buildAnnotations(configJSON, "v1.0.0")

		if annotations[ocispec.AnnotationTitle] != "test" {
			t.Errorf("title = %q, want %q", annotations[ocispec.AnnotationTitle], "test")
		}
		if annotations[ocispec.AnnotationVersion] != "v1.0.0" {
			t.Errorf("version = %q, want %q", annotations[ocispec.AnnotationVersion], "v1.0.0")
		}
		if annotations[ocispec.AnnotationDescription] != "A test" {
			t.Errorf("description = %q, want %q", annotations[ocispec.AnnotationDescription], "A test")
		}
	})

	t.Run("version from tag not config", func(t *testing.T) {
		configJSON := []byte(`{"name":"test"}`)
		annotations := buildAnnotations(configJSON, "v2.0.0")

		if annotations[ocispec.AnnotationVersion] != "v2.0.0" {
			t.Errorf("version = %q, want %q", annotations[ocispec.AnnotationVersion], "v2.0.0")
		}
	})

	t.Run("empty metadata", func(t *testing.T) {
		configJSON := []byte(`{}`)
		annotations := buildAnnotations(configJSON, "")
		if annotations != nil {
			t.Errorf("expected nil annotations for empty config and tag, got %v", annotations)
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		annotations := buildAnnotations([]byte("not json"), "v1.0.0")
		if annotations != nil {
			t.Errorf("expected nil annotations for invalid JSON, got %v", annotations)
		}
	})
}
