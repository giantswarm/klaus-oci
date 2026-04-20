package oci

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
)

// Client is an ORAS-based client for interacting with OCI registries
// that host Klaus artifacts (plugins and personalities).
type Client struct {
	plainHTTP   bool
	authClient  *auth.Client
	concurrency int

	// cache configuration captured from WithCache*. The store itself is
	// created lazily on first use so construction errors surface on the
	// first cache-using call rather than forcing NewClient to change
	// signature.
	cacheCfg  cacheConfig
	storeOnce sync.Once
	store     CacheStore
	storeErr  error
}

const defaultConcurrency = 10

// ClientOption configures the OCI client.
type ClientOption func(*Client)

// WithPlainHTTP disables TLS for registry communication.
// This is useful for local testing with insecure registries.
func WithPlainHTTP(plain bool) ClientOption {
	return func(c *Client) { c.plainHTTP = plain }
}

// WithConcurrency sets the maximum number of concurrent registry operations
// for batch listing methods. Defaults to 10.
func WithConcurrency(n int) ClientOption {
	return func(c *Client) {
		if n > 0 {
			c.concurrency = n
		}
	}
}

// WithRegistryAuthEnv sets the environment variable name to check for
// base64-encoded Docker config JSON credentials. If empty (the default),
// no environment variable is checked and only Docker/Podman config files
// are used for credential resolution.
func WithRegistryAuthEnv(envName string) ClientOption {
	return func(c *Client) {
		c.authClient = newAuthClient(envName)
	}
}

// WithCache enables the on-disk registry response cache rooted at dir.
// The cache accelerates tag resolves, tag lists, catalog lookups, and
// manifest/blob fetches. See CacheStore for invalidation details. An
// empty dir leaves the cache disabled.
func WithCache(dir string) ClientOption {
	return func(c *Client) { c.cacheCfg.dir = dir }
}

// WithCacheTTL overrides the default fresh and stale TTLs for reference
// and tag-list entries. The catalog TTL is not affected. A non-positive
// value keeps the default for that field.
func WithCacheTTL(fresh, stale time.Duration) ClientOption {
	return func(c *Client) {
		if fresh > 0 {
			c.cacheCfg.freshTTL = fresh
		}
		if stale > 0 {
			c.cacheCfg.staleTTL = stale
		}
	}
}

// WithCacheMaxSize overrides the default LRU budget (bytes) for the
// content store. A non-positive value disables eviction.
func WithCacheMaxSize(bytes int64) ClientOption {
	return func(c *Client) { c.cacheCfg.maxBytes = bytes }
}

// WithBackgroundRefresh toggles async revalidation of stale-but-usable
// cache entries. Default on. When off, stale entries still return
// immediately but no background probe is issued; the entry is refetched
// synchronously only when it ages past the stale TTL.
func WithBackgroundRefresh(enabled bool) ClientOption {
	return func(c *Client) { c.cacheCfg.backgroundRefresh = enabled }
}

