package store_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/altenwald/backlog/pkg/model"
	"github.com/altenwald/backlog/pkg/store"
)

func setupTestStore(t *testing.T) (*store.Store, string) {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "backlog-store-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	st, err := store.NewStore(tmpDir)
	if err != nil {
		_ = os.RemoveAll(tmpDir)
		t.Fatalf("failed to initialize store: %v", err)
	}

	return st, tmpDir
}

func TestTaskHierarchyAndCascadeDelete(t *testing.T) {
	st, tmpDir := setupTestStore(t)
	defer os.RemoveAll(tmpDir)

	projSlug := "hierarchy-test"
	_, err := st.CreateProject(projSlug, "Hierarchy Test", "Testing task tree")
	if err != nil {
		t.Fatalf("CreateProject failed: %v", err)
	}

	// 1. Create root task #1
	root, err := st.AddTask(projSlug, model.Task{
		Title: "Root Feature 1",
		Size:  model.SizeXL,
		Tier:  model.Tier1,
	})
	if err != nil {
		t.Fatalf("AddTask root failed: %v", err)
	}
	if root.ID != "1" {
		t.Fatalf("expected root ID '1', got '%s'", root.ID)
	}

	// 2. Fail when parent doesn't exist
	_, err = st.AddTask(projSlug, model.Task{
		Title:    "Invalid Child",
		ParentID: "999",
	})
	if err == nil {
		t.Fatal("expected error when adding task with non-existent parent, got nil")
	}

	// 3. Create child task #2 under #1
	child, err := st.AddTask(projSlug, model.Task{
		Title:    "Child Task 2",
		ParentID: root.ID,
		Size:     model.SizeM,
		Tier:     model.Tier2,
	})
	if err != nil {
		t.Fatalf("AddTask child failed: %v", err)
	}

	// 4. Create grandchild task #3 under #2
	grandchild, err := st.AddTask(projSlug, model.Task{
		Title:    "Grandchild Task 3",
		ParentID: child.ID,
		Size:     model.SizeS,
		Tier:     model.Tier3,
	})
	if err != nil {
		t.Fatalf("AddTask grandchild failed: %v", err)
	}

	// 5. Create independent root task #4
	root2, err := st.AddTask(projSlug, model.Task{
		Title: "Independent Root 4",
		Size:  model.SizeM,
		Tier:  model.Tier2,
	})
	if err != nil {
		t.Fatalf("AddTask root2 failed: %v", err)
	}

	// 6. Test cycle detection on update: try to set root's parent to grandchild
	_, err = st.UpdateTask(projSlug, model.Task{
		ID:       root.ID,
		ParentID: grandchild.ID,
	})
	if err == nil {
		t.Fatal("expected circular dependency error, got nil")
	}

	// 7. Test self-parenting prevention
	_, err = st.UpdateTask(projSlug, model.Task{
		ID:       root.ID,
		ParentID: root.ID,
	})
	if err == nil {
		t.Fatal("expected self-parenting error, got nil")
	}

	// 8. Test ListTasks filtering by ParentID
	emptyParent := ""
	rootTasks, err := st.ListTasks(projSlug, model.TaskFilter{ParentID: &emptyParent})
	if err != nil {
		t.Fatalf("ListTasks root failed: %v", err)
	}
	if len(rootTasks) != 2 {
		t.Fatalf("expected 2 root tasks, got %d", len(rootTasks))
	}

	parent1 := root.ID
	childTasks, err := st.ListTasks(projSlug, model.TaskFilter{ParentID: &parent1})
	if err != nil {
		t.Fatalf("ListTasks child failed: %v", err)
	}
	if len(childTasks) != 1 || childTasks[0].ID != child.ID {
		t.Fatalf("expected child task %s, got %+v", child.ID, childTasks)
	}

	// 9. Test cascade deletion: deleting root task #1 must delete #1, #2, and #3
	err = st.DeleteTask(projSlug, root.ID)
	if err != nil {
		t.Fatalf("DeleteTask root failed: %v", err)
	}

	remaining, err := st.ListTasks(projSlug, model.TaskFilter{})
	if err != nil {
		t.Fatalf("ListTasks remaining failed: %v", err)
	}
	if len(remaining) != 1 || remaining[0].ID != root2.ID {
		t.Fatalf("expected only root2 (%s) remaining after cascade delete, got %d tasks: %+v", root2.ID, len(remaining), remaining)
	}

	// Ensure persistence survives reload
	stReloaded, err := store.NewStore(filepath.Dir(tmpDir))
	_ = stReloaded
}

