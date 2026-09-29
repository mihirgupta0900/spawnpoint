// Package config loads and saves ~/.spawnpoint/config.toml.
//
// The file format is shared with the original Python release, so a config
// written by either version reads the same in the other.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

var (
	DefaultCopyGlobs = []string{".env*"}
	DefaultCopyFiles = []string{"AGENT.md", "CLAUDE.md", "GEMINI.md"}
	DefaultCopyDirs  = []string{".vscode", "docs"}

	commonCodeDirs = []string{"~/code", "~/projects", "~/repos", "~/src", "~/dev"}
)

type Config struct {
	ScanDirs               []string
	WorktreeDir            string
	ScanDepth              int
	CopyGlobs              []string
	CopyFiles              []string
	CopyDirs               []string
	AdditionalWorktreeDirs []string
	AutoInstallDeps        bool
	CheckUpdates           bool
}

func Default() *Config {
	return &Config{
		WorktreeDir:     filepath.Join(Dir(), "workspaces"),
		ScanDepth:       2,
		CopyGlobs:       clone(DefaultCopyGlobs),
		CopyFiles:       clone(DefaultCopyFiles),
		CopyDirs:        clone(DefaultCopyDirs),
		AutoInstallDeps: true,
		CheckUpdates:    true,
	}
}

// WorktreeDirs returns the primary worktree dir followed by any additional
// (legacy) locations, deduplicated.
func (c *Config) WorktreeDirs() []string {
	dirs := []string{c.WorktreeDir}
	for _, d := range c.AdditionalWorktreeDirs {
		if d != c.WorktreeDir {
			dirs = append(dirs, d)
		}
	}
	return dirs
}

// ValidScanDirs returns the configured scan dirs that exist.
func (c *Config) ValidScanDirs() []string {
	var out []string
	for _, d := range c.ScanDirs {
		if isDir(d) {
			out = append(out, d)
		}
	}
	return out
}

func home() string {
	if h, err := os.UserHomeDir(); err == nil {
		return h
	}
	return "."
}

// Dir is ~/.spawnpoint, or $SPAWNPOINT_DIR when set.
func Dir() string {
	if d := os.Getenv("SPAWNPOINT_DIR"); d != "" {
		return d
	}
	return filepath.Join(home(), ".spawnpoint")
}
func Path() string          { return filepath.Join(Dir(), "config.toml") }
func TemplatesPath() string { return filepath.Join(Dir(), "templates.toml") }
func CDPathFile() string    { return filepath.Join(Dir(), ".cd_path") }

func Exists() bool {
	info, err := os.Stat(Path())
	return err == nil && info.Mode().IsRegular()
}

// ExpandPath expands ~ and resolves the path to an absolute, symlink-free form
// where possible.
func ExpandPath(p string) string {
	if p == "~" {
		p = home()
	} else if strings.HasPrefix(p, "~/") {
		p = filepath.Join(home(), p[2:])
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return resolved
	}
	return abs
}

// DetectScanDirs returns common code directories that exist on this machine.
func DetectScanDirs() []string {
	var found []string
	for _, d := range commonCodeDirs {
		p := filepath.Join(home(), strings.TrimPrefix(d, "~/"))
		if isDir(p) {
			found = append(found, p)
		}
	}
	return found
}

type fileFormat struct {
	ScanDirs               *[]string `toml:"scan_dirs"`
	WorktreeDir            *string   `toml:"worktree_dir"`
	ScanDepth              *int      `toml:"scan_depth"`
	CopyGlobs              *[]string `toml:"copy_patterns_globs"`
	CopyFiles              *[]string `toml:"copy_patterns_files"`
	CopyDirs               *[]string `toml:"copy_patterns_dirs"`
	AdditionalWorktreeDirs *[]string `toml:"additional_worktree_dirs"`
	AutoInstallDeps        *bool     `toml:"auto_install_deps"`
	CheckUpdates           *bool     `toml:"check_updates"`
}

