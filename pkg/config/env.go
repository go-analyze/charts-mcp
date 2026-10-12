package config

import (
	"os"
	"strconv"
	"strings"
)

// Environment variables documented in the tool schemas and specs-v0.md.
const (
	// EnvChartRoot bounds file access; paths outside it are rejected unless
	// EnvAllowAbsPaths is set. Defaults to the working directory.
	EnvChartRoot = "CHART_MCP_ROOT"
	// EnvAllowAbsPaths relaxes the EnvChartRoot restriction when truthy.
	EnvAllowAbsPaths = "ALLOW_ABS_PATHS"
)

var (
	chartRoot    string
	allowAbsPath bool
)

// LoadEnv reads configuration from the environment once at startup. Tests may
// call it again after changing the environment.
func LoadEnv() {
	var root string
	if value := strings.TrimSpace(os.Getenv(EnvChartRoot)); value != "" {
		root = value
	} else if wd, err := os.Getwd(); err == nil {
		root = wd
	}
	allowAbs, _ := strconv.ParseBool(strings.TrimSpace(os.Getenv(EnvAllowAbsPaths)))
	chartRoot, allowAbsPath = root, allowAbs
}

// ChartRoot returns the configured root directory for file access.
func ChartRoot() string {
	return chartRoot
}

// AllowAbsPaths reports whether paths outside ChartRoot are accepted.
func AllowAbsPaths() bool {
	return allowAbsPath
}
