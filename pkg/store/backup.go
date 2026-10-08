package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/altenwald/backlog/pkg/model"
)

const backupsDirName = "backups"

// Backup writes a consistent copy of the database to dest. It is safe to
// call while the store is in use.
func (s *projectDB) Backup(dest string) error {
	if _, err := os.Stat(dest); err == nil {
		return fmt.Errorf("%s already exists", dest)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return err
	}
	_, err := s.db.Exec(`VACUUM INTO ?`, dest)
	return err
}

// ExportProjectJSON returns the project in the same JSON layout the JSON
// store used, so it can be read back with ImportProjectJSON.
func (s *projectDB) ExportProjectJSON(slug string) ([]byte, error) {
	p, err := s.getFullProject(slug)
	if err != nil {
		return nil, err
	}
	return json.MarshalIndent(p, "", "  ")
}

// ExportProjectMarkdown renders the specification and the task list as a
// readable Markdown document.
func (s *projectDB) ExportProjectMarkdown(slug string) (string, error) {
	p, err := s.getFullProject(slug)
	if err != nil {
		return "", err
	}

	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", p.Name)
	if p.Description != "" {
		fmt.Fprintf(&b, "%s\n\n", p.Description)
	}
	if spec := model.JoinSpec(p.Spec); spec != "" {
		fmt.Fprintf(&b, "%s\n\n", spec)
	}

	writeTasks := func(title string, done bool) {
		var tasks []model.Task
		for _, t := range p.Tasks {
			if t.Done == done {
				tasks = append(tasks, t)
			}
		}
		if len(tasks) == 0 {
			return
		}
		fmt.Fprintf(&b, "## %s\n\n", title)
		for _, t := range tasks {
			check := " "
			if t.Done {
				check = "x"
			}
			fmt.Fprintf(&b, "### [%s] #%s %s\n\n", check, t.ID, t.Title)
			meta := []string{t.Tier.ShortLabel(), string(t.Size)}
			if t.ParentID != "" {
				meta = append(meta, "parent #"+t.ParentID)
			}
			if len(t.DependsOn) > 0 {
				meta = append(meta, "depends on #"+strings.Join(t.DependsOn, ", #"))
			}
			if t.Assignee != "" {
				meta = append(meta, "@"+strings.TrimPrefix(t.Assignee, "@"))
			}
			if t.Deprecated {
				meta = append(meta, "deprecated")
			}
			fmt.Fprintf(&b, "_%s_\n\n", strings.Join(meta, " · "))
			if d := strings.TrimSpace(t.Description); d != "" {
				fmt.Fprintf(&b, "%s\n\n", d)
			}
			if r := strings.TrimSpace(t.Resolution); r != "" {
				fmt.Fprintf(&b, "**Resolution:** %s\n\n", r)
			}
		}
	}
	writeTasks("Open tasks", false)
	writeTasks("Completed tasks", true)
	return b.String(), nil
}

// getFullProject reads tasks and specification from one consistent snapshot.
func (s *projectDB) getFullProject(slug string) (*model.Project, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	return readFullProject(tx, s.resolveSlug(tx, slug))
}
