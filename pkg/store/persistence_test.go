package store_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/altenwald/backlog/pkg/model"
	"github.com/altenwald/backlog/pkg/store"
)

func TestCorruptProjectFileFailsLoudly(t *testing.T) {
	tmpDir := t.TempDir()
	projectsDir := filepath.Join(tmpDir, "projects")
	if err := os.MkdirAll(projectsDir, 0755); err != nil {
		t.Fatal(err)
	}
	broken := filepath.Join(projectsDir, "broken.json")
	content := []byte(`{"slug": "broken", "tasks": [`)
	if err := os.WriteFile(broken, content, 0644); err != nil {
		t.Fatal(err)
	}

	_, err := store.NewStore(tmpDir)
	if err == nil {
		t.Fatal("expected error loading a truncated project file, got nil")
	}
	if !strings.Contains(err.Error(), "broken.json") {
		t.Fatalf("expected error to name the file, got: %v", err)
	}

	got, _ := os.ReadFile(broken)
	if string(got) != string(content) {
		t.Fatal("corrupt file must be left untouched")
	}
}

func TestCorruptConfigFailsLoudly(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "config.json"), []byte(`{"active_project":`), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := store.NewStore(tmpDir); err == nil {
		t.Fatal("expected error loading a truncated config file, got nil")
	}
}

func TestReloadKeepsTasks(t *testing.T) {
	tmpDir := t.TempDir()
	st, err := store.NewStore(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateProject("persist", "Persist", ""); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		if _, err := st.AddTask("persist", model.Task{Title: "task"}); err != nil {
			t.Fatal(err)
		}
	}

	_ = st.Close()

	reloaded, err := store.NewStore(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	tasks, err := reloaded.ListTasks("persist", model.TaskFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 5 {
		t.Fatalf("expected 5 tasks after reload, got %d", len(tasks))
	}
}
