package theme

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Store struct {
	root string
}

type CustomTheme struct {
	Manifest   Manifest
	HasPreview bool
	UpdatedAt  *time.Time
}

type CustomThemeWarning struct {
	ID  string
	Err error
}

func NewStore(root string) (*Store, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, errors.New("theme root is required")
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("create theme root: %w", err)
	}
	return &Store{root: root}, nil
}

func (s *Store) Root() string {
	if s == nil {
		return ""
	}
	return s.root
}

func (s *Store) rootDir() (string, error) {
	if s == nil || strings.TrimSpace(s.root) == "" {
		return "", ErrThemeStorage
	}
	return s.root, nil
}

func (s *Store) themeDir(id string) (string, error) {
	root, err := s.rootDir()
	if err != nil {
		return "", err
	}
	id, err = NormalizeID(id)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, id), nil
}

func (s *Store) LoadCustomManifest(id string) (Manifest, error) {
	dir, err := s.themeDir(id)
	if err != nil {
		return Manifest{}, err
	}
	raw, err := os.ReadFile(filepath.Join(dir, "theme.json"))
	if err != nil {
		return Manifest{}, fmt.Errorf("read theme.json: %w", err)
	}
	return ParseManifest(raw)
}

func (s *Store) LoadCustomCSS(id string) ([]byte, error) {
	dir, err := s.themeDir(id)
	if err != nil {
		return nil, err
	}
	tokens, err := os.ReadFile(filepath.Join(dir, "tokens.css"))
	if err != nil {
		return nil, fmt.Errorf("read tokens.css: %w", err)
	}
	recipes, err := os.ReadFile(filepath.Join(dir, "recipes.css"))
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("read recipes.css: %w", err)
		}
		recipes = []byte{}
	}
	return buildActiveCSS(id, tokens, recipes)
}

func (s *Store) LoadCustomPreview(id string) ([]byte, error) {
	dir, err := s.themeDir(id)
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(filepath.Join(dir, "preview.png"))
	if err != nil {
		return nil, fmt.Errorf("read preview.png: %w", err)
	}
	return raw, nil
}

func (s *Store) RemoveCustom(id string) error {
	dir, err := s.themeDir(id)
	if err != nil {
		return err
	}
	ok, err := dirExists(dir)
	if err != nil {
		return err
	}
	if !ok {
		return os.ErrNotExist
	}
	return os.RemoveAll(dir)
}

func (s *Store) CustomExists(id string) (bool, error) {
	dir, err := s.themeDir(id)
	if err != nil {
		return false, err
	}
	return dirExists(dir)
}

func dirExists(dir string) (bool, error) {
	info, err := os.Stat(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	return info.IsDir(), nil
}

func (s *Store) ListCustom() ([]CustomTheme, error) {
	items, _, err := s.ListCustomWithWarnings()
	return items, err
}

func (s *Store) ListCustomWithWarnings() ([]CustomTheme, []CustomThemeWarning, error) {
	root, err := s.rootDir()
	if err != nil {
		return nil, nil, err
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, nil, fmt.Errorf("read custom theme dir: %w", err)
	}

	items := make([]CustomTheme, 0, len(entries))
	warnings := make([]CustomThemeWarning, 0)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		id := strings.TrimSpace(entry.Name())
		if err := ValidateID(id); err != nil {
			warnings = append(warnings, CustomThemeWarning{
				ID:  entry.Name(),
				Err: fmt.Errorf("invalid theme id: %w", err),
			})
			continue
		}

		manifest, err := s.LoadCustomManifest(id)
		if err != nil {
			warnings = append(warnings, CustomThemeWarning{
				ID:  id,
				Err: fmt.Errorf("load theme manifest: %w", err),
			})
			continue
		}
		if manifest.ID != id {
			warnings = append(warnings, CustomThemeWarning{
				ID:  id,
				Err: fmt.Errorf("manifest id %q does not match directory id %q", manifest.ID, id),
			})
			continue
		}
		if _, err := s.LoadCustomCSS(id); err != nil {
			warnings = append(warnings, CustomThemeWarning{
				ID:  id,
				Err: fmt.Errorf("load theme css: %w", err),
			})
			continue
		}

		items = append(items, CustomTheme{
			Manifest:   manifest,
			HasPreview: s.hasPreview(id),
			UpdatedAt:  dirUpdatedAt(entry),
		})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Manifest.Name == items[j].Manifest.Name {
			return items[i].Manifest.ID < items[j].Manifest.ID
		}
		return items[i].Manifest.Name < items[j].Manifest.Name
	})
	return items, warnings, nil
}

func (s *Store) hasPreview(id string) bool {
	dir, err := s.themeDir(id)
	if err != nil {
		return false
	}
	_, err = os.Stat(filepath.Join(dir, "preview.png"))
	return err == nil
}

func dirUpdatedAt(entry os.DirEntry) *time.Time {
	info, err := entry.Info()
	if err != nil {
		return nil
	}
	updatedAt := info.ModTime()
	return &updatedAt
}
