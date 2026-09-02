package server_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/altenwald/backlog/pkg/model"
	"github.com/altenwald/backlog/pkg/server"
	"github.com/altenwald/backlog/pkg/store"
	"github.com/go-chi/chi/v5"
)

func TestAPIHandlerFullEndpoints(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "backlog-api-endpoints-test-*")
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

	client := ts.Client()

	// 1. GET /health
	resp, err := client.Get(ts.URL + "/health")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /health failed: %v, status %d", err, resp.StatusCode)
	}

	// 2. POST /api/projects
	bodyProj, _ := json.Marshal(map[string]string{
		"slug":        "api-proj",
		"name":        "API Proj",
		"description": "Created via REST API",
	})
	resp, err = client.Post(ts.URL+"/api/projects", "application/json", bytes.NewReader(bodyProj))
	if err != nil || resp.StatusCode != http.StatusCreated {
		t.Fatalf("POST /api/projects failed: %v, status %d", err, resp.StatusCode)
	}

	// 3. POST /api/projects/active & GET /api/projects/active
	bodyActive, _ := json.Marshal(map[string]string{"slug": "api-proj"})
	resp, err = client.Post(ts.URL+"/api/projects/active", "application/json", bytes.NewReader(bodyActive))
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/projects/active failed: %v", err)
	}

	resp, err = client.Get(ts.URL + "/api/projects/active")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/projects/active failed: %v", err)
	}

	// 4. GET /api/projects
	resp, err = client.Get(ts.URL + "/api/projects")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/projects failed: %v", err)
	}

	// 5. GET /api/projects/api-proj
	resp, err = client.Get(ts.URL + "/api/projects/api-proj")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/projects/api-proj failed: %v", err)
	}

	// 6. POST /api/projects/api-proj/tasks
	bodyTask, _ := json.Marshal(model.Task{
		Title: "Task API 1",
		Size:  model.SizeM,
		Tier:  model.Tier2,
	})
	resp, err = client.Post(ts.URL+"/api/projects/api-proj/tasks", "application/json", bytes.NewReader(bodyTask))
	if err != nil || resp.StatusCode != http.StatusCreated {
		t.Fatalf("POST tasks failed: %v, status %d", err, resp.StatusCode)
	}

	var createdTask model.Task
	_ = json.NewDecoder(resp.Body).Decode(&createdTask)

	// 7. GET /api/projects/api-proj/tasks/{id}
	resp, err = client.Get(ts.URL + "/api/projects/api-proj/tasks/" + createdTask.ID)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("GET task failed: %v", err)
	}

	// 8. PUT /api/projects/api-proj/tasks/{id}
	bodyUpdate, _ := json.Marshal(map[string]any{
		"title": "Task API 1 Updated",
	})
	req, _ := http.NewRequest(http.MethodPut, ts.URL+"/api/projects/api-proj/tasks/"+createdTask.ID, bytes.NewReader(bodyUpdate))
	req.Header.Set("Content-Type", "application/json")
	resp, err = client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("PUT task failed: %v", err)
	}

	// 9. POST /api/projects/api-proj/tasks/{id}/assign
	bodyAssign, _ := json.Marshal(map[string]string{"assignee": "alice"})
	resp, err = client.Post(ts.URL+"/api/projects/api-proj/tasks/"+createdTask.ID+"/assign", "application/json", bytes.NewReader(bodyAssign))
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("POST assign failed: %v", err)
	}

	// 10. POST /api/projects/api-proj/tasks/{id}/done
	bodyDone, _ := json.Marshal(map[string]any{"done": true, "resolution": "completed successfully"})
	resp, err = client.Post(ts.URL+"/api/projects/api-proj/tasks/"+createdTask.ID+"/done", "application/json", bytes.NewReader(bodyDone))
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("POST done failed: %v", err)
	}

	// 11. GET summary and top
	resp, err = client.Get(ts.URL + "/api/projects/api-proj/summary")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("GET summary failed: %v", err)
	}

	resp, err = client.Get(ts.URL + "/api/projects/api-proj/top?limit=3")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("GET top failed: %v", err)
	}

	// 12. GET /api/projects/api-proj/tasks with query params
	resp, err = client.Get(ts.URL + "/api/projects/api-proj/tasks?tier=2&done=true&assignee=alice&search=Updated")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("GET tasks with query failed: %v", err)
	}

	// 13. DELETE /api/projects/api-proj/tasks/{id}
	reqDelTask, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/projects/api-proj/tasks/"+createdTask.ID, nil)
	resp, err = client.Do(reqDelTask)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("DELETE task failed: %v", err)
	}

	// 14. DELETE /api/projects/api-proj
	reqDelProj, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/projects/api-proj", nil)
	resp, err = client.Do(reqDelProj)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("DELETE project failed: %v", err)
	}
}
