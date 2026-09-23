package config

import "runtime/debug"

// Version is injected at build time via ldflags or from go install.
var Version = "dev"

func init() {
	if Version != "dev" {
		return
	} else if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		Version = info.Main.Version
	}
}
