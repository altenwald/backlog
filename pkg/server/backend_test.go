package server_test

import (
	"os"
	"testing"

	"github.com/altenwald/backlog/pkg/model"
	"github.com/altenwald/backlog/pkg/server"
	"github.com/altenwald/backlog/pkg/store"
)

func TestStoreBackend(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "backlog-backend-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	st, err := store.NewStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	be := server.NewStoreBackend(st)

	// 1. Create project
	_, err = be.CreateProject("backend-proj", "Backend Proj", "Desc")
	if err != nil {
		t.Fatalf("CreateProject failed: %v", err)
	}

	// 2. Set active and get active
	err = be.SetActiveProject("backend-proj")
	if err != nil {
		t.Fatalf("SetActiveProject failed: %v", err)
	}
	if be.GetActiveProjectSlug() != "backend-proj" {
		t.Fatalf("expected slug backend-proj, got %s", be.GetActiveProjectSlug())
	}
	active, err := be.GetActiveProject()
	if err != nil || active["active_project"] != "backend-proj" {
		t.Fatalf("expected active backend-proj, got %+v", active)
	}

	// 3. ListProjects and GetProject
	projs, err := be.ListProjects()
	if err != nil || len(projs) != 1 {
		t.Fatalf("expected 1 project, got %d", len(projs))
	}
	p, err := be.GetProject("backend-proj")
	if err != nil || p.Slug != "backend-proj" {
		t.Fatalf("expected project backend-proj, got %+v", p)
	}

	// 4. AddTask & ListTasks
	task, err := be.AddTask("backend-proj", model.Task{
		Title: "Task 1",
		Size:  model.SizeM,
		Tier:  model.Tier2,
	})
	if err != nil {
		t.Fatalf("AddTask failed: %v", err)
	}

	tasks, err := be.ListTasks("backend-proj", model.TaskFilter{})
	if err != nil || len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}

	// 5. UpdateTask, AssignTask, CompleteTask
	_, err = be.UpdateTask("backend-proj", model.Task{ID: task.ID, Title: "Task 1 Updated"})
	if err != nil {
		t.Fatalf("UpdateTask failed: %v", err)
	}

	_, err = be.AssignTask("backend-proj", task.ID, "bob")
	if err != nil {
		t.Fatalf("AssignTask failed: %v", err)
	}

	_, err = be.CompleteTask("backend-proj", task.ID, true, "resolution summary")
	if err != nil {
		t.Fatalf("CompleteTask failed: %v", err)
	}

	// 6. Summary & Top Priorities
	sum, err := be.GetSummary("backend-proj")
	if err != nil || sum.TotalTasks != 1 || sum.CompletedTasks != 1 {
		t.Fatalf("unexpected summary: %+v", sum)
	}

	_, err = be.GetTopPriorities("backend-proj", 5)
	if err != nil {
		t.Fatalf("GetTopPriorities failed: %v", err)
	}

	// 7. DeleteTask & DeleteProject
	err = be.DeleteTask("backend-proj", task.ID)
	if err != nil {
		t.Fatalf("DeleteTask failed: %v", err)
	}

	err = be.DeleteProject("backend-proj")
	if err != nil {
		t.Fatalf("DeleteProject failed: %v", err)
	}
}
