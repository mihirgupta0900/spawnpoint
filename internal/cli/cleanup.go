package cli

import (
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mihirgupta0900/spawnpoint/internal/config"
	"github.com/mihirgupta0900/spawnpoint/internal/ui"
	"github.com/mihirgupta0900/spawnpoint/internal/workspace"
)

const (
	branchesDelete = "Delete all branches"
	branchesKeep   = "Keep branches"
	branchesAsk    = "Ask per branch"
)

func cleanupCmd() *cobra.Command {
	var (
		af                           agentFlags
		names                        string
		deleteBranches, keepBranches bool
	)
	cmd := &cobra.Command{
		Use:     "cleanup",
		Aliases: []string{"rm"},
		Short:   "Remove workspaces (worktrees, folders and optionally branches)",
		Example: "sp cleanup\nspawnpoint cleanup --no-input --json --workspaces feat-billing --delete-branches",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := ensureConfig(af)
			if err != nil {
				return err
			}
			var pref string
			switch {
			case deleteBranches && keepBranches:
				return fail("Pass only one of --delete-branches / --keep-branches.")
			case deleteBranches:
				pref = branchesDelete
			case keepBranches:
				pref = branchesKeep
			}
			return runCleanup(cfg, af, names, pref)
		},
	}
	af.register(cmd, "never prompt; requires --workspaces and --delete-branches or --keep-branches")
	cmd.Flags().StringVar(&names, "workspaces", "", "comma-separated workspace names (see `spawnpoint list`)")
	cmd.Flags().BoolVar(&deleteBranches, "delete-branches", false, "also delete the branches from the parent repos")
	cmd.Flags().BoolVar(&keepBranches, "keep-branches", false, "keep the branches in the parent repos")
	return cmd
}

// existingWorkspaces scans all worktree dirs, oldest first.
func existingWorkspaces(cfg *config.Config) []*workspace.Workspace {
	var dirs []string
	for _, d := range cfg.WorktreeDirs() {
		if workspace.Exists(d) {
			dirs = append(dirs, d)
		}
	}
	all := workspace.ScanWorkspaces(dirs)
	workspace.SortByAge(all, false)
	return all
}

type removedJSON struct {
	Workspace string        `json:"workspace"`
	Worktrees []removedTree `json:"worktrees"`
}

type removedTree struct {
	Repo          string `json:"repo"`
	Branch        string `json:"branch"`
	BranchDeleted bool   `json:"branch_deleted"`
}

func runCleanup(cfg *config.Config, af agentFlags, names, pref string) error {
	all := existingWorkspaces(cfg)
	if len(all) == 0 {
		if af.json {
			return emit(map[string][]removedJSON{"removed": {}})
		}
		ui.Warning("No workspaces found.")
		return nil
	}

	var selected []*workspace.Workspace
	var err error
	if af.noInput {
		if err := requireFlag(names, "--workspaces"); err != nil {
			return err
		}
		if selected, err = resolveWorkspaces(parseCSV(names), all); err != nil {
			return err
		}
		if pref == "" {
			return fail("--no-input requires --delete-branches or --keep-branches.")
		}
	} else {
		labels := make([]string, len(all))
		byLabel := map[string]*workspace.Workspace{}
		var pre []string
		for i, ws := range all {
			labels[i] = ws.Label()
			byLabel[labels[i]] = ws
		}
		for _, n := range parseCSV(names) {
			for _, ws := range all {
				if ws.Name == n {
					pre = append(pre, ws.Label())
				}
			}
		}
		picked, err := ui.Pick(ui.PickOptions{Title: "Select workspaces to remove", Items: labels, Selected: pre, Multi: true})
		if err != nil {
			return err
		}
		if len(picked) == 0 {
			ui.Println("No workspaces selected.")
			return nil
		}
		for _, l := range picked {
			selected = append(selected, byLabel[l])
		}
		if pref == "" {
			if pref, err = ui.Select("Delete branches from parent repos?", []string{branchesDelete, branchesKeep, branchesAsk}, branchesDelete); err != nil {
				return err
			}
		}
	}

	ui.Blank()
	ui.Header("Plan")
	for _, ws := range selected {
		ui.Println("  " + ui.Bold(ws.Name+"/"))
		for _, wt := range ws.Worktrees {
			line := "    " + wt.ParentName() + ": remove worktree"
			if wt.Dirty {
				line += " " + ui.Warn("(dirty — will force)")
			}
			switch pref {
			case branchesDelete:
				line += ui.Dim(", delete branch " + wt.Branch)
			case branchesAsk:
				line += ui.Dim(", ask about " + wt.Branch)
			}
			ui.Println(line)
		}
	}
	ui.Blank()
	if !af.noInput {
		ok, err := ui.Confirm("Proceed with cleanup?", false)
		if err != nil {
			return err
		}
		if !ok {
			ui.Println(ui.Dim("Aborted."))
			return nil
		}
	}

	report := []removedJSON{}
	parents := map[string]bool{}
	for _, ws := range selected {
		ui.Println(ui.Bold(ws.Name))
		decisions := map[string]bool{}
		trees := []removedTree{}
		for _, wt := range ws.Worktrees {
			del := pref == branchesDelete
			if pref == branchesAsk {
				key := wt.Parent + ":" + wt.Branch
				if _, asked := decisions[key]; !asked {
					if decisions[key], err = ui.Confirm("Delete branch '"+wt.Branch+"' from "+wt.ParentName()+"?", true); err != nil {
						return err
					}
				}
				del = decisions[key]
			}
			deleted, notes := workspace.RemoveWorktree(wt, del)
			line := "  " + ui.Good("✓") + " removed " + wt.ParentName()
			if deleted {
				line += ui.Dim(" + branch " + wt.Branch)
			}
			ui.Println(line)
			for _, n := range notes {
				ui.Hint("%s", ui.Warn(n))
			}
			trees = append(trees, removedTree{wt.ParentName(), wt.Branch, deleted})
			if wt.Parent != "" && workspace.Exists(wt.Parent) {
				parents[wt.Parent] = true
			}
		}
		os.RemoveAll(ws.Path)
		report = append(report, removedJSON{ws.Name, trees})
	}

	keys := make([]string, 0, len(parents))
	for p := range parents {
		keys = append(keys, p)
	}
	sort.Strings(keys)
	for _, p := range keys {
		workspace.Git(p, "worktree", "prune")
	}

	names = ""
	for _, r := range report {
		names += r.Workspace + " "
	}
	ui.Done(ui.Dim("Removed " + strings.TrimSpace(names)))
	if af.json {
		return emit(map[string][]removedJSON{"removed": report})
	}
	return nil
}
