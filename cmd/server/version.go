package main

import "runtime/debug"

const versionUnknown = "dev"

func buildVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok || info.Main.Version == "" || info.Main.Version == "(devel)" {
		return versionUnknown
	}

	return info.Main.Version
}
