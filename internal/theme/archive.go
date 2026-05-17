package theme

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const (
	ArchiveMaxBytes   int64 = 20 << 20
	ExtractedMaxBytes int64 = 50 << 20

	zipEntryLimit int64 = 20 << 20
)

var allowedFiles = []string{
	"theme.json",
	"tokens.css",
	"recipes.css",
	"preview.png",
	"README.md",
}

type Installed struct {
	Manifest   Manifest
	HasPreview bool
}

var (
	ErrInvalidPackage = errors.New("invalid theme package")
	ErrThemeStorage   = errors.New("theme storage unavailable")
)

type InstallError struct {
	kind error
	err  error
}

func (e *InstallError) Error() string {
	if e == nil || e.err == nil {
		return ""
	}
	return e.err.Error()
}

func (e *InstallError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

func (e *InstallError) Is(target error) bool {
	return e != nil && target == e.kind
}

func invalidPackage(err error) error {
	if err == nil {
		return nil
	}
	return &InstallError{kind: ErrInvalidPackage, err: err}
}

func themeStorageError(err error) error {
	if err == nil {
		return nil
	}
	return &InstallError{kind: ErrThemeStorage, err: err}
}

func (s *Store) InstallZip(archive []byte) (Installed, error) {
	root, err := s.rootDir()
	if err != nil {
		return Installed{}, themeStorageError(err)
	}
	files, err := readZip(archive)
	if err != nil {
		return Installed{}, invalidPackage(err)
	}

	manifestRaw, ok := files["theme.json"]
	if !ok {
		return Installed{}, invalidPackage(errors.New("theme.json is required"))
	}
	if tokens, ok := files["tokens.css"]; !ok || len(tokens) == 0 {
		return Installed{}, invalidPackage(errors.New("tokens.css is required"))
	}
	if _, ok := files["recipes.css"]; !ok {
		files["recipes.css"] = []byte{}
	}

	manifest, err := ParseManifest(manifestRaw)
	if err != nil {
		return Installed{}, invalidPackage(err)
	}
	if IsReservedID(manifest.ID) {
		return Installed{}, invalidPackage(fmt.Errorf("%s is a reserved theme id", manifest.ID))
	}
	if _, err := buildActiveCSS(manifest.ID, files["tokens.css"], files["recipes.css"]); err != nil {
		return Installed{}, invalidPackage(err)
	}

	dir, err := s.themeDir(manifest.ID)
	if err != nil {
		return Installed{}, themeStorageError(err)
	}
	tmpDir, err := os.MkdirTemp(root, manifest.ID+".tmp-*")
	if err != nil {
		return Installed{}, themeStorageError(fmt.Errorf("create theme temp dir: %w", err))
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.RemoveAll(tmpDir)
		}
	}()

	for _, name := range allowedFiles {
		raw, ok := files[name]
		if !ok {
			continue
		}
		if err := os.WriteFile(filepath.Join(tmpDir, name), raw, 0o644); err != nil {
			return Installed{}, themeStorageError(fmt.Errorf("write %s: %w", name, err))
		}
	}

	if err := replaceThemeDir(root, manifest.ID, tmpDir, dir); err != nil {
		return Installed{}, themeStorageError(fmt.Errorf("install theme: %w", err))
	}
	cleanup = false

	return Installed{
		Manifest:   manifest,
		HasPreview: len(files["preview.png"]) > 0,
	}, nil
}

func replaceThemeDir(root, id, nextDir, targetDir string) error {
	info, err := os.Stat(targetDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return os.Rename(nextDir, targetDir)
		}
		return fmt.Errorf("stat current theme: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("current theme path is not a directory: %s", targetDir)
	}

	backupDir, err := reserveBackupDir(root, id)
	if err != nil {
		return err
	}
	if err := os.Rename(targetDir, backupDir); err != nil {
		return fmt.Errorf("backup current theme: %w", err)
	}

	if err := os.Rename(nextDir, targetDir); err != nil {
		_ = os.RemoveAll(targetDir)
		if rollbackErr := os.Rename(backupDir, targetDir); rollbackErr != nil {
			return fmt.Errorf("%w (rollback failed: %v)", err, rollbackErr)
		}
		return err
	}

	_ = os.RemoveAll(backupDir)
	return nil
}