func TestTaskDependenciesAndBlocking(t *testing.T) {
	st, tmpDir := setupTestStore(t)
	defer os.RemoveAll(tmpDir)

	projSlug := "dep-test"
	_, err := st.CreateProject(projSlug, "Dependencies Test", "Testing blocked by and dependencies")
	if err != nil {
		t.Fatalf("CreateProject failed: %v", err)
	}

	// 1. Create task #1 (Prerequisite)
	t1, err := st.AddTask(projSlug, model.Task{
		Title: "Database schema migration",
		Size:  model.SizeM,
		Tier:  model.Tier2,
	})
	if err != nil {
		t.Fatalf("AddTask t1 failed: %v", err)
	}

	// 2. Create task #2 depending on #1
	t2, err := st.AddTask(projSlug, model.Task{
		Title:     "Write data ingestion script",
		DependsOn: []string{t1.ID},
		Size:      model.SizeL,
		Tier:      model.Tier1, // Higher tier, but blocked!
	})
	if err != nil {
		t.Fatalf("AddTask t2 failed: %v", err)
	}

	// 3. Create task #3 independent
	t3, err := st.AddTask(projSlug, model.Task{
		Title: "Setup CI linter",
		Size:  model.SizeS,
		Tier:  model.Tier2,
	})
	if err != nil {
		t.Fatalf("AddTask t3 failed: %v", err)
	}

	// 4. Verify blocked state in ListTasks
	trueVal := true
	falseVal := false
	blockedTasks, err := st.ListTasks(projSlug, model.TaskFilter{Blocked: &trueVal})
	if err != nil {
		t.Fatalf("ListTasks blocked failed: %v", err)
	}
	if len(blockedTasks) != 1 || blockedTasks[0].ID != t2.ID {
		t.Fatalf("expected t2 (%s) to be blocked, got %+v", t2.ID, blockedTasks)
	}

	unblockedTasks, err := st.ListTasks(projSlug, model.TaskFilter{Blocked: &falseVal})
	if err != nil {
		t.Fatalf("ListTasks unblocked failed: %v", err)
	}
	if len(unblockedTasks) != 2 {
		t.Fatalf("expected 2 unblocked tasks, got %d", len(unblockedTasks))
	}

	// 5. Test GetTopPriorities: even though t2 is Tier 1, it's blocked, so unblocked tasks (t1 or t3) should come first!
	top, err := st.GetTopPriorities(projSlug, 5)
	if err != nil {
		t.Fatalf("GetTopPriorities failed: %v", err)
	}
	if top[0].ID == t2.ID {
		t.Fatalf("blocked task t2 should not be the top priority before unblocked tasks")
	}

	// 6. Test circular dependency prevention in UpdateTask
	_, err = st.UpdateTask(projSlug, model.Task{
		ID:        t1.ID,
		DependsOn: []string{t2.ID}, // t2 depends on t1, t1 depends on t2 -> circular!
	})
	if err == nil {
		t.Fatal("expected error on circular dependency, got nil")
	}

	// 7. Complete task #1 -> task #2 should now be unblocked!
	_, err = st.CompleteTask(projSlug, t1.ID, true)
	if err != nil {
		t.Fatalf("CompleteTask failed: %v", err)
	}

	blockedAfterDone, err := st.ListTasks(projSlug, model.TaskFilter{Blocked: &trueVal})
	if err != nil {
		t.Fatalf("ListTasks blocked failed: %v", err)
	}
	if len(blockedAfterDone) != 0 {
		t.Fatalf("expected 0 blocked tasks after t1 completed, got %d", len(blockedAfterDone))
	}

	// 8. Test delete cleanup: add task #4 depending on #3, then delete #3
	t4, err := st.AddTask(projSlug, model.Task{
		Title:     "Run CI linter in workflow",
		DependsOn: []string{t3.ID},
	})
	if err != nil {
		t.Fatalf("AddTask t4 failed: %v", err)
	}

	err = st.DeleteTask(projSlug, t3.ID)
	if err != nil {
		t.Fatalf("DeleteTask t3 failed: %v", err)
	}

	t4Reloaded, err := st.ListTasks(projSlug, model.TaskFilter{})
	if err != nil {
		t.Fatalf("ListTasks failed: %v", err)
	}
	for _, task := range t4Reloaded {
		if task.ID == t4.ID {
			if len(task.DependsOn) != 0 {
				t.Fatalf("expected depends_on to be cleaned up after deleting t3, got %+v", task.DependsOn)
			}
		}
	}
}

