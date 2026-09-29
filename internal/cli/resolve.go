package cli

import (
	"strings"

	"github.com/mihirgupta0900/spawnpoint/internal/config"
	"github.com/mihirgupta0900/spawnpoint/internal/ui"
	"github.com/mihirgupta0900/spawnpoint/internal/workspace"
)

// resolveRepos maps requested names onto repos, by display name or bare
// directory name. Unknown or ambiguous names exit 1, listing valid choices so
// an agent can correct its call. hint is an extra line printed on failure.
func resolveRepos(requested []string, repos []workspace.Repo, hint string) ([]workspace.Repo, error) {
	byName := map[string]workspace.Repo{}
	byBase := map[string][]string{}
	for _, r := range repos {
		byName[r.Name] = r
		byBase[r.Base()] = append(byBase[r.Base()], r.Name)
	}
	var out []workspace.Repo
	for _, name := range requested {
		if r, ok := byName[name]; ok {
			out = append(out, r)
			continue
		}
		if matches, ok := byBase[name]; ok {
			if len(matches) > 1 {
				ui.Error("repo '%s' is ambiguous; matches: %s. Use the full name.", name, strings.Join(matches, ", "))
				if hint != "" {
					ui.Println(hint)
				}
				return nil, exit(1)
			}
			out = append(out, byName[matches[0]])
			continue
		}
		ui.Error("repo '%s' not found. Valid: %s", name, orNone(sortedKeys(byName)))
		if hint != "" {
			ui.Println(hint)
		}
		return nil, exit(1)
	}
	return out, nil
}

// resolveWorkspaces maps names onto scanned workspaces.
func resolveWorkspaces(requested []string, all []*workspace.Workspace) ([]*workspace.Workspace, error) {
	byName := map[string]*workspace.Workspace{}
	for _, ws := range all {
		byName[ws.Name] = ws
	}
	var out []*workspace.Workspace
	for _, name := range requested {
		ws, ok := byName[name]
		if !ok {
			return nil, fail("workspace '%s' not found. Valid: %s", name, orNone(sortedKeys(byName)))
		}
		out = append(out, ws)
	}
	return out, nil
}

// scanRepos returns repos under the valid scan dirs, or exits explaining why
// there are none.
func scanRepos(cfg *config.Config) ([]workspace.Repo, []string, error) {
	valid := cfg.ValidScanDirs()
	if len(cfg.ScanDirs) == 0 {
		ui.Error("No scan directories configured.")
		ui.Hint("Run %s to set up.", ui.Bold("spawnpoint init"))
		return nil, nil, exit(1)
	}
	if len(valid) == 0 {
		ui.Error("None of your scan directories exist:")
		for _, d := range cfg.ScanDirs {
			ui.Hint("%s", d)
		}
		return nil, nil, exit(1)
	}
	return workspace.FindRepos(valid, cfg.ScanDepth), valid, nil
}

// templateRepoHint explains a bad repo name that came from a template.
func templateRepoHint(name string) string {
	return "Those repo names came from template '" + name + "'. Update it with " +
		ui.Bold("spawnpoint template save "+name+" --repos ...") + "."
}

// preselectTemplateRepos maps a template's repo names onto picker labels,
// warning about (and skipping) repos that no longer exist.
func preselectTemplateRepos(t *config.Template, repos []workspace.Repo) []string {
	label := map[string]string{}
	for _, r := range repos {
		label[r.Name] = r.Name
	}
	for _, r := range repos {
		if _, ok := label[r.Base()]; !ok {
			label[r.Base()] = r.Name
		}
	}
	var labels, missing []string
	seen := map[string]bool{}
	for _, name := range t.Repos {
		l, ok := label[name]
		switch {
		case !ok:
			missing = append(missing, name)
		case !seen[l]:
			seen[l] = true
			labels = append(labels, l)
		}
	}
	if len(missing) > 0 {
		ui.Warning("Template '%s' lists repos that are no longer available: %s", t.Name, strings.Join(missing, ", "))
	}
	return labels
}

func repoNames(repos []workspace.Repo) []string {
	out := make([]string, len(repos))
	for i, r := range repos {
		out[i] = r.Name
	}
	return out
}

func reposByName(repos []workspace.Repo, names []string) []workspace.Repo {
	byName := map[string]workspace.Repo{}
	for _, r := range repos {
		byName[r.Name] = r
	}
	out := make([]workspace.Repo, 0, len(names))
	for _, n := range names {
		out = append(out, byName[n])
	}
	return out
}
