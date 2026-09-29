package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

// Worktree is one repo checkout inside a workspace.
type Worktree struct {
	Path     string
	Parent   string // main checkout the worktree belongs to ("" if gone)
	Branch   string
	Dirty    bool
	Modified time.Time
}

// ParentName is the parent repo's directory name, or "unknown".
func (w Worktree) ParentName() string {
	if w.Parent == "" {
		return "unknown"
	}
	return filepath.Base(w.Parent)
}

// Workspace is a folder under the worktree dir: either a single worktree
// (single-repo layout) or a folder of worktrees, one per repo.
type Workspace struct {
	Path      string
	Name      string
	Worktrees []Worktree
}

// Oldest is the oldest worktree mtime (the folder's own mtime if empty).
func (w *Workspace) Oldest() time.Time {
	if len(w.Worktrees) == 0 {
		if info, err := os.Stat(w.Path); err == nil {
			return info.ModTime()
		}
		return time.Time{}
	}
	oldest := w.Worktrees[0].Modified
	for _, wt := range w.Worktrees[1:] {
		if wt.Modified.Before(oldest) {
			oldest = wt.Modified
		}
	}
	return oldest
}

func (w *Workspace) Dirty() bool {
	for _, wt := range w.Worktrees {
		if wt.Dirty {
			return true
		}
	}
	return false
}

// Branches returns the distinct branches checked out, sorted.
func (w *Workspace) Branches() []string {
	set := map[string]bool{}
	for _, wt := range w.Worktrees {
		set[wt.Branch] = true
	}
	out := make([]string, 0, len(set))
	for b := range set {
		out = append(out, b)
	}
	sort.Strings(out)
	return out
}

// Label is the one-line summary used in pickers.
func (w *Workspace) Label() string {
	n := len(w.Worktrees)
	repos := "repos"
	if n == 1 {
		repos = "repo"
	}
	state := "clean"
	if w.Dirty() {
		state = "dirty"
	}
	return fmt.Sprintf("%s  (%d %s, %s, %s)", w.Name, n, repos, state, FormatAge(w.Oldest()))
}

// GitDirOf reads a worktree's .git file and returns the gitdir it points to.
func GitDirOf(worktree string) (string, bool) {
	gitFile := filepath.Join(worktree, ".git")
	if !isFile(gitFile) {
		return "", false
	}
	data, err := os.ReadFile(gitFile)
	if err != nil {
		return "", false
	}
	content := strings.TrimSpace(string(data))
	rest, ok := strings.CutPrefix(content, "gitdir:")
	if !ok {
		return "", false
	}
	gitdir := strings.TrimSpace(rest)
	if !filepath.IsAbs(gitdir) {
		gitdir = filepath.Join(worktree, gitdir)
	}
	return filepath.Clean(gitdir), true
}

// parentRepo resolves the main checkout from a worktree's gitdir, which
// looks like /path/to/repo/.git/worktrees/<name>.
func parentRepo(worktree string) (string, bool) {
	gitdir, ok := GitDirOf(worktree)
	if !ok {
		return "", false
	}
	parent := filepath.Dir(filepath.Dir(filepath.Dir(gitdir)))
	if exists(filepath.Join(parent, ".git")) {
		return parent, true
	}
	return "", false
}

func scanWorktree(path string) (Worktree, bool) {
	parent, ok := parentRepo(path)
	if !ok {
		return Worktree{}, false
	}
	info, err := os.Stat(path)
	if err != nil {
		return Worktree{}, false
	}
	gitdir, _ := GitDirOf(path)
	branch := branchFromGitDir(gitdir)
	if branch == "" {
		branch = CurrentBranch(path)
	}
	return Worktree{
		Path:     path,
		Parent:   parent,
		Branch:   branch,
		Dirty:    IsDirty(path),
		Modified: info.ModTime(),
	}, true
}

// workers bounds concurrent git subprocesses: enough to hide per-process
// latency without thrashing the disk.
var workers = max(8, 2*runtime.NumCPU())

