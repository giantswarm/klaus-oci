package oci

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// cacheRegistry is a minimal OCI distribution API server with optional
// ETag support for tag listing, HEAD support for manifests, and request
// counters for testing caching behaviour.
type cacheRegistry struct {
	mu sync.Mutex

	// repos maps repo name -> tag -> manifest digest.
	repos map[string]map[string]string
	// manifests maps manifest digest -> bytes.
	manifests map[string][]byte
	// blobs maps blob digest -> bytes.
	blobs map[string][]byte
	// tagListETag maps repo -> current ETag string.
	tagListETag map[string]string
	// catalogRepos is the list of all repos in the catalog.
	catalogRepos []string

	// Request counters by bucket.
	headCount     atomic.Int64
	tagsCount     atomic.Int64
	catalogCount  atomic.Int64
	manifestCount atomic.Int64
	blobCount     atomic.Int64
	// notModCount counts 304 responses actually emitted.
	notModCount atomic.Int64
}

func newCacheRegistry() *cacheRegistry {
	return &cacheRegistry{
		repos:       map[string]map[string]string{},
		manifests:   map[string][]byte{},
		blobs:       map[string][]byte{},
		tagListETag: map[string]string{},
	}
}

func (r *cacheRegistry) addManifest(repo, tag string, body []byte) string {
	digest := "sha256:" + sum256Hex(body)
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.repos[repo] == nil {
		r.repos[repo] = map[string]string{}
	}
	r.repos[repo][tag] = digest
	r.manifests[digest] = body
	r.tagListETag[repo] = fmt.Sprintf("%q", sum256Hex([]byte(fmt.Sprintf("%v", r.repos[repo])))[:12])
	if !contains(r.catalogRepos, repo) {
		r.catalogRepos = append(r.catalogRepos, repo)
		sort.Strings(r.catalogRepos)
	}
	return digest
}

func (r *cacheRegistry) addBlob(body []byte) string {
	digest := "sha256:" + sum256Hex(body)
	r.mu.Lock()
	defer r.mu.Unlock()
	r.blobs[digest] = body
	return digest
}

// handler implements the subset of the distribution API the tests need.
func (r *cacheRegistry) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		path := req.URL.Path
		switch {
		case path == "/v2/" || path == "/v2":
			w.WriteHeader(http.StatusOK)
			return
		case path == "/v2/_catalog":
			r.catalogCount.Add(1)
			last := req.URL.Query().Get("last")
			r.mu.Lock()
			out := []string{}
			for _, name := range r.catalogRepos {
				if last == "" || name > last {
					out = append(out, name)
				}
			}
			r.mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string][]string{"repositories": out})
			return
		case strings.HasSuffix(path, "/tags/list"):
			r.tagsCount.Add(1)
			repo := strings.TrimPrefix(path, "/v2/")
			repo = strings.TrimSuffix(repo, "/tags/list")
			r.mu.Lock()
			tagMap := r.repos[repo]
			etag := r.tagListETag[repo]
			var tags []string
			for t := range tagMap {
				tags = append(tags, t)
			}
			r.mu.Unlock()
			sort.Strings(tags)
			if tagMap == nil {
				http.NotFound(w, req)
				return
			}
			if match := req.Header.Get("If-None-Match"); match != "" && match == etag {
				r.notModCount.Add(1)
				w.Header().Set("ETag", etag)
				w.WriteHeader(http.StatusNotModified)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("ETag", etag)
			_ = json.NewEncoder(w).Encode(map[string]any{"name": repo, "tags": tags})
			return
		case strings.Contains(path, "/manifests/"):
			idx := strings.Index(path, "/manifests/")
			repo := strings.TrimPrefix(path[:idx], "/v2/")
			ref := path[idx+len("/manifests/"):]
			r.mu.Lock()
			tagMap := r.repos[repo]
			digest := ref
			if !strings.HasPrefix(ref, "sha256:") {
				digest = tagMap[ref]
			}
			body := r.manifests[digest]
			r.mu.Unlock()
			if digest == "" || body == nil {
				http.NotFound(w, req)
				return
			}
			w.Header().Set("Docker-Content-Digest", digest)
			w.Header().Set("Content-Type", ocispec.MediaTypeImageManifest)
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(body)))
			if req.Method == http.MethodHead {
				r.headCount.Add(1)
				w.WriteHeader(http.StatusOK)
				return
			}
			r.manifestCount.Add(1)
			_, _ = w.Write(body)
			return
		case strings.Contains(path, "/blobs/"):
			idx := strings.Index(path, "/blobs/")
			digest := path[idx+len("/blobs/"):]
			r.mu.Lock()
			body := r.blobs[digest]
			r.mu.Unlock()
			if body == nil {
				http.NotFound(w, req)
				return
			}
			r.blobCount.Add(1)
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Header().Set("Docker-Content-Digest", digest)
			_, _ = w.Write(body)
			return
		}
		http.NotFound(w, req)
	})
}

