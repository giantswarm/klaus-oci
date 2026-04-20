package oci

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"golang.org/x/sync/singleflight"
	orasoci "oras.land/oras-go/v2/content/oci"
	"oras.land/oras-go/v2/registry/remote/auth"
)

// ocispecDigest narrows a digest string into a go-digest.Digest without
// validating. Callers that need validation should use digest.Digest.Validate.
func ocispecDigest(s string) digest.Digest {
	return digest.Digest(s)
}

// diskCache implements CacheStore with a directory rooted at cfg.dir.
//
// Layout on disk:
//
//	<root>/blobs/     -- layer A, oci-layout content store (oras-go)
//	<root>/refs/      -- layer B, per-tag digest index (hashed filenames)
//	<root>/tags/      -- layer C, per-repo tag list with ETag
//	<root>/catalog/   -- layer D, per-base repository catalog
//
// Index layers are written via temp+rename for multi-process safety.
type diskCache struct {
	cfg        cacheConfig
	authClient *auth.Client
	plainHTTP  bool
	storage    *orasoci.Storage

	sf singleflight.Group

	bg     sync.WaitGroup
	closed atomic.Bool
}

// Index entry shapes. `Key` is repeated inside the file so a reader can
// verify the entry belongs to the expected query (defense in depth against
// hash collisions or tampered files).

type refIndexEntry struct {
	Key       string    `json:"key"`
	Digest    string    `json:"digest"`
	Size      int64     `json:"size,omitempty"`
	MediaType string    `json:"media_type,omitempty"`
	ETag      string    `json:"etag,omitempty"`
	FetchedAt time.Time `json:"fetched_at"`
}

type tagIndexEntry struct {
	Key       string    `json:"key"`
	Tags      []string  `json:"tags"`
	ETag      string    `json:"etag,omitempty"`
	FetchedAt time.Time `json:"fetched_at"`
}

type catalogIndexEntry struct {
	Key       string    `json:"key"`
	Repos     []string  `json:"repos"`
	FetchedAt time.Time `json:"fetched_at"`
}

func newDiskCache(cfg cacheConfig, authClient *auth.Client, plainHTTP bool) (*diskCache, error) {
	if cfg.dir == "" {
		return nil, errors.New("cache directory required")
	}
	for _, sub := range []string{"blobs", "refs", "tags", "catalog"} {
		if err := os.MkdirAll(filepath.Join(cfg.dir, sub), 0o755); err != nil {
			return nil, fmt.Errorf("cache setup: %w", err)
		}
	}
	storage, err := orasoci.NewStorage(filepath.Join(cfg.dir, "blobs"))
	if err != nil {
		return nil, fmt.Errorf("creating blob storage: %w", err)
	}
	return &diskCache{
		cfg:        cfg,
		authClient: authClient,
		plainHTTP:  plainHTTP,
		storage:    storage,
	}, nil
}

// Close waits for in-flight background work to finish. Safe to call more
// than once.
func (d *diskCache) Close() error {
	if d.closed.Swap(true) {
		return nil
	}
	d.bg.Wait()
	return nil
}

// ResolveTag looks up the manifest digest for a tag reference, consulting
// the reference index (layer B) and falling back to a HEAD probe.
func (d *diskCache) ResolveTag(ctx context.Context, ref string) (string, error) {
	desc, err := d.ResolveManifest(ctx, ref)
	if err != nil {
		return "", err
	}
	return desc.Digest.String(), nil
}

// ResolveManifest returns the full manifest descriptor for a tag reference,
// including size and media type captured from the HEAD response. Falls back
// to an entry with a synthetic descriptor if the ref contains a digest.
func (d *diskCache) ResolveManifest(ctx context.Context, ref string) (ocispec.Descriptor, error) {
	if strings.Contains(ref, "@") {
		dgst := digest.Digest(digestFromRef(ref))
		if err := dgst.Validate(); err != nil {
			return ocispec.Descriptor{}, fmt.Errorf("cache: invalid digest in %q: %w", ref, err)
		}
		return ocispec.Descriptor{
			Digest:    dgst,
			MediaType: ocispec.MediaTypeImageManifest,
		}, nil
	}
	host, repo, tag, err := splitFullRef(ref)
	if err != nil {
		return ocispec.Descriptor{}, err
	}
	key := host + "/" + repo + ":" + tag
	path := d.indexPath("refs", key)

	entry, ok := readRefIndex(path)
	if ok && entry.Key == key {
		age := time.Since(entry.FetchedAt)
		if age < d.cfg.freshTTL {
			return descriptorFromRefEntry(entry), nil
		}
		if age < d.cfg.staleTTL {
			d.maybeRevalidateRef(ctx, host, repo, tag, key, path, entry)
			return descriptorFromRefEntry(entry), nil
		}
	}

	v, err, _ := d.sf.Do("ref:"+key, func() (any, error) {
		desc, etag, err := d.probeTag(ctx, host, repo, tag, "")
		if err != nil {
			return nil, err
		}
		next := refIndexEntry{
			Key:       key,
			Digest:    desc.Digest.String(),
			Size:      desc.Size,
			MediaType: desc.MediaType,
			ETag:      etag,
			FetchedAt: time.Now(),
		}
		_ = writeIndexAtomic(path, next)
		return desc, nil
	})
	if err != nil {
		return ocispec.Descriptor{}, err
	}
	return v.(ocispec.Descriptor), nil
}

