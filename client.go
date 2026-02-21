package oci

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
)

// Client is an ORAS-based client for interacting with OCI registries
// that host Klaus artifacts (plugins and personalities).
type Client struct {
	plainHTTP   bool
	authClient  *auth.Client
	concurrency int
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
// for batch methods like ListArtifacts. Defaults to 10.
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

// NewClient creates a new OCI client for Klaus artifacts.
func NewClient(opts ...ClientOption) *Client {
	c := &Client{
		authClient:  newAuthClient(""),
		concurrency: defaultConcurrency,
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// Resolve resolves a reference (tag or digest) to its manifest digest.
func (c *Client) Resolve(ctx context.Context, ref string) (string, error) {
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

// ListRepositories queries the OCI registry catalog to find all repositories
// under the given base path. The base path format is
// "registry.example.com/org/prefix" (e.g.,
// "gsoci.azurecr.io/giantswarm/klaus-plugins"). Returns fully-qualified
// repository references (e.g.,
// "gsoci.azurecr.io/giantswarm/klaus-plugins/gs-base").
//
// The catalog is queried using a seek position to skip repositories that
// sort before the prefix, and enumeration stops as soon as a page is
// encountered where all entries sort after the prefix. This makes the
// operation significantly faster on large registries.
func (c *Client) ListRepositories(ctx context.Context, registryBase string) ([]string, error) {
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