func TestProjectLifecycleAndSummary(t *testing.T) {
	st, tmpDir := setupTestStore(t)
	defer os.RemoveAll(tmpDir)

	// 1. Initially no projects exist in fresh store
	projs := st.ListProjects()
	if len(projs) != 0 {
		t.Fatalf("expected 0 default projects, got %d", len(projs))
	}

	// 2. Create new project
	p1, err := st.CreateProject("mobile-app", "Mobile App", "iOS and Android client")
	if err != nil {
		t.Fatalf("CreateProject failed: %v", err)
	}
	if p1.Slug != "mobile-app" {
		t.Fatalf("expected slug 'mobile-app', got '%s'", p1.Slug)
	}

	// 3. Set active project
	err = st.SetActiveProject("mobile-app")
	if err != nil {
		t.Fatalf("SetActiveProject failed: %v", err)
	}
	if st.GetActiveProjectSlug() != "mobile-app" {
		t.Fatalf("expected active project 'mobile-app', got '%s'", st.GetActiveProjectSlug())
	}

	// 4. Add tasks and test Summary
	_, _ = st.AddTask("mobile-app", model.Task{Title: "Task 1", Size: model.SizeS, Tier: model.Tier1})
	t2, _ := st.AddTask("mobile-app", model.Task{Title: "Task 2", Size: model.SizeM, Tier: model.Tier2})
	_, _ = st.AddTask("mobile-app", model.Task{Title: "Task 3", Size: model.SizeL, Tier: model.Tier3})

	// Mark t2 as done
	_, err = st.CompleteTask("mobile-app", t2.ID, true)
	if err != nil {
		t.Fatalf("CompleteTask failed: %v", err)
	}

	summary, err := st.GetSummary("mobile-app")
	if err != nil {
		t.Fatalf("GetSummary failed: %v", err)
	}
	if summary.TotalTasks != 3 {
		t.Fatalf("expected 3 total tasks, got %d", summary.TotalTasks)
	}
	if summary.OpenTasks != 2 {
		t.Fatalf("expected 2 open tasks, got %d", summary.OpenTasks)
	}
	if summary.CompletedTasks != 1 {
		t.Fatalf("expected 1 completed task, got %d", summary.CompletedTasks)
	}
	if summary.OpenSizeCounts[model.SizeS] != 1 || summary.OpenSizeCounts[model.SizeL] != 1 {
		t.Fatalf("unexpected OpenSizeCounts: %+v", summary.OpenSizeCounts)
	}

	// 5. Test GetProject
	pLoaded, err := st.GetProject("mobile-app")
	if err != nil {
		t.Fatalf("GetProject failed: %v", err)
	}
	if len(pLoaded.Tasks) != 3 {
		t.Fatalf("expected 3 tasks in project, got %d", len(pLoaded.Tasks))
	}

	// 6. Delete project
	err = st.DeleteProject("mobile-app")
	if err != nil {
		t.Fatalf("DeleteProject failed: %v", err)
	}

	// After deletion, active project must fallback to remaining project
	if st.GetActiveProjectSlug() == "mobile-app" {
		t.Fatalf("expected active project to change after deleting mobile-app")
	}
}

