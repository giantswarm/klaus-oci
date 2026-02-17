package oci

import (
	"testing"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

func TestAnnotationsFromConfig(t *testing.T) {
	t.Run("full metadata", func(t *testing.T) {
		configJSON := []byte(`{"name":"test","version":"1.0.0","description":"A test"}`)
		annotations := annotationsFromConfig(configJSON)

		if annotations[ocispec.AnnotationTitle] != "test" {
			t.Errorf("title = %q, want %q", annotations[ocispec.AnnotationTitle], "test")
		}
		if annotations[ocispec.AnnotationVersion] != "1.0.0" {
			t.Errorf("version = %q, want %q", annotations[ocispec.AnnotationVersion], "1.0.0")
		}
		if annotations[ocispec.AnnotationDescription] != "A test" {
			t.Errorf("description = %q, want %q", annotations[ocispec.AnnotationDescription], "A test")
		}
	})

	t.Run("empty metadata", func(t *testing.T) {
		configJSON := []byte(`{}`)
		annotations := annotationsFromConfig(configJSON)
		if annotations != nil {
			t.Errorf("expected nil annotations for empty config, got %v", annotations)
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		annotations := annotationsFromConfig([]byte("not json"))
		if annotations != nil {
			t.Errorf("expected nil annotations for invalid JSON, got %v", annotations)
		}
	})
}
