package oci

import (
	"context"
	"fmt"

	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
)

// Client is an ORAS-based client for interacting with OCI registries
// that host Klaus artifacts (plugins and personalities).
type Client struct {
	plainHTTP  bool
	authClient *auth.Client
}

// ClientOption configures the OCI client.
type ClientOption func(*Client)

// WithPlainHTTP disables TLS for registry communication.
// This is useful for local testing with insecure registries.
func WithPlainHTTP(plain bool) ClientOption {
	return func(c *Client) { c.plainHTTP = plain }
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
		authClient: newAuthClient(""),
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
