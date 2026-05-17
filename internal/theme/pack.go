package theme

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var packZipFixedTime = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

// PackDir validates a theme source directory and produces a zip archive that can be uploaded.
func PackDir(sourceDir string) (Manifest, []byte, error) {
	sourceDir = strings.TrimSpace(sourceDir)
	if sourceDir == "" {
		return Manifest{}, nil, errors.New("theme source dir is required")
	}

	info, err := os.Stat(sourceDir)
	if err != nil {
		return Manifest{}, nil, fmt.Errorf("stat theme source dir: %w", err)
	}
	if !info.IsDir() {
		return Manifest{}, nil, fmt.Errorf("theme source path is not a directory: %s", sourceDir)
	}

	files := make(map[string][]byte, len(allowedFiles))
	for _, name := range allowedFiles {
		raw, readErr := os.ReadFile(filepath.Join(sourceDir, name))
		if readErr != nil {
			if errors.Is(readErr, os.ErrNotExist) {
				continue
			}
			return Manifest{}, nil, fmt.Errorf("read %s: %w", name, readErr)
		}
		files[name] = raw
	}

	manifestRaw, ok := files["theme.json"]
	if !ok || len(manifestRaw) == 0 {
		return Manifest{}, nil, errors.New("theme.json is required")
	}
	tokens, ok := files["tokens.css"]
	if !ok || len(tokens) == 0 {
		return Manifest{}, nil, errors.New("tokens.css is required")
	}
	recipes := files["recipes.css"]

	manifest, err := ParseManifest(manifestRaw)
	if err != nil {
		return Manifest{}, nil, err
	}
	if IsReservedID(manifest.ID) {
		return Manifest{}, nil, fmt.Errorf("%s is a reserved theme id", manifest.ID)
	}
	if _, err := buildActiveCSS(manifest.ID, tokens, recipes); err != nil {
		return Manifest{}, nil, err
	}

	archive, err := buildThemeArchive(files)
	if err != nil {
		return Manifest{}, nil, err
	}
	return manifest, archive, nil
}

func buildThemeArchive(files map[string][]byte) ([]byte, error) {
	var totalSize int64
	for _, name := range allowedFiles {
		raw, ok := files[name]
		if !ok {
			continue
		}
		size := int64(len(raw))
		if size > zipEntryLimit {
			return nil, fmt.Errorf("file too large in theme package: %s", name)
		}
		if totalSize+size > ExtractedMaxBytes {
			return nil, errors.New("theme package exceeds extracted size limit")
		}
		totalSize += size
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, name := range allowedFiles {
		raw, ok := files[name]
		if !ok {
			continue
		}

		header := &zip.FileHeader{
			Name:   name,
			Method: zip.Deflate,
		}
		header.SetMode(0o644)
		header.SetModTime(packZipFixedTime)

		w, err := zw.CreateHeader(header)
		if err != nil {
			return nil, fmt.Errorf("create zip entry %s: %w", name, err)
		}
		if _, err := w.Write(raw); err != nil {
			return nil, fmt.Errorf("write zip entry %s: %w", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("close zip writer: %w", err)
	}
	return buf.Bytes(), nil
}
