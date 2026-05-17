package theme

import (
	"context"
	"errors"
	"fmt"
	"os"
)

type ActiveStore interface {
	GetActiveThemeID(context.Context) (string, error)
	SetActiveThemeID(context.Context, string) error
}

type Active struct {
	ConfiguredID string
	ID           string
	MissingID    string
	BrokenID     string
	BrokenErr    error
	Manifest     Manifest
	CSS          []byte
}

func DefaultActive() Active {
	return Active{
		ConfiguredID: DefaultID,
		ID:           DefaultID,
		CSS:          []byte{},
	}
}

func ReadActiveID(ctx context.Context, st ActiveStore) (string, error) {
	if st == nil {
		return "", errors.New("theme store is unavailable")
	}

	rawID, err := st.GetActiveThemeID(ctx)
	if err != nil {
		return "", err
	}
	id, err := NormalizeActiveID(rawID)
	if err != nil {
		return DefaultID, nil
	}
	return id, nil
}

func SaveActiveID(ctx context.Context, st ActiveStore, id string) error {
	if st == nil {
		return errors.New("theme store is unavailable")
	}

	id, err := NormalizeActiveID(id)
	if err != nil {
		return err
	}
	if err := st.SetActiveThemeID(ctx, id); err != nil {
		return err
	}
	SetRuntimeActiveID(id)
	return nil
}

func ResolveActive(ctx context.Context, st ActiveStore, themes *Store) (Active, error) {
	id, err := ReadActiveID(ctx, st)
	if err != nil {
		return DefaultActive(), err
	}
	return themes.LoadActive(id)
}

func (s *Store) LoadActive(id string) (Active, error) {
	id, err := NormalizeActiveID(id)
	if err != nil {
		return DefaultActive(), err
	}
	if IsDefault(id) {
		return DefaultActive(), nil
	}

	active := Active{
		ConfiguredID: id,
		ID:           id,
	}
	if IsBuiltin(id) {
		manifest, err := BuiltinManifest(id)
		if err != nil {
			return DefaultActive(), fmt.Errorf("load builtin theme manifest: %w", err)
		}
		css, err := BuiltinCSS(id)
		if err != nil {
			return DefaultActive(), fmt.Errorf("load builtin theme css: %w", err)
		}
		active.Manifest = manifest
		active.CSS = css
		return active, nil
	}

	if s == nil {
		return DefaultActive(), ErrThemeStorage
	}
	ok, err := s.CustomExists(id)
	if err != nil {
		return DefaultActive(), err
	}
	if !ok {
		return missingActive(id), nil
	}

	manifest, err := s.LoadCustomManifest(id)
	if err != nil {
		return brokenActive(id, fmt.Errorf("load custom theme manifest: %w", err)), nil
	}
	if manifest.ID != id {
		return brokenActive(id, fmt.Errorf("custom theme manifest id %q does not match directory id %q", manifest.ID, id)), nil
	}

	css, err := s.LoadCustomCSS(id)
	if err != nil {
		return brokenActive(id, fmt.Errorf("load custom theme css: %w", err)), nil
	}
	active.Manifest = manifest
	active.CSS = css
	return active, nil
}

func (s *Store) ThemeExists(id string) (bool, error) {
	id, err := NormalizeID(id)
	if err != nil {
		return false, err
	}
	if IsDefault(id) || IsBuiltin(id) {
		return true, nil
	}
	if s == nil {
		return false, ErrThemeStorage
	}
	active, err := s.LoadActive(id)
	if err != nil {
		return false, err
	}
	return active.ID == id, nil
}

func (s *Store) LoadPreview(id string) ([]byte, error) {
	id, err := NormalizeID(id)
	if err != nil {
		return nil, err
	}
	if IsDefault(id) {
		return nil, os.ErrNotExist
	}
	if IsBuiltin(id) {
		return BuiltinPreview(id)
	}
	if s == nil {
		return nil, ErrThemeStorage
	}
	return s.LoadCustomPreview(id)
}

func missingActive(id string) Active {
	active := DefaultActive()
	active.ConfiguredID = id
	active.MissingID = id
	return active
}

func brokenActive(id string, err error) Active {
	active := DefaultActive()
	active.ConfiguredID = id
	active.BrokenID = id
	active.BrokenErr = err
	return active
}
