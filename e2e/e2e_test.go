// Package e2e drives the spawnpoint binary against real git repos in a
// throwaway $HOME. It pins the --no-input / --json contract that agents and
// scripts depend on.
//
// By default it builds the binary from this module. Set SPAWNPOINT_BIN to test
// another build (e.g. the legacy Python CLI, with SPAWNPOINT_IMPL=python to
// skip the cases where the Go port intentionally differs).
package e2e

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var bin string

func TestMain(m *testing.M) {
	if b := os.Getenv("SPAWNPOINT_BIN"); b != "" {
		bin = b
		os.Exit(m.Run())
	}
	dir, err := os.MkdirTemp("", "sp-e2e-bin")
	if err != nil {
		panic(err)
	}
	bin = filepath.Join(dir, "spawnpoint")
	build := exec.Command("go", "build", "-o", bin, "..")
	build.Stdout, build.Stderr = os.Stderr, os.Stderr
	if err := build.Run(); err != nil {
		panic(err)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

func goOnly(t *testing.T) {
	if os.Getenv("SPAWNPOINT_IMPL") == "python" {
		t.Skip("behaviour intentionally differs from the Python release")
	}
}

type env struct {
	t      *testing.T
	home   string
	code   string
	wsRoot string
}

func realpath(t *testing.T, p string) string {
	r, err := filepath.EvalSymlinks(p)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = gitEnv(os.Environ())
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v in %s: %v\n%s", args, dir, err, out)
	}
	return strings.TrimSpace(string(out))
}

func gitEnv(base []string) []string {
	return append(base,
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=t@example.com",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=t@example.com",
		"GIT_CONFIG_NOSYSTEM=1",
	)
}

// newEnv makes $HOME with ~/code/{api,web,worker}, each cloned from a bare
// "origin" with a main branch, plus a config pointing at them.
func newEnv(t *testing.T, repos ...string) *env {
	t.Helper()
	root := realpath(t, t.TempDir())
	e := &env{t: t, home: filepath.Join(root, "home")}
	e.code = filepath.Join(e.home, "code")
	e.wsRoot = filepath.Join(e.home, ".spawnpoint", "workspaces")
	if err := os.MkdirAll(e.code, 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(e.home, ".gitconfig"), []byte("[init]\n\tdefaultBranch = main\n"), 0o644)

	if len(repos) == 0 {
		repos = []string{"api", "web", "worker"}
	}
	for _, name := range repos {
		remote := filepath.Join(root, "remotes", name+".git")
		os.MkdirAll(remote, 0o755)
		e.git(remote, "init", "--bare", "-b", "main")
		work := filepath.Join(e.code, name)
		e.git(e.code, "clone", "-q", remote, name)
		os.WriteFile(filepath.Join(work, "README.md"), []byte("# "+name+"\n"), 0o644)
		os.WriteFile(filepath.Join(work, ".gitignore"), []byte(".env*\nCLAUDE.md\nnode_modules/\n"), 0o644)
		e.git(work, "add", ".")
		e.git(work, "commit", "-qm", "init")
		e.git(work, "push", "-q", "origin", "main")
		e.git(work, "remote", "set-head", "origin", "--auto")
		os.WriteFile(filepath.Join(work, ".env"), []byte("SECRET="+name+"\n"), 0o644)
		os.WriteFile(filepath.Join(work, "CLAUDE.md"), []byte("agent notes\n"), 0o644)
	}

	os.MkdirAll(filepath.Join(e.home, ".spawnpoint"), 0o755)
	cfg := "scan_dirs = ['" + e.code + "']\n" +
		"worktree_dir = '" + e.wsRoot + "'\n" +
		"auto_install_deps = false\n" +
		"check_updates = false\n"
	if err := os.WriteFile(filepath.Join(e.home, ".spawnpoint", "config.toml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	return e
}

func (e *env) env() []string {
	var base []string
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "HOME=") && !strings.HasPrefix(kv, "XDG_CONFIG_HOME=") {
			base = append(base, kv)
		}
	}
	return gitEnv(append(base, "HOME="+e.home, "NO_COLOR=1"))
}

func (e *env) git(dir string, args ...string) string {
	e.t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = e.env()
	out, err := cmd.CombinedOutput()
	if err != nil {
		e.t.Fatalf("git %v in %s: %v\n%s", args, dir, err, out)
	}
	return strings.TrimSpace(string(out))
}

type result struct {
	stdout, stderr string
	code           int
}

func (e *env) run(dir string, args ...string) result {
	e.t.Helper()
	if dir == "" {
		dir = e.home
	}
	parts := strings.Fields(bin) // allow "uv run spawnpoint"-style wrappers
	cmd := exec.Command(parts[0], append(parts[1:], args...)...)
	cmd.Dir = dir
	cmd.Env = e.env()
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := cmd.Run()
	r := result{out.String(), errb.String(), 0}
	if ee, ok := err.(*exec.ExitError); ok {
		r.code = ee.ExitCode()
	} else if err != nil {
		e.t.Fatal(err)
	}
	return r
}

// ok runs a command that must succeed.
func (e *env) ok(dir string, args ...string) result {
	e.t.Helper()
	r := e.run(dir, args...)
	if r.code != 0 {
		e.t.Fatalf("spawnpoint %v exited %d\nstdout: %s\nstderr: %s", args, r.code, r.stdout, r.stderr)
	}
	return r
}

// json runs a command that must succeed and decodes its stdout.
func (e *env) json(dir string, v any, args ...string) {
	e.t.Helper()
	r := e.ok(dir, args...)
	if err := json.Unmarshal([]byte(r.stdout), v); err != nil {
		e.t.Fatalf("stdout of %v is not JSON: %v\n%s\nstderr: %s", args, err, r.stdout, r.stderr)
	}
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

type createOut struct {
	Workspace string  `json:"workspace"`
	Branch    string  `json:"branch"`
	Template  *string `json:"template"`
	Repos     []struct {
		Name   string  `json:"name"`
		Branch string  `json:"branch"`
		Base   *string `json:"base"`
		Action string  `json:"action"`
		Status string  `json:"status"`
	} `json:"repos"`
}

func TestRepos(t *testing.T) {
	e := newEnv(t)
	var repos []struct{ Name, Path string }
	e.json("", &repos, "repos", "--json")
	if len(repos) != 3 || repos[0].Name != "api" || repos[2].Name != "worker" {
		t.Fatalf("repos = %+v", repos)
	}
	if repos[0].Path != filepath.Join(e.code, "api") {
		t.Fatalf("path = %s", repos[0].Path)
	}
}

func TestCreateMultiRepo(t *testing.T) {
	e := newEnv(t)
	var out createOut
	e.json("", &out, "create", "--no-input", "--json", "--repos", "api,web", "--branch", "feat/x")

	ws := filepath.Join(e.wsRoot, "feat-x")
	if out.Workspace != ws || out.Branch != "feat/x" || out.Template != nil {
		t.Fatalf("out = %+v", out)
	}
	if len(out.Repos) != 2 {
		t.Fatalf("repos = %+v", out.Repos)
	}
	for _, r := range out.Repos {
		if r.Action != "create" || r.Status != "created" || r.Base == nil || *r.Base != "main" {
			t.Fatalf("repo = %+v", r)
		}
	}
	for _, f := range []string{"api/.env", "api/CLAUDE.md", "web/.env", "api/README.md"} {
		if !exists(filepath.Join(ws, f)) {
			t.Errorf("missing %s", f)
		}
	}
	if b := e.git(filepath.Join(ws, "api"), "rev-parse", "--abbrev-ref", "HEAD"); b != "feat/x" {
		t.Fatalf("branch = %s", b)
	}
	// New branches must not track the base branch (git push would target main).
	if up := e.run(filepath.Join(ws, "api"), "--version"); up.code != 0 {
		t.Fatal(up.stderr)
	}
	cmd := exec.Command("git", "config", "branch.feat/x.merge")
	cmd.Dir = filepath.Join(e.code, "api")
	cmd.Env = e.env()
	if out, _ := cmd.Output(); len(bytes.TrimSpace(out)) != 0 {
		t.Fatalf("feat/x tracks %s", out)
	}
	cdFile, _ := os.ReadFile(filepath.Join(e.home, ".spawnpoint", ".cd_path"))
	if string(cdFile) != ws {
		t.Fatalf(".cd_path = %q", cdFile)
	}
}

func TestCreatePrintsPathWithoutJSON(t *testing.T) {
	e := newEnv(t)
	r := e.ok("", "create", "--no-input", "--repos", "api", "--branch", "solo")
	if got := strings.TrimSpace(r.stdout); got != filepath.Join(e.wsRoot, "solo") {
		t.Fatalf("stdout = %q", r.stdout)
	}
	// Single repo: the workspace folder is the worktree itself.
	if !exists(filepath.Join(e.wsRoot, "solo", ".git")) {
		t.Fatal("single-repo worktree missing")
	}
}

func TestCreateExistingBranch(t *testing.T) {
	e := newEnv(t)
	e.git(filepath.Join(e.code, "api"), "branch", "feat/old")
	var out createOut
	e.json("", &out, "create", "--no-input", "--json", "--repos", "api,web", "--branch", "feat/old")
	if out.Repos[0].Action != "add" || out.Repos[0].Base != nil || out.Repos[1].Action != "create" {
		t.Fatalf("repos = %+v", out.Repos)
	}
}

func TestCreateErrors(t *testing.T) {
	e := newEnv(t)
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"create", "--no-input", "--repos", "nope", "--branch", "x"}, "Valid: api, web, worker"},
		{[]string{"create", "--no-input", "--repos", "api"}, "--no-input requires --branch"},
		{[]string{"create", "--no-input", "--branch", "x"}, "--no-input requires --repos or --template"},
		{[]string{"create", "--no-input", "--template", "missing", "--branch", "x"}, "template 'missing' not found"},
	}
	for _, c := range cases {
		r := e.run("", c.args...)
		if r.code == 0 || !strings.Contains(r.stderr, c.want) {
			t.Errorf("%v: code=%d stderr=%q, want %q", c.args, r.code, r.stderr, c.want)
		}
		if r.stdout != "" {
			t.Errorf("%v: stdout should be empty, got %q", c.args, r.stdout)
		}
	}
}

