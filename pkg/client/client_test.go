package client_test

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

func setupTestServer(t *testing.T) (*client.Client, func()) {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "backlog-client-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	st, err := store.NewStore(tmpDir)
	if err != nil {
		_ = os.RemoveAll(tmpDir)
		t.Fatalf("failed to initialize store: %v", err)
	}

	r := chi.NewRouter()
	handler := server.NewAPIHandler(st)
	handler.RegisterRoutes(r)

	ts := httptest.NewServer(r)
	c := client.NewClient(ts.URL)

	cleanup := func() {
		ts.Close()
		_ = os.RemoveAll(tmpDir)
	}

	return c, cleanup
}

func TestClientEndToEnd(t *testing.T) {
	c, cleanup := setupTestServer(t)
	defer cleanup()

	// 1. IsServerRunning
	if !c.IsServerRunning() {
		t.Fatal("expected server to be running")
	}

	// 2. ListProjects initially empty
	projs, err := c.ListProjects()
	if err != nil {
		t.Fatalf("ListProjects failed: %v", err)
	}
	if len(projs) != 0 {
		t.Fatalf("expected 0 initial projects, got %d", len(projs))
	}

	// 3. CreateProject & GetProject
	p2, err := c.CreateProject("api-client", "API Client Proj", "For testing client")
	if err != nil {
		t.Fatalf("CreateProject failed: %v", err)
	}
	if p2.Slug != "api-client" {
		t.Fatalf("expected slug api-client, got %s", p2.Slug)
	}

	projsAfter, err := c.ListProjects()
	if err != nil || len(projsAfter) != 1 {
		t.Fatalf("expected 1 project after creation, got %d", len(projsAfter))
	}

	gotProj, err := c.GetProject("api-client")
	if err != nil || gotProj.Slug != "api-client" {
		t.Fatalf("expected project api-client, got %+v", gotProj)
	}

	// 4. AddTask (Root and Subtask with Dependency)
	t1, err := c.AddTask("api-client", model.Task{
		Title: "Setup DB",
		Size:  model.SizeM,
		Tier:  model.Tier2,
	})
	if err != nil {
		t.Fatalf("AddTask t1 failed: %v", err)
	}

	t2, err := c.AddTask("api-client", model.Task{
		Title:     "Subtask Migrations",
		ParentID:  t1.ID,
		DependsOn: []string{t1.ID},
		Size:      model.SizeS,
		Tier:      model.Tier1,
	})
	if err != nil {
		t.Fatalf("AddTask t2 failed: %v", err)
	}

	// 5. GetTask
	fetchedT1, err := c.GetTask("api-client", t1.ID)
	if err != nil {
		t.Fatalf("GetTask failed: %v", err)
	}
	if fetchedT1.Title != "Setup DB" {
		t.Fatalf("expected title Setup DB, got %s", fetchedT1.Title)
	}

	// 6. ListTasks with filters
	trueVal := true
	blockedTasks, err := c.ListTasks("api-client", model.TaskFilter{Blocked: &trueVal})
	if err != nil {
		t.Fatalf("ListTasks blocked failed: %v", err)
	}
	if len(blockedTasks) != 1 || blockedTasks[0].ID != t2.ID {
		t.Fatalf("expected t2 to be blocked, got %+v", blockedTasks)
	}

	// 7. UpdateTask
	updatedT1, err := c.UpdateTask("api-client", model.Task{
		ID:    t1.ID,
		Title: "Setup DB Updated",
	})
	if err != nil {
		t.Fatalf("UpdateTask failed: %v", err)
	}
	if updatedT1.Title != "Setup DB Updated" {
		t.Fatalf("expected title updated, got %s", updatedT1.Title)
	}

	// 8. AssignTask
	assigned, err := c.AssignTask("api-client", t1.ID, "dev1")
	if err != nil {
		t.Fatalf("AssignTask failed: %v", err)
	}
	if assigned.Assignee != "dev1" {
		t.Fatalf("expected assignee dev1, got %s", assigned.Assignee)
	}

	// 9. CompleteTask
	completed, err := c.CompleteTask("api-client", t1.ID, true)
	if err != nil {
		t.Fatalf("CompleteTask failed: %v", err)
	}
	if !completed.Done {
		t.Fatal("expected task to be marked done")
	}

	// 10. GetSummary & GetTopPriorities
	sum, err := c.GetSummary("api-client")
	if err != nil {
		t.Fatalf("GetSummary failed: %v", err)
	}
	if sum.TotalTasks != 2 || sum.OpenTasks != 1 || sum.CompletedTasks != 1 {
		t.Fatalf("unexpected summary: %+v", sum)
	}

	top, err := c.GetTopPriorities("api-client", 5)
	if err != nil {
		t.Fatalf("GetTopPriorities failed: %v", err)
	}
	if len(top) != 1 || top[0].ID != t2.ID {
		t.Fatalf("expected top priority t2, got %+v", top)
	}

	// 11. DeleteTask
	err = c.DeleteTask("api-client", t1.ID)
	if err != nil {
		t.Fatalf("DeleteTask failed: %v", err)
	}

	// 12. DeleteProject
	err = c.DeleteProject("api-client")
	if err != nil {
		t.Fatalf("DeleteProject failed: %v", err)
	}
}

