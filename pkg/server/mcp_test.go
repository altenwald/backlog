package server

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/altenwald/backlog/pkg/model"
	"github.com/altenwald/backlog/pkg/store"
)

func TestMCPServerTools(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "backlog-mcp-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	st, err := store.NewStore(tempDir)
	if err != nil {
		t.Fatal(err)
	}

	srv := NewMCPServer(st)
	if srv == nil {
		t.Fatal("expected non-nil MCP server")
	}

	// 1. Add Task with assignee & resolution
	task, err := st.AddTask("dymmer", model.Task{
		Title:       "Test Task MCP",
		Description: "Test description",
		Size:        model.SizeM,
		Tier:        model.Tier1,
		Assignee:    "claude",
		Resolution:  "initial notes",
	})
	if err != nil {
		t.Fatal(err)
	}
	if task.Assignee != "claude" {
		t.Fatalf("expected assignee 'claude', got '%s'", task.Assignee)
	}
	if task.InsertedAt.IsZero() || task.UpdatedAt.IsZero() {
		t.Fatal("expected InsertedAt and UpdatedAt to be set")
	}

	// 2. Assign task to antigravity
	assigned, err := st.AssignTask("dymmer", task.ID, "antigravity")
	if err != nil {
		t.Fatal(err)
	}
	if assigned.Assignee != "antigravity" {
		t.Fatalf("expected assignee 'antigravity', got '%s'", assigned.Assignee)
	}

	// 3. List tasks filtered by assignee
	assigneeFilter := "antigravity"
	tasks, err := st.ListTasks("dymmer", model.TaskFilter{Assignee: &assigneeFilter})
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) == 0 || tasks[0].Assignee != "antigravity" {
		t.Fatalf("expected tasks matching assignee antigravity, got %+v", tasks)
	}

	// 4. Complete task with resolution
	completed, err := st.CompleteTask("dymmer", task.ID, true, "Implemented cleanly via commit xyz")
	if err != nil {
		t.Fatal(err)
	}
	if !completed.Done || completed.Resolution != "Implemented cleanly via commit xyz" {
		t.Fatalf("expected done with resolution, got %+v", completed)
	}
	if completed.TerminatedAt == nil || completed.TerminatedAt.IsZero() {
		t.Fatal("expected TerminatedAt to be set when completed")
	}

	// Verify JSON serialization includes inserted_at, updated_at, terminated_at, assignee and resolution
	data, err := json.Marshal(completed)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	_ = json.Unmarshal(data, &m)
	if m["assignee"] != "antigravity" {
		t.Fatalf("JSON missing assignee field: %s", string(data))
	}
	if m["resolution"] != "Implemented cleanly via commit xyz" {
		t.Fatalf("JSON missing resolution field: %s", string(data))
	}
	if _, ok := m["inserted_at"]; !ok {
		t.Fatalf("JSON missing inserted_at field: %s", string(data))
	}
	if _, ok := m["updated_at"]; !ok {
		t.Fatalf("JSON missing updated_at field: %s", string(data))
	}
	if _, ok := m["terminated_at"]; !ok {
		t.Fatalf("JSON missing terminated_at field: %s", string(data))
	}
}
