package oci

import (
	"encoding/json"
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

	t.Run("plugin config blob with version excluded", func(t *testing.T) {
		plugin := Plugin{
			Name:        "gs-base",
			Version:     "should-not-appear",
			Description: "A base plugin",
		}
		configJSON, _ := json.Marshal(plugin)

		annotations := buildAnnotations(configJSON, "v1.0.0")

		if annotations[ocispec.AnnotationTitle] != "gs-base" {
			t.Errorf("title = %q, want %q", annotations[ocispec.AnnotationTitle], "gs-base")
		}
		if annotations[ocispec.AnnotationVersion] != "v1.0.0" {
			t.Errorf("version = %q, want %q (from tag)", annotations[ocispec.AnnotationVersion], "v1.0.0")
		}
		if annotations[ocispec.AnnotationDescription] != "A base plugin" {
			t.Errorf("description = %q, want %q", annotations[ocispec.AnnotationDescription], "A base plugin")
		}
	})

	t.Run("personality config blob", func(t *testing.T) {
		personality := Personality{
			Name:        "sre",
			Description: "SRE personality",
			Toolchain: ToolchainReference{
				Repository: "gsoci.azurecr.io/giantswarm/klaus-toolchains/go",
				Tag:        "latest",
			},
		}
		configJSON, _ := json.Marshal(personality)

		annotations := buildAnnotations(configJSON, "v2.0.0")

		if annotations[ocispec.AnnotationTitle] != "sre" {
			t.Errorf("title = %q, want %q", annotations[ocispec.AnnotationTitle], "sre")
		}
		if annotations[ocispec.AnnotationVersion] != "v2.0.0" {
			t.Errorf("version = %q, want %q", annotations[ocispec.AnnotationVersion], "v2.0.0")
		}
		if annotations[ocispec.AnnotationDescription] != "SRE personality" {
			t.Errorf("description = %q", annotations[ocispec.AnnotationDescription])
		}
	})

	t.Run("name only", func(t *testing.T) {
		configJSON := []byte(`{"name":"minimal"}`)
		annotations := buildAnnotations(configJSON, "v1.0.0")

		if annotations[ocispec.AnnotationTitle] != "minimal" {
			t.Errorf("title = %q, want %q", annotations[ocispec.AnnotationTitle], "minimal")
		}
		if _, ok := annotations[ocispec.AnnotationDescription]; ok {
			t.Errorf("description should not be present, got %q", annotations[ocispec.AnnotationDescription])
		}
	})
}
