package oci

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// maxExtractFileSize is the per-file size limit during extraction (100 MB).
const maxExtractFileSize = 100 << 20

// extractTarGz extracts a gzip-compressed tar archive to destDir.
// It validates paths to prevent directory traversal attacks and limits
// individual file sizes.
func extractTarGz(r io.Reader, destDir string) error {
	gzr, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("creating gzip reader: %w", err)
	}
	defer gzr.Close()

	cleanDest := filepath.Clean(destDir)
	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("reading tar entry: %w", err)
		}

		name := filepath.Clean(header.Name)
		if strings.HasPrefix(name, "..") || filepath.IsAbs(name) {
			return fmt.Errorf("invalid path in archive: %s", header.Name)
		}

		target := filepath.Join(destDir, name)

		if !strings.HasPrefix(filepath.Clean(target), cleanDest) {
			return fmt.Errorf("path escapes destination: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return fmt.Errorf("creating directory %s: %w", target, err)
			}

		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return fmt.Errorf("creating parent directory for %s: %w", target, err)
			}

			mode := os.FileMode(header.Mode) & 0o777
			if mode == 0 {
				mode = 0o644
			}

			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
			if err != nil {
				return fmt.Errorf("creating file %s: %w", target, err)
			}

			n, err := io.Copy(f, io.LimitReader(tr, maxExtractFileSize+1))
			if closeErr := f.Close(); closeErr != nil && err == nil {
				err = closeErr
			}
			if err != nil {
				return fmt.Errorf("extracting file %s: %w", target, err)
			}

			if n > maxExtractFileSize {
				return fmt.Errorf("file %s exceeds max size (%d bytes)", header.Name, maxExtractFileSize)
			}

		default:
			// Skip symlinks and other types for security.
		}
	}

	return nil
}

// createTarGz creates a gzip-compressed tar archive of the given directory.
// Hidden files starting with ".oci-cache" (cache metadata) are excluded.
func createTarGz(sourceDir string) ([]byte, error) {
	var buf bytes.Buffer
	gzw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gzw)

	err := filepath.WalkDir(sourceDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}

		if relPath == "." {
			return nil
		}

		// Skip cache metadata files.
		if filepath.Base(relPath) == cacheFileName {
			return nil
		}

		// Skip symlinks and other non-regular, non-directory entries.
		if !d.IsDir() && !d.Type().IsRegular() {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(relPath)

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()

		_, err = io.Copy(tw, f)
		return err
	})

	if err != nil {
		return nil, err
	}

	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := gzw.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// cleanAndCreate removes an existing directory and recreates it.
func cleanAndCreate(dir string) error {
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("cleaning destination %s: %w", dir, err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating destination %s: %w", dir, err)
	}
	return nil
}