// descriptorFromRefEntry builds a descriptor from an index entry, defaulting
// the media type to OCI manifest when the entry predates media-type storage.
func descriptorFromRefEntry(e refIndexEntry) ocispec.Descriptor {
	mt := e.MediaType
	if mt == "" {
		mt = ocispec.MediaTypeImageManifest
	}
	return ocispec.Descriptor{
		Digest:    ocispecDigest(e.Digest),
		Size:      e.Size,
		MediaType: mt,
	}
}

// Tags returns the cached tag list for a repository, refreshing with a
// conditional GET when stale.
func (d *diskCache) Tags(ctx context.Context, repo string) ([]string, error) {
	host, name := splitHostPath(repo)
	if host == "" || name == "" {
		return nil, fmt.Errorf("cache: invalid repository %q", repo)
	}
	key := host + "/" + name
	path := d.indexPath("tags", key)

	entry, ok := readTagIndex(path)
	if ok && entry.Key == key {
		age := time.Since(entry.FetchedAt)
		if age < d.cfg.freshTTL {
			return append([]string(nil), entry.Tags...), nil
		}
		if age < d.cfg.staleTTL {
			d.maybeRevalidateTags(ctx, host, name, key, path, entry)
			return append([]string(nil), entry.Tags...), nil
		}
	}

	v, err, _ := d.sf.Do("tags:"+key, func() (any, error) {
		var prevETag string
		if ok {
			prevETag = entry.ETag
		}
		tags, newETag, notModified, err := d.fetchTags(ctx, host, name, prevETag)
		if err != nil {
			return nil, err
		}
		if notModified && ok {
			next := entry
			next.FetchedAt = time.Now()
			_ = writeIndexAtomic(path, next)
			return append([]string(nil), entry.Tags...), nil
		}
		next := tagIndexEntry{Key: key, Tags: tags, ETag: newETag, FetchedAt: time.Now()}
		_ = writeIndexAtomic(path, next)
		return append([]string(nil), tags...), nil
	})
	if err != nil {
		return nil, err
	}
	return v.([]string), nil
}

// Repositories returns the cached catalog for a registry base.
func (d *diskCache) Repositories(ctx context.Context, base string) ([]string, error) {
	host, prefix := splitHostPath(base)
	if host == "" {
		return nil, fmt.Errorf("cache: invalid base %q", base)
	}
	key := host + "/" + prefix
	path := d.indexPath("catalog", key)

	entry, ok := readCatalogIndex(path)
	if ok && entry.Key == key {
		age := time.Since(entry.FetchedAt)
		if age < d.cfg.freshTTL {
			return append([]string(nil), entry.Repos...), nil
		}
		if age < d.cfg.catalogStaleTTL {
			d.maybeRevalidateCatalog(ctx, host, prefix, key, path)
			return append([]string(nil), entry.Repos...), nil
		}
	}

	v, err, _ := d.sf.Do("catalog:"+key, func() (any, error) {
		repos, err := d.fetchCatalog(ctx, host, prefix)
		if err != nil {
			return nil, err
		}
		next := catalogIndexEntry{Key: key, Repos: repos, FetchedAt: time.Now()}
		_ = writeIndexAtomic(path, next)
		return append([]string(nil), repos...), nil
	})
	if err != nil {
		return nil, err
	}
	return v.([]string), nil
}

// maxUnknownSizeBytes bounds in-memory reads when a descriptor's size is
// unknown. 64 MiB is large enough for any realistic manifest, index, or
// config blob while preventing unbounded memory growth from a hostile or
// misbehaving registry.
const maxUnknownSizeBytes int64 = 64 * 1024 * 1024

