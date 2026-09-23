// Package appdata resolves the vectile data directory — one folder holding
// the database, the embedding model, downloaded plugin binaries, and
// config.json. Everything the app persists lives under
// os.UserConfigDir()/vectile.
package appdata

import (
	"os"
	"path/filepath"
)

// ModelName is the embedding model file vectile expects in the models dir.
const ModelName = "bge-m3-Q4_K_M.gguf"

// Dir is the resolved app-data directory, set by Init.
var Dir string

// Init creates the app-data directory tree (db/, models/, plugins/) if missing.
func Init() (string, error) {
	cfgDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	Dir = filepath.Join(cfgDir, "vectile")
	for _, sub := range []string{"db", "models", "plugins"} {
		if err := os.MkdirAll(filepath.Join(Dir, sub), 0o755); err != nil {
			return "", err
		}
	}
	return Dir, nil
}

// ConfigPath returns the JSON config file path.
func ConfigPath() string { return filepath.Join(Dir, "config.json") }

// DBPath returns the SQLite database path.
func DBPath() string { return filepath.Join(Dir, "db", "vectile.db") }

// ModelsDir returns the folder vectile keeps embedding models in. Imported
// .gguf files are copied here, and the folder is scanned to keep the models
// list in sync with what's on disk.
func ModelsDir() string { return filepath.Join(Dir, "models") }

// PluginsDir returns the folder the app installs downloaded binaries into.
// Each plugin owns a subtree, e.g. plugins/tesseract/<version>/.
func PluginsDir() string { return filepath.Join(Dir, "plugins") }

// ModelPath returns the embedding model path. VECTILE_EMBED_MODEL overrides
// the default (models/bge-m3-Q4_K_M.gguf). The model is placed there manually;
// the app never downloads or copies it.
func ModelPath() string {
	if p := os.Getenv("VECTILE_EMBED_MODEL"); p != "" {
		return p
	}
	return filepath.Join(Dir, "models", ModelName)
}
