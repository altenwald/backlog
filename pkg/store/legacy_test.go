package store_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/altenwald/backlog/pkg/model"
	"github.com/altenwald/backlog/pkg/store"
)

const legacyProjectJSON = `{
  "slug": "legacy",
  "name": "Legacy Project",
  "description": "from JSON",
  "specification": "# Spec\n\n## Scope\nEverything.",
  "created_at": "2025-01-02T03:04:05Z",
  "updated_at": "2025-02-02T03:04:05Z",
  "tasks": [
    {"id": "3", "parent_id": "1", "title": "Child before parent", "description": "", "size": "S", "tier": 2, "done": false, "created_at": "2025-01-03T00:00:00Z", "updated_at": "2025-01-03T00:00:00Z"},
    {"id": "1", "title": "Root", "description": "Árbol con tildes", "size": "L", "tier": 1, "done": true, "resolution": "done in abc123", "created_at": "2025-01-02T00:00:00Z", "updated_at": "2025-01-02T00:00:00Z", "done_at": "2025-01-05T00:00:00Z"},
    {"id": "7", "title": "Depends", "description": "", "size": "M", "tier": 3, "done": false, "depends_on": ["3", "99", "3"], "assignee": "claude", "inserted_at": "2025-01-04T00:00:00Z", "updated_at": "2025-01-04T00:00:00Z"}
  ]
}`

func writeLegacy(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, "projects"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "projects", "legacy.json"), []byte(legacyProjectJSON), 0644); err != nil {
		t.Fatal(err)
	}
	cfg := `{"active_project": "legacy", "mcp_user_instructions": "be nice"}`
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(cfg), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestImportLegacyJSON(t *testing.T) {
	dir := t.TempDir()
	writeLegacy(t, dir)

	st, err := store.NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	if got := st.GetActiveProjectSlug(); got != "legacy" {
		t.Fatalf("active project = %q, want legacy", got)
	}
	if got := st.GetMCPUserInstructions(); got != "be nice" {
		t.Fatalf("MCP instructions = %q", got)
	}

	p, err := st.GetProject("legacy")
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "Legacy Project" || p.InsertedAt.Year() != 2025 {
		t.Fatalf("unexpected project: %+v", p)
	}
	sections, err := st.ListSpecSections("legacy")
	if err != nil || len(sections) != 2 || sections[0].ID != "main" || sections[1].ID != "scope" {
		t.Fatalf("legacy spec not split into sections: %v %+v", err, sections)
	}
	if len(p.Tasks) != 3 || p.Tasks[0].ID != "3" || p.Tasks[1].ID != "1" {
		t.Fatalf("tasks not imported in file order: %+v", p.Tasks)
	}

	root := p.Tasks[1]
	if !root.Done || root.TerminatedAt == nil || root.TerminatedAt.Day() != 5 {
		t.Fatalf("legacy done_at not migrated: %+v", root)
	}
	if p.Tasks[0].ParentID != "1" {
		t.Fatalf("parent lost: %+v", p.Tasks[0])
	}
	if deps := p.Tasks[2].DependsOn; len(deps) != 1 || deps[0] != "3" {
		t.Fatalf("expected dangling and duplicate deps dropped, got %v", deps)
	}

	// Unicode search keeps working (SQLite's lower() is ASCII-only).
	res, err := st.ListTasks("legacy", model.TaskFilter{Search: "árbol"})
	if err != nil || len(res) != 1 || res[0].ID != "1" {
		t.Fatalf("search: %v %+v", err, res)
	}

	// New IDs continue after the highest imported one.
	nt, err := st.AddTask("legacy", model.Task{Title: "new"})
	if err != nil || nt.ID != "8" {
		t.Fatalf("AddTask after import: %v %+v", err, nt)
	}

	// JSON files are moved aside, not deleted.
	if _, err := os.Stat(filepath.Join(dir, "projects")); !os.IsNotExist(err) {
		t.Fatalf("projects dir should have been moved, stat err = %v", err)
	}
	backups, _ := filepath.Glob(filepath.Join(dir, "json-backup-*", "projects", "legacy.json"))
	if len(backups) != 1 {
		t.Fatalf("expected JSON backup, found %v", backups)
	}

	// Reopening does not import again.
	_ = st.Close()
	st2, err := store.NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	tasks, _ := st2.ListTasks("legacy", model.TaskFilter{})
	if len(tasks) != 4 {
		t.Fatalf("expected 4 tasks after reopen, got %d", len(tasks))
	}
}

func TestImportLegacyCorruptLeavesFilesUntouched(t *testing.T) {
	dir := t.TempDir()
	writeLegacy(t, dir)
	broken := filepath.Join(dir, "projects", "broken.json")
	if err := os.WriteFile(broken, []byte(`{"slug": "broken", "tasks": [`), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := store.NewStore(dir)
	if err == nil || !strings.Contains(err.Error(), "broken.json") {
		t.Fatalf("expected error naming broken.json, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "backlog.db")); !os.IsNotExist(err) {
		t.Fatal("backlog.db must not exist after a failed import")
	}
	if _, err := os.Stat(filepath.Join(dir, "projects", "legacy.json")); err != nil {
		t.Fatal("legacy JSON must stay in place after a failed import")
	}
}

func TestDeleteTaskDoesNotReuseID(t *testing.T) {
	st, err := store.NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateProject("ids", "", ""); err != nil {
		t.Fatal(err)
	}
	a, _ := st.AddTask("ids", model.Task{Title: "a"})
	b, _ := st.AddTask("ids", model.Task{Title: "b", DependsOn: []string{a.ID}})
	if err := st.DeleteTask("ids", b.ID); err != nil {
		t.Fatal(err)
	}
	c, err := st.AddTask("ids", model.Task{Title: "c"})
	if err != nil {
		t.Fatal(err)
	}
	if c.ID == b.ID {
		t.Fatalf("ID %s was reused after delete", c.ID)
	}
	if err := st.DeleteTask("ids", a.ID); err != nil {
		t.Fatal(err)
	}
}
