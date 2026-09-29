package workspace

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Repo is a git repo found under a scan dir.
type Repo struct {
	Name string `json:"name"` // display name, relative to its scan dir
	Path string `json:"path"`
}

// Base is the repo's directory name.
func (r Repo) Base() string { return filepath.Base(r.Path) }

func exists(p string) bool {
	_, err := os.Lstat(p)
	return err == nil
}

func isDir(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}

func isFile(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.Mode().IsRegular()
}

func isGitRepo(p string) bool { return exists(filepath.Join(p, ".git")) }

// FindRepos returns git repos under scanDirs, descending up to depth levels
// but never into a repo. Results are sorted by path and named relative to the
// first scan dir that contains them.
func FindRepos(scanDirs []string, depth int) []Repo {
	depth = max(depth, 1)
	seen := map[string]bool{}
	var paths []string
	var walk func(dir string, level int)
	walk = func(dir string, level int) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		for _, e := range entries {
			p := filepath.Join(dir, e.Name())
			if !isDir(p) {
				continue
			}
			if isGitRepo(p) {
				if !seen[p] {
					seen[p] = true
					paths = append(paths, p)
				}
			} else if level < depth {
				walk(p, level+1)
			}
		}
	}
	for _, d := range scanDirs {
		if isDir(d) {
			walk(d, 1)
		}
	}
	sort.Strings(paths)

	repos := make([]Repo, len(paths))
	for i, p := range paths {
		repos[i] = Repo{Name: DisplayName(p, scanDirs), Path: p}
	}
	return repos
}

// DisplayName makes path relative to the first scan dir containing it.
func DisplayName(path string, scanDirs []string) string {
	for _, d := range scanDirs {
		if rel, err := filepath.Rel(d, path); err == nil && rel != "." && !strings.HasPrefix(rel, "..") {
			return rel
		}
	}
	return path
}
