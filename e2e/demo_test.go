package e2e

import (
	"os"
	"path/filepath"
	"testing"
)

// TestDemoFixture builds a persistent demo environment for screenshots and
// recordings when SP_DEMO_DIR is set:
//
//	SP_DEMO_DIR=/tmp/sp-demo go test ./e2e -run TestDemoFixture
//	SPAWNPOINT_DIR=/tmp/sp-demo/config spawnpoint create
func TestDemoFixture(t *testing.T) {
	dir := os.Getenv("SP_DEMO_DIR")
	if dir == "" {
		t.Skip("set SP_DEMO_DIR to build the demo fixture")
	}
	os.RemoveAll(dir)
	code := filepath.Join(dir, "code")
	os.MkdirAll(code, 0o755)
	for _, name := range []string{"api", "web", "worker", "billing-docs", "infra", "mobile", "design-system"} {
		remote := filepath.Join(dir, "remotes", name+".git")
		os.MkdirAll(remote, 0o755)
		git(t, remote, "init", "-q", "--bare", "-b", "main")
		git(t, code, "clone", "-q", remote, name)
		work := filepath.Join(code, name)
		os.WriteFile(filepath.Join(work, "README.md"), []byte("# "+name+"\n"), 0o644)
		os.WriteFile(filepath.Join(work, ".gitignore"), []byte(".env*\nCLAUDE.md\nnode_modules/\n"), 0o644)
		if name == "web" {
			os.WriteFile(filepath.Join(work, "package.json"), []byte(`{"name":"web","version":"1.0.0"}`), 0o644)
		}
		git(t, work, "add", ".")
		git(t, work, "commit", "-qm", "init")
		git(t, work, "push", "-q", "origin", "main")
		git(t, work, "remote", "set-head", "origin", "--auto")
		os.WriteFile(filepath.Join(work, ".env"), []byte("SECRET="+name+"\n"), 0o644)
		os.WriteFile(filepath.Join(work, "CLAUDE.md"), []byte("notes\n"), 0o644)
	}
	cfgDir := filepath.Join(dir, "config")
	os.MkdirAll(cfgDir, 0o755)
	cfg := "scan_dirs = ['" + code + "']\n" +
		"worktree_dir = '" + filepath.Join(dir, "workspaces") + "'\n" +
		"auto_install_deps = true\n" +
		"check_updates = false\n"
	os.WriteFile(filepath.Join(cfgDir, "config.toml"), []byte(cfg), 0o644)
}
