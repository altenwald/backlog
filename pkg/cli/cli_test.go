package cli

import (
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/altenwald/backlog/pkg/model"
	"github.com/altenwald/backlog/pkg/server"
	"github.com/altenwald/backlog/pkg/store"
	"github.com/go-chi/chi/v5"
)

func TestPrintTasksHierarchically(t *testing.T) {
	tasks := []model.Task{
		{
			ID:    "1",
			Title: "Root Task 1",
			Size:  model.SizeXL,
			Tier:  model.Tier1,
			Done:  true,
		},
		{
			ID:        "2",
			ParentID:  "1",
			Title:     "Child Task 2",
			Size:      model.SizeM,
			Tier:      model.Tier2,
			Assignee:  "manuel",
			DependsOn: []string{"1"},
		},
		{
			ID:        "3",
			ParentID:  "2",
			Title:     "Grandchild Task 3",
			Size:      model.SizeS,
			Tier:      model.Tier3,
			DependsOn: []string{"2"}, // Task 2 is not done -> Task 3 is blocked!
		},
		{
			ID:    "4",
			Title: "Independent Task 4",
			Size:  model.SizeXS,
			Tier:  model.Tier5,
		},
	}

	// Should format and print without crashing
	printTasksHierarchically(tasks)
}

func TestCLICommandsWithStore(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "backlog-cli-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	st, err := store.NewStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to initialize store: %v", err)
	}

	projSlug := "cli-proj"
	_, err = st.CreateProject(projSlug, "CLI Project", "For testing CLI commands")
	if err != nil {
		t.Fatalf("CreateProject failed: %v", err)
	}
	_ = st.SetActiveProject(projSlug)

	t1, err := st.AddTask(projSlug, model.Task{
		Title: "CLI Task 1",
		Size:  model.SizeM,
		Tier:  model.Tier2,
	})
	if err != nil {
		t.Fatalf("AddTask failed: %v", err)
	}

	_, _ = st.AddTask(projSlug, model.Task{
		Title:     "CLI Task 2 (Blocked)",
		ParentID:  t1.ID,
		DependsOn: []string{t1.ID},
		Size:      model.SizeS,
		Tier:      model.Tier1,
	})

	// Configure CLI flags to point to our test store directory and unused API port
	oldDataDir := flagDataDir
	oldAPIURL := flagAPIURL
	oldProject := flagProject
	defer func() {
		flagDataDir = oldDataDir
		flagAPIURL = oldAPIURL
		flagProject = oldProject
	}()

	flagDataDir = tmpDir
	flagAPIURL = "http://127.0.0.1:59999" // unreachable port so it uses local store
	flagProject = projSlug

	// 1. Test projects command
	err = projectsCmd.RunE(projectsCmd, []string{})
	if err != nil {
		t.Fatalf("projectsCmd failed: %v", err)
	}

	// 2. Test summary command
	err = summaryCmd.RunE(summaryCmd, []string{})
	if err != nil {
		t.Fatalf("summaryCmd failed: %v", err)
	}

	// 3. Test list command
	err = listCmd.RunE(listCmd, []string{})
	if err != nil {
		t.Fatalf("listCmd failed: %v", err)
	}

	// 4. Test list with filters
	flagTier = 2
	err = listCmd.RunE(listCmd, []string{})
	if err != nil {
		t.Fatalf("listCmd with tier failed: %v", err)
	}

	flagTier = 0
	flagBlocked = "true"
	err = listCmd.RunE(listCmd, []string{})
	if err != nil {
		t.Fatalf("listCmd with blocked failed: %v", err)
	}

	// 5. Test missing project error
	flagProject = ""
	err = listCmd.RunE(listCmd, []string{})
	if err == nil || !strings.Contains(err.Error(), "must specify a project") {
		t.Fatalf("expected error about specifying a project, got: %v", err)
	}
	err = summaryCmd.RunE(summaryCmd, []string{})
	if err == nil || !strings.Contains(err.Error(), "must specify a project") {
		t.Fatalf("expected error about specifying a project, got: %v", err)
	}
}

