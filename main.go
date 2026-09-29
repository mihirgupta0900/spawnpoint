// Spawnpoint gives every task its own folder: a git worktree for each repo it
// touches, all on the same branch, with env files copied and dependencies
// installed.
package main

import (
	"os"
	"runtime/debug"

	"github.com/mihirgupta0900/spawnpoint/internal/cli"
)

// version is set at release time with -ldflags "-X main.version=1.2.3".
var version = ""

func main() {
	os.Exit(cli.Execute(resolveVersion()))
}

// resolveVersion prefers the release version, then the module version (for
// `go install ...@v1.2.3`), then "dev".
func resolveVersion() string {
	if version != "" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		if v := info.Main.Version; v != "" && v != "(devel)" {
			if v[0] == 'v' {
				v = v[1:]
			}
			return v
		}
	}
	return "dev"
}
