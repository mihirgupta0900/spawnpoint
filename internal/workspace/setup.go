package workspace

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mihirgupta0900/spawnpoint/internal/config"
	"github.com/mihirgupta0900/spawnpoint/internal/ui"
)

// CopyEssentialFiles copies the configured env/agent files and dirs from the
// source repo into a new worktree. Existing files in the worktree win.
func CopyEssentialFiles(src, dst string, cfg *config.Config) []string {
	var copied []string
	for _, pattern := range cfg.CopyGlobs {
		matches, _ := filepath.Glob(filepath.Join(src, pattern))
		for _, m := range matches {
			if !isFile(m) {
				continue
			}
			target := filepath.Join(dst, filepath.Base(m))
			if exists(target) {
				continue
			}
			if err := copyFile(m, target); err != nil {
				ui.Hint("%s", ui.Bad(fmt.Sprintf("failed to copy %s: %v", filepath.Base(m), err)))
				continue
			}
			copied = append(copied, filepath.Base(m))
		}
	}
	for _, name := range cfg.CopyFiles {
		from, to := filepath.Join(src, name), filepath.Join(dst, name)
		if !isFile(from) || exists(to) {
			continue
		}
		if err := copyFile(from, to); err != nil {
			ui.Hint("%s", ui.Bad(fmt.Sprintf("failed to copy %s: %v", name, err)))
			continue
		}
		copied = append(copied, name)
	}
	for _, name := range cfg.CopyDirs {
		from := filepath.Join(src, name)
		if !isDir(from) {
			continue
		}
		if err := copyTree(from, filepath.Join(dst, name)); err != nil {
			ui.Hint("%s", ui.Bad(fmt.Sprintf("failed to copy %s/: %v", name, err)))
			continue
		}
		copied = append(copied, name+"/")
	}
	return copied
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Chtimes(dst, info.ModTime(), info.ModTime())
}

func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if !isFile(p) { // skip sockets, broken links, etc.
			return nil
		}
		return copyFile(p, target)
	})
}

type installStep struct {
	label string
	cmds  [][]string
}

// detectInstalls picks the install commands for a worktree, one per
// ecosystem present.
func detectInstalls(dir string) []installStep {
	has := func(name string) bool { return exists(filepath.Join(dir, name)) }
	_, uvErr := exec.LookPath("uv")
	hasUV := uvErr == nil

	var steps []installStep
	switch {
	case has("pnpm-lock.yaml"):
		steps = append(steps, installStep{"pnpm install", [][]string{{"pnpm", "install"}}})
	case has("bun.lockb") || has("bun.lock"):
		steps = append(steps, installStep{"bun install", [][]string{{"bun", "install"}}})
	case has("yarn.lock"):
		steps = append(steps, installStep{"yarn install", [][]string{{"yarn", "install"}}})
	case has("package.json"):
		steps = append(steps, installStep{"npm install", [][]string{{"npm", "install"}}})
	}
	if has("go.mod") {
		steps = append(steps, installStep{"go mod download", [][]string{{"go", "mod", "download"}}})
	}
	if has("Gemfile") {
		steps = append(steps, installStep{"bundle install", [][]string{{"bundle", "install"}}})
	}
	switch {
	case has("uv.lock") && hasUV:
		steps = append(steps, installStep{"uv sync", [][]string{{"uv", "sync"}}})
	case has("poetry.lock"):
		steps = append(steps, installStep{"poetry install", [][]string{{"poetry", "install"}}})
	case has("requirements.txt") && hasUV:
		steps = append(steps, installStep{"uv pip install -r requirements.txt", [][]string{
			{"uv", "venv"}, {"uv", "pip", "install", "-r", "requirements.txt"},
		}})
	case has("requirements.txt"):
		steps = append(steps, installStep{"pip install -r requirements.txt", [][]string{
			{"python3", "-m", "venv", ".venv"}, {filepath.Join(".venv", "bin", "pip"), "install", "-r", "requirements.txt"},
		}})
	}
	return steps
}

// cleanEnv drops variables that would leak the caller's Python/Node
// environment into the new worktree's installs.
func cleanEnv() []string {
	var env []string
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "VIRTUAL_ENV=") {
			continue
		}
		env = append(env, kv)
	}
	return env
}

// SetupDependencies installs dependencies for every ecosystem detected in
// dir. Install output is shown only if a step fails (or with --debug).
func SetupDependencies(dir string) []string {
	var done []string
	for _, step := range detectInstalls(dir) {
		var out bytes.Buffer
		var failed error
		ui.Spin("installing deps · "+step.label, func() {
			for _, c := range step.cmds {
				cmd := exec.Command(c[0], c[1:]...)
				if strings.Contains(c[0], string(filepath.Separator)) {
					cmd.Path = filepath.Join(dir, c[0])
				}
				cmd.Dir, cmd.Env = dir, cleanEnv()
				if slog.Default().Enabled(context.Background(), slog.LevelDebug) {
					cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
				} else {
					cmd.Stdout, cmd.Stderr = &out, &out
				}
				if err := cmd.Run(); err != nil {
					failed = fmt.Errorf("%s: %w", strings.Join(c, " "), err)
					return
				}
			}
		})
		if failed != nil {
			ui.Hint("%s", ui.Bad("✗ "+step.label+" failed: "+failed.Error()))
			for _, line := range tail(out.String(), 15) {
				ui.Hint("    %s", line)
			}
			continue
		}
		done = append(done, step.label)
	}
	return done
}

func tail(s string, n int) []string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return lines
}
