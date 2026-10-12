package source

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

// ScopedPath resolves path against root. Relative paths must stay inside root;
// absolute paths are rejected unless allowAbs is set, in which case they resolve
// to their cleaned self.
func ScopedPath(root string, allowAbs bool, path string) (string, error) {
	if path == "" {
		return "", errors.New("source path is empty")
	}
	root = filepath.Clean(fallbackRoot(root))
	if filepath.IsAbs(path) {
		if !allowAbs {
			return "", outsideRootError(root, path)
		}
		return filepath.Clean(path), nil
	}

	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve root: %w", err)
	}
	resolved := filepath.Join(rootAbs, path) // Join cleans; escapes are checked below
	rel, err := filepath.Rel(rootAbs, resolved)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", outsideRootError(rootAbs, path)
	}
	return resolved, nil
}

// fallbackRoot substitutes the working directory when no root is configured.
func fallbackRoot(root string) string {
	if root == "" {
		return "."
	}
	return root
}

func outsideRootError(root, path string) error {
	return fmt.Errorf("path %q resolves outside CHART_MCP_ROOT (%s); place the file under the root or set ALLOW_ABS_PATHS=true",
		path, root)
}
