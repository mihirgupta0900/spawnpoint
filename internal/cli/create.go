package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mihirgupta0900/spawnpoint/internal/config"
	"github.com/mihirgupta0900/spawnpoint/internal/ui"
	"github.com/mihirgupta0900/spawnpoint/internal/workspace"
)

func createCmd() *cobra.Command {
	var (
		af                                          agentFlags
		yes                                         bool
		repos, branch, base, template, saveTemplate string
	)
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Pick repos and spawn a workspace for a branch",
		Long:  "Pick repos and spawn a workspace: one git worktree per repo on the same branch,\nwith env files copied and dependencies installed.",
		Example: strings.Join([]string{
			"sp create",
			"sp create -t billing",
			"spawnpoint create --no-input --json --repos api,web --branch feat/billing",
		}, "\n"),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := ensureConfig(af)
			if err != nil {
				return err
			}
			return runCreate(cfg, createOpts{af, yes, repos, branch, base, template, saveTemplate})
		},
	}
	f := cmd.Flags()
	f.BoolVarP(&yes, "yes", "y", false, "use each repo's default branch as the base without asking")
	af.register(cmd, "never prompt; requires --branch and --repos or --template")
	f.StringVar(&repos, "repos", "", "comma-separated repo names (see `spawnpoint repos`)")
	f.StringVarP(&branch, "branch", "b", "", "branch name")
	f.StringVar(&base, "base", "", "base branch for new branches (default: each repo's default branch)")
	f.StringVarP(&template, "template", "t", "", "start from a saved template's repos")
	f.StringVar(&saveTemplate, "save-template", "", "save the selected repos as a template with this name")
	return cmd
}

type createOpts struct {
	agentFlags
	yes                                         bool
	repos, branch, base, template, saveTemplate string
}

