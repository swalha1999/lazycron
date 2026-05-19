package backend

import (
	"path/filepath"
	"testing"
)

// TestEnsureInsideDir is the authoritative coverage for the path-traversal
// guard shared by every Backend's DeleteHistory (issues #35, #45, #51).
func TestEnsureInsideDir(t *testing.T) {
	base := t.TempDir() // absolute, used purely as a path prefix

	t.Run("rejects", func(t *testing.T) {
		cases := map[string]string{
			"absolute path outside base":     "/tmp/some-other-file",
			"parent traversal":               filepath.Join(base, "..", "other-dir", "file.txt"),
			"sibling with shared prefix":      base + "-other/file.txt",
			"absolute path with traversal":   base + "/../sensitive/file.txt",
			"nested traversal back into base": filepath.Join(base, "sub", "..", "..", "escape.txt"),
		}
		for name, target := range cases {
			if _, err := ensureInsideDir(base, target); err == nil {
				t.Errorf("%s: expected error for target %q, got nil", name, target)
			}
		}
	})

	t.Run("accepts", func(t *testing.T) {
		cases := map[string]string{
			"direct child": filepath.Join(base, "valid.json"),
			"nested child": filepath.Join(base, "sub", "deep", "valid.json"),
		}
		for name, target := range cases {
			safe, err := ensureInsideDir(base, target)
			if err != nil {
				t.Errorf("%s: unexpected error for target %q: %v", name, target, err)
				continue
			}
			if safe != filepath.Clean(target) {
				t.Errorf("%s: safe = %q, want %q", name, safe, filepath.Clean(target))
			}
		}
	})
}