func TestResolveProject(t *testing.T) {
	// Branch 1: CLI flag takes priority
	got := resolveProject("my-proj")
	if got != "my-proj" {
		t.Fatalf("expected my-proj, got %s", got)
	}

	// Branch 2: BACKLOG_PROJECT env var used when flag is empty
	t.Setenv("BACKLOG_PROJECT", "env-proj")
	got = resolveProject("")
	if got != "env-proj" {
		t.Fatalf("expected env-proj from env var, got %s", got)
	}

	// Branch 3: Both empty -> returns ""
	t.Setenv("BACKLOG_PROJECT", "")
	got = resolveProject("")
	if got != "" {
		t.Fatalf("expected empty string, got %s", got)
	}
}

func TestCLISettingsCommands(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "backlog-cli-settings-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	oldDataDir := flagDataDir
	oldAPIURL := flagAPIURL
	defer func() {
		flagDataDir = oldDataDir
		flagAPIURL = oldAPIURL
	}()

	flagDataDir = tmpDir
	flagAPIURL = "http://127.0.0.1:59998" // unreachable -> uses local store

	// 1. Get initial settings (store mode)
	if err := settingsGetCmd.RunE(settingsGetCmd, []string{}); err != nil {
		t.Fatalf("settingsGetCmd failed: %v", err)
	}

	// 2. Set custom instructions (store mode)
	custom := "8. CLI RULE: maintain 100% tests."
	if err := settingsSetCmd.RunE(settingsSetCmd, []string{custom}); err != nil {
		t.Fatalf("settingsSetCmd failed: %v", err)
	}

	// 3. Get updated settings (store mode)
	if err := settingsGetCmd.RunE(settingsGetCmd, []string{}); err != nil {
		t.Fatalf("settingsGetCmd after set failed: %v", err)
	}

	// 4. Reset instructions (store mode)
	if err := settingsResetCmd.RunE(settingsResetCmd, []string{}); err != nil {
		t.Fatalf("settingsResetCmd failed: %v", err)
	}

	// 5. Test resolveInstructionsArg
	argVal, err := resolveInstructionsArg("plain text")
	if err != nil || argVal != "plain text" {
		t.Fatalf("expected plain text, got %q err=%v", argVal, err)
	}

	r, w, err := os.Pipe()
	if err == nil {
		_, _ = w.Write([]byte("from stdin"))
		_ = w.Close()
		oldStdin := os.Stdin
		os.Stdin = r
		stdinVal, stdinErr := resolveInstructionsArg("-")
		os.Stdin = oldStdin
		_ = r.Close()
		if stdinErr != nil || stdinVal != "from stdin" {
			t.Fatalf("expected from stdin, got %q err=%v", stdinVal, stdinErr)
		}
	}

	// 6. Test with running HTTP server (daemon mode)
	st, err := store.NewStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	router := chi.NewRouter()
	h := server.NewAPIHandler(st)
	h.RegisterRoutes(router)
	ts := httptest.NewServer(router)
	defer ts.Close()

	flagAPIURL = ts.URL

	if err := settingsGetCmd.RunE(settingsGetCmd, []string{}); err != nil {
		t.Fatalf("settingsGetCmd via HTTP failed: %v", err)
	}
	if err := settingsSetCmd.RunE(settingsSetCmd, []string{"daemon instructions"}); err != nil {
		t.Fatalf("settingsSetCmd via HTTP failed: %v", err)
	}
	if err := settingsGetCmd.RunE(settingsGetCmd, []string{}); err != nil {
		t.Fatalf("settingsGetCmd via HTTP after set failed: %v", err)
	}
	if err := settingsResetCmd.RunE(settingsResetCmd, []string{}); err != nil {
		t.Fatalf("settingsResetCmd via HTTP failed: %v", err)
	}
}
