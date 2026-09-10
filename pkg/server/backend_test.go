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

	// 9. Deprecate and Specification
	_, err = be.CreateProject("spec-store-proj", "Spec Proj", "")
	if err != nil {
		t.Fatalf("CreateProject failed: %v", err)
	}
	tStore, err := be.AddTask("spec-store-proj", model.Task{Title: "T1", Tier: model.Tier2, Size: model.SizeS})
	if err != nil {
		t.Fatalf("AddTask failed: %v", err)
	}
	depT, err := be.DeprecateTask("spec-store-proj", tStore.ID, true)
	if err != nil || !depT.Deprecated || !depT.Done {
		t.Fatalf("expected task deprecated, got %+v err=%v", depT, err)
	}
	err = be.UpdateProjectSpecification("spec-store-proj", "# Store Spec")
	if err != nil {
		t.Fatalf("UpdateProjectSpecification failed: %v", err)
	}
	spec, err := be.GetProjectSpecification("spec-store-proj")
	if err != nil || spec != "# Store Spec" {
		t.Fatalf("expected # Store Spec, got %q err=%v", spec, err)
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

	// Deprecate & Specification via clientBackend
	_, err = be.CreateProject("spec-client-proj", "Spec Client Proj", "")
	if err != nil {
		t.Fatalf("CreateProject via clientBackend failed: %v", err)
	}
	tClient, err := be.AddTask("spec-client-proj", model.Task{Title: "T2", Tier: model.Tier1, Size: model.SizeM})
	if err != nil {
		t.Fatalf("AddTask via clientBackend failed: %v", err)
	}
	depTC, err := be.DeprecateTask("spec-client-proj", tClient.ID, true)
	if err != nil || !depTC.Deprecated || !depTC.Done {
		t.Fatalf("expected task deprecated via clientBackend, got %+v err=%v", depTC, err)
	}
	err = be.UpdateProjectSpecification("spec-client-proj", "# Client Spec")
	if err != nil {
		t.Fatalf("UpdateProjectSpecification via clientBackend failed: %v", err)
	}
	specC, err := be.GetProjectSpecification("spec-client-proj")
	if err != nil || specC != "# Client Spec" {
		t.Fatalf("expected # Client Spec, got %q err=%v", specC, err)
	}
}