func TestListAndCD(t *testing.T) {
	e := newEnv(t)
	var empty []any
	e.json("", &empty, "list", "--json")
	if len(empty) != 0 {
		t.Fatalf("list = %v", empty)
	}

	e.ok("", "create", "--no-input", "--repos", "api,web", "--branch", "feat/x")
	os.WriteFile(filepath.Join(e.wsRoot, "feat-x", "web", "new.txt"), []byte("x"), 0o644)

	var list []struct {
		Name     string
		Path     string
		Repos    int
		Branches []string
		Dirty    bool
	}
	e.json("", &list, "list", "--json")
	if len(list) != 1 || list[0].Name != "feat-x" || list[0].Repos != 2 || !list[0].Dirty ||
		len(list[0].Branches) != 1 || list[0].Branches[0] != "feat/x" {
		t.Fatalf("list = %+v", list)
	}

	r := e.ok("", "list", "--cd", "--no-input", "--workspace", "feat-x")
	if strings.TrimSpace(r.stdout) != filepath.Join(e.wsRoot, "feat-x") {
		t.Fatalf("cd stdout = %q", r.stdout)
	}
	if r := e.run("", "list", "--cd", "--no-input", "--workspace", "nope"); r.code == 0 || !strings.Contains(r.stderr, "Valid: feat-x") {
		t.Fatalf("bad workspace: %+v", r)
	}
}