func reserveBackupDir(root, id string) (string, error) {
	backupDir, err := os.MkdirTemp(root, id+".bak-*")
	if err != nil {
		return "", fmt.Errorf("create theme backup dir: %w", err)
	}
	if err := os.Remove(backupDir); err != nil {
		return "", fmt.Errorf("reserve theme backup dir: %w", err)
	}
	return backupDir, nil
}

func readZip(archive []byte) (map[string][]byte, error) {
	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return nil, fmt.Errorf("read theme archive: %w", err)
	}

	files := make(map[string][]byte, len(reader.File))
	var totalRead int64
	for _, file := range reader.File {
		if file.FileInfo().IsDir() {
			continue
		}

		path, err := normalizeArchivePath(file.Name)
		if err != nil {
			return nil, fmt.Errorf("invalid file path in theme package %q: %w", file.Name, err)
		}
		if path.Skip {
			continue
		}
		name := path.Name
		if !slices.Contains(allowedFiles, name) {
			return nil, fmt.Errorf("unsupported file in theme package: %s", name)
		}
		if _, exists := files[name]; exists {
			return nil, fmt.Errorf("duplicate file in theme package: %s", name)
		}
		if file.UncompressedSize64 > uint64(zipEntryLimit) {
			return nil, fmt.Errorf("file too large in theme package: %s", file.Name)
		}
		if file.UncompressedSize64 > 0 && totalRead+int64(file.UncompressedSize64) > ExtractedMaxBytes {
			return nil, errors.New("theme package exceeds extracted size limit")
		}

		rc, err := file.Open()
		if err != nil {
			return nil, fmt.Errorf("open archived file %s: %w", file.Name, err)
		}
		readLimit := zipEntryLimit
		left := ExtractedMaxBytes - totalRead
		if left < readLimit {
			readLimit = left
		}
		if readLimit <= 0 {
			_ = rc.Close()
			return nil, errors.New("theme package exceeds extracted size limit")
		}
		raw, readErr := io.ReadAll(io.LimitReader(rc, readLimit+1))
		closeErr := rc.Close()
		if readErr != nil {
			return nil, fmt.Errorf("read archived file %s: %w", file.Name, readErr)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("close archived file %s: %w", file.Name, closeErr)
		}
		if int64(len(raw)) > readLimit {
			if readLimit < zipEntryLimit {
				return nil, errors.New("theme package exceeds extracted size limit")
			}
			return nil, fmt.Errorf("file too large in theme package: %s", file.Name)
		}

		totalRead += int64(len(raw))
		files[name] = raw
	}
	return files, nil
}

type archivePath struct {
	Name string
	Skip bool
}

func normalizeArchivePath(name string) (archivePath, error) {
	name = strings.ReplaceAll(name, "\\", "/")
	name = strings.TrimSpace(name)
	if name == "" {
		return archivePath{Skip: true}, nil
	}
	if strings.HasPrefix(name, "__MACOSX/") {
		return archivePath{Skip: true}, nil
	}
	if strings.HasPrefix(name, "/") {
		return archivePath{}, errors.New("absolute paths are not allowed")
	}
	if strings.Contains(name, "/") {
		return archivePath{}, errors.New("path must be a plain file name")
	}
	if name == "." || name == ".." {
		return archivePath{}, errors.New("path traversal is not allowed")
	}
	if strings.Contains(name, "..") {
		return archivePath{}, errors.New("path traversal is not allowed")
	}
	return archivePath{Name: name}, nil
}
