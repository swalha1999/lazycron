package backend

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ensureInsideDir resolves target and verifies it stays within baseDir.
// On success it returns the cleaned, absolute path of target; otherwise it
// returns an error describing why target would escape baseDir.
//
// This is the single security boundary shared by every Backend's
// DeleteHistory implementation. Arbitrary-file-deletion / path-traversal
// bugs in that sink were fixed under issues #35, #45 and #51 — keeping the
// guard in one place ensures a future hardening fix cannot land in one
// backend while silently missing the others.
func ensureInsideDir(baseDir, target string) (string, error) {
	absBase, err := filepath.Abs(filepath.Clean(baseDir))
	if err != nil {
		return "", fmt.Errorf("failed to resolve history dir: %w", err)
	}

	absTarget, err := filepath.Abs(filepath.Clean(target))
	if err != nil {
		return "", fmt.Errorf("failed to resolve file path: %w", err)
	}

	rel, err := filepath.Rel(absBase, absTarget)
	if err != nil {
		return "", fmt.Errorf("refusing to delete file outside history dir")
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("refusing to delete file outside history dir")
	}

	return absTarget, nil
}
