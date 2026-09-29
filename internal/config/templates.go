package config

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

// Template is a named set of repos to spawn together, plus an optional
// default base branch.
//
// Templates live in their own file (~/.spawnpoint/templates.toml) because the
// CLI writes them, while config.toml stays a hand-edited settings file that
// `init` and `config --reset` regenerate.
type Template struct {
	Name        string   `json:"name"`
	Repos       []string `json:"repos"`
	Base        *string  `json:"base"`
	Description *string  `json:"description"`
}

// LoadTemplates returns templates keyed by name (empty if none are saved).
func LoadTemplates() (map[string]*Template, error) {
	out := map[string]*Template{}
	data, err := os.ReadFile(TemplatesPath())
	if os.IsNotExist(err) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}

	var raw struct {
		Templates map[string]map[string]any `toml:"templates"`
	}
	if _, err := toml.Decode(string(data), &raw); err != nil {
		return nil, fmt.Errorf("%s is not valid TOML: %w\nFix the file by hand, or delete it to start over", TemplatesPath(), err)
	}
	for name, body := range raw.Templates {
		t := &Template{Name: name, Repos: []string{}}
		if repos, ok := body["repos"].([]any); ok {
			for _, r := range repos {
				t.Repos = append(t.Repos, fmt.Sprint(r))
			}
		}
		if s, ok := body["base"].(string); ok && s != "" {
			t.Base = &s
		}
		if s, ok := body["description"].(string); ok && s != "" {
			t.Description = &s
		}
		out[name] = t
	}
	return out, nil
}

// SaveTemplates writes all templates, sorted by name.
func SaveTemplates(templates map[string]*Template) (string, error) {
	if err := os.MkdirAll(Dir(), 0o755); err != nil {
		return "", err
	}
	lines := []string{
		"# Spawnpoint workspace templates",
		"# Managed by `spawnpoint template save` and `spawnpoint template delete`.",
		"# Repo names must match the names shown by `spawnpoint repos`.",
		"",
	}
	for _, name := range SortedNames(templates) {
		t := templates[name]
		lines = append(lines, "[templates."+BasicString(name)+"]")
		if t.Description != nil {
			lines = append(lines, "description = "+BasicString(*t.Description))
		}
		quoted := make([]string, len(t.Repos))
		for i, r := range t.Repos {
			quoted[i] = BasicString(r)
		}
		lines = append(lines, "repos = ["+strings.Join(quoted, ", ")+"]")
		if t.Base != nil {
			lines = append(lines, "base = "+BasicString(*t.Base))
		}
		lines = append(lines, "")
	}
	return TemplatesPath(), os.WriteFile(TemplatesPath(), []byte(strings.Join(lines, "\n")), 0o644)
}

// UpsertTemplate saves t and reports whether it replaced an existing template.
func UpsertTemplate(t *Template) (path string, existed bool, err error) {
	templates, err := LoadTemplates()
	if err != nil {
		return "", false, err
	}
	_, existed = templates[t.Name]
	templates[t.Name] = t
	path, err = SaveTemplates(templates)
	return path, existed, err
}

// MergeTemplate builds a template, keeping the saved base/description when
// the caller doesn't override them.
func MergeTemplate(name string, repos []string, base, description string) (*Template, error) {
	templates, err := LoadTemplates()
	if err != nil {
		return nil, err
	}
	t := &Template{Name: name, Repos: repos}
	existing := templates[name]
	switch {
	case base != "":
		t.Base = &base
	case existing != nil:
		t.Base = existing.Base
	}
	switch {
	case description != "":
		t.Description = &description
	case existing != nil:
		t.Description = existing.Description
	}
	return t, nil
}

// Detail is a one-line summary for pickers and tables.
func (t *Template) Detail() string {
	const max = 60
	detail := strings.Join(t.Repos, ", ")
	if t.Description != nil {
		detail = *t.Description
	}
	if len(detail) > max {
		detail = detail[:max-3] + "..."
	}
	return detail
}

func SortedNames[V any](m map[string]V) []string {
	names := make([]string, 0, len(m))
	for n := range m {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
