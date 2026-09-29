package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/spf13/cobra"

	"github.com/mihirgupta0900/spawnpoint/internal/config"
	"github.com/mihirgupta0900/spawnpoint/internal/ui"
	"github.com/mihirgupta0900/spawnpoint/internal/workspace"
)

func lightCleanupCmd() *cobra.Command {
	var (
		af           agentFlags
		names, types string
	)
	cmd := &cobra.Command{
		Use:   "light-cleanup",
		Short: "Free disk space: delete node_modules, .venv, build caches (keeps code)",
		Long: "Delete reinstallable directories (node_modules, .venv, .next, target, …) from\n" +
			"workspaces to free disk space. Code, branches and worktrees are untouched.",
		Example: "sp light-cleanup\nspawnpoint light-cleanup --no-input --json --workspaces feat-billing --artifact-types node_modules",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := ensureConfig(af)
			if err != nil {
				return err
			}
			return runLightCleanup(cfg, af, names, types)
		},
	}
	af.register(cmd, "never prompt; requires --workspaces")
	cmd.Flags().StringVar(&names, "workspaces", "", "comma-separated workspace names")
	cmd.Flags().StringVar(&types, "artifact-types", "", "comma-separated dir names to delete, e.g. node_modules,.venv (default: all found)")
	return cmd
}

type deletedArtifact struct {
	Path  string `json:"path"`
	Size  int64  `json:"size_bytes"`
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

type lightReport struct {
	Workspace string            `json:"workspace"`
	Artifacts []deletedArtifact `json:"artifacts"`
}

func runLightCleanup(cfg *config.Config, af agentFlags, names, types string) error {
	emptyJSON := func() error {
		if af.json {
			return emit(map[string]any{"freed_bytes": 0, "workspaces": []lightReport{}})
		}
		return nil
	}
	all := existingWorkspaces(cfg)
	if len(all) == 0 {
		ui.Warning("No workspaces found.")
		return emptyJSON()
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
	} else {
		labels := make([]string, len(all))
		byLabel := map[string]*workspace.Workspace{}
		for i, ws := range all {
			labels[i] = ws.Label()
			byLabel[labels[i]] = ws
		}
		picked, err := ui.Pick(ui.PickOptions{Title: "Select workspaces for light cleanup", Items: labels, Multi: true})
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
	}

	type found struct {
		ws   *workspace.Workspace
		arts []workspace.Artifact
	}
	var results []found
	ui.Spin("Measuring reinstallable directories…", func() {
		for _, ws := range selected {
			var arts []workspace.Artifact
			for _, wt := range ws.Worktrees {
				arts = append(arts, workspace.FindArtifacts(wt.Path)...)
			}
			if len(arts) > 0 {
				workspace.MeasureSizes(arts)
				results = append(results, found{ws, arts})
			}
		}
	})
	if len(results) == 0 {
		ui.Success("Nothing to clean: no reinstallable directories found.")
		return emptyJSON()
	}

	totals := map[string]int64{}
	for _, r := range results {
		for _, a := range r.arts {
			totals[a.Type] += a.Size
		}
	}

	chosen := map[string]bool{}
	if af.noInput {
		want := map[string]bool{}
		for _, t := range parseCSV(types) {
			want[t] = true
		}
		for t := range totals {
			if len(want) == 0 || want[t] {
				chosen[t] = true
			}
		}
	} else {
		var labels, values []string
		for _, at := range workspace.ArtifactTypes {
			if size, ok := totals[at.Name]; ok {
				labels = append(labels, fmt.Sprintf("%-14s %s  %s", at.Name, ui.Dim(at.Desc), ui.Accent(workspace.FormatBytes(size))))
				values = append(values, at.Name)
			}
		}
		picked, err := ui.Checklist("Which directories should go?", labels, values)
		if err != nil {
			return err
		}
		if len(picked) == 0 {
			ui.Println("Nothing selected.")
			return nil
		}
		for _, t := range picked {
			chosen[t] = true
		}
	}

	var total int64
	var plan []found
	for _, r := range results {
		var keep []workspace.Artifact
		for _, a := range r.arts {
			if chosen[a.Type] {
				keep = append(keep, a)
				total += a.Size
			}
		}
		if len(keep) > 0 {
			sort.SliceStable(keep, func(i, j int) bool { return keep[i].Size > keep[j].Size })
			plan = append(plan, found{r.ws, keep})
		}
	}
	if len(plan) == 0 {
		ui.Warning("No matching directories after filtering.")
		return emptyJSON()
	}

	ui.Blank()
	ui.Header("Plan · frees ~" + workspace.FormatBytes(total))
	for _, r := range plan {
		var sum int64
		for _, a := range r.arts {
			sum += a.Size
		}
		ui.Println("  " + ui.Bold(r.ws.Name+"/") + ui.Dim("  ~"+workspace.FormatBytes(sum)))
		for _, a := range r.arts {
			rel, _ := filepath.Rel(r.ws.Path, a.Path)
			ui.Println("    " + ui.Dim(rel) + "  " + workspace.FormatBytes(a.Size))
		}
	}
	ui.Blank()
	if !af.noInput {
		ok, err := ui.Confirm("Proceed with light cleanup?", true)
		if err != nil {
			return err
		}
		if !ok {
			ui.Println(ui.Dim("Aborted."))
			return nil
		}
	}

	var freed int64
	report := []lightReport{}
	for _, r := range plan {
		entry := lightReport{Workspace: r.ws.Name, Artifacts: []deletedArtifact{}}
		for _, a := range r.arts {
			rel, _ := filepath.Rel(r.ws.Path, a.Path)
			if err := os.RemoveAll(a.Path); err != nil {
				ui.Println("  " + ui.Bad("✗") + " " + rel + ": " + err.Error())
				entry.Artifacts = append(entry.Artifacts, deletedArtifact{a.Path, a.Size, false, err.Error()})
				continue
			}
			freed += a.Size
			ui.Println("  " + ui.Good("✓") + " " + rel + ui.Dim("  "+workspace.FormatBytes(a.Size)))
			entry.Artifacts = append(entry.Artifacts, deletedArtifact{Path: a.Path, Size: a.Size, OK: true})
		}
		report = append(report, entry)
	}
	ui.Done("Freed ~" + workspace.FormatBytes(freed) + ".")
	if af.json {
		return emit(struct {
			Freed      int64         `json:"freed_bytes"`
			Workspaces []lightReport `json:"workspaces"`
		}{freed, report})
	}
	return nil
}
