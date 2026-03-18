package template

import (
	"os"
	"path/filepath"

	"github.com/swalha1999/lazycron/template/builtin"
)

// customDir returns the path to ~/.lazycron/templates/.
func customDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".lazycron", "templates")
}

// LoadBuiltin returns all templates embedded in the binary.
func LoadBuiltin() ([]Template, error) {
	entries, err := builtin.FS.ReadDir("templates")
	if err != nil {
		return nil, err
	}
	return loadFromEmbedDir("templates", entries)
}

func loadFromEmbedDir(base string, entries []os.DirEntry) ([]Template, error) {
	var templates []Template
	for _, entry := range entries {
		path := filepath.Join(base, entry.Name())
		if entry.IsDir() {
			sub, err := builtin.FS.ReadDir(path)
			if err != nil {
				continue
			}
			nested, err := loadFromEmbedDir(path, sub)
			if err != nil {
				continue
			}
			templates = append(templates, nested...)
			continue
		}
		if filepath.Ext(entry.Name()) != ".yaml" && filepath.Ext(entry.Name()) != ".yml" {
			continue
		}
		data, err := builtin.FS.ReadFile(path)
		if err != nil {
			continue
		}
		t, err := Parse(data)
		if err != nil {
			continue
		}
		templates = append(templates, *t)
	}
	return templates, nil
}

// LoadCustom returns templates from ~/.lazycron/templates/.
func LoadCustom() ([]Template, error) {
	dir := customDir()
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil, nil
	}

	var templates []Template
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		ext := filepath.Ext(info.Name())
		if ext != ".yaml" && ext != ".yml" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		t, err := Parse(data)
		if err != nil {
			return nil
		}
		templates = append(templates, *t)
		return nil
	})
	if err != nil {
		return templates, err
	}
	return templates, nil
}

// LoadAll loads both built-in and custom templates.
// Custom templates with the same name as a built-in template override it.
func LoadAll() ([]Template, error) {
	builtins, err := LoadBuiltin()
	if err != nil {
		return nil, err
	}

	custom, _ := LoadCustom()

	// Index built-ins by name for override detection
	byName := make(map[string]int, len(builtins))
	for i, t := range builtins {
		byName[t.Name] = i
	}

	// Custom templates override built-ins with the same name
	for _, t := range custom {
		if idx, exists := byName[t.Name]; exists {
			builtins[idx] = t
		} else {
			builtins = append(builtins, t)
		}
	}

	return builtins, nil
}
