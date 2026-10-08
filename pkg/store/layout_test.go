package store

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/altenwald/backlog/pkg/model"
)

func TestSplitSQLitePreservesProjectsAndSource(t *testing.T) {
	dir := t.TempDir()
	old, e := newProjectDB(dir)
	if e != nil {
		t.Fatal(e)
	}
	for _, slug := range []string{"alpha", "beta"} {
		if _, e = old.CreateProject(slug, slug, "description"); e != nil {
			t.Fatal(e)
		}
		a, e := old.AddTask(slug, model.Task{Title: "parent", Size: model.SizeL})
		if e != nil {
			t.Fatal(e)
		}
		if _, e = old.AddTask(slug, model.Task{Title: "child", ParentID: a.ID, DependsOn: []string{a.ID}}); e != nil {
			t.Fatal(e)
		}
		if e = old.UpdateProjectSpecification(slug, "# Plan\n\n## Scope\nOriginal text"); e != nil {
			t.Fatal(e)
		}
	}
	_ = old.SetActiveProject("beta")
	_ = old.SaveMCPUserInstructions("keep this")
	_, e = old.db.Exec(`UPDATE projects SET next_task_id=100 WHERE slug='alpha'`)
	if e != nil {
		t.Fatal(e)
	}
	originals := map[string]*model.Project{}
	for _, slug := range []string{"alpha", "beta"} {
		originals[slug], e = old.getFullProject(slug)
		if e != nil {
			t.Fatal(e)
		}
	}
	old.Close()
	source, _ := os.ReadFile(filepath.Join(dir, "backlog.db"))
	st, e := NewStore(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer st.Close()
	if st.GetActiveProjectSlug() != "beta" || st.GetMCPUserInstructions() != "keep this" {
		t.Fatal("settings lost")
	}
	paths := map[string]bool{}
	for slug, want := range originals {
		d, e := st.openProject(slug)
		if e != nil {
			t.Fatal(e)
		}
		got, e := d.getFullProject(slug)
		d.Close()
		if e != nil {
			t.Fatal(e)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("project changed: %+v != %+v", got, want)
		}
		var path string
		_ = st.db.QueryRow(`SELECT path FROM project_files WHERE slug=?`, slug).Scan(&path)
		if paths[path] {
			t.Fatal("projects share a file")
		}
		paths[path] = true
	}
	now, _ := os.ReadFile(filepath.Join(dir, "backlog.db"))
	if string(source) != string(now) {
		t.Fatal("source modified")
	}
	task, e := st.AddTask("alpha", model.Task{Title: "next"})
	if e != nil || task.ID != "100" {
		t.Fatalf("counter lost: %+v %v", task, e)
	}
	reopened, e := NewStore(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer reopened.Close()
	tasks, e := reopened.ListTasks("alpha", model.TaskFilter{})
	if e != nil || len(tasks) != 3 {
		t.Fatalf("reimported source: %v %v", tasks, e)
	}
}
func TestLayoutFailureLeavesNoActivatedProjects(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "projects"), 0700)
	valid := `{"slug":"good","name":"Good","tasks":[]}`
	bad := `{"slug":"bad","name":"Bad","tasks":[{"id":"1","title":"orphan","parent_id":"99"}]}`
	os.WriteFile(filepath.Join(dir, "projects", "a.json"), []byte(valid), 0600)
	os.WriteFile(filepath.Join(dir, "projects", "b.json"), []byte(bad), 0600)
	if _, e := NewStore(dir); e == nil {
		t.Fatal("invalid project silently imported")
	}
	b, _ := os.ReadFile(filepath.Join(dir, "projects", "b.json"))
	if string(b) != bad {
		t.Fatal("original changed")
	}
	db, e := openDB(filepath.Join(dir, "catalog.db"))
	if e != nil {
		t.Fatal(e)
	}
	var n int
	_ = db.QueryRow(`SELECT count(*) FROM project_files`).Scan(&n)
	db.Close()
	if n != 0 {
		t.Fatal("partial migration activated")
	}
	os.WriteFile(filepath.Join(dir, "projects", "b.json"), []byte(`{"slug":"bad","name":"Repaired","tasks":[]}`), 0600)
	st, e := NewStore(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer st.Close()
	if len(st.ListProjects()) != 2 {
		t.Fatal("cannot resume migration")
	}
}
func TestStandaloneProjectTransfer(t *testing.T) {
	st, e := NewStore(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer st.Close()
	st.CreateProject("one", "One", "")
	st.CreateProject("private", "Private", "")
	st.AddTask("one", model.Task{Title: "transferred"})
	st.SetProjectOpen("one", true)
	path := filepath.Join(t.TempDir(), "one.db")
	if e = st.ProjectBackup("one", path); e != nil {
		t.Fatal(e)
	}
	other, e := NewStore(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer other.Close()
	p, e := other.ImportProjectDB(path)
	if e != nil {
		t.Fatal(e)
	}
	if p.Open || len(p.Tasks) != 1 || len(other.ListProjects()) != 1 {
		t.Fatalf("bad import: %+v", p)
	}
	if _, e = other.ImportProjectDB(path); e == nil {
		t.Fatal("duplicate import replaced data")
	}
}
func rawArgs(v ...any) []json.RawMessage {
	out := []json.RawMessage{}
	for _, a := range v {
		b, _ := json.Marshal(a)
		out = append(out, b)
	}
	return out
}
func TestRemoteCASPermissionAndReceipt(t *testing.T) {
	st, e := NewStore(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer st.Close()
	st.CreateProject("p", "P", "")
	a, _ := st.AddTask("p", model.Task{Title: "original"})
	args := rawArgs("p", model.TaskUpdate{ID: a.ID, Title: "remote"})
	pre := Precondition{UpdatedAt: a.UpdatedAt, RequestID: "client:1"}
	if _, e = st.RemoteCall("UpdateTask", args, pre); !errors.Is(e, ErrClosed) {
		t.Fatalf("closed: %v", e)
	}
	st.SetProjectOpen("p", true)
	if _, e = st.RemoteCall("UpdateTask", args, pre); e != nil {
		t.Fatal(e)
	}
	if _, e = st.RemoteCall("UpdateTask", args, pre); !errors.Is(e, ErrAlreadyApplied) {
		t.Fatalf("duplicate: %v", e)
	}
	pre.RequestID = "client:2"
	if _, e = st.RemoteCall("UpdateTask", args, pre); !errors.Is(e, ErrStale) {
		t.Fatalf("stale: %v", e)
	}
	pre.UpdatedAt = time.Time{}
	if _, e = st.RemoteCall("UpdateTask", args, pre); !errors.Is(e, ErrStale) {
		t.Fatalf("missing precondition: %v", e)
	}
	p, _ := st.GetProject("p")
	if p.Tasks[0].Title != "remote" {
		t.Fatal("stale write applied")
	}
	if _, e = st.RemoteCall("DeleteProject", rawArgs("p"), pre); e == nil {
		t.Fatal("remote catalog write allowed")
	}
}
func TestRemoteSpecVersionAndEmptyMain(t *testing.T) {
	st, _ := NewStore(t.TempDir())
	defer st.Close()
	st.CreateProject("p", "P", "")
	st.SetProjectOpen("p", true)
	title, body := "Main", "first"
	a := rawArgs("p", "main", &title, &body, []time.Time{{}})
	if _, e := st.RemoteCall("UpdateSpecSection", a, Precondition{}); e != nil {
		t.Fatal(e)
	}
	if _, e := st.RemoteCall("UpdateSpecSection", a, Precondition{}); !errors.Is(e, ErrStale) {
		t.Fatal(e)
	}
}
func TestWatchOverflowAndCancellation(t *testing.T) {
	st, _ := NewStore(t.TempDir())
	defer st.Close()
	ch, cancel := st.Watch()
	for i := 0; i < 100; i++ {
		st.Publish(Event{Type: EventTaskUpdated})
	}
	found := false
	for len(ch) > 0 {
		if (<-ch).Type == "resync" {
			found = true
		}
	}
	if !found {
		t.Fatal("no resync on overflow")
	}
	cancel()
	cancel()
	st.Publish(Event{})
	if _, ok := <-ch; ok {
		t.Fatal("watch not closed")
	}
}
func TestOnlyOneConcurrentCASWins(t *testing.T) {
	st, _ := NewStore(t.TempDir())
	defer st.Close()
	st.CreateProject("p", "P", "")
	a, _ := st.AddTask("p", model.Task{Title: "v1"})
	st.SetProjectOpen("p", true)
	result := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() {
			_, e := st.RemoteCall("UpdateTask", rawArgs("p", model.TaskUpdate{ID: a.ID, Title: "next"}), Precondition{UpdatedAt: a.UpdatedAt})
			result <- e
		}()
	}
	wins, stales := 0, 0
	for i := 0; i < 2; i++ {
		e := <-result
		if e == nil {
			wins++
		} else if errors.Is(e, ErrStale) {
			stales++
		} else {
			t.Fatal(e)
		}
	}
	if wins != 1 || stales != 1 {
		t.Fatalf("wins %d stale %d", wins, stales)
	}
}

func TestMigrateOriginalSQLiteSchemaV2(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, dbFileName)
	all := migrations
	migrations = all[:2]
	db, e := openDB(path)
	migrations = all
	if e != nil {
		t.Fatal(e)
	}
	_, e = db.Exec(`INSERT INTO projects(slug,name,next_task_id,inserted_at,updated_at) VALUES('old','Old',42,'2025-01-01T00:00:00Z','2025-01-01T00:00:00Z')`)
	if e != nil {
		t.Fatal(e)
	}
	db.Close()
	before, _ := os.ReadFile(path)
	st, e := NewStore(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer st.Close()
	p, e := st.GetProject("old")
	if e != nil || p.Open {
		t.Fatalf("old project: %+v %v", p, e)
	}
	task, e := st.AddTask("old", model.Task{Title: "after migration"})
	if e != nil || task.ID != "42" {
		t.Fatalf("counter: %+v %v", task, e)
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatal("old SQLite source was modified")
	}
}
func TestCascadeChangesInvalidateDependentVersion(t *testing.T) {
	st, _ := NewStore(t.TempDir())
	defer st.Close()
	st.CreateProject("p", "P", "")
	parent, _ := st.AddTask("p", model.Task{Title: "parent"})
	child, _ := st.AddTask("p", model.Task{Title: "child", ParentID: parent.ID})
	dependent, _ := st.AddTask("p", model.Task{Title: "dependent", DependsOn: []string{child.ID}})
	if e := st.DeleteTask("p", parent.ID); e != nil {
		t.Fatal(e)
	}
	if _, e := st.UpdateTask("p", model.TaskUpdate{ID: dependent.ID, Title: "old edit", ExpectedUpdatedAt: &dependent.UpdatedAt}); !errors.Is(e, ErrStale) {
		t.Fatalf("dependency change did not invalidate old version: %v", e)
	}
}
func TestWatchCancelAfterClose(t *testing.T) {
	st, _ := NewStore(t.TempDir())
	_, cancel := st.Watch()
	st.Close()
	cancel()
	st.Close()
}

// Build the old monolithic layout solely as a migration fixture.
func newProjectDB(dir string) (*projectDB, error) {
	db, e := openDB(filepath.Join(dir, dbFileName))
	if e != nil {
		return nil, e
	}
	return &projectDB{db: db}, nil
}
