package oci

import (
	"context"
	"testing"

	"oras.land/oras-go/v2/registry/remote/auth"
)

func TestWithPullCredentials(t *testing.T) {
	cred := auth.Credential{Username: "pull-user", Password: "pull-pass"}
	f := func(ctx context.Context, hostport string) (auth.Credential, error) {
		return cred, nil
	}

	var po pullOptions
	WithPullCredentials(f)(&po)

	if po.credentialFunc == nil {
		t.Fatal("expected credentialFunc to be set")
	}

	got, err := po.credentialFunc(context.Background(), "registry.example.com")
	if err != nil {
		t.Fatalf("credentialFunc: %v", err)
	}
	if got != cred {
		t.Errorf("credential = %+v, want %+v", got, cred)
	}
}

func TestPullOptions_NilByDefault(t *testing.T) {
	var po pullOptions
	if po.credentialFunc != nil {
		t.Error("expected nil credentialFunc by default")
	}
}
