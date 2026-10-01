package store_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/altenwald/backlog/pkg/model"
	"github.com/altenwald/backlog/pkg/store"
)

func TestBackupExportImportRoundTrip(t *testing.T) {
	dir := t.TempDir()
	st, err := store.NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateProject("round", "Round Trip", "desc"); err != nil {
		t.Fatal(err)
	}
	a, _ := st.AddTask("round", model.Task{Title: "first", Tier: model.Tier1})
	if _, err := st.AddTask("round", model.Task{Title: "second", ParentID: a.ID, DependsOn: []string{a.ID}}); err != nil {
		t.Fatal(err)
	}
	if err := st.UpdateProjectSpecification("round", "## Goals\nShip it 🚀"); err != nil {
		t.Fatal(err)
	}

	// Backup opens as a working store.
	backupDir := t.TempDir()
	if err := st.Backup(filepath.Join(backupDir, "backlog.db")); err != nil {
		t.Fatal(err)
	}
	restored, err := store.NewStore(backupDir)
	if err != nil {
		t.Fatal(err)
	}
	if tasks, _ := restored.ListTasks("round", model.TaskFilter{}); len(tasks) != 2 {
		t.Fatalf("backup has %d tasks, want 2", len(tasks))
	}

	// Daily backup is written once.
	first, err := st.AutoBackup(3)
	if err != nil || first == "" {
		t.Fatalf("AutoBackup: %q %v", first, err)
	}
	if again, _ := st.AutoBackup(3); again != "" {
		t.Fatalf("second AutoBackup on the same day wrote %q", again)
	}

	md, err := st.ExportProjectMarkdown("round")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"# Round Trip", "Ship it 🚀", "#2 second", "depends on #1"} {
		if !strings.Contains(md, want) {
			t.Fatalf("markdown export missing %q:\n%s", want, md)
		}
	}

	data, err := st.ExportProjectJSON("round")
	if err != nil {
		t.Fatal(err)
	}
	other, err := store.NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := other.ImportProjectJSON(data); err != nil {
		t.Fatal(err)
	}
	p, err := other.GetProject("round")
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Tasks) != 2 || p.Tasks[1].ParentID != a.ID || len(p.Tasks[1].DependsOn) != 1 {
		t.Fatalf("import lost structure: %+v", p.Tasks)
	}
	if _, err := other.ImportProjectJSON(data); err == nil {
		t.Fatal("expected error importing an existing slug")
	}
}