func sum256Hex(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

func newCacheTestClient(t *testing.T, reg *cacheRegistry, opts ...ClientOption) (*Client, string, string) {
	t.Helper()
	ts := httptest.NewServer(reg.handler())
	t.Cleanup(ts.Close)
	host := strings.TrimPrefix(ts.URL, "http://")
	dir := t.TempDir()
	allOpts := append([]ClientOption{WithPlainHTTP(true), WithCache(dir)}, opts...)
	c := NewClient(allOpts...)
	t.Cleanup(func() { _ = c.CloseCache() })
	return c, host, dir
}

// TestCacheHit_NoNetwork verifies that the second call for a fresh entry
// does no network traffic.
func TestCacheHit_NoNetwork(t *testing.T) {
	reg := newCacheRegistry()
	digest := reg.addManifest("team/hello", "v1.0.0", []byte(`{"manifest":1}`))

	c, host, _ := newCacheTestClient(t, reg)
	ref := host + "/team/hello:v1.0.0"

	got, err := c.Resolve(context.Background(), ref)
	if err != nil {
		t.Fatal(err)
	}
	if got != digest {
		t.Fatalf("digest = %q, want %q", got, digest)
	}
	firstHead := reg.headCount.Load()

	got, err = c.Resolve(context.Background(), ref)
	if err != nil {
		t.Fatal(err)
	}
	if got != digest {
		t.Fatalf("digest = %q, want %q", got, digest)
	}
	if reg.headCount.Load() != firstHead {
		t.Errorf("expected no extra HEAD on fresh cache hit, head count %d -> %d", firstHead, reg.headCount.Load())
	}
}

// TestStaleWhileRevalidate returns the cached digest immediately and
// kicks off a background HEAD probe.
func TestStaleWhileRevalidate(t *testing.T) {
	reg := newCacheRegistry()
	digest := reg.addManifest("team/swr", "v1", []byte(`{"manifest":"a"}`))

	// Set a tiny fresh window so the next call is stale-but-usable.
	c, host, _ := newCacheTestClient(t, reg,
		WithCacheTTL(1*time.Millisecond, 1*time.Hour))

	ref := host + "/team/swr:v1"
	if _, err := c.Resolve(context.Background(), ref); err != nil {
		t.Fatal(err)
	}
	firstHead := reg.headCount.Load()
	time.Sleep(20 * time.Millisecond)

	got, err := c.Resolve(context.Background(), ref)
	if err != nil {
		t.Fatal(err)
	}
	if got != digest {
		t.Fatalf("digest = %q, want %q", got, digest)
	}

	// Wait for the background refresh to land.
	if err := c.CloseCache(); err != nil {
		t.Fatal(err)
	}
	if reg.headCount.Load() <= firstHead {
		t.Errorf("expected background HEAD probe after stale read: head count %d -> %d", firstHead, reg.headCount.Load())
	}
}

// TestTagList_ConditionalGet_NotModified verifies that a stale tag list
// is refreshed with a conditional GET and the server's 304 is honoured.
func TestTagList_ConditionalGet_NotModified(t *testing.T) {
	reg := newCacheRegistry()
	reg.addManifest("team/cgtags", "v1", []byte(`{"m":1}`))
	reg.addManifest("team/cgtags", "v2", []byte(`{"m":2}`))

	c, host, _ := newCacheTestClient(t, reg,
		WithCacheTTL(1*time.Millisecond, 10*time.Millisecond),
		WithBackgroundRefresh(false))

	repo := host + "/team/cgtags"
	got, err := c.List(context.Background(), repo)
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(got)
	if len(got) != 2 {
		t.Fatalf("tags = %v", got)
	}
	tagsBefore := reg.tagsCount.Load()
	notModBefore := reg.notModCount.Load()

	time.Sleep(25 * time.Millisecond)
	got, err = c.List(context.Background(), repo)
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(got)
	if len(got) != 2 {
		t.Fatalf("tags after revalidation = %v", got)
	}
	if reg.notModCount.Load() <= notModBefore {
		t.Errorf("expected at least one 304 response: notMod %d -> %d", notModBefore, reg.notModCount.Load())
	}
	if reg.tagsCount.Load() <= tagsBefore {
		t.Errorf("expected a conditional GET to be issued, tags count %d -> %d", tagsBefore, reg.tagsCount.Load())
	}
}

// TestTagList_ConditionalGet_Modified verifies that when the tag list
// changes, the cache is updated with the new list.
func TestTagList_ConditionalGet_Modified(t *testing.T) {
	reg := newCacheRegistry()
	reg.addManifest("team/cgtags2", "v1", []byte(`{"m":1}`))

	c, host, _ := newCacheTestClient(t, reg,
		WithCacheTTL(1*time.Millisecond, 10*time.Millisecond),
		WithBackgroundRefresh(false))

	repo := host + "/team/cgtags2"
	_, err := c.List(context.Background(), repo)
	if err != nil {
		t.Fatal(err)
	}

	// Publish a new tag -- ETag changes server-side.
	reg.addManifest("team/cgtags2", "v2", []byte(`{"m":2}`))

	time.Sleep(25 * time.Millisecond)
	got, err := c.List(context.Background(), repo)
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(got)
	if len(got) != 2 || got[0] != "v1" || got[1] != "v2" {
		t.Fatalf("tags = %v, want [v1 v2]", got)
	}
}

// TestHeadDigestMismatch verifies that when a HEAD probe returns a new
// digest, the cache updates its reference entry.
func TestHeadDigestMismatch(t *testing.T) {
	reg := newCacheRegistry()
	reg.addManifest("team/mv", "v1", []byte(`{"m":"old"}`))

	c, host, _ := newCacheTestClient(t, reg,
		WithCacheTTL(1*time.Millisecond, 10*time.Millisecond),
		WithBackgroundRefresh(false))

	ref := host + "/team/mv:v1"
	firstDigest, err := c.Resolve(context.Background(), ref)
	if err != nil {
		t.Fatal(err)
	}

	// Mutate the manifest on the server side (simulates a tag moving).
	newDigest := reg.addManifest("team/mv", "v1", []byte(`{"m":"new"}`))
	if newDigest == firstDigest {
		t.Fatalf("precondition: new digest should differ")
	}

	time.Sleep(25 * time.Millisecond)
	got, err := c.Resolve(context.Background(), ref)
	if err != nil {
		t.Fatal(err)
	}
	if got != newDigest {
		t.Errorf("digest after stale revalidate = %q, want %q", got, newDigest)
	}
}

// TestSingleflightCoalescing verifies that concurrent misses for the same
// key collapse to a single HEAD.
func TestSingleflightCoalescing(t *testing.T) {
	reg := newCacheRegistry()
	reg.addManifest("team/sf", "v1", []byte(`{"m":"sf"}`))

	c, host, _ := newCacheTestClient(t, reg)
	ref := host + "/team/sf:v1"

	// N concurrent Resolves on an empty cache.
	const n = 20
	var wg sync.WaitGroup
	errs := make([]error, n)
	for i := range n {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := c.Resolve(context.Background(), ref)
			errs[i] = err
		}(i)
	}
	wg.Wait()
	for _, e := range errs {
		if e != nil {
			t.Fatalf("concurrent Resolve error: %v", e)
		}
	}
	if got := reg.headCount.Load(); got > int64(n/2) {
		t.Errorf("HEAD count %d; expected single-flight coalescing to keep it much lower", got)
	}
	if got := reg.headCount.Load(); got == 0 {
		t.Errorf("expected at least one HEAD")
	}
}

