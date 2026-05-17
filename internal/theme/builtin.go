package theme

import (
	"embed"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
)

const builtinRoot = "builtin"

//go:embed builtin/*/*
var builtinFS embed.FS

func BuiltinManifest(id string) (Manifest, error) {
	raw, err := builtinFS.ReadFile(filepath.ToSlash(filepath.Join(builtinRoot, id, "theme.json")))
	if err != nil {
		return Manifest{}, fmt.Errorf("read builtin manifest: %w", err)
	}
	return ParseManifest(raw)
}

func BuiltinCSS(id string) ([]byte, error) {
	tokens, err := builtinFS.ReadFile(filepath.ToSlash(filepath.Join(builtinRoot, id, "tokens.css")))
	if err != nil {
		return nil, fmt.Errorf("read builtin tokens: %w", err)
	}
	recipes, err := builtinFS.ReadFile(filepath.ToSlash(filepath.Join(builtinRoot, id, "recipes.css")))
	if err != nil {
		return nil, fmt.Errorf("read builtin recipes: %w", err)
	}
	return buildActiveCSS(id, tokens, recipes)
}

func BuiltinPreview(id string) ([]byte, error) {
	raw, err := builtinFS.ReadFile(filepath.ToSlash(filepath.Join(builtinRoot, id, "preview.png")))
	if err != nil {
		return nil, fmt.Errorf("read builtin preview: %w", err)
	}
	return raw, nil
}

func BuiltinManifestIDs() ([]string, error) {
	entries, err := fs.ReadDir(builtinFS, builtinRoot)
	if err != nil {
		return nil, fmt.Errorf("read builtin theme dir: %w", err)
	}

	ids := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		ids = append(ids, entry.Name())
	}
	sort.Strings(ids)
	return ids, nil
}

func ListBuiltinManifests() ([]Manifest, error) {
	ids, err := BuiltinManifestIDs()
	if err != nil {
		return nil, err
	}

	out := make([]Manifest, 0, len(ids))
	for _, id := range ids {
		item, itemErr := BuiltinManifest(id)
		if itemErr != nil {
			return nil, itemErr
		}
		out = append(out, item)
	}
	return out, nil
}

func IsBuiltin(id string) bool {
	if id == "" {
		return false
	}
	_, err := BuiltinManifest(id)
	return err == nil
}
