package ui

import (
	"os"
	"path/filepath"
	"strings"
)

// DirLister abstracts filesystem directory operations for path completion.
type DirLister interface {
	// ListDirs returns subdirectory names in the given directory path.
	ListDirs(path string) ([]string, error)
	// HomeDir returns the home directory.
	HomeDir() (string, error)
}

// listDirs returns subdirectory names, using the lister if set, else local os.
func (c *completerModel) listDirs(path string) ([]string, error) {
	if c.lister != nil {
		return c.lister.ListDirs(path)
	}
	return localListDirs(path)
}

// localListDirs reads subdirectory names from the local filesystem.
func localListDirs(path string) ([]string, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	var dirs []string
	for _, entry := range entries {
		name := entry.Name()
		isDir := entry.IsDir()
		if !isDir && entry.Type()&os.ModeSymlink != 0 {
			target, err := filepath.EvalSymlinks(filepath.Join(path, name))
			if err == nil {
				if info, err := os.Stat(target); err == nil {
					isDir = info.IsDir()
				}
			}
		}
		if isDir {
			dirs = append(dirs, name)
		}
	}
	return dirs, nil
}

// hasSubdirs checks if a directory contains subdirectories.
func (c *completerModel) hasSubdirs(path string) bool {
	dirs, err := c.listDirs(path)
	if err != nil {
		return false
	}
	return len(dirs) > 0
}

// homeDir returns the home directory, using the lister if set, else local os.
func (c *completerModel) homeDir() (string, error) {
	if c.lister != nil {
		return c.lister.HomeDir()
	}
	return os.UserHomeDir()
}

func (c *completerModel) expandTilde(path string) string {
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := c.homeDir()
		if err != nil {
			return path
		}
		if path == "~" {
			return home
		}
		return filepath.Join(home, path[2:])
	}
	return path
}

func (c *completerModel) collapseTilde(path string) string {
	home, err := c.homeDir()
	if err != nil {
		return path
	}
	if strings.HasPrefix(path, home) {
		return "~" + path[len(home):]
	}
	return path
}