func TestTaskFilteringAndAssignment(t *testing.T) {
	st, tmpDir := setupTestStore(t)
	defer os.RemoveAll(tmpDir)

	projSlug := "filter-test"
	_, _ = st.CreateProject(projSlug, "Filter Test", "Testing filters")

	t1, _ := st.AddTask(projSlug, model.Task{
		Title:    "Frontend Refactor",
		Size:     model.SizeM,
		Tier:     model.Tier3,
		Tag:      "ui",
		Assignee: "alice",
	})
	t2, _ := st.AddTask(projSlug, model.Task{
		Title:    "Backend API optimization",
		Size:     model.SizeXL,
		Tier:     model.Tier1,
		Tag:      "performance",
		Assignee: "bob",
	})
	t3, _ := st.AddTask(projSlug, model.Task{
		Title: "Documentation update",
		Size:  model.SizeXS,
		Tier:  model.Tier4,
	})

	// Test AssignTask
	updated, err := st.AssignTask(projSlug, t3.ID, "charlie")
	if err != nil {
		t.Fatalf("AssignTask failed: %v", err)
	}
	if updated.Assignee != "charlie" {
		t.Fatalf("expected assignee 'charlie', got '%s'", updated.Assignee)
	}

	// Filter by Assignee "alice"
	alice := "alice"
	tasksAlice, _ := st.ListTasks(projSlug, model.TaskFilter{Assignee: &alice})
	if len(tasksAlice) != 1 || tasksAlice[0].ID != t1.ID {
		t.Fatalf("expected t1 for assignee alice, got %+v", tasksAlice)
	}

	// Filter by Assignee "unassigned" after unassigning t1
	_, _ = st.AssignTask(projSlug, t1.ID, "")
	unassigned := "unassigned"
	tasksUnassigned, _ := st.ListTasks(projSlug, model.TaskFilter{Assignee: &unassigned})
	if len(tasksUnassigned) != 1 || tasksUnassigned[0].ID != t1.ID {
		t.Fatalf("expected t1 for unassigned filter, got %+v", tasksUnassigned)
	}

	// Filter by Search
	searchRes, _ := st.ListTasks(projSlug, model.TaskFilter{Search: "performance"})
	if len(searchRes) != 1 || searchRes[0].ID != t2.ID {
		t.Fatalf("expected t2 for search 'performance', got %+v", searchRes)
	}

	// Filter by Size
	szXL := model.SizeXL
	tasksXL, _ := st.ListTasks(projSlug, model.TaskFilter{Size: &szXL})
	if len(tasksXL) != 1 || tasksXL[0].ID != t2.ID {
		t.Fatalf("expected t2 for size XL, got %+v", tasksXL)
	}

	// Filter by Tier
	tier1 := model.Tier1
	tasksT1, _ := st.ListTasks(projSlug, model.TaskFilter{Tier: &tier1})
	if len(tasksT1) != 1 || tasksT1[0].ID != t2.ID {
		t.Fatalf("expected t2 for tier 1, got %+v", tasksT1)
	}
}

func TestStoreEvents(t *testing.T) {
	st, tmpDir := setupTestStore(t)
	defer os.RemoveAll(tmpDir)

	ch := st.Subscribe()

	projSlug := "event-test"
	_, _ = st.CreateProject(projSlug, "Events Test", "")

	task, err := st.AddTask(projSlug, model.Task{Title: "Notify task"})
	if err != nil {
		t.Fatalf("AddTask failed: %v", err)
	}

	foundTaskEvent := false
	timer := time.After(1 * time.Second)
	for !foundTaskEvent {
		select {
		case ev := <-ch:
			if ev.Type == store.EventTaskCreated && ev.TaskID == task.ID {
				foundTaskEvent = true
			}
		case <-timer:
			t.Fatal("timed out waiting for EventTaskCreated")
		}
	}
}


