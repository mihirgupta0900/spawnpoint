package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mihirgupta0900/spawnpoint/internal/config"
	"github.com/mihirgupta0900/spawnpoint/internal/ui"
)

func templateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "template",
		Aliases: []string{"templates"},
		Short:   "Manage templates: saved sets of repos you spawn together",
		Example: "sp template save billing --repos api,web --base main\nsp create -t billing",
	}
	cmd.AddCommand(templateListCmd(), templateSaveCmd(), templateShowCmd(), templateDeleteCmd())
	return cmd
}

func getTemplate(name string) (*config.Template, error) {
	templates, err := config.LoadTemplates()
	if err != nil {
		return nil, fail("%v", err)
	}
	if t, ok := templates[name]; ok {
		return t, nil
	}
	ui.Error("template '%s' not found. Valid: %s", name, orNone(sortedKeys(templates)))
	ui.Hint("Create one with %s.", ui.Bold("spawnpoint template save <name>"))
	return nil, exit(1)
}

func promptForTemplate(templates map[string]*config.Template) (*config.Template, error) {
	const manual = "Pick repos manually"
	labels, values := []string{manual}, []string{""}
	for _, name := range sortedKeys(templates) {
		labels = append(labels, fmt.Sprintf("%s  %s", name, ui.Dim("("+templates[name].Detail()+")")))
		values = append(values, name)
	}
	name, err := ui.SelectLabeled("Start from a template?", labels, values, "")
	if err != nil || name == "" {
		return nil, err
	}
	return templates[name], nil
}

func strOr(p *string, def string) string {
	if p == nil {
		return def
	}
	return *p
}

func templateListCmd() *cobra.Command {
	var af agentFlags
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List saved templates",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			templates, err := config.LoadTemplates()
			if err != nil {
				return fail("%v", err)
			}
			if af.json {
				out := []*config.Template{}
				for _, name := range sortedKeys(templates) {
					out = append(out, templates[name])
				}
				return emit(out)
			}
			if len(templates) == 0 {
				ui.Warning("No templates saved yet.")
				ui.Hint("Create one with %s, or %s.", ui.Bold("spawnpoint template save <name>"), ui.Bold("spawnpoint create --save-template <name>"))
				return nil
			}
			var rows [][]string
			for _, name := range sortedKeys(templates) {
				t := templates[name]
				repos := strings.Join(t.Repos, ", ")
				if repos == "" {
					repos = ui.Dim("none")
				}
				rows = append(rows, []string{ui.Bold(name), repos, strOr(t.Base, ui.Dim("auto")), strOr(t.Description, "")})
			}
			ui.Table([]string{"Template", "Repos", "Base", "Description"}, rows)
			return nil
		},
	}
	af.register(cmd, "")
	return cmd
}

func templateShowCmd() *cobra.Command {
	var af agentFlags
	cmd := &cobra.Command{
		Use:   "show <name>",
		Short: "Show one template",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			t, err := getTemplate(args[0])
			if err != nil {
				return err
			}
			if af.json {
				return emit(t)
			}
			ui.Println(ui.Bold(t.Name))
			if t.Description != nil {
				ui.Println("  " + *t.Description)
			}
			ui.Println("  " + ui.Dim("Repos ") + orNone(t.Repos))
			ui.Println("  " + ui.Dim("Base  ") + strOr(t.Base, "auto-detected per repo"))
			return nil
		},
	}
	af.register(cmd, "")
	return cmd
}

func templateSaveCmd() *cobra.Command {
	var (
		af                       agentFlags
		repos, base, description string
	)
	cmd := &cobra.Command{
		Use:     "save <name>",
		Short:   "Create or update a template (omit --repos to pick interactively)",
		Example: "sp template save billing\nspawnpoint template save billing --no-input --repos api,web --base main",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := strings.TrimSpace(args[0])
			if name == "" {
				return fail("Template name cannot be empty.")
			}
			cfg, err := ensureConfig(af)
			if err != nil {
				return err
			}
			all, _, err := scanRepos(cfg)
			if err != nil {
				return err
			}
			if len(all) == 0 {
				return fail("No git repositories found.")
			}
			if af.noInput {
				if err := requireFlag(repos, "--repos"); err != nil {
					return err
				}
			}

			var selected []string
			if repos != "" {
				picked, err := resolveRepos(parseCSV(repos), all, "")
				if err != nil {
					return err
				}
				selected = repoNames(picked)
			} else {
				templates, err := config.LoadTemplates()
				if err != nil {
					return fail("%v", err)
				}
				var pre []string
				if existing := templates[name]; existing != nil {
					pre = preselectTemplateRepos(existing, all)
				}
				selected, err = ui.Pick(ui.PickOptions{
					Title: fmt.Sprintf("Select repositories for template '%s'", name), Items: repoNames(all), Selected: pre, Multi: true,
				})
				if err != nil {
					return err
				}
				if len(selected) == 0 {
					ui.Println("No repositories selected. Nothing saved.")
					return nil
				}
			}
			if len(selected) == 0 {
				return fail("A template needs at least one repo.")
			}

			t, err := config.MergeTemplate(name, selected, base, description)
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
			ui.Success("%s template '%s' %s", verb, name, ui.Dim("("+strings.Join(t.Repos, ", ")+")"))
			ui.Hint("%s", tilde(path))
			ui.Hint("Use it with %s", ui.Bold("spawnpoint create --template "+name))
			if af.json {
				return emit(t)
			}
			return nil
		},
	}
	af.register(cmd, "never prompt; requires --repos")
	cmd.Flags().StringVar(&repos, "repos", "", "comma-separated repo names (skips the picker)")
	cmd.Flags().StringVar(&base, "base", "", "default base branch for this template")
	cmd.Flags().StringVarP(&description, "description", "d", "", "short description shown in listings")
	return cmd
}

func templateDeleteCmd() *cobra.Command {
	var af agentFlags
	cmd := &cobra.Command{
		Use:     "delete <name>",
		Aliases: []string{"rm"},
		Short:   "Delete a template (workspaces are untouched)",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			t, err := getTemplate(name)
			if err != nil {
				return err
			}
			if !af.noInput {
				ok, err := ui.Confirm(fmt.Sprintf("Delete template '%s' (%s)?", name, strings.Join(t.Repos, ", ")), false)
				if err != nil {
					return err
				}
				if !ok {
					ui.Println(ui.Dim("Aborted."))
					return nil
				}
			}
			templates, err := config.LoadTemplates()
			if err != nil {
				return fail("%v", err)
			}
			delete(templates, name)
			if _, err := config.SaveTemplates(templates); err != nil {
				return fail("%v", err)
			}
			ui.Success("Deleted template '%s'.", name)
			ui.Hint("Existing workspaces are untouched.")
			if af.json {
				return emit(map[string]string{"deleted": name})
			}
			return nil
		},
	}
	af.register(cmd, "skip the confirmation prompt")
	return cmd
}
