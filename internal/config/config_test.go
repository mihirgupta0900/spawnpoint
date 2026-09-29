package config

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	code := filepath.Join(home, "code")
	os.MkdirAll(code, 0o755)

	cfg := Default()
	cfg.ScanDirs = []string{code}
	cfg.ScanDepth = 3
	cfg.AutoInstallDeps = false
	if _, err := Save(cfg); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(Path())
	// Same layout the Python release wrote: ~ paths, single-quoted strings.
	for _, want := range []string{"scan_dirs = ['~/code']", "worktree_dir = '~/.spawnpoint/workspaces'", "copy_patterns_globs = ['.env*']", "auto_install_deps = false"} {
		if !contains(string(data), want) {
			t.Errorf("config missing %q:\n%s", want, data)
		}
	}

	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	resolved, _ := filepath.EvalSymlinks(code)
	if !slices.Equal(got.ScanDirs, []string{resolved}) || got.ScanDepth != 3 || got.AutoInstallDeps || !got.CheckUpdates {
		t.Fatalf("loaded = %+v", got)
	}
}

func TestLoadPartialConfigKeepsDefaults(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	os.MkdirAll(Dir(), 0o755)
	os.WriteFile(Path(), []byte("scan_depth = 1\n"), 0o644)
	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.ScanDepth != 1 || !got.AutoInstallDeps || !slices.Equal(got.CopyFiles, DefaultCopyFiles) {
		t.Fatalf("loaded = %+v", got)
	}
}

func TestTemplateEscapingRoundTrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	tricky := "we\"ird\\name.with dots\n\x01"
	base := "staging"
	if _, err := SaveTemplates(map[string]*Template{tricky: {Name: tricky, Repos: []string{"a/b", "c"}, Base: &base}}); err != nil {
		t.Fatal(err)
	}
	got, err := LoadTemplates()
	if err != nil {
		t.Fatal(err)
	}
	tmpl := got[tricky]
	if tmpl == nil || !slices.Equal(tmpl.Repos, []string{"a/b", "c"}) || *tmpl.Base != "staging" || tmpl.Description != nil {
		t.Fatalf("templates = %+v", got)
	}
}

func contains(s, sub string) bool { return len(s) >= len(sub) && (s == sub || indexOf(s, sub) >= 0) }

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
