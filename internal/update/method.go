package update

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"
)

// Channel is baked in at build time (-X .../update.Channel=pypi) for builds
// published somewhere other than GitHub releases.
var Channel = ""

// Method is how this binary was installed, and so how to upgrade it.
type Method struct {
	Name string   // homebrew, pipx, uv, pip, go, binary
	Cmd  []string // upgrade command; nil means replace the binary in place
	Exe  string   // resolved path of the running binary
}

// Display is a human-readable upgrade command.
func (m Method) Display() string {
	if m.Cmd == nil {
		return "download the latest release into " + m.Exe
	}
	return strings.Join(m.Cmd, " ")
}

// Detect works out how the running binary was installed.
func Detect() Method {
	exe, err := os.Executable()
	if err == nil {
		if r, err := filepath.EvalSymlinks(exe); err == nil {
			exe = r
		}
	}
	return detect(exe, Channel, goInstalled(exe))
}

func detect(exe, channel string, fromGo bool) Method {
	p := filepath.ToSlash(exe)
	bin := filepath.Dir(exe)
	env := filepath.Dir(bin) // venv / tool dir for Python-installed builds
	switch {
	case strings.Contains(p, "/Cellar/") || strings.Contains(p, "/Caskroom/") || strings.Contains(p, "/linuxbrew/"):
		return Method{"homebrew", []string{"brew", "upgrade", "spawnpoint"}, exe}
	case fileExists(filepath.Join(env, "pipx_metadata.json")) || strings.Contains(p, "/pipx/venvs/"):
		return Method{"pipx", []string{"pipx", "upgrade", "spawnpoint"}, exe}
	case fileExists(filepath.Join(env, "uv-receipt.toml")) || strings.Contains(p, "/uv/tools/"):
		return Method{"uv", []string{"uv", "tool", "upgrade", "spawnpoint"}, exe}
	case channel == "pypi":
		// pip into a venv or the user/system site: upgrade with the Python
		// installed next to it.
		for _, py := range []string{"python3", "python"} {
			if cand := filepath.Join(bin, py); fileExists(cand) {
				return Method{"pip", []string{cand, "-m", "pip", "install", "--upgrade", "spawnpoint"}, exe}
			}
		}
		return Method{"pip", []string{"python3", "-m", "pip", "install", "--upgrade", "spawnpoint"}, exe}
	case fromGo:
		return Method{"go", []string{"go", "install", "github.com/" + Repo + "@latest"}, exe}
	default:
		return Method{"binary", nil, exe}
	}
}

// goInstalled reports a `go install`ed build: it carries a module version and
// lives in GOBIN or GOPATH/bin.
func goInstalled(exe string) bool {
	info, ok := debug.ReadBuildInfo()
	if !ok || info.Main.Version == "" || info.Main.Version == "(devel)" {
		return false
	}
	dir := filepath.Dir(exe)
	for _, env := range []string{"GOBIN", "GOPATH"} {
		out, err := exec.Command("go", "env", env).Output()
		if err != nil {
			continue
		}
		v := strings.TrimSpace(string(out))
		if env == "GOPATH" && v != "" {
			v = filepath.Join(filepath.SplitList(v)[0], "bin")
		}
		if v != "" && v == dir {
			return true
		}
	}
	return false
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
