package server_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/altenwald/backlog/pkg/server"
	"github.com/altenwald/backlog/pkg/store"
	"github.com/go-chi/chi/v5"
)

func TestServerLifecycle(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "backlog-server-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	st, err := store.NewStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// Use an ephemeral port
	srv := server.NewServer(st, 18484)
	if srv.GetMCPServer() == nil {
		t.Fatal("expected MCP server to be initialized")
	}

	go func() {
		_ = srv.Start()
	}()

	time.Sleep(100 * time.Millisecond)

	// Stop server
	err = srv.Stop(context.Background())
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func TestAPIHandlerErrors(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "backlog-api-test-*")
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

	// 1. Invalid JSON body on create project
	req := httptest.NewRequest("POST", "/api/projects", bytes.NewReader([]byte("not json")))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for bad json, got %d", rec.Code)
	}

	// 2. Non-existent project 404
	req = httptest.NewRequest("GET", "/api/projects/unknown-proj", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown project, got %d", rec.Code)
	}

	// 3. Non-existent task 404
	req = httptest.NewRequest("GET", "/api/projects/unknown-proj/tasks/999", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown task, got %d", rec.Code)
	}

	// 4. Bad JSON on task create
	req = httptest.NewRequest("POST", "/api/projects/unknown-proj/tasks", bytes.NewReader([]byte("{invalid")))
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid json, got %d", rec.Code)
	}
}