// NewClient creates a new OCI client for Klaus artifacts.
func NewClient(opts ...ClientOption) *Client {
	c := &Client{
		authClient:  newAuthClient(""),
		concurrency: defaultConcurrency,
		cacheCfg:    defaultCacheConfig(),
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// cacheStore returns the lazily-initialised cache store, or (nil, nil) when
// caching is not configured.
func (c *Client) cacheStore() (CacheStore, error) {
	if c.cacheCfg.dir == "" {
		return nil, nil
	}
	c.storeOnce.Do(func() {
		c.store, c.storeErr = newDiskCache(c.cacheCfg, c.authClient, c.plainHTTP)
	})
	return c.store, c.storeErr
}

// CloseCache releases any cache resources held by the client. It is safe
// to call even when no cache was configured.
func (c *Client) CloseCache() error {
	store, err := c.cacheStore()
	if err != nil || store == nil {
		return err
	}
	return store.Close()
}

// Resolve resolves a reference (tag or digest) to its manifest digest.
func (c *Client) Resolve(ctx context.Context, ref string) (string, error) {
	if hasDigest(ref) {
		return digestFromRef(ref), nil
	}

	store, err := c.cacheStore()
	if err != nil {
		return "", err
	}
	if store != nil {
		if digest, cerr := store.ResolveTag(ctx, ref); cerr == nil {
			return digest, nil
		}
		// Cache failed: fall through to the uncached path rather than
		// propagating cache errors to callers.
	}

	repo, tag, err := c.newRepository(ref)
	if err != nil {
		return "", err
	}
	if tag == "" {
		return "", fmt.Errorf("reference %q must include a tag or digest", ref)
	}
	desc, err := repo.Resolve(ctx, tag)
	if err != nil {
		return "", fmt.Errorf("resolving %s: %w", ref, err)
	}
	return desc.Digest.String(), nil
}

// listRepositories queries the OCI registry catalog to find all repositories
// under the given base path.
func (c *Client) listRepositories(ctx context.Context, registryBase string) ([]string, error) {
	store, err := c.cacheStore()
	if err != nil {
		return nil, err
	}
	if store != nil {
		if repos, cerr := store.Repositories(ctx, registryBase); cerr == nil {
			return repos, nil
		}
	}

	host, prefix := SplitRegistryBase(registryBase)

	reg, err := remote.NewRegistry(host)
	if err != nil {
		return nil, fmt.Errorf("creating registry client for %s: %w", host, err)
	}
	reg.PlainHTTP = c.plainHTTP
	reg.Client = c.authClient

	// Seek past repositories that sort before our prefix by using the
	// catalog's `last` parameter. We trim the trailing "/" from the prefix
	// so the enumeration begins just before the first matching repo (the
	// catalog returns entries strictly after the `last` value).
	seekPos := strings.TrimSuffix(prefix, "/")

	var repos []string
	err = reg.Repositories(ctx, seekPos, func(batch []string) error {
		for _, name := range batch {
			if !strings.HasPrefix(name, prefix) {
				if name > prefix {
					return errStopIteration
				}
				continue
			}
			repos = append(repos, host+"/"+name)
		}
		return nil
	})
	if err != nil && !errors.Is(err, errStopIteration) {
		return nil, fmt.Errorf("listing repositories in %s: %w", registryBase, err)
	}

	return repos, nil
}

// errStopIteration is a sentinel used to break out of paginated catalog
// enumeration once all matching repositories have been found.
var errStopIteration = errors.New("stop iteration")

// List returns all tags in the given repository.
func (c *Client) List(ctx context.Context, repository string) ([]string, error) {
	store, err := c.cacheStore()
	if err != nil {
		return nil, err
	}
	if store != nil {
		if tags, cerr := store.Tags(ctx, repository); cerr == nil {
			return tags, nil
		}
	}

	repo, err := c.newRepositoryFromName(repository)
	if err != nil {
		return nil, err
	}

	var tags []string
	err = repo.Tags(ctx, "", func(t []string) error {
		tags = append(tags, t...)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("listing tags for %s: %w", repository, err)
	}

	return tags, nil
}

// fetchWithStore fetches a manifest or blob through the cache store when
// configured, falling back to a direct registry fetch on miss or error.
// repoName is the "host/repo" path used to satisfy a cache miss.
func (c *Client) fetchWithStore(ctx context.Context, repo *remote.Repository, repoName string, desc ocispec.Descriptor) (io.ReadCloser, error) {
	store, err := c.cacheStore()
	if err == nil && store != nil {
		if rc, cerr := store.Fetch(ctx, repoName, desc); cerr == nil {
			return rc, nil
		}
	}
	return repo.Fetch(ctx, desc)
}

// resolveDescriptor returns the manifest descriptor for ref. When a cache
// store is configured it is consulted first, and its ResolveManifest
// result carries enough information (size, media type) for the content
// store's verified Push path. Cache errors fall back to the registry via
// the oras-go repository's Resolve (a HEAD).
func (c *Client) resolveDescriptor(ctx context.Context, repo *remote.Repository, ref, tag string) (ocispec.Descriptor, error) {
	store, err := c.cacheStore()
	if err == nil && store != nil {
		if desc, cerr := store.ResolveManifest(ctx, ref); cerr == nil {
			return desc, nil
		}
	}
	return repo.Resolve(ctx, tag)
}

// newRepository creates a remote.Repository from a full OCI reference string
// (e.g. "registry.example.com/repo:tag") and returns the repository client
// and the tag/digest portion.
func (c *Client) newRepository(ref string) (*remote.Repository, string, error) {
	repo, err := remote.NewRepository(ref)
	if err != nil {
		return nil, "", fmt.Errorf("parsing reference %q: %w", ref, err)
	}

	tag := repo.Reference.Reference
	repo.PlainHTTP = c.plainHTTP
	repo.Client = c.authClient

	return repo, tag, nil
}

// newRepositoryFromName creates a remote.Repository from a repository name
// (without tag or digest), used for listing tags.
func (c *Client) newRepositoryFromName(name string) (*remote.Repository, error) {
	repo, err := remote.NewRepository(name)
	if err != nil {
		return nil, fmt.Errorf("creating repository for %q: %w", name, err)
	}

	repo.PlainHTTP = c.plainHTTP
	repo.Client = c.authClient

	return repo, nil
}
