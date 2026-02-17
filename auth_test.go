package oci

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"oras.land/oras-go/v2/registry/remote/auth"
)

func TestCredentialFromJSON(t *testing.T) {
	cfg := dockerConfig{
		Auths: map[string]dockerAuthEntry{
			"registry.example.com": {
				Auth: base64.StdEncoding.EncodeToString([]byte("user:pass")),
			},
		},
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	t.Run("exact match", func(t *testing.T) {
		cred, ok := credentialFromJSON(data, "registry.example.com")
		if !ok {
			t.Fatal("expected credential to be found")
		}
		if cred.Username != "user" || cred.Password != "pass" {
			t.Errorf("credential = %s:%s, want user:pass", cred.Username, cred.Password)
		}
	})

	t.Run("match without port", func(t *testing.T) {
		cred, ok := credentialFromJSON(data, "registry.example.com:443")
		if !ok {
			t.Fatal("expected credential to be found via host-only fallback")
		}
		if cred.Username != "user" {
			t.Errorf("Username = %q, want %q", cred.Username, "user")
		}
	})

	t.Run("no match", func(t *testing.T) {
		cred, ok := credentialFromJSON(data, "other.registry.io")
		if ok {
			t.Errorf("expected no credential, got %+v", cred)
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		_, ok := credentialFromJSON([]byte("not json"), "registry.example.com")
		if ok {
			t.Error("expected false for invalid JSON")
		}
	})
}

func TestCredentialFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	cfg := dockerConfig{
		Auths: map[string]dockerAuthEntry{
			"myregistry.io": {
				Auth: base64.StdEncoding.EncodeToString([]byte("admin:secret")),
			},
		},
	}
	data, _ := json.Marshal(cfg)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	cred, ok := credentialFromFile(path, "myregistry.io")
	if !ok {
		t.Fatal("expected credential from file")
	}
	if cred.Username != "admin" || cred.Password != "secret" {
		t.Errorf("credential = %s:%s", cred.Username, cred.Password)
	}
}

func TestCredentialFromFile_Missing(t *testing.T) {
	_, ok := credentialFromFile("/nonexistent/path", "registry.example.com")
	if ok {
		t.Error("expected false for missing file")
	}
}

func TestResolveCredential_EnvVar(t *testing.T) {
	cfg := dockerConfig{
		Auths: map[string]dockerAuthEntry{
			"envregistry.io": {
				Auth: base64.StdEncoding.EncodeToString([]byte("envuser:envpass")),
			},
		},
	}
	data, _ := json.Marshal(cfg)
	encoded := base64.StdEncoding.EncodeToString(data)

	const envName = "TEST_KLAUS_OCI_AUTH"
	t.Setenv(envName, encoded)

	cred, err := resolveCredential(envName, "envregistry.io")
	if err != nil {
		t.Fatalf("resolveCredential: %v", err)
	}
	if cred.Username != "envuser" {
		t.Errorf("Username = %q, want %q", cred.Username, "envuser")
	}
}

func TestResolveCredential_FallbackAnonymous(t *testing.T) {
	cred, err := resolveCredential("", "nonexistent.registry.io")
	if err != nil {
		t.Fatalf("resolveCredential: %v", err)
	}
	if cred != auth.EmptyCredential {
		t.Errorf("expected empty credential, got %+v", cred)
	}
}