// ScanWorkspaces finds workspaces across dirs (deduplicated by resolved
// path), scanning worktrees in parallel.
func ScanWorkspaces(dirs []string) []*Workspace {
	type job struct{ ws, wt string }
	var jobs []job
	var order []string
	seen := map[string]bool{}

	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			p := filepath.Join(dir, e.Name())
			if !isDir(p) {
				continue
			}
			resolved, err := filepath.EvalSymlinks(p)
			if err != nil {
				resolved = p
			}
			if seen[resolved] {
				continue
			}
			seen[resolved] = true
			order = append(order, p)

			if isFile(filepath.Join(p, ".git")) {
				jobs = append(jobs, job{p, p})
				continue
			}
			subs, _ := os.ReadDir(p)
			for _, s := range subs {
				sp := filepath.Join(p, s.Name())
				if isDir(sp) && isFile(filepath.Join(sp, ".git")) {
					jobs = append(jobs, job{p, sp})
				}
			}
		}
	}

	results := make([]*Worktree, len(jobs))
	var wg sync.WaitGroup
	sem := make(chan struct{}, workers)
	for i, j := range jobs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			if wt, ok := scanWorktree(j.wt); ok {
				results[i] = &wt
			}
		}()
	}
	wg.Wait()

	byPath := map[string]*Workspace{}
	for i, j := range jobs {
		if results[i] == nil {
			continue
		}
		ws := byPath[j.ws]
		if ws == nil {
			ws = &Workspace{Path: j.ws, Name: filepath.Base(j.ws)}
			byPath[j.ws] = ws
		}
		ws.Worktrees = append(ws.Worktrees, *results[i])
	}
	var out []*Workspace
	for _, p := range order {
		if ws := byPath[p]; ws != nil {
			out = append(out, ws)
		}
	}
	return out
}

// SortByAge sorts workspaces by their oldest worktree mtime.
func SortByAge(wss []*Workspace, newestFirst bool) {
	sort.SliceStable(wss, func(i, j int) bool {
		if newestFirst {
			return wss[i].Oldest().After(wss[j].Oldest())
		}
		return wss[i].Oldest().Before(wss[j].Oldest())
	})
}

// FormatAge renders a coarse "3d ago" style age.
func FormatAge(t time.Time) string {
	d := time.Since(t)
	days := int(d.Hours() / 24)
	switch {
	case days == 0:
		if h := int(d.Hours()); h > 0 {
			return fmt.Sprintf("%dh ago", h)
		}
		return "just now"
	case days < 30:
		return fmt.Sprintf("%dd ago", days)
	default:
		return fmt.Sprintf("%dmo ago", days/30)
	}
}

// Detect reports the workspace containing cwd (searching dirs), along with
// its branch, inferred from an existing worktree in it.
func Detect(cwd string, dirs []string) (workspace, branch string, ok bool) {
	if resolved, err := filepath.EvalSymlinks(cwd); err == nil {
		cwd = resolved
	}
	for _, dir := range dirs {
		if resolved, err := filepath.EvalSymlinks(dir); err == nil {
			dir = resolved
		} else {
			continue
		}
		rel, err := filepath.Rel(dir, cwd)
		if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
			continue
		}
		first := strings.Split(rel, string(filepath.Separator))[0]
		ws := filepath.Join(dir, first)
		if !isDir(ws) {
			continue
		}
		if b := inferBranch(ws); b != "" {
			return ws, b, true
		}
	}
	return "", "", false
}

func inferBranch(ws string) string {
	if isFile(filepath.Join(ws, ".git")) {
		if r := Git(ws, "rev-parse", "--abbrev-ref", "HEAD"); r.OK() {
			return strings.TrimSpace(r.Stdout)
		}
	}
	entries, _ := os.ReadDir(ws)
	for _, e := range entries {
		p := filepath.Join(ws, e.Name())
		if isDir(p) && isFile(filepath.Join(p, ".git")) {
			if r := Git(p, "rev-parse", "--abbrev-ref", "HEAD"); r.OK() {
				return strings.TrimSpace(r.Stdout)
			}
		}
	}
	return ""
}

// RepoNamesIn returns the repo (worktree dir) names present in a workspace.
func RepoNamesIn(ws string) map[string]bool {
	names := map[string]bool{}
	if isFile(filepath.Join(ws, ".git")) {
		// Single-repo layout: the folder is named after the branch, so use
		// the parent repo's name.
		if parent, ok := parentRepo(ws); ok {
			names[filepath.Base(parent)] = true
		}
	}
	entries, _ := os.ReadDir(ws)
	for _, e := range entries {
		p := filepath.Join(ws, e.Name())
		if isDir(p) && isFile(filepath.Join(p, ".git")) {
			names[e.Name()] = true
		}
	}
	return names
}

// IsSingleRepo reports whether ws is itself a worktree (single-repo layout).
func IsSingleRepo(ws string) bool { return isFile(filepath.Join(ws, ".git")) }

// Exists reports whether p exists.
func Exists(p string) bool { return exists(p) }
