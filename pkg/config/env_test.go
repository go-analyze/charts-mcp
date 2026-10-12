package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// LoadEnv mutates package globals, so this test stays sequential.
func TestLoadEnv(t *testing.T) {
	t.Setenv(EnvChartRoot, t.TempDir())
	t.Setenv(EnvAllowAbsPaths, "true")
	LoadEnv()

	require.NotEmpty(t, ChartRoot())
	assert.True(t, AllowAbsPaths())

	t.Setenv(EnvChartRoot, "")
	t.Setenv(EnvAllowAbsPaths, "1")
	LoadEnv()

	wd, err := os.Getwd()
	require.NoError(t, err)
	assert.Equal(t, wd, ChartRoot())
	assert.True(t, AllowAbsPaths())

	t.Setenv(EnvAllowAbsPaths, "")
	LoadEnv()
	assert.False(t, AllowAbsPaths())
}
