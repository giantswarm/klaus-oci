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
	if err := os.MkdirAll(filepath.Join(srcDir, "subdir"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "hello.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "subdir", "world.txt"), []byte("world"), 0o644); err != nil {
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
	content, err := os.ReadFile(filepath.Join(destDir, "hello.txt"))
	if err != nil {
		t.Fatalf("reading hello.txt: %v", err)
	}
	if string(content) != "hello" {
		t.Errorf("hello.txt = %q, want %q", content, "hello")
	}

	content, err = os.ReadFile(filepath.Join(destDir, "subdir", "world.txt"))
	if err != nil {
		t.Fatalf("reading subdir/world.txt: %v", err)
	}
	if string(content) != "world" {
		t.Errorf("subdir/world.txt = %q, want %q", content, "world")
	}
}

func TestCreateTarGz_SkipsCacheFile(t *testing.T) {
	srcDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(srcDir, "data.txt"), []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, cacheFileName), []byte("cache"), 0o644); err != nil {
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
	defer gzr.Close()

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
	tw.WriteHeader(&tar.Header{
		Name:     "../escape.txt",
		Mode:     0o644,
		Size:     4,
		Typeflag: tar.TypeReg,
	})
	tw.Write([]byte("evil"))
	tw.Close()
	gzw.Close()

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
	tw.WriteHeader(&tar.Header{
		Name:     "huge.bin",
		Mode:     0o644,
		Size:     maxExtractFileSize + 100,
		Typeflag: tar.TypeReg,
	})
	// Write just enough to exceed the limit check.
	bigData := make([]byte, maxExtractFileSize+100)
	tw.Write(bigData)
	tw.Close()
	gzw.Close()

	destDir := t.TempDir()
	err := extractTarGz(&buf, destDir)
	if err == nil {
		t.Error("expected error for oversized file")
	}
}