type addOut struct {
	Workspace string `json:"workspace"`
	Branch    string `json:"branch"`
	Added     []struct {
		Name, Action, Status string
	} `json:"added"`
}

func TestAdd(t *testing.T) {
	e := newEnv(t)
	e.ok("", "create", "--no-input", "--repos", "api,web", "--branch", "feat/x")
	ws := filepath.Join(e.wsRoot, "feat-x")

	// Repos already in the workspace aren't offered again.
	if r := e.run(ws, "add", "--no-input", "--repos", "api"); r.code == 0 || !strings.Contains(r.stderr, "not found") {
		t.Fatalf("re-adding api: %+v", r)
	}

	var out addOut
	e.json(filepath.Join(ws, "api"), &out, "add", "--no-input", "--json", "--repos", "worker")
	if out.Workspace != ws || out.Branch != "feat/x" || len(out.Added) != 1 || out.Added[0].Status != "added" {
		t.Fatalf("add = %+v", out)
	}
	if b := e.git(filepath.Join(ws, "worker"), "rev-parse", "--abbrev-ref", "HEAD"); b != "feat/x" {
		t.Fatalf("worker branch = %s", b)
	}

	if r := e.run(e.home, "add", "--no-input", "--repos", "worker"); r.code == 0 || !strings.Contains(r.stderr, "Not inside a spawnpoint workspace") {
		t.Fatalf("outside workspace: %+v", r)
	}
}