// Fetch serves a manifest or blob from the content store. Misses are
// satisfied by fetching from the registry, verifying the digest, and
// inserting into the store. Descriptors with an unknown size bypass the
// content store (oras-go's Push requires exact size for verification)
// but the response is still digest-verified in process before being
// returned to the caller.
func (d *diskCache) Fetch(ctx context.Context, repo string, desc ocispec.Descriptor) (io.ReadCloser, error) {
	if err := desc.Digest.Validate(); err != nil {
		return nil, fmt.Errorf("cache: invalid descriptor digest: %w", err)
	}
	if desc.Size > 0 {
		if exists, err := d.storage.Exists(ctx, desc); err == nil && exists {
			return d.storage.Fetch(ctx, desc)
		}
	}

	v, err, _ := d.sf.Do("blob:"+desc.Digest.String(), func() (any, error) {
		host, name := splitHostPath(repo)
		if host == "" || name == "" {
			return nil, fmt.Errorf("cache: invalid repository %q", repo)
		}
		rc, err := d.fetchContent(ctx, host, name, desc)
		if err != nil {
			return nil, err
		}
		defer rc.Close()

		var limit int64
		if desc.Size > 0 {
			limit = desc.Size
		} else {
			limit = maxUnknownSizeBytes
		}
		// Read one byte past the limit so an oversize body fails fast
		// rather than silently truncating.
		data, err := io.ReadAll(io.LimitReader(rc, limit+1))
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", desc.Digest, err)
		}
		if int64(len(data)) > limit {
			return nil, fmt.Errorf("cache: content for %s exceeds size limit %d", desc.Digest, limit)
		}
		if desc.Size > 0 && int64(len(data)) != desc.Size {
			return nil, fmt.Errorf("cache: short read for %s: got %d want %d", desc.Digest, len(data), desc.Size)
		}
		// Always verify the digest before the bytes leave this function.
		// The oras-go content store would do this on Push, but we cannot
		// rely on Push for the unknown-size path.
		if err := verifyDigest(desc.Digest, data); err != nil {
			return nil, err
		}
		if desc.Size > 0 {
			pushErr := d.storage.Push(ctx, desc, bytes.NewReader(data))
			if pushErr != nil && !isAlreadyExists(pushErr) {
				return nil, fmt.Errorf("storing %s: %w", desc.Digest, pushErr)
			}
			d.evictIfNeeded()
		}
		return data, nil
	})
	if err != nil {
		return nil, err
	}
	data := v.([]byte)
	return io.NopCloser(bytes.NewReader(data)), nil
}

// verifyDigest returns an error if the sha256 of data does not match d.
// Only sha256 digests are supported; anything else is rejected.
func verifyDigest(d digest.Digest, data []byte) error {
	if d.Algorithm() != digest.SHA256 {
		return fmt.Errorf("cache: unsupported digest algorithm %q", d.Algorithm())
	}
	sum := sha256.Sum256(data)
	got := digest.NewDigestFromBytes(digest.SHA256, sum[:])
	if got != d {
		return fmt.Errorf("cache: digest mismatch: got %s want %s", got, d)
	}
	return nil
}

// --- revalidation helpers ---

func (d *diskCache) maybeRevalidateRef(ctx context.Context, host, repo, tag, key, path string, prev refIndexEntry) {
	if !d.cfg.backgroundRefresh || d.closed.Load() {
		return
	}
	d.bg.Add(1)
	go func() {
		defer d.bg.Done()
		bgCtx, cancel := detachCtx(ctx, 30*time.Second)
		defer cancel()
		_, _, _ = d.sf.Do("ref:"+key, func() (any, error) {
			desc, etag, err := d.probeTag(bgCtx, host, repo, tag, prev.ETag)
			if err != nil {
				return nil, nil
			}
			next := refIndexEntry{
				Key:       key,
				Digest:    desc.Digest.String(),
				Size:      desc.Size,
				MediaType: desc.MediaType,
				ETag:      etag,
				FetchedAt: time.Now(),
			}
			_ = writeIndexAtomic(path, next)
			return nil, nil
		})
	}()
}

func (d *diskCache) maybeRevalidateTags(ctx context.Context, host, repo, key, path string, prev tagIndexEntry) {
	if !d.cfg.backgroundRefresh || d.closed.Load() {
		return
	}
	d.bg.Add(1)
	go func() {
		defer d.bg.Done()
		bgCtx, cancel := detachCtx(ctx, 30*time.Second)
		defer cancel()
		_, _, _ = d.sf.Do("tags:"+key, func() (any, error) {
			tags, etag, notModified, err := d.fetchTags(bgCtx, host, repo, prev.ETag)
			if err != nil {
				return nil, nil
			}
			if notModified {
				next := prev
				next.FetchedAt = time.Now()
				_ = writeIndexAtomic(path, next)
				return nil, nil
			}
			next := tagIndexEntry{Key: key, Tags: tags, ETag: etag, FetchedAt: time.Now()}
			_ = writeIndexAtomic(path, next)
			return nil, nil
		})
	}()
}

