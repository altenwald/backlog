package cli

import (
	"os"
	"testing"

	"github.com/altenwald/backlog/pkg/model"
	"github.com/altenwald/backlog/pkg/store"
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
			Tag:       "ui",
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
}