func TestClientSettings(t *testing.T) {
	c, cleanup := setupTestServer(t)
	defer cleanup()

	// 1. Initial settings should be empty
	s, err := c.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings failed: %v", err)
	}
	if s.MCPUserInstructions != "" {
		t.Fatalf("expected empty initial instructions, got %q", s.MCPUserInstructions)
	}

	// 2. Update settings
	custom := "6. CUSTOM RULE: write thorough tests."
	updated, err := c.UpdateSettings(client.SettingsInfo{MCPUserInstructions: custom})
	if err != nil {
		t.Fatalf("UpdateSettings failed: %v", err)
	}
	if updated.MCPUserInstructions != custom {
		t.Fatalf("expected %q, got %q", custom, updated.MCPUserInstructions)
	}

	// 3. Get settings again
	s2, err := c.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings after update failed: %v", err)
	}
	if s2.MCPUserInstructions != custom {
		t.Fatalf("expected %q, got %q", custom, s2.MCPUserInstructions)
	}

	// 4. Reset settings (empty string)
	reset, err := c.UpdateSettings(client.SettingsInfo{MCPUserInstructions: ""})
	if err != nil {
		t.Fatalf("UpdateSettings reset failed: %v", err)
	}
	if reset.MCPUserInstructions != "" {
		t.Fatalf("expected empty instructions after reset, got %q", reset.MCPUserInstructions)
	}
}

func TestClientDeprecateAndSpec(t *testing.T) {
	c, cleanup := setupTestServer(t)
	defer cleanup()

	_, err := c.CreateProject("deprecate-client", "Deprecate Client", "Desc")
	if err != nil {
		t.Fatalf("CreateProject failed: %v", err)
	}

	task, err := c.AddTask("deprecate-client", model.Task{
		Title: "Task for deprecation",
		Size:  model.SizeM,
		Tier:  model.Tier2,
	})
	if err != nil {
		t.Fatalf("AddTask failed: %v", err)
	}

	// 1. Deprecate task
	depTask, err := c.DeprecateTask("deprecate-client", task.ID, true)
	if err != nil {
		t.Fatalf("DeprecateTask failed: %v", err)
	}
	if !depTask.Deprecated || !depTask.Done {
		t.Fatalf("expected task deprecated=true done=true, got deprecated=%v done=%v", depTask.Deprecated, depTask.Done)
	}

	// 2. Undeprecate task
	undepTask, err := c.DeprecateTask("deprecate-client", task.ID, false)
	if err != nil {
		t.Fatalf("Undeprecate failed: %v", err)
	}
	if undepTask.Deprecated {
		t.Fatal("expected task deprecated=false")
	}

	// 3. Specification
	spec, err := c.GetProjectSpecification("deprecate-client")
	if err != nil || spec != "" {
		t.Fatalf("expected empty initial spec, got %q err=%v", spec, err)
	}

	testSpec := "# Client Spec\n\nReference to #1"
	if err := c.UpdateProjectSpecification("deprecate-client", testSpec); err != nil {
		t.Fatalf("UpdateProjectSpecification failed: %v", err)
	}

	specAfter, err := c.GetProjectSpecification("deprecate-client")
	if err != nil || specAfter != testSpec {
		t.Fatalf("expected spec %q, got %q err=%v", testSpec, specAfter, err)
	}
}