func (d *diskCache) maybeRevalidateCatalog(ctx context.Context, host, prefix, key, path string) {
	if !d.cfg.backgroundRefresh || d.closed.Load() {
		return
	}
	d.bg.Add(1)
	go func() {
		defer d.bg.Done()
		bgCtx, cancel := detachCtx(ctx, 60*time.Second)
		defer cancel()
		_, _, _ = d.sf.Do("catalog:"+key, func() (any, error) {
			repos, err := d.fetchCatalog(bgCtx, host, prefix)
			if err != nil {
				return nil, nil
			}
			next := catalogIndexEntry{Key: key, Repos: repos, FetchedAt: time.Now()}
			_ = writeIndexAtomic(path, next)
			return nil, nil
		})
	}()
}

// --- file layout helpers ---

func (d *diskCache) indexPath(sub, key string) string {
	sum := sha256.Sum256([]byte(key))
	name := hex.EncodeToString(sum[:])
	return filepath.Join(d.cfg.dir, sub, name+".json")
}

func readRefIndex(path string) (refIndexEntry, bool) {
	var e refIndexEntry
	data, err := os.ReadFile(path)
	if err != nil {
		return e, false
	}
	if err := json.Unmarshal(data, &e); err != nil {
		return e, false
	}
	return e, true
}

func readTagIndex(path string) (tagIndexEntry, bool) {
	var e tagIndexEntry
	data, err := os.ReadFile(path)
	if err != nil {
		return e, false
	}
	if err := json.Unmarshal(data, &e); err != nil {
		return e, false
	}
	return e, true
}

func readCatalogIndex(path string) (catalogIndexEntry, bool) {
	var e catalogIndexEntry
	data, err := os.ReadFile(path)
	if err != nil {
		return e, false
	}
	if err := json.Unmarshal(data, &e); err != nil {
		return e, false
	}
	return e, true
}

// writeIndexAtomic serializes v to JSON and writes it to path via a temp
// file in the same directory followed by os.Rename. Two processes racing
// on the same path each produce a valid final state -- the later renamer
// wins.
func writeIndexAtomic(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	var suffix [8]byte
	_, _ = rand.Read(suffix[:])
	tmp := filepath.Join(dir, filepath.Base(path)+".tmp."+hex.EncodeToString(suffix[:]))
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

// --- LRU eviction for the content store ---

type blobInfo struct {
	path  string
	size  int64
	mtime time.Time
}

// evictIfNeeded walks the blob tree and evicts oldest entries until the
// total byte count falls within the configured limit. It is a best-effort
// background operation; it returns no error and swallows filesystem races.
func (d *diskCache) evictIfNeeded() {
	limit := d.cfg.maxBytes
	if limit <= 0 {
		return
	}
	root := filepath.Join(d.cfg.dir, "blobs", "sha256")
	var blobs []blobInfo
	var total int64
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		blobs = append(blobs, blobInfo{path: path, size: info.Size(), mtime: info.ModTime()})
		total += info.Size()
		return nil
	})
	if total <= limit {
		return
	}
	sort.Slice(blobs, func(i, j int) bool { return blobs[i].mtime.Before(blobs[j].mtime) })
	for _, b := range blobs {
		if total <= limit {
			return
		}
		if err := os.Remove(b.path); err == nil {
			total -= b.size
		}
	}
}

// --- utility helpers ---

// splitHostPath splits "host/path" into host and path. Returns empty path
// if the input has no slash.
func splitHostPath(s string) (host, path string) {
	idx := strings.Index(s, "/")
	if idx < 0 {
		return s, ""
	}
	return s[:idx], s[idx+1:]
}

// splitFullRef splits "host/repo:tag" into host, repo, tag. Returns an
// error if the ref does not contain a tag.
func splitFullRef(ref string) (host, repo, tag string, err error) {
	h, rest := splitHostPath(ref)
	if h == "" || rest == "" {
		return "", "", "", fmt.Errorf("cache: invalid reference %q", ref)
	}
	idx := strings.LastIndex(rest, ":")
	if idx < 0 {
		return "", "", "", fmt.Errorf("cache: reference %q missing tag", ref)
	}
	return h, rest[:idx], rest[idx+1:], nil
}

func digestFromRef(ref string) string {
	idx := strings.Index(ref, "@")
	if idx < 0 {
		return ""
	}
	return ref[idx+1:]
}

// detachCtx returns a fresh background context bounded by timeout. Used
// for background revalidation so in-flight refreshes are not cancelled
// when the caller's (typically request-scoped) context ends.
func detachCtx(_ context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), timeout)
}

func isAlreadyExists(err error) bool {
	return err != nil && strings.Contains(err.Error(), "already exists")
}
