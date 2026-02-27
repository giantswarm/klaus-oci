package oci

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

const cacheFileName = ".oci-cache.json"

// CacheEntry holds metadata about a cached artifact.
type CacheEntry struct {
	// Digest is the OCI manifest digest.
	Digest string `json:"digest"`
	// Ref is the original OCI reference that was pulled.
	Ref string `json:"ref"`
	// PulledAt is when the artifact was last pulled.
	PulledAt time.Time `json:"pulledAt"`
	// ConfigJSON is the raw OCI config blob, persisted so that metadata
	// remains available on cache hits without re-fetching.
	ConfigJSON json.RawMessage `json:"configJSON,omitempty"`
	// Annotations are the OCI manifest annotations, persisted so that
	// common metadata is available on cache hits.
	Annotations map[string]string `json:"annotations,omitempty"`
}

// IsCached returns true if the directory has a cache entry matching the given
// manifest digest.
func IsCached(dir string, digest string) bool {
	entry, err := ReadCacheEntry(dir)
	if err != nil {
		return false
	}
	return entry.Digest == digest
}

// ReadCacheEntry reads the cache metadata from a directory.
func ReadCacheEntry(dir string) (*CacheEntry, error) {
	path := filepath.Join(dir, cacheFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var entry CacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, err
	}

	return &entry, nil
}

// WriteCacheEntry writes cache metadata to a directory.
// The PulledAt timestamp is always set to the current time.
func WriteCacheEntry(dir string, entry CacheEntry) error {
	entry.PulledAt = time.Now()

	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return err
	}

	path := filepath.Join(dir, cacheFileName)
	return os.WriteFile(path, data, 0o644)
}
