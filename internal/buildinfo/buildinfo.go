// Package buildinfo отдаёт версию сборки из debug.ReadBuildInfo: тег git или
// псевдоверсия, без build info — "dev" (D-079).
package buildinfo

import "runtime/debug"

const Unknown = "dev"

func Version() string {
	info, ok := debug.ReadBuildInfo()
	if !ok || info.Main.Version == "" || info.Main.Version == "(devel)" {
		return Unknown
	}

	return info.Main.Version
}