// TestAtomicWriteConcurrent verifies that multiple writers racing on the
// same index file all produce a valid final state (one of them wins).
func TestAtomicWriteConcurrent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "race.json")

	const n = 16
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			entry := refIndexEntry{
				Key:       "k",
				Digest:    fmt.Sprintf("sha256:%064d", i),
				FetchedAt: time.Now(),
			}
			if err := writeIndexAtomic(path, entry); err != nil {
				t.Errorf("write %d: %v", i, err)
			}
		}(i)
	}
	wg.Wait()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read final: %v", err)
	}
	var final refIndexEntry
	if err := json.Unmarshal(data, &final); err != nil {
		t.Fatalf("final file is not valid JSON: %v\n%s", err, string(data))
	}
	if final.Key != "k" {
		t.Errorf("final key = %q, want %q", final.Key, "k")
	}
	// No temp files should linger.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") || strings.Contains(e.Name(), ".tmp.") {
			t.Errorf("lingering temp file: %s", e.Name())
		}
	}
}

// TestContentStoreCachesBlobs verifies that after a blob is fetched once,
// a second fetch of the same digest is served from the local content store.
func TestContentStoreCachesBlobs(t *testing.T) {
	reg := newCacheRegistry()
	body := []byte("hello world")
	digest := reg.addBlob(body)

	c, host, _ := newCacheTestClient(t, reg)
	store, err := c.cacheStore()
	if err != nil {
		t.Fatal(err)
	}
	desc := ocispec.Descriptor{
		Digest:    ocispecDigest(digest),
		Size:      int64(len(body)),
		MediaType: "application/octet-stream",
	}
	rc, err := store.Fetch(context.Background(), host+"/team/blob", desc)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(rc)
	rc.Close()
	if string(got) != string(body) {
		t.Fatalf("blob mismatch: %q vs %q", got, body)
	}
	firstBlob := reg.blobCount.Load()

	rc, err = store.Fetch(context.Background(), host+"/team/blob", desc)
	if err != nil {
		t.Fatal(err)
	}
	got2, _ := io.ReadAll(rc)
	rc.Close()
	if string(got2) != string(body) {
		t.Fatal("second fetch content mismatch")
	}
	if reg.blobCount.Load() != firstBlob {
		t.Errorf("expected content store hit: blob count %d -> %d", firstBlob, reg.blobCount.Load())
	}
}