func runCreate(cfg *config.Config, o createOpts) error {
	// Validate a named template before the repo scan so typos fail fast.
	var active *config.Template
	if o.template != "" {
		t, err := getTemplate(o.template)
		if err != nil {
			return err
		}
		active = t
	}

	all, _, err := scanRepos(cfg)
	if err != nil {
		return err
	}
	if len(all) == 0 {
		ui.Warning("No git repositories found.")
		return nil
	}
	ui.Println(ui.Dim(fmt.Sprintf("Found %d repos in %s", len(all), strings.Join(tildeAll(cfg.ValidScanDirs()), ", "))))

	// Offer saved templates as a starting point when nothing was passed by flag.
	if active == nil && !o.noInput && o.repos == "" {
		templates, err := config.LoadTemplates()
		if err != nil {
			return fail("%v", err)
		}
		if len(templates) > 0 {
			if active, err = promptForTemplate(templates); err != nil {
				return err
			}
		}
	}

	if active != nil {
		ui.Println(ui.Accent("Template ") + ui.Bold(active.Name))
		if o.repos != "" {
			ui.Hint("--repos overrides its repo list")
		} else {
			ui.Hint("repos: %s", strings.Join(active.Repos, ", "))
		}
		if o.base == "" && active.Base != nil {
			o.base = *active.Base
			ui.Hint("base branch: %s", o.base)
		}
	}

	var selected []workspace.Repo
	if o.noInput {
		switch {
		case o.repos != "":
			// An explicit --repos wins over the template's repo list.
			selected, err = resolveRepos(parseCSV(o.repos), all, "")
		case active != nil:
			selected, err = resolveRepos(active.Repos, all, templateRepoHint(active.Name))
		default:
			return fail("--no-input requires --repos or --template.")
		}
		if err != nil {
			return err
		}
		if len(selected) == 0 {
			return fail("No repositories to spawn.")
		}
	} else {
		var pre []string
		if active != nil {
			pre = preselectTemplateRepos(active, all)
		} else if o.repos != "" {
			if selected, err = resolveRepos(parseCSV(o.repos), all, ""); err != nil {
				return err
			}
			pre = repoNames(selected)
		}
		labels, err := ui.Pick(ui.PickOptions{Title: "Select repositories", Items: repoNames(all), Selected: pre, Multi: true})
		if err != nil {
			return err
		}
		if len(labels) == 0 {
			ui.Println("No repositories selected.")
			return nil
		}
		selected = reposByName(all, labels)
	}

	branch := o.branch
	if o.noInput {
		if err := requireFlag(branch, "--branch"); err != nil {
			return err
		}
	} else if branch == "" {
		if branch, err = ui.Input("Branch name:", ""); err != nil {
			return err
		}
		if branch == "" {
			return fail("Branch name cannot be empty.")
		}
	}

	prepare(selected)

	if err := os.MkdirAll(cfg.WorktreeDir, 0o755); err != nil {
		return fail("%v", err)
	}
	root := resolveDir(cfg.WorktreeDir)
	dirName := strings.ReplaceAll(branch, "/", "-")
	single := len(selected) == 1
	actions, err := planActions(selected, planOpts{
		branch: branch, base: o.base, yes: o.yes, noInput: o.noInput,
		target: func(r workspace.Repo) string {
			if single {
				return filepath.Join(root, dirName)
			}
			return filepath.Join(root, dirName, r.Base())
		},
	})
	if err != nil {
		return err
	}
	if len(actions) == 0 {
		ui.Warning("Nothing to do.")
		return nil
	}

	printPlan(actions)
	if !o.noInput {
		ok, err := ui.Confirm("Proceed?", true)
		if err != nil {
			return err
		}
		if !ok {
			ui.Println(ui.Dim("Aborted."))
			return nil
		}
	}

	// Remember this repo set for next time.
	names := repoNames(selected)
	save := o.saveTemplate
	if save == "" && !o.noInput {
		if save, err = offerSaveTemplate(names, active); err != nil {
			return err
		}
	}
	if save != "" {
		t, err := config.MergeTemplate(save, names, o.base, "")
		if err != nil {
			return fail("%v", err)
		}
		path, existed, err := config.UpsertTemplate(t)
		if err != nil {
			return fail("%v", err)
		}
		verb := "Saved"
		if existed {
			verb = "Updated"
		}
		ui.Success("%s template '%s' %s", verb, save, ui.Dim("("+tilde(path)+")"))
		ui.Hint("Reuse it with: spawnpoint create -t %s", save)
	}

	build(actions, cfg, "created")

	wsPath := filepath.Join(root, dirName)
	if single {
		wsPath = actions[0].Target
	}
	ui.Done("Workspace: " + ui.Accent(tilde(wsPath)))
	writeCDPath(wsPath)

	if o.json {
		var tmpl *string
		if active != nil {
			tmpl = &active.Name
		}
		return emit(struct {
			Workspace string       `json:"workspace"`
			Branch    string       `json:"branch"`
			Template  *string      `json:"template"`
			Repos     []repoResult `json:"repos"`
		}{wsPath, branch, tmpl, results(actions)})
	}
	if o.noInput {
		// Plain stdout so agents can capture the workspace path via $(...).
		fmt.Println(wsPath)
	}
	return nil
}

// offerSaveTemplate offers to remember an interactively picked repo set.
// Skipped for a single repo and when the set still matches its template.
func offerSaveTemplate(names []string, active *config.Template) (string, error) {
	if len(names) < 2 {
		return "", nil
	}
	if active != nil && sameSet(names, active.Repos) {
		return "", nil
	}
	ok, err := ui.Confirm(fmt.Sprintf("Save these %d repos as a template for next time?", len(names)), false)
	if err != nil || !ok {
		return "", err
	}
	name, err := ui.Input("Template name:", "")
	if err != nil {
		return "", err
	}
	if name == "" {
		ui.Println(ui.Dim("No name given, template not saved."))
		return "", nil
	}
	templates, err := config.LoadTemplates()
	if err != nil {
		return "", fail("%v", err)
	}
	if _, exists := templates[name]; exists {
		ok, err := ui.Confirm(fmt.Sprintf("Template '%s' already exists. Overwrite it?", name), false)
		if err != nil {
			return "", err
		}
		if !ok {
			ui.Println(ui.Dim("Template not saved."))
			return "", nil
		}
	}
	return name, nil
}

func sameSet(a, b []string) bool {
	a, b = slices.Clone(a), slices.Clone(b)
	slices.Sort(a)
	slices.Sort(b)
	return slices.Equal(slices.Compact(a), slices.Compact(b))
}

func tildeAll(ps []string) []string {
	out := make([]string, len(ps))
	for i, p := range ps {
		out[i] = tilde(p)
	}
	return out
}

func resolveDir(p string) string {
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return p
}
