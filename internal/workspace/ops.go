package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Action is a planned worktree for one repo.
type Action struct {
	Repo   Repo
	Target string
	Branch string
	Type   string // "add" (branch exists) or "create" (new branch from Base)
	Base   string
	Status string // "created" / "added" on success, "failed" otherwise
}

// PrepareAll runs Prepare (fetch, set-head, prune) on repos concurrently.
func PrepareAll(repos []Repo) {
	var wg sync.WaitGroup
	sem := make(chan struct{}, workers)
	for _, r := range repos {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			Prepare(r.Path)
		}()
	}
	wg.Wait()
}

// BranchExists reports whether branch exists locally or on origin.
func BranchExists(repo, branch string) bool {
	return RefExists(repo, branch) || RefExists(repo, "origin/"+branch)
}

// AddWorktree creates the worktree for a. New branches start from the latest
// origin/<base> when it exists, without tracking it (so `git push` doesn't
// target the base branch).
func AddWorktree(a *Action) error {
	if err := os.MkdirAll(filepath.Dir(a.Target), 0o755); err != nil {
		return err
	}
	var r Result
	if a.Type == "add" {
		r = Git(a.Repo.Path, "worktree", "add", a.Target, a.Branch)
	} else {
		start := a.Base
		if RefExists(a.Repo.Path, "origin/"+a.Base) {
			start = "origin/" + a.Base
		}
		r = Git(a.Repo.Path, "worktree", "add", "--no-track", "-b", a.Branch, a.Target, start)
	}
	if !r.OK() {
		return fmt.Errorf("%s", strings.TrimSpace(r.Stderr))
	}
	Git(a.Target, "submodule", "update", "--init", "--recursive")
	return nil
}

// RemoveWorktree removes wt (forcing if dirty) and optionally its branch.
// It reports whether the branch was deleted and any notes worth showing.
func RemoveWorktree(wt Worktree, deleteBranch bool) (branchDeleted bool, notes []string) {
	if wt.Parent == "" || !exists(wt.Parent) {
		os.RemoveAll(wt.Path)
		return false, []string{"parent repo gone, removed directory"}
	}

	args := []string{"worktree", "remove", wt.Path}
	if wt.Dirty {
		args = append(args, "--force")
	}
	if r := Git(wt.Parent, args...); !r.OK() {
		notes = append(notes, "worktree remove failed: "+strings.TrimSpace(r.Stderr))
		if exists(wt.Path) {
			os.RemoveAll(wt.Path)
			notes = append(notes, "removed directory instead")
		}
		// Prune so git forgets the worktree (needed before deleting its branch).
		Git(wt.Parent, "worktree", "prune")
	}

	if !deleteBranch || wt.Branch == "unknown" || wt.Branch == "HEAD" {
		return false, notes
	}
	r := Git(wt.Parent, "branch", "-d", wt.Branch)
	if r.OK() {
		return true, notes
	}
	if strings.Contains(r.Stderr, "not fully merged") {
		notes = append(notes, fmt.Sprintf("branch '%s' not merged, force-deleted", wt.Branch))
		return Git(wt.Parent, "branch", "-D", wt.Branch).OK(), notes
	}
	if !strings.Contains(r.Stderr, "not found") {
		notes = append(notes, "branch delete skipped: "+strings.TrimSpace(r.Stderr))
	}
	return false, notes
}

// SingleRepoName returns the parent repo name of a single-repo workspace.
func SingleRepoName(ws string) string {
	if parent, ok := parentRepo(ws); ok {
		return filepath.Base(parent)
	}
	if gitdir, ok := GitDirOf(ws); ok {
		return filepath.Base(filepath.Dir(filepath.Dir(filepath.Dir(gitdir))))
	}
	return filepath.Base(ws)
}

// Restructure converts a single-repo workspace (the folder is the worktree)
// into the multi-repo layout (<ws>/<repo>/), fixing git's links both ways.
func Restructure(ws string) (string, error) {
	gitdir, ok := GitDirOf(ws)
	if !ok {
		return "", fmt.Errorf("%s is not a worktree", ws)
	}
	repoName := SingleRepoName(ws)

	tmp, err := os.MkdirTemp(filepath.Dir(ws), ".sp-restructure-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmp)

	entries, err := os.ReadDir(ws)
	if err != nil {
		return "", err
	}
	for _, e := range entries {
		if err := os.Rename(filepath.Join(ws, e.Name()), filepath.Join(tmp, e.Name())); err != nil {
			return "", err
		}
	}
	sub := filepath.Join(ws, repoName)
	if err := os.Rename(tmp, sub); err != nil {
		return "", err
	}

	// The worktree moved: point its .git at the (absolute) gitdir, and the
	// gitdir back at the new location.
	if err := os.WriteFile(filepath.Join(sub, ".git"), []byte("gitdir: "+gitdir+"\n"), 0o644); err != nil {
		return "", err
	}
	back := filepath.Join(gitdir, "gitdir")
	if exists(back) {
		if err := os.WriteFile(back, []byte(filepath.Join(sub, ".git")+"\n"), 0o644); err != nil {
			return "", err
		}
	}
	return repoName, nil
}