// TestFetchDigestMismatchRejected verifies that if a registry returns bytes
// whose sha256 does not match the requested digest, Fetch rejects them and
// does not leak the bogus content to callers.
func TestFetchDigestMismatchRejected(t *testing.T) {
	reg := newCacheRegistry()
	// Register a blob under a digest that will not match the body bytes.
	body := []byte("trustworthy")
	bogusDigest := "sha256:" + sum256Hex([]byte("something else entirely"))
	reg.mu.Lock()
	reg.blobs[bogusDigest] = body
	reg.mu.Unlock()

	c, host, _ := newCacheTestClient(t, reg)
	store, err := c.cacheStore()
	if err != nil {
		t.Fatal(err)
	}
	desc := ocispec.Descriptor{
		Digest:    ocispecDigest(bogusDigest),
		Size:      int64(len(body)),
		MediaType: "application/octet-stream",
	}
	_, err = store.Fetch(context.Background(), host+"/team/bogus", desc)
	if err == nil {
		t.Fatal("expected digest-mismatch error, got nil")
	}
	if !strings.Contains(err.Error(), "digest mismatch") && !strings.Contains(err.Error(), "short read") {
		t.Errorf("expected digest mismatch / short read error, got: %v", err)
	}
}

// TestCatalogCaching verifies that repository catalog listings are served
// from the cache on repeat calls within the fresh window.
func TestCatalogCaching(t *testing.T) {
	reg := newCacheRegistry()
	reg.addManifest("team/one/alpha", "v1", []byte(`{}`))
	reg.addManifest("team/one/beta", "v1", []byte(`{}`))
	reg.addManifest("team/two/gamma", "v1", []byte(`{}`))

	c, host, _ := newCacheTestClient(t, reg)
	base := host + "/team/one"

	first, err := c.listRepositories(context.Background(), base)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 2 {
		t.Fatalf("first list = %v", first)
	}
	firstCatalog := reg.catalogCount.Load()

	second, err := c.listRepositories(context.Background(), base)
	if err != nil {
		t.Fatal(err)
	}
	if len(second) != 2 {
		t.Fatalf("second list = %v", second)
	}
	if reg.catalogCount.Load() != firstCatalog {
		t.Errorf("expected no catalog roundtrip on fresh cache hit: %d -> %d", firstCatalog, reg.catalogCount.Load())
	}
}

