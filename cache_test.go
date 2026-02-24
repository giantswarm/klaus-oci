package oci

import (
	"encoding/json"
	"testing"
)

func TestCache_WriteAndRead(t *testing.T) {
	dir := t.TempDir()

	configJSON := json.RawMessage(`{"name":"test","version":"1.0.0"}`)
	entry := CacheEntry{
		Digest:     "sha256:abc123def456",
		Ref:        "registry.example.com/test:v1.0.0",
		ConfigJSON: configJSON,
	}

	if err := WriteCacheEntry(dir, entry); err != nil {
		t.Fatalf("WriteCacheEntry: %v", err)
	}

	got, err := ReadCacheEntry(dir)
	if err != nil {
		t.Fatalf("ReadCacheEntry: %v", err)
	}

	if got.Digest != entry.Digest {
		t.Errorf("Digest = %q, want %q", got.Digest, entry.Digest)
	}
	if got.Ref != entry.Ref {
		t.Errorf("Ref = %q, want %q", got.Ref, entry.Ref)
	}
	if got.PulledAt.IsZero() {
		t.Error("PulledAt should be set")
	}
	var gotConfig, wantConfig map[string]interface{}
	if err := json.Unmarshal(got.ConfigJSON, &gotConfig); err != nil {
		t.Fatalf("unmarshal got ConfigJSON: %v", err)
	}
	if err := json.Unmarshal(configJSON, &wantConfig); err != nil {
		t.Fatalf("unmarshal want ConfigJSON: %v", err)
	}
	if gotConfig["name"] != wantConfig["name"] || gotConfig["version"] != wantConfig["version"] {
		t.Errorf("ConfigJSON fields mismatch: got %v, want %v", gotConfig, wantConfig)
	}
}

func TestIsCached(t *testing.T) {
	dir := t.TempDir()
	digest := "sha256:abc123def456"

	if IsCached(dir, digest) {
		t.Error("expected not cached before write")
	}

	if err := WriteCacheEntry(dir, CacheEntry{Digest: digest, Ref: "test:v1"}); err != nil {
		t.Fatalf("WriteCacheEntry: %v", err)
	}

	if !IsCached(dir, digest) {
		t.Error("expected cached after write")
	}

	if IsCached(dir, "sha256:different") {
		t.Error("expected not cached for different digest")
	}
}

func TestReadCacheEntry_Missing(t *testing.T) {
	dir := t.TempDir()
	_, err := ReadCacheEntry(dir)
	if err == nil {
		t.Error("expected error for missing cache file")
	}
}
