package oci

import (
	"context"
	"testing"

	"oras.land/oras-go/v2/registry/remote/auth"
)

func TestWithCredentialFunc(t *testing.T) {
	called := false
	creds := auth.Credential{Username: "k8s-sa", Password: "token123"}

	client := NewClient(WithCredentialFunc(func(ctx context.Context, hostport string) (auth.Credential, error) {
		called = true
		return creds, nil
	}))

	got, err := client.authClient.Credential(context.Background(), "registry.example.com")
	if err != nil {
		t.Fatalf("Credential: %v", err)
	}
	if !called {
		t.Error("expected custom credential func to be called")
	}
	if got != creds {
		t.Errorf("credential = %+v, want %+v", got, creds)
	}
}

func TestWithCredentialFunc_OverridesDefault(t *testing.T) {
	customCred := auth.Credential{Username: "custom", Password: "secret"}

	client := NewClient(
		WithRegistryAuthEnv("SOME_ENV"),
		WithCredentialFunc(func(ctx context.Context, hostport string) (auth.Credential, error) {
			return customCred, nil
		}),
	)

	got, err := client.authClient.Credential(context.Background(), "registry.example.com")
	if err != nil {
		t.Fatalf("Credential: %v", err)
	}
	if got != customCred {
		t.Errorf("expected WithCredentialFunc to override WithRegistryAuthEnv, got %+v", got)
	}
}

func TestWithPlainHTTP(t *testing.T) {
	client := NewClient(WithPlainHTTP(true))
	if !client.plainHTTP {
		t.Error("expected plainHTTP to be true")
	}
}

func TestNewClient_DefaultAuth(t *testing.T) {
	client := NewClient()
	if client.authClient == nil {
		t.Fatal("expected non-nil authClient")
	}

	cred, err := client.authClient.Credential(context.Background(), "nonexistent.registry.io")
	if err != nil {
		t.Fatalf("Credential: %v", err)
	}
	if cred != auth.EmptyCredential {
		t.Errorf("expected empty credential for unknown host, got %+v", cred)
	}
}
