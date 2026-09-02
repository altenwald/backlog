package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/altenwald/backlog/pkg/client"
	"github.com/altenwald/backlog/pkg/model"
	"github.com/altenwald/backlog/pkg/store"
	"github.com/go-chi/chi/v5"
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

	// 0. Create test project
	_, err = st.CreateProject("testproj", "Test Project", "Testing")
	if err != nil {
		t.Fatal(err)
	}
	_ = st.SetActiveProject("testproj")

	// 1. Add Task with assignee & resolution
	task, err := st.AddTask("testproj", model.Task{
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
	assigned, err := st.AssignTask("testproj", task.ID, "antigravity")
	if err != nil {
		t.Fatal(err)
	}
	if assigned.Assignee != "antigravity" {
		t.Fatalf("expected assignee 'antigravity', got '%s'", assigned.Assignee)
	}

	// 3. List tasks filtered by assignee
	assigneeFilter := "antigravity"
	tasks, err := st.ListTasks("testproj", model.TaskFilter{Assignee: &assigneeFilter})
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) == 0 || tasks[0].Assignee != "antigravity" {
		t.Fatalf("expected tasks matching assignee antigravity, got %+v", tasks)
	}

	// 4. Complete task with resolution
	completed, err := st.CompleteTask("testproj", task.ID, true, "Implemented cleanly via commit xyz")
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

	// 5. Test MCP Initialize returns instructions
	initMsg := []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test-client","version":"1.0"}}}`)
	resp := srv.HandleMessage(context.Background(), initMsg)
	respBytes, err := json.Marshal(resp)
	if err != nil {
		t.Fatal(err)
	}
	var initResp struct {
		Result struct {
			Instructions string `json:"instructions"`
		} `json:"result"`
	}
	if err := json.Unmarshal(respBytes, &initResp); err != nil {
		t.Fatal(err)
	}
	if initResp.Result.Instructions == "" {
		t.Fatal("expected instructions in initialize response")
	}

	// 6. Test create_project tool
	createMsg := []byte(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"create_project","arguments":{"slug":"newproj","name":"New Project","description":"Desc"}}}`)
	createResp := srv.HandleMessage(context.Background(), createMsg)
	createRespBytes, err := json.Marshal(createResp)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(createRespBytes), "created successfully") {
		t.Fatalf("expected created successfully, got %s", string(createRespBytes))
	}

	// 7. Test add_task with parent_id and depends_on via MCP
	addTaskMsg := []byte(`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"add_task","arguments":{"project":"newproj","title":"MCP Root","tier":2,"size":"M"}}}`)
	addResp := srv.HandleMessage(context.Background(), addTaskMsg)
	addRespBytes, _ := json.Marshal(addResp)
	if !strings.Contains(string(addRespBytes), "Task created in 'newproj'") {
		t.Fatalf("expected task created, got %s", string(addRespBytes))
	}

	addSubMsg := []byte(`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"add_task","arguments":{"project":"newproj","title":"MCP Child","parent_id":"1","depends_on":"1","tier":1,"size":"S"}}}`)
	addSubResp := srv.HandleMessage(context.Background(), subTaskBytes(srv, addSubMsg))
	_ = addSubResp

	// 8. Test list_tasks via MCP with blocked filter
	listMsg := []byte(`{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"list_tasks","arguments":{"project":"newproj","blocked":true}}}`)
	listResp := srv.HandleMessage(context.Background(), listMsg)
	listRespBytes, _ := json.Marshal(listResp)
	if !strings.Contains(string(listRespBytes), "MCP Child") {
		t.Fatalf("expected blocked list to contain MCP Child, got %s", string(listRespBytes))
	}

	// 9. Test update_task via MCP
	updateMsg := []byte(`{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"update_task","arguments":{"project":"newproj","task_id":"1","title":"MCP Root Updated"}}}`)
	updateResp := srv.HandleMessage(context.Background(), updateMsg)
	updateRespBytes, _ := json.Marshal(updateResp)
	if !strings.Contains(string(updateRespBytes), "updated in 'newproj'") {
		t.Fatalf("expected task updated, got %s", string(updateRespBytes))
	}

	// 10. Test get_summary & get_top_priorities via MCP
	sumMsg := []byte(`{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"get_summary","arguments":{"project":"newproj"}}}`)
	sumResp := srv.HandleMessage(context.Background(), sumMsg)
	sumRespBytes, _ := json.Marshal(sumResp)
	if !strings.Contains(string(sumRespBytes), "total_tasks") {
		t.Fatalf("expected summary json, got %s", string(sumRespBytes))
	}

	topMsg := []byte(`{"jsonrpc":"2.0","id":8,"method":"tools/call","params":{"name":"get_top_priorities","arguments":{"project":"newproj","limit":2}}}`)
	topResp := srv.HandleMessage(context.Background(), topMsg)
	topRespBytes, _ := json.Marshal(topResp)
	if !strings.Contains(string(topRespBytes), "MCP Root") {
		t.Fatalf("expected top priorities, got %s", string(topRespBytes))
	}

	// 11. Test list_projects & set_active_project via MCP
	listProjsMsg := []byte(`{"jsonrpc":"2.0","id":9,"method":"tools/call","params":{"name":"list_projects","arguments":{}}}`)
	listProjsResp := srv.HandleMessage(context.Background(), listProjsMsg)
	listProjsBytes, _ := json.Marshal(listProjsResp)
	if !strings.Contains(string(listProjsBytes), "newproj") {
		t.Fatalf("expected newproj in list, got %s", string(listProjsBytes))
	}

	setActiveMsg := []byte(`{"jsonrpc":"2.0","id":10,"method":"tools/call","params":{"name":"set_active_project","arguments":{"project":"newproj"}}}`)
	setActiveResp := srv.HandleMessage(context.Background(), setActiveMsg)
	setActiveBytes, _ := json.Marshal(setActiveResp)
	if !strings.Contains(string(setActiveBytes), "switched to") {
		t.Fatalf("expected active project set, got %s", string(setActiveBytes))
	}

	// 12. Test complete_task via MCP
	completeMsg := []byte(`{"jsonrpc":"2.0","id":11,"method":"tools/call","params":{"name":"complete_task","arguments":{"project":"newproj","task_id":"1","resolution":"done via mcp"}}}`)
	completeResp := srv.HandleMessage(context.Background(), completeMsg)
	completeBytes, _ := json.Marshal(completeResp)
	if !strings.Contains(string(completeBytes), "completed in 'newproj'") {
		t.Fatalf("expected completed, got %s", string(completeBytes))
	}
}

func subTaskBytes(srv any, msg []byte) []byte {
	return msg
}

func TestMCPServerWithClientBackend(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "backlog-mcp-client-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	st, err := store.NewStore(tempDir)
	if err != nil {
		t.Fatal(err)
	}

	r := chi.NewRouter()
	api := NewAPIHandler(st)
	api.RegisterRoutes(r)
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, http.StatusOK, map[string]any{"status": "ok"})
	})

	ts := httptest.NewServer(r)
	defer ts.Close()

	c := client.NewClient(ts.URL)
	if !c.IsServerRunning() {
		t.Fatal("expected test server to be running")
	}

	be := NewClientBackend(c)
	srv := NewMCPServerWithBackend(be)

	// Create project via MCP through clientBackend
	createMsg := []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"create_project","arguments":{"slug":"testhttp","name":"HTTP Test","description":"Testing HTTP MCP"}}}`)
	createResp := srv.HandleMessage(context.Background(), createMsg)
	createRespBytes, _ := json.Marshal(createResp)
	if !strings.Contains(string(createRespBytes), "created successfully") {
		t.Fatalf("expected created successfully, got %s", string(createRespBytes))
	}

	// Add task via MCP through clientBackend
	addMsg := []byte(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"add_task","arguments":{"project":"testhttp","title":"Task via HTTP MCP","tier":1,"size":"S"}}}`)
	addResp := srv.HandleMessage(context.Background(), addMsg)
	addRespBytes, _ := json.Marshal(addResp)
	if !strings.Contains(string(addRespBytes), "Task created in 'testhttp'") {
		t.Fatalf("expected task created, got %s", string(addRespBytes))
	}

	// Verify it was persisted to in-memory store and disk
	tasks, err := st.ListTasks("testhttp", model.TaskFilter{})
	if err != nil || len(tasks) != 1 {
		t.Fatalf("expected 1 task in store, got %d (err: %v)", len(tasks), err)
	}

	// Delete project via MCP through clientBackend
	deleteMsg := []byte(`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"delete_project","arguments":{"project":"testhttp"}}}`)
	deleteResp := srv.HandleMessage(context.Background(), deleteMsg)
	deleteRespBytes, _ := json.Marshal(deleteResp)
	if !strings.Contains(string(deleteRespBytes), "deleted successfully") {
		t.Fatalf("expected deleted successfully, got %s", string(deleteRespBytes))
	}

	// Verify it was deleted from store
	if _, err := st.GetProject("testhttp"); err == nil {
		t.Fatal("expected project 'testhttp' to be deleted from store")
	}
}
