package oci

import (
	"context"
	"io"
	"time"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// Default TTLs and size limit for the on-disk cache. Exposed as constants so
// callers can reason about cache behavior.
const (
	// DefaultCacheFreshTTL is the window within which a cache entry is
	// returned with zero network traffic.
	DefaultCacheFreshTTL = 30 * time.Second
	// DefaultCacheStaleTTL is the default outer bound for tag and reference
	// entries: beyond this age the entry is discarded and refetched
	// synchronously.
	DefaultCacheStaleTTL = 24 * time.Hour
	// DefaultCacheCatalogStaleTTL is the outer bound for catalog entries.
	// Registry catalogs change slowly so we keep them much longer.
	DefaultCacheCatalogStaleTTL = 7 * 24 * time.Hour
	// DefaultCacheMaxSize is the default LRU budget (bytes) for the content
	// store.
	DefaultCacheMaxSize int64 = 2 * 1024 * 1024 * 1024
)

// CacheStore provides cached lookups against an OCI registry.
//
// When a CacheStore is attached to a Client via WithCache, every read path
// (tag resolve, tag list, repository catalog, manifest/blob fetch) consults
// the store first and falls back to the network on miss. The store is a
// pure cache: it never serves stale data that contradicts the registry,
// only data that is either fresh or known-good-pending-revalidation.
//
// Implementations must be safe for concurrent use, and must be safe to use
// across concurrent processes sharing the same backing directory.
type CacheStore interface {
	// ResolveTag returns the manifest digest for a tag reference of the form
	// "host/repo:tag". Entries are revalidated with HEAD probes against the
	// registry.
	ResolveTag(ctx context.Context, ref string) (string, error)

	// ResolveManifest returns the full manifest descriptor (digest, size,
	// media type) for a tag reference. Callers that plan to fetch the
	// manifest body should use this method rather than ResolveTag so the
	// returned descriptor carries enough information for downstream
	// verification.
	ResolveManifest(ctx context.Context, ref string) (ocispec.Descriptor, error)

	// Tags returns all tags for a repository of the form "host/repo". Tag
	// lists are revalidated with conditional GETs (If-None-Match).
	Tags(ctx context.Context, repo string) ([]string, error)

	// Repositories returns all repositories under a registry base of the
	// form "host/prefix". Empty prefixes enumerate the whole registry.
	Repositories(ctx context.Context, base string) ([]string, error)

	// Fetch returns a reader for the content of desc. The content is served
	// from the local content store when present; otherwise it is fetched
	// from the registry and inserted into the store before returning.
	Fetch(ctx context.Context, repo string, desc ocispec.Descriptor) (io.ReadCloser, error)

	// Close releases resources. It waits for any in-flight background
	// revalidations to complete.
	Close() error
}

// cacheConfig holds the tunable parameters for the on-disk cache. It is
// populated by WithCache*, and passed to newDiskCache during lazy init.
type cacheConfig struct {
	dir               string
	freshTTL          time.Duration
	staleTTL          time.Duration
	catalogStaleTTL   time.Duration
	maxBytes          int64
	backgroundRefresh bool
}

func defaultCacheConfig() cacheConfig {
	return cacheConfig{
		freshTTL:          DefaultCacheFreshTTL,
		staleTTL:          DefaultCacheStaleTTL,
		catalogStaleTTL:   DefaultCacheCatalogStaleTTL,
		maxBytes:          DefaultCacheMaxSize,
		backgroundRefresh: true,
	}
}
