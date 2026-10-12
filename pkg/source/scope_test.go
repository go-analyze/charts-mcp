package source

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScopedPathInsideRoot(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	resolved, err := ScopedPath(root, false, "data/sales.csv")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(root, "data", "sales.csv"), resolved)
}

func TestScopedPathEscapeRejected(t *testing.T) {
	t.Parallel()

	_, err := ScopedPath(t.TempDir(), false, "../outside.csv")
	assert.ErrorContains(t, err, "CHART_MCP_ROOT")
}

func TestScopedPathAbsolute(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	absolute := filepath.Join(root, "out.json")

	_, err := ScopedPath(root, false, absolute)
	assert.ErrorContains(t, err, "ALLOW_ABS_PATHS=true")

	resolved, err := ScopedPath(root, true, absolute)
	require.NoError(t, err)
	assert.Equal(t, absolute, resolved)
}

func TestScopedPathEmptyRejected(t *testing.T) {
	t.Parallel()

	_, err := ScopedPath(t.TempDir(), false, "")
	assert.ErrorContains(t, err, "empty")
}
