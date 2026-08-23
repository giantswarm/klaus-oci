package oci

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

func TestCreateAndExtractTarGz(t *testing.T) {
	// Create a source directory with test files.
	srcDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(srcDir, "subdir"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "hello.txt"), []byte(testValueHello), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "subdir", "world.txt"), []byte("world"), 0o600); err != nil {
		t.Fatal(err)
	}

	// Create tar.gz.
	data, err := createTarGz(srcDir)
	if err != nil {
		t.Fatalf("createTarGz: %v", err)
	}

	if len(data) == 0 {
		t.Fatal("expected non-empty archive")
	}

	// Extract to a new directory.
	destDir := t.TempDir()
	if err := extractTarGz(bytes.NewReader(data), destDir); err != nil {
		t.Fatalf("extractTarGz: %v", err)
	}

	// Verify extracted files.
	content, err := os.ReadFile(filepath.Clean(filepath.Join(destDir, "hello.txt")))
	if err != nil {
		t.Fatalf("reading hello.txt: %v", err)
	}
	if string(content) != testValueHello {
		t.Errorf("hello.txt = %q, want %q", content, testValueHello)
	}

	content, err = os.ReadFile(filepath.Clean(filepath.Join(destDir, "subdir", "world.txt")))
	if err != nil {
		t.Fatalf("reading subdir/world.txt: %v", err)
	}
	if string(content) != "world" {
		t.Errorf("subdir/world.txt = %q, want %q", content, "world")
	}
}

func TestCreateTarGz_SkipsCacheFile(t *testing.T) {
	srcDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(srcDir, "data.txt"), []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, cacheFileName), []byte("cache"), 0o600); err != nil {
		t.Fatal(err)
	}

	data, err := createTarGz(srcDir)
	if err != nil {
		t.Fatalf("createTarGz: %v", err)
	}

	// Verify the cache file is not in the archive.
	gzr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = gzr.Close() }()

	tr := tar.NewReader(gzr)
	for {
		header, err := tr.Next()
		if err != nil {
			break
		}
		if header.Name == cacheFileName {
			t.Errorf("cache file %q should not be in archive", cacheFileName)
		}
	}
}

func TestExtractTarGz_PathTraversal(t *testing.T) {
	var buf bytes.Buffer
	gzw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gzw)

	// Write a malicious entry with path traversal.
	if err := tw.WriteHeader(&tar.Header{
		Name:     "../escape.txt",
		Mode:     0o644,
		Size:     4,
		Typeflag: tar.TypeReg,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write([]byte("evil")); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzw.Close(); err != nil {
		t.Fatal(err)
	}

	destDir := t.TempDir()
	err := extractTarGz(&buf, destDir)
	if err == nil {
		t.Error("expected error for path traversal attempt")
	}
}

func TestExtractTarGz_FileSizeLimit(t *testing.T) {
	var buf bytes.Buffer
	gzw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gzw)

	// Write a header claiming a huge file.
	if err := tw.WriteHeader(&tar.Header{
		Name:     "huge.bin",
		Mode:     0o644,
		Size:     maxExtractFileSize + 100,
		Typeflag: tar.TypeReg,
	}); err != nil {
		t.Fatal(err)
	}
	// Write just enough to exceed the limit check.
	bigData := make([]byte, maxExtractFileSize+100)
	if _, err := tw.Write(bigData); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzw.Close(); err != nil {
		t.Fatal(err)
	}

	destDir := t.TempDir()
	err := extractTarGz(&buf, destDir)
	if err == nil {
		t.Error("expected error for oversized file")
	}
}
