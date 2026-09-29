package workspace

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
)

// ArtifactType is a reinstallable directory that's safe to delete.
type ArtifactType struct{ Name, Desc string }

var ArtifactTypes = []ArtifactType{
	{"node_modules", "Node.js dependencies"},
	{".venv", "Python virtualenv (.venv)"},
	{"venv", "Python virtualenv (venv)"},
	{"env", "Python virtualenv (env)"},
	{".tox", "Tox test environments"},
	{"__pycache__", "Python bytecode cache"},
	{".pytest_cache", "Pytest cache"},
	{"dist", "Build output (dist)"},
	{"build", "Build output (build)"},
	{".next", "Next.js build cache"},
	{".nuxt", "Nuxt.js build cache"},
	{".turbo", "Turborepo cache"},
	{"target", "Rust/Java build artifacts (target)"},
	{".gradle", "Gradle cache"},
	{".parcel-cache", "Parcel build cache"},
	{".cache", "Generic cache directory"},
}

var artifactNames = func() map[string]bool {
	m := map[string]bool{}
	for _, a := range ArtifactTypes {
		m[a.Name] = true
	}
	return m
}()

// Artifact is one artifact dir found in a workspace.
type Artifact struct {
	Path string
	Type string
	Size int64
}

// FindArtifacts walks root (up to 6 levels, skipping symlinks and .git) and
// returns artifact dirs without descending into them.
func FindArtifacts(root string) []Artifact {
	var found []Artifact
	var walk func(dir string, depth int)
	walk = func(dir string, depth int) {
		if depth > 6 {
			return
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		for _, e := range entries {
			if !e.IsDir() || e.Type()&fs.ModeSymlink != 0 || e.Name() == ".git" {
				continue
			}
			p := filepath.Join(dir, e.Name())
			if artifactNames[e.Name()] {
				found = append(found, Artifact{Path: p, Type: e.Name()})
			} else {
				walk(p, depth+1)
			}
		}
	}
	walk(root, 0)
	return found
}

// MeasureSizes fills in Size for each artifact, in parallel.
func MeasureSizes(arts []Artifact) {
	var wg sync.WaitGroup
	sem := make(chan struct{}, workers)
	for i := range arts {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			arts[i].Size = dirSize(arts[i].Path)
		}()
	}
	wg.Wait()
}

func dirSize(root string) int64 {
	var total int64
	filepath.WalkDir(root, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.Type().IsRegular() {
			if info, err := d.Info(); err == nil {
				total += info.Size()
			}
		}
		return nil
	})
	return total
}

// FormatBytes renders n as "12.3 MB".
func FormatBytes(n int64) string {
	f := float64(n)
	for _, unit := range []string{"B", "KB", "MB", "GB"} {
		if f < 1024 {
			return fmt.Sprintf("%.1f %s", f, unit)
		}
		f /= 1024
	}
	return fmt.Sprintf("%.1f TB", f)
}
