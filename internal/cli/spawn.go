package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/mihirgupta0900/spawnpoint/internal/config"
	"github.com/mihirgupta0900/spawnpoint/internal/ui"
	"github.com/mihirgupta0900/spawnpoint/internal/workspace"
)

// Shared by create and add: plan a worktree per repo, show the plan, then
// build them.

type planOpts struct {
	branch  string
	base    string // explicit --base (or a template's base): applies to every repo
	yes     bool   // take the detected default base without asking
	noInput bool
	target  func(workspace.Repo) string
}

func prepare(repos []workspace.Repo) {
	noun := "repos"
	if len(repos) == 1 {
		noun = "repo"
	}
	ui.Spin(fmt.Sprintf("Fetching %d %s…", len(repos), noun), func() { workspace.PrepareAll(repos) })
	ui.Success("Fetched %s", strings.Join(repoNames(repos), ", "))
}

func planActions(repos []workspace.Repo, o planOpts) ([]*workspace.Action, error) {
	var actions, creates []*workspace.Action
	for _, repo := range repos {
		target := o.target(repo)
		if workspace.Exists(target) {
			ui.Warning("Skipping %s: %s already exists.", repo.Base(), tilde(target))
			continue
		}
		a := &workspace.Action{Repo: repo, Target: target, Branch: o.branch, Type: "add"}
		if !workspace.BranchExists(repo.Path, o.branch) {
			a.Type = "create"
			creates = append(creates, a)
		}
		actions = append(actions, a)
	}
	if len(creates) == 0 {
		return actions, nil
	}
	if o.base != "" {
		for _, a := range creates {
			a.Base = o.base
		}
		return actions, nil
	}

	detected := make([]string, len(creates))
	for i, a := range creates {
		detected[i] = workspace.DetectDefaultBranch(a.Repo.Path)
	}

	// Interactively, ask once when every new branch would start from the
	// same default instead of repeating the question per repo.
	if !o.noInput && !o.yes && len(creates) > 1 && allSame(detected) && detected[0] != "" {
		const other = "Other (manual input)"
		names := make([]string, len(creates))
		for i, a := range creates {
			names[i] = a.Repo.Base()
		}
		title := fmt.Sprintf("'%s' is new in %s. Create it from:", o.branch, strings.Join(names, ", "))
		choice, err := ui.Select(title, []string{detected[0], other}, detected[0])
		if err != nil {
			return nil, err
		}
		if choice == other {
			if choice, err = ui.Input("Base branch:", ""); err != nil {
				return nil, err
			}
		}
		for _, a := range creates {
			a.Base = choice
		}
		return actions, nil
	}

	for i, a := range creates {
		base, err := chooseBase(a.Repo, detected[i], o)
		if err != nil {
			return nil, err
		}
		a.Base = base
	}
	return actions, nil
}

func allSame(s []string) bool {
	for _, v := range s[1:] {
		if v != s[0] {
			return false
		}
	}
	return true
}

func chooseBase(repo workspace.Repo, detected string, o planOpts) (string, error) {
	switch {
	case o.noInput:
		if detected == "" {
			return "", fail("[%s] branch '%s' not found and no default branch detected. Pass --base.", repo.Base(), o.branch)
		}
		return detected, nil
	case o.yes && detected != "":
		return detected, nil
	case detected != "":
		const other = "Other (manual input)"
		choice, err := ui.Select(fmt.Sprintf("[%s] Branch '%s' not found. Create from:", repo.Base(), o.branch), []string{detected, other}, detected)
		if err != nil || choice != other {
			return choice, err
		}
		return ui.Input(fmt.Sprintf("[%s] Enter base branch:", repo.Base()), "")
	default:
		return ui.Input(fmt.Sprintf("[%s] Branch '%s' not found. Enter base branch:", repo.Base(), o.branch), "")
	}
}

func printPlan(actions []*workspace.Action) {
	width := 0
	for _, a := range actions {
		width = max(width, len(a.Repo.Base()))
	}
	ui.Blank()
	ui.Header("Plan")
	for _, a := range actions {
		name := ui.Bold(fmt.Sprintf("%-*s", width, a.Repo.Base()))
		if a.Type == "add" {
			ui.Println("  " + name + "  " + ui.Good("add worktree") + " for " + ui.Accent(a.Branch))
		} else {
			ui.Println("  " + name + "  " + ui.Accent2("create branch") + " " + ui.Accent(a.Branch) + " from " + ui.Accent(a.Base))
		}
	}
	ui.Blank()
}

// build creates each worktree, then copies config files and installs deps.
func build(actions []*workspace.Action, cfg *config.Config, okStatus string) {
	ui.Blank()
	for _, a := range actions {
		a.Status = "failed"
		ui.Println(ui.Bold(a.Repo.Base()) + " " + ui.Dim(tilde(a.Target)))
		if err := workspace.AddWorktree(a); err != nil {
			ui.Hint("%s", ui.Bad("✗ "+err.Error()))
			continue
		}
		a.Status = okStatus
		if a.Type == "add" {
			ui.Println("  " + ui.Good("✓") + " worktree on " + ui.Accent(a.Branch))
		} else {
			ui.Println("  " + ui.Good("✓") + " worktree on " + ui.Accent(a.Branch) + ui.Dim(" (from "+a.Base+")"))
		}
		if copied := workspace.CopyEssentialFiles(a.Repo.Path, a.Target, cfg); len(copied) > 0 {
			ui.Println("  " + ui.Good("✓") + " copied " + ui.Dim(strings.Join(copied, ", ")))
		}
		if cfg.AutoInstallDeps {
			for _, step := range workspace.SetupDependencies(a.Target) {
				ui.Println("  " + ui.Good("✓") + " " + step)
			}
		}
	}
}

type repoResult struct {
	Name   string  `json:"name"`
	Branch string  `json:"branch"`
	Base   *string `json:"base"`
	Action string  `json:"action"`
	Status string  `json:"status"`
}

func results(actions []*workspace.Action) []repoResult {
	out := make([]repoResult, len(actions))
	for i, a := range actions {
		r := repoResult{Name: a.Repo.Base(), Branch: a.Branch, Action: a.Type, Status: a.Status}
		if a.Type == "create" {
			base := a.Base
			r.Base = &base
		}
		out[i] = r
	}
	return out
}

func writeCDPath(path string) {
	_ = os.MkdirAll(config.Dir(), 0o755)
	_ = os.WriteFile(config.CDPathFile(), []byte(path), 0o644)
}
