package oci

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"oras.land/oras-go/v2/registry/remote/auth"
)

// dockerConfig represents the Docker/Podman credential config file format.
type dockerConfig struct {
	Auths map[string]dockerAuthEntry `json:"auths"`
}

// dockerAuthEntry holds a single registry credential.
type dockerAuthEntry struct {
	Auth string `json:"auth"` // base64(username:password)
}

// newAuthClientFromFunc creates an auth.Client that uses the given
// credential function for authentication.
func newAuthClientFromFunc(f auth.CredentialFunc) *auth.Client {
	return &auth.Client{
		Client:     http.DefaultClient,
		Cache:      auth.NewCache(),
		Credential: f,
	}
}

// newAuthClient creates an auth.Client that resolves credentials from
// Docker/Podman config files. If registryAuthEnv is non-empty, the named
// environment variable is checked first for a base64-encoded Docker config JSON.
func newAuthClient(registryAuthEnv string) *auth.Client {
	return newAuthClientFromFunc(func(ctx context.Context, hostport string) (auth.Credential, error) {
		return resolveCredential(registryAuthEnv, hostport)
	})
}

// resolveCredential resolves registry credentials in priority order:
//  1. Environment variable (if registryAuthEnv is non-empty): base64-encoded Docker config JSON
//  2. Docker config at ~/.docker/config.json
//  3. Podman auth at $XDG_RUNTIME_DIR/containers/auth.json
//  4. Anonymous (empty credential)
func resolveCredential(registryAuthEnv, hostport string) (auth.Credential, error) {
	if registryAuthEnv != "" {
		if envAuth := os.Getenv(registryAuthEnv); envAuth != "" {
			if cred, ok := credentialFromEnv(envAuth, hostport); ok {
				return cred, nil
			}
		}
	}

	if home, err := os.UserHomeDir(); err == nil {
		dockerCfg := filepath.Join(home, ".docker", "config.json")
		if cred, ok := credentialFromFile(dockerCfg, hostport); ok {
			return cred, nil
		}
	}

	if runtimeDir := os.Getenv("XDG_RUNTIME_DIR"); runtimeDir != "" {
		podmanAuth := filepath.Join(runtimeDir, "containers", "auth.json")
		if cred, ok := credentialFromFile(podmanAuth, hostport); ok {
			return cred, nil
		}
	}

	return auth.EmptyCredential, nil
}

// credentialFromEnv decodes a base64 Docker config JSON from the env var value.
func credentialFromEnv(envValue, hostport string) (auth.Credential, bool) {
	data, err := base64.StdEncoding.DecodeString(envValue)
	if err != nil {
		return auth.EmptyCredential, false
	}
	return credentialFromJSON(data, hostport)
}

// credentialFromFile reads a Docker/Podman config file and extracts
// credentials for the given registry host.
func credentialFromFile(path, hostport string) (auth.Credential, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return auth.EmptyCredential, false
	}
	return credentialFromJSON(data, hostport)
}

// credentialFromJSON extracts credentials for a specific host from
// a Docker-format config JSON.
func credentialFromJSON(data []byte, hostport string) (auth.Credential, bool) {
	var cfg dockerConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return auth.EmptyCredential, false
	}

	entry, ok := cfg.Auths[hostport]
	if !ok {
		// Try without port (e.g. "registry.example.com" for "registry.example.com:443").
		host := hostport
		if idx := strings.LastIndex(host, ":"); idx > 0 {
			host = host[:idx]
		}
		entry, ok = cfg.Auths[host]
	}
	if !ok {
		return auth.EmptyCredential, false
	}

	decoded, err := base64.StdEncoding.DecodeString(entry.Auth)
	if err != nil {
		return auth.EmptyCredential, false
	}

	parts := strings.SplitN(string(decoded), ":", 2)
	if len(parts) != 2 {
		return auth.EmptyCredential, false
	}

	return auth.Credential{
		Username: parts[0],
		Password: parts[1],
	}, true
}