func TestAddRestructuresSingleRepo(t *testing.T) {
	goOnly(t) // Python keyed single-repo workspaces by folder, not repo, name.
	e := newEnv(t)
	e.ok("", "create", "--no-input", "--repos", "api", "--branch", "solo")
	ws := filepath.Join(e.wsRoot, "solo")

	var out addOut
	e.json(ws, &out, "add", "--no-input", "--json", "--repos", "web")
	if len(out.Added) != 1 || out.Added[0].Name != "web" {
		t.Fatalf("add = %+v", out)
	}
	for _, repo := range []string{"api", "web"} {
		if b := e.git(filepath.Join(ws, repo), "rev-parse", "--abbrev-ref", "HEAD"); b != "solo" {
			t.Fatalf("%s branch = %s", repo, b)
		}
	}
	if !exists(filepath.Join(ws, "api", ".env")) {
		t.Fatal("restructure lost untracked files")
	}
	// git in the parent repo must know the worktree's new location.
	if list := e.git(filepath.Join(e.code, "api"), "worktree", "list"); !strings.Contains(list, filepath.Join(ws, "api")) {
		t.Fatalf("worktree list:\n%s", list)
	}
	// And api is no longer offered.
	if r := e.run(ws, "add", "--no-input", "--repos", "api"); r.code == 0 {
		t.Fatal("api offered again after restructure")
	}
}

type tmplOut struct {
	Name        string   `json:"name"`
	Repos       []string `json:"repos"`
	Base        *string  `json:"base"`
	Description *string  `json:"description"`
}

func TestTemplates(t *testing.T) {
	e := newEnv(t)
	var saved tmplOut
	e.json("", &saved, "template", "save", "stack", "--no-input", "--json", "--repos", "api,worker", "--base", "main", "-d", "The stack")
	if saved.Name != "stack" || strings.Join(saved.Repos, ",") != "api,worker" || *saved.Base != "main" || *saved.Description != "The stack" {
		t.Fatalf("saved = %+v", saved)
	}

	// Updating keeps base/description when omitted.
	e.json("", &saved, "template", "save", "stack", "--no-input", "--json", "--repos", "api,web")
	if strings.Join(saved.Repos, ",") != "api,web" || saved.Base == nil || *saved.Base != "main" {
		t.Fatalf("updated = %+v", saved)
	}

	var list []tmplOut
	e.json("", &list, "template", "list", "--json")
	if len(list) != 1 || list[0].Name != "stack" {
		t.Fatalf("list = %+v", list)
	}
	var shown tmplOut
	e.json("", &shown, "template", "show", "stack", "--json")
	if shown.Name != "stack" {
		t.Fatalf("show = %+v", shown)
	}

	var out createOut
	e.json("", &out, "create", "--no-input", "--json", "--template", "stack", "--branch", "feat/t")
	if out.Template == nil || *out.Template != "stack" || len(out.Repos) != 2 {
		t.Fatalf("create -t = %+v", out)
	}

	var del map[string]string
	e.json("", &del, "template", "delete", "stack", "--no-input", "--json")
	if del["deleted"] != "stack" {
		t.Fatalf("delete = %v", del)
	}
	e.json("", &list, "template", "list", "--json")
	if len(list) != 0 {
		t.Fatalf("list after delete = %+v", list)
	}
}

func TestTemplatesFileCompat(t *testing.T) {
	// A templates.toml written by the Python release, including a name that
	// needs escaping.
	e := newEnv(t)
	legacy := "# Spawnpoint workspace templates\n\n" +
		"[templates.\"image-gen\"]\ndescription = \"Image generation stack\"\nrepos = [\"api\", \"web\"]\nbase = \"main\"\n\n" +
		"[templates.\"we\\\"ird\"]\nrepos = [\"worker\"]\n"
	os.WriteFile(filepath.Join(e.home, ".spawnpoint", "templates.toml"), []byte(legacy), 0o644)
	var list []tmplOut
	e.json("", &list, "template", "list", "--json")
	if len(list) != 2 || list[0].Name != "image-gen" || list[1].Name != `we"ird` || list[1].Base != nil {
		t.Fatalf("list = %+v", list)
	}
}

