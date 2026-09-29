// Package workspace implements the git and filesystem work behind every
// command: finding repos, creating and removing worktrees, copying config
// files, installing dependencies, and scanning existing workspaces.
package workspace

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Result of a git (or other) subprocess.
type Result struct {
	Stdout, Stderr string
	Code           int
}

func (r Result) OK() bool { return r.Code == 0 }

// Run executes name args... in dir, capturing output. A missing binary or a
// timeout reports Code -1.
func Run(dir string, timeout time.Duration, name string, args ...string) Result {
	ctx := context.Background()
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	start := time.Now()
	err := cmd.Run()
	r := Result{Stdout: out.String(), Stderr: errb.String()}
	if err != nil {
		r.Code = -1
		if ee, ok := err.(*exec.ExitError); ok {
			r.Code = ee.ExitCode()
		}
	}
	slog.Debug("exec", "dir", dir, "cmd", name+" "+strings.Join(args, " "), "code", r.Code, "took", time.Since(start).Round(time.Millisecond))
	return r
}

// Git runs git in dir with no timeout.
func Git(dir string, args ...string) Result { return Run(dir, 0, "git", args...) }

// RefExists reports whether ref resolves in repo.
func RefExists(repo, ref string) bool {
	return Git(repo, "rev-parse", "--verify", "--quiet", ref).OK()
}

// Prepare fetches, refreshes origin/HEAD and prunes stale worktrees.
func Prepare(repo string) {
	Git(repo, "fetch")
	Git(repo, "remote", "set-head", "origin", "--auto")
	Git(repo, "worktree", "prune")
}

var headBranchRe = regexp.MustCompile(`HEAD branch:\s+(\S+)`)

// DetectDefaultBranch finds the repo's default branch:
//  1. refs/remotes/origin/HEAD (local, instant)
//  2. git remote show origin (network, 10s timeout)
//  3. gh repo view (needs gh + auth, 15s timeout)
func DetectDefaultBranch(repo string) string {
	if r := Git(repo, "symbolic-ref", "refs/remotes/origin/HEAD"); r.OK() {
		ref := strings.TrimSpace(r.Stdout)
		if b, ok := strings.CutPrefix(ref, "refs/remotes/origin/"); ok && b != "" {
			return b
		}
	}
	if r := Run(repo, 10*time.Second, "git", "remote", "show", "origin"); r.OK() {
		if m := headBranchRe.FindStringSubmatch(r.Stdout); m != nil && m[1] != "(unknown)" {
			return m[1]
		}
	}
	if r := Run(repo, 15*time.Second, "gh", "repo", "view", "--json", "defaultBranchRef", "-q", ".defaultBranchRef.name"); r.OK() {
		if b := strings.TrimSpace(r.Stdout); b != "" {
			return b
		}
	}
	return ""
}

// CurrentBranch returns the checked-out branch, or "unknown".
func CurrentBranch(dir string) string {
	r := Git(dir, "rev-parse", "--abbrev-ref", "HEAD")
	if !r.OK() {
		return "unknown"
	}
	return strings.TrimSpace(r.Stdout)
}

// IsDirty reports uncommitted changes (including untracked files) in dir.
// Rename detection and optional index locks are skipped: they cost time and
// don't change the answer.
func IsDirty(dir string) bool {
	r := Git(dir, "--no-optional-locks", "status", "--porcelain", "--no-renames", "--ignore-submodules=dirty")
	return r.OK() && strings.TrimSpace(r.Stdout) != ""
}

// branchFromGitDir reads the checked-out branch straight from a worktree's
// HEAD file, avoiding a git subprocess. It returns "" if HEAD is detached or
// unreadable.
func branchFromGitDir(gitdir string) string {
	data, err := os.ReadFile(filepath.Join(gitdir, "HEAD"))
	if err != nil {
		return ""
	}
	ref, ok := strings.CutPrefix(strings.TrimSpace(string(data)), "ref: refs/heads/")
	if !ok {
		return ""
	}
	return ref
}