// Load reads the config file, falling back to defaults for missing keys (or
// the whole file).
func Load() (*Config, error) {
	cfg := Default()
	data, err := os.ReadFile(Path())
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return nil, err
	}

	var f fileFormat
	if _, err := toml.Decode(string(data), &f); err != nil {
		return nil, fmt.Errorf("%s is not valid TOML: %w", Path(), err)
	}
	if f.ScanDirs != nil {
		cfg.ScanDirs = expandAll(*f.ScanDirs)
	}
	if f.WorktreeDir != nil {
		cfg.WorktreeDir = ExpandPath(*f.WorktreeDir)
	}
	if f.ScanDepth != nil {
		cfg.ScanDepth = *f.ScanDepth
	}
	if f.CopyGlobs != nil {
		cfg.CopyGlobs = *f.CopyGlobs
	}
	if f.CopyFiles != nil {
		cfg.CopyFiles = *f.CopyFiles
	}
	if f.CopyDirs != nil {
		cfg.CopyDirs = *f.CopyDirs
	}
	if f.AdditionalWorktreeDirs != nil {
		cfg.AdditionalWorktreeDirs = expandAll(*f.AdditionalWorktreeDirs)
	}
	if f.AutoInstallDeps != nil {
		cfg.AutoInstallDeps = *f.AutoInstallDeps
	}
	if f.CheckUpdates != nil {
		cfg.CheckUpdates = *f.CheckUpdates
	}
	return cfg, nil
}

// Save writes the config with comments, using ~ for paths under $HOME.
func Save(cfg *Config) (string, error) {
	if err := os.MkdirAll(Dir(), 0o755); err != nil {
		return "", err
	}
	tilde := func(ps []string) []string {
		out := make([]string, len(ps))
		for i, p := range ps {
			out[i] = tildePath(p)
		}
		return out
	}
	lines := []string{
		"# Spawnpoint configuration",
		"# https://github.com/mihirgupta0900/spawnpoint",
		"",
		"# Directories to scan for git repos",
		"scan_dirs = " + tomlList(tilde(cfg.ScanDirs)),
		"",
		"# Where workspaces are created",
		"worktree_dir = " + tomlString(tildePath(cfg.WorktreeDir)),
		"",
		"# Additional directories to scan during cleanup (for worktrees created at previous locations)",
		"additional_worktree_dirs = " + tomlList(tilde(cfg.AdditionalWorktreeDirs)),
		"",
		"# How deep to scan for repos (1-4)",
		fmt.Sprintf("scan_depth = %d", cfg.ScanDepth),
		"",
		"# Glob patterns for files to copy into new worktrees",
		"copy_patterns_globs = " + tomlList(cfg.CopyGlobs),
		"",
		"# Specific files to copy",
		"copy_patterns_files = " + tomlList(cfg.CopyFiles),
		"",
		"# Directories to copy",
		"copy_patterns_dirs = " + tomlList(cfg.CopyDirs),
		"",
		"# Auto-install dependencies after worktree creation",
		fmt.Sprintf("auto_install_deps = %t", cfg.AutoInstallDeps),
		"",
		"# Check for new versions on startup",
		fmt.Sprintf("check_updates = %t", cfg.CheckUpdates),
		"",
	}
	return Path(), os.WriteFile(Path(), []byte(strings.Join(lines, "\n")), 0o644)
}

func tildePath(p string) string {
	h := home()
	if rel, err := filepath.Rel(h, p); err == nil && rel != "." && !strings.HasPrefix(rel, "..") {
		return "~/" + rel
	}
	return p
}

// tomlString prefers a literal ('...') string, as the Python release wrote,
// and falls back to an escaped basic string when the value needs it.
func tomlString(s string) string {
	if !strings.ContainsAny(s, "'\n\r\t") {
		return "'" + s + "'"
	}
	return BasicString(s)
}

func tomlList(items []string) string {
	quoted := make([]string, len(items))
	for i, s := range items {
		quoted[i] = tomlString(s)
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}

// BasicString quotes s as a TOML basic ("...") string.
func BasicString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if r < 0x20 || r == 0x7f {
				fmt.Fprintf(&b, `\u%04X`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

func expandAll(ps []string) []string {
	out := make([]string, len(ps))
	for i, p := range ps {
		out[i] = ExpandPath(p)
	}
	return out
}

func clone(s []string) []string { return append([]string(nil), s...) }

func isDir(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}
