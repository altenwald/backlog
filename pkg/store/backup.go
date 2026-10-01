package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/altenwald/backlog/pkg/model"
)

const backupsDirName = "backups"

// Backup writes a consistent copy of the database to dest. It is safe to
// call while the store is in use.
func (s *Store) Backup(dest string) error {
	if _, err := os.Stat(dest); err == nil {
		return fmt.Errorf("%s already exists", dest)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return err
	}
	_, err := s.db.Exec(`VACUUM INTO ?`, dest)
	return err
}

// AutoBackup writes backups/backlog-<date>.db once per day and keeps the
// newest keep files. It returns the path written, or "" if today's backup
// already exists.
func (s *Store) AutoBackup(keep int) (string, error) {
	dir := filepath.Join(s.dataDir, backupsDirName)
	dest := filepath.Join(dir, "backlog-"+time.Now().Format("2006-01-02")+".db")
	if _, err := os.Stat(dest); err == nil {
		return "", nil
	}
	if err := s.Backup(dest); err != nil {
		return "", err
	}

	matches, err := filepath.Glob(filepath.Join(dir, "backlog-*.db"))
	if err != nil {
		return dest, err
	}
	sort.Strings(matches) // date names sort chronologically
	for len(matches) > keep && keep > 0 {
		_ = os.Remove(matches[0])
		matches = matches[1:]
	}
	return dest, nil
}

// ExportProjectJSON returns the project in the same JSON layout the JSON
// store used, so it can be read back with ImportProjectJSON.
func (s *Store) ExportProjectJSON(slug string) ([]byte, error) {
	p, err := s.getFullProject(slug)
	if err != nil {
		return nil, err
	}
	return json.MarshalIndent(p, "", "  ")
}

// ExportProjectMarkdown renders the specification and the task list as a
// readable Markdown document.
func (s *Store) ExportProjectMarkdown(slug string) (string, error) {
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

// ImportProjectJSON adds a project exported with ExportProjectJSON (or a
// project file from the old JSON store). It fails if the slug exists.
func (s *Store) ImportProjectJSON(data []byte) (*model.Project, error) {
	var p model.Project
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("invalid project JSON: %w", err)
	}
	p.Slug = strings.ToLower(strings.TrimSpace(p.Slug))
	if p.Slug == "" {
		return nil, errors.New("project JSON has no slug")
	}

	err := s.withTx(func(tx *sql.Tx) error {
		if projectExists(tx, p.Slug) == nil {
			return fmt.Errorf("project '%s' already exists", p.Slug)
		}
		return importProject(tx, &p)
	})
	if err != nil {
		return nil, err
	}

	go s.notify(Event{
		Type:        EventProjectCreated,
		ProjectSlug: p.Slug,
	})
	return &p, nil
}

// getFullProject returns a project with its tasks and spec sections.
func (s *Store) getFullProject(slug string) (*model.Project, error) {
	p, err := s.GetProject(slug)
	if err != nil {
		return nil, err
	}
	if p.Spec, err = loadSections(s.db, p.Slug); err != nil {
		return nil, err
	}
	return p, nil
}
