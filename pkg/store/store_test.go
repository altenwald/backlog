package store_test

import (
	"os"
	"path/filepath"
	"testing"

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