// TestFullResolveZeroNetwork simulates a klausctl "resolve personality
// deps" workflow: the first invocation populates the cache, the second
// does zero network calls within the fresh window.
func TestFullResolveZeroNetwork(t *testing.T) {
	reg := newCacheRegistry()
	reg.addManifest("team/personas/sre", "v0.1.0", []byte(`{"personality":1}`))
	reg.addManifest("team/plugins/gs-base", "v1.0.0", []byte(`{"plugin":1}`))
	reg.addManifest("team/plugins/gs-base", "v1.1.0", []byte(`{"plugin":2}`))
	reg.addManifest("team/toolchains/go", "v2.0.0", []byte(`{"toolchain":1}`))

	c, host, _ := newCacheTestClient(t, reg)
	ctx := context.Background()

	// First invocation: cold cache.
	_, err := c.ResolvePersonalityRef_withBase(ctx, "sre", host+"/team/personas")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.ResolvePluginRef_withBase(ctx, "gs-base", host+"/team/plugins"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.ResolveToolchainRef_withBase(ctx, "go", host+"/team/toolchains"); err != nil {
		t.Fatal(err)
	}

	before := reg.tagsCount.Load() + reg.headCount.Load() + reg.catalogCount.Load() + reg.blobCount.Load() + reg.manifestCount.Load()

	// Second invocation: everything must hit the cache.
	if _, err := c.ResolvePersonalityRef_withBase(ctx, "sre", host+"/team/personas"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.ResolvePluginRef_withBase(ctx, "gs-base", host+"/team/plugins"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.ResolveToolchainRef_withBase(ctx, "go", host+"/team/toolchains"); err != nil {
		t.Fatal(err)
	}

	after := reg.tagsCount.Load() + reg.headCount.Load() + reg.catalogCount.Load() + reg.blobCount.Load() + reg.manifestCount.Load()
	if after != before {
		t.Errorf("expected zero network calls on second invocation within fresh TTL; delta = %d", after-before)
	}
}

// ResolvePersonalityRef_withBase / ResolvePluginRef_withBase /
// ResolveToolchainRef_withBase are test-only conveniences that swap in a
// custom registry base without changing the public API.
func (c *Client) ResolvePersonalityRef_withBase(ctx context.Context, ref, base string) (string, error) {
	return resolveArtifactRef(ctx, c, ref, base)
}

func (c *Client) ResolvePluginRef_withBase(ctx context.Context, ref, base string) (string, error) {
	return resolveArtifactRef(ctx, c, ref, base)
}

func (c *Client) ResolveToolchainRef_withBase(ctx context.Context, ref, base string) (string, error) {
	return resolveArtifactRef(ctx, c, ref, base)
}
