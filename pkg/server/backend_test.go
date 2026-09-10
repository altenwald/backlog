package server_test

import (
	"net/http/httptest"
	"os"
	"testing"

	"github.com/altenwald/backlog/pkg/client"
	"github.com/altenwald/backlog/pkg/model"
	"github.com/altenwald/backlog/pkg/server"
	"github.com/altenwald/backlog/pkg/store"
	"github.com/go-chi/chi/v5"
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

	// 2. ListProjects and GetProject
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

	// 8. Settings
	inst, err := be.GetSettings()
	if err != nil || inst != "" {
		t.Fatalf("expected empty settings, got %q err=%v", inst, err)
	}
	err = be.UpdateSettings("test instructions")
	if err != nil {
		t.Fatalf("UpdateSettings failed: %v", err)
	}
	inst, err = be.GetSettings()
	if err != nil || inst != "test instructions" {
		t.Fatalf("expected test instructions, got %q err=%v", inst, err)
	}
}

// TestClientBackend exercises all clientBackend methods, which proxy calls to a
// real HTTP server via client.Client, ensuring the proxy layer is covered.
func TestClientBackend(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "backlog-client-backend-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	st, err := store.NewStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	r := chi.NewRouter()
	h := server.NewAPIHandler(st)
	h.RegisterRoutes(r)
	ts := httptest.NewServer(r)
	defer ts.Close()

	c := client.NewClient(ts.URL)
	be := server.NewClientBackend(c)

	// CreateProject (delegates to client, which is already covered, but needed for setup)
	_, err = be.CreateProject("cb-proj", "CB Proj", "desc")
	if err != nil {
		t.Fatalf("CreateProject via clientBackend failed: %v", err)
	}

	// ListProjects (0% in clientBackend)
	projs, err := be.ListProjects()
	if err != nil || len(projs) != 1 {
		t.Fatalf("ListProjects via clientBackend: expected 1 project, got %d err=%v", len(projs), err)
	}

	// GetProject (0% in clientBackend)
	p, err := be.GetProject("cb-proj")
	if err != nil || p.Slug != "cb-proj" {
		t.Fatalf("GetProject via clientBackend failed: %v / %+v", err, p)
	}

	// AddTask to have data for subsequent operations
	task, err := be.AddTask("cb-proj", model.Task{
		Title: "CB Task",
		Size:  model.SizeS,
		Tier:  model.Tier1,
	})
	if err != nil {
		t.Fatalf("AddTask via clientBackend failed: %v", err)
	}

	// ListTasks (0% in clientBackend)
	tasks, err := be.ListTasks("cb-proj", model.TaskFilter{})
	if err != nil || len(tasks) != 1 {
		t.Fatalf("ListTasks via clientBackend: expected 1 task, got %d err=%v", len(tasks), err)
	}

	// UpdateTask (0% in clientBackend)
	_, err = be.UpdateTask("cb-proj", model.Task{ID: task.ID, Title: "CB Task Updated"})
	if err != nil {
		t.Fatalf("UpdateTask via clientBackend failed: %v", err)
	}

	// AssignTask (0% in clientBackend)
	_, err = be.AssignTask("cb-proj", task.ID, "dev")
	if err != nil {
		t.Fatalf("AssignTask via clientBackend failed: %v", err)
	}

	// CompleteTask (0% in clientBackend)
	_, err = be.CompleteTask("cb-proj", task.ID, true, "done")
	if err != nil {
		t.Fatalf("CompleteTask via clientBackend failed: %v", err)
	}

	// GetSummary (already covered but verify it works through clientBackend)
	sum, err := be.GetSummary("cb-proj")
	if err != nil || sum.TotalTasks != 1 {
		t.Fatalf("GetSummary via clientBackend failed: %v / %+v", err, sum)
	}

	// GetTopPriorities (0% in clientBackend)
	_, err = be.GetTopPriorities("cb-proj", 5)
	if err != nil {
		t.Fatalf("GetTopPriorities via clientBackend failed: %v", err)
	}

	// DeleteTask (0% in clientBackend)
	err = be.DeleteTask("cb-proj", task.ID)
	if err != nil {
		t.Fatalf("DeleteTask via clientBackend failed: %v", err)
	}

	// DeleteProject (already covered but exercises the proxy)
	err = be.DeleteProject("cb-proj")
	if err != nil {
		t.Fatalf("DeleteProject via clientBackend failed: %v", err)
	}

	// Settings via clientBackend
	inst, err := be.GetSettings()
	if err != nil || inst != "" {
		t.Fatalf("expected empty settings via clientBackend, got %q err=%v", inst, err)
	}
	err = be.UpdateSettings("proxy instructions")
	if err != nil {
		t.Fatalf("UpdateSettings via clientBackend failed: %v", err)
	}
	inst, err = be.GetSettings()
	if err != nil || inst != "proxy instructions" {
		t.Fatalf("expected proxy instructions, got %q err=%v", inst, err)
	}
}