func TestCleanup(t *testing.T) {
	goOnly(t) // Python printed status text to stdout here, breaking --json.
	e := newEnv(t)
	e.ok("", "create", "--no-input", "--repos", "api,web", "--branch", "feat/x")
	e.ok("", "create", "--no-input", "--repos", "worker", "--branch", "keep")

	if r := e.run("", "cleanup", "--no-input", "--workspaces", "feat-x"); r.code == 0 || !strings.Contains(r.stderr, "--delete-branches or --keep-branches") {
		t.Fatalf("missing branch pref: %+v", r)
	}

	var out struct {
		Removed []struct {
			Workspace string
			Worktrees []struct {
				Repo          string
				Branch        string
				BranchDeleted bool `json:"branch_deleted"`
			}
		}
	}
	e.json("", &out, "cleanup", "--no-input", "--json", "--workspaces", "feat-x", "--delete-branches")
	if len(out.Removed) != 1 || len(out.Removed[0].Worktrees) != 2 || !out.Removed[0].Worktrees[0].BranchDeleted {
		t.Fatalf("cleanup = %+v", out)
	}
	if exists(filepath.Join(e.wsRoot, "feat-x")) {
		t.Fatal("workspace dir still exists")
	}
	if b := e.git(filepath.Join(e.code, "api"), "branch", "--list", "feat/x"); b != "" {
		t.Fatalf("branch not deleted: %q", b)
	}

	e.json("", &out, "cleanup", "--no-input", "--json", "--workspaces", "keep", "--keep-branches")
	if out.Removed[0].Worktrees[0].BranchDeleted {
		t.Fatal("branch deleted despite --keep-branches")
	}
	if b := e.git(filepath.Join(e.code, "worker"), "branch", "--list", "keep"); b == "" {
		t.Fatal("kept branch is gone")
	}
}

func TestLightCleanup(t *testing.T) {
	goOnly(t) // Python printed status text to stdout here, breaking --json.
	e := newEnv(t)
	e.ok("", "create", "--no-input", "--repos", "api,web", "--branch", "feat/x")
	ws := filepath.Join(e.wsRoot, "feat-x")
	for _, d := range []string{"api/node_modules/pkg", "web/.venv/lib", "web/src/__pycache__"} {
		os.MkdirAll(filepath.Join(ws, d), 0o755)
		os.WriteFile(filepath.Join(ws, d, "blob"), bytes.Repeat([]byte("x"), 4096), 0o644)
	}

	var out struct {
		Freed      int64 `json:"freed_bytes"`
		Workspaces []struct {
			Workspace string
			Artifacts []struct {
				Path string
				OK   bool
			}
		}
	}
	e.json("", &out, "light-cleanup", "--no-input", "--json", "--workspaces", "feat-x", "--artifact-types", "node_modules,.venv")
	if out.Freed != 8192 || len(out.Workspaces[0].Artifacts) != 2 {
		t.Fatalf("light-cleanup = %+v", out)
	}
	if exists(filepath.Join(ws, "api/node_modules")) || !exists(filepath.Join(ws, "web/src/__pycache__")) {
		t.Fatal("wrong dirs deleted")
	}
	if !exists(filepath.Join(ws, "api/README.md")) {
		t.Fatal("code was deleted")
	}
}

func TestHeadlessFirstRunWritesConfig(t *testing.T) {
	e := newEnv(t)
	os.Remove(filepath.Join(e.home, ".spawnpoint", "config.toml"))
	var repos []struct{ Name string }
	e.json("", &repos, "repos", "--json")
	if len(repos) != 3 {
		t.Fatalf("repos = %+v", repos)
	}
	cfg, err := os.ReadFile(filepath.Join(e.home, ".spawnpoint", "config.toml"))
	if err != nil || !strings.Contains(string(cfg), "scan_dirs = ['~/code']") {
		t.Fatalf("config = %s (%v)", cfg, err)
	}
}
