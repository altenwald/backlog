package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/altenwald/backlog/pkg/model"
	"github.com/altenwald/backlog/pkg/store"
)

func TestSettingsWindowLifecycle(t *testing.T) {
	a := test.NewApp()
	w := a.NewWindow("Test")
	tmpDir, err := os.MkdirTemp("", "backlog-ui-settings-lifecycle-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	st, err := store.NewStore(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// 1. Open settings window
	ShowSettingsDialog(w, st)

	activeSettingsMu.Lock()
	win := activeSettingsWindow
	activeSettingsMu.Unlock()

	if win == nil {
		t.Fatal("expected activeSettingsWindow to be non-nil")
	}

	// 2. Calling again while open focuses existing window
	ShowSettingsDialog(w, st)

	activeSettingsMu.Lock()
	win2 := activeSettingsWindow
	activeSettingsMu.Unlock()

	if win2 != win {
		t.Fatal("expected activeSettingsWindow to be the same instance")
	}

	// 3. Find content objects and test interactions
	content := win.Content()
	if content == nil {
		t.Fatal("expected window content to be non-nil")
	}

	// 4. Close the window and verify activeSettingsWindow is reset
	win.Close()

	activeSettingsMu.Lock()
	winClosed := activeSettingsWindow
	activeSettingsMu.Unlock()

	if winClosed != nil {
		t.Fatal("expected activeSettingsWindow to be nil after close")
	}

	// 5. Open again and test saving custom instructions
	ShowSettingsDialog(w, st)
	activeSettingsMu.Lock()
	win3 := activeSettingsWindow
	activeSettingsMu.Unlock()
	if win3 == nil {
		t.Fatal("expected fresh activeSettingsWindow to be created")
	}

	// Test store saving via st
	customText := "Follow strict TDD and review rules."
	if err := st.SaveMCPUserInstructions(customText); err != nil {
		t.Fatalf("failed to save instructions: %v", err)
	}
	if got := st.GetMCPUserInstructions(); got != customText {
		t.Fatalf("expected %q, got %q", customText, got)
	}

	win3.Close()
}

func TestSettingsWindowSizeAndLayout(t *testing.T) {
	a := test.NewApp()
	w := a.NewWindow("Test")
	tmpDir, err := os.MkdirTemp("", "backlog-ui-settings-size-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	st, err := store.NewStore(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	ShowSettingsDialog(w, st)

	activeSettingsMu.Lock()
	win := activeSettingsWindow
	activeSettingsMu.Unlock()
	if win == nil {
		t.Fatal("expected window")
	}

	c := win.Content()
	if c.Size().Width < 600 || c.Size().Height < 500 {
		t.Fatalf("expected content size >= 600x500, got %+v", c.Size())
	}

	win.Close()
}

func TestAboutWindowLifecycle(t *testing.T) {
	a := test.NewApp()
	w := a.NewWindow("Test")

	// 1. Open About window
	ShowAboutDialog(w)

	activeAboutMu.Lock()
	win := activeAboutWindow
	activeAboutMu.Unlock()

	if win == nil {
		t.Fatal("expected activeAboutWindow to be non-nil")
	}
	if win.Title() != "About Backlog" {
		t.Fatalf("expected title 'About Backlog', got %q", win.Title())
	}

	// 2. Calling again while open focuses existing window
	ShowAboutDialog(w)

	activeAboutMu.Lock()
	win2 := activeAboutWindow
	activeAboutMu.Unlock()

	if win2 != win {
		t.Fatal("expected activeAboutWindow to be the same instance")
	}

	// 3. Verify content
	content := win.Content()
	if content == nil {
		t.Fatal("expected window content to be non-nil")
	}

	// 4. Close the window and verify activeAboutWindow is reset
	win.Close()

	activeAboutMu.Lock()
	winClosed := activeAboutWindow
	activeAboutMu.Unlock()

	if winClosed != nil {
		t.Fatal("expected activeAboutWindow to be nil after close")
	}

	// 5. Open again with nil parent (e.g. when main window is hidden or called independently)
	ShowAboutDialog(nil)
	activeAboutMu.Lock()
	win3 := activeAboutWindow
	activeAboutMu.Unlock()
	if win3 == nil {
		t.Fatal("expected fresh activeAboutWindow to be created with nil parent")
	}
	win3.Close()
}

func TestOtherDialogs(t *testing.T) {
	a := test.NewApp()
	w := a.NewWindow("Test")

	// Test ShowAddTaskDialog
	savedTask := false
	ShowAddTaskDialog(w, "my-project", func(task model.Task) {
		savedTask = true
	})

	// Test ShowEditTaskDialog
	editedTask := false
	task := model.Task{
		ID:    "1",
		Title: "Test Task",
		Size:  model.SizeM,
		Tier:  model.Tier2,
	}
	ShowEditTaskDialog(w, task, func(updated model.Task) {
		editedTask = true
	})

	// Test ShowDeleteProjectDialog
	deletedProject := false
	ShowDeleteProjectDialog(w, "My Project", "my-project", func() {
		deletedProject = true
	})

	_ = savedTask
	_ = editedTask
	_ = deletedProject
}

func findSpecLink(segs []widget.RichTextSegment, text string) *widget.HyperlinkSegment {
	for _, seg := range segs {
		switch s := seg.(type) {
		case *widget.HyperlinkSegment:
			if strings.HasPrefix(s.Text, text) {
				return s
			}
		case *widget.ParagraphSegment:
			if l := findSpecLink(s.Texts, text); l != nil {
				return l
			}
		case *widget.ListSegment:
			if l := findSpecLink(s.Items, text); l != nil {
				return l
			}
		}
	}
	return nil
}

func TestSpecViewWiki(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "backlog-ui-spec-wiki-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	st, err := store.NewStore(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateProject("test-proj", "Test Project", "Description"); err != nil {
		t.Fatal(err)
	}
	_ = st.SetActiveProject("test-proj")
	_ = st.UpdateProjectSpecification("test-proj", "Intro, see [the plan](spec:plan)\n\n## Scope\n\nTicket #1")

	a := test.NewApp()
	w := a.NewWindow("Test")
	v := NewSpecView(st, w)

	// The main page opens first; split pages are linked from it.
	v.Refresh()
	if v.selectedID != model.SpecMainID || len(v.index) != 2 {
		t.Fatalf("unexpected initial state: selected=%q index=%+v", v.selectedID, v.index)
	}

	// Following a link opens the page, and Back returns.
	link := findSpecLink(v.richText.Segments, "Scope")
	if link == nil {
		t.Fatal("expected a link to the scope page")
	}
	link.OnTapped()
	if v.selectedID != "scope" || v.bodyEntry.Text != "Ticket #1" {
		t.Fatalf("expected scope page, got %q / %q", v.selectedID, v.bodyEntry.Text)
	}
	if findSpecLink(v.richText.Segments, model.SpecMainTitle) == nil {
		t.Fatal("expected a 'Linked from' backlink to the main page")
	}
	v.Back()
	if v.selectedID != model.SpecMainID {
		t.Fatalf("expected Back to return to main, got %q", v.selectedID)
	}

	// A link to a missing page is marked and creates it with the linked id.
	missing := findSpecLink(v.richText.Segments, "the plan")
	if missing == nil || !strings.HasSuffix(missing.Text, specMissingSuffix) {
		t.Fatalf("expected a missing-page link, got %+v", missing)
	}
	v.createPage("plan", "The plan")
	if v.selectedID != "plan" || !v.editMode {
		t.Fatalf("expected new page open in edit mode, got %q edit=%v", v.selectedID, v.editMode)
	}

	// Edits are saved to the open page only, with an inserted link.
	v.bodyEntry.SetText("Steps: ")
	v.bodyEntry.CursorRow, v.bodyEntry.CursorColumn = 0, 7
	insertAtCursor(v.bodyEntry, model.SpecLink("Scope", "scope"))
	v.saveBtn.OnTapped()
	pages, _ := st.GetSpecSections("test-proj", []string{"plan"})
	if pages[0].Body != "Steps: [Scope](spec:scope)" || v.modified {
		t.Fatalf("unexpected saved page %+v (modified=%v)", pages[0], v.modified)
	}

	// External updates of the open page are reflected.
	body := "Updated via MCP"
	if _, err := st.UpdateSpecSection("test-proj", "plan", nil, &body); err != nil {
		t.Fatal(err)
	}
	v.Refresh()
	if v.bodyEntry.Text != body || v.status.Text != "Updated" {
		t.Fatalf("expected external update, got %q (status %q)", v.bodyEntry.Text, v.status.Text)
	}

	// An unreachable page is flagged; deleting the open page falls back to main.
	if _, err := st.AddSpecSection("test-proj", "", "Orphan", "", -1); err != nil {
		t.Fatal(err)
	}
	v.Refresh()
	if !v.unlinked["orphan"] {
		t.Fatalf("expected orphan page to be unlinked, got %v", v.unlinked)
	}
	_ = st.DeleteSpecSection("test-proj", "plan")
	v.Refresh()
	if v.selectedID != model.SpecMainID {
		t.Fatalf("expected fallback to main, got %q", v.selectedID)
	}
}

func TestDeleteActiveProjectAndSwitchToEmptySpecProject(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "backlog-ui-crash-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	st, err := store.NewStore(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Create legacy project p1 without specification
	_, err = st.CreateProject("p1", "Project 1", "Legacy project without specification")
	if err != nil {
		t.Fatal(err)
	}

	// Create project p2
	_, err = st.CreateProject("p2", "Project 2", "Second project")
	if err != nil {
		t.Fatal(err)
	}

	// Add tasks to p1 and p2
	_, _ = st.AddTask("p1", model.Task{ID: "1", Title: "Task 1 in p1", Description: "Description 1"})
	_, _ = st.AddTask("p2", model.Task{ID: "1", Title: "Task 1 in p2", Description: "Description 2"})

	_ = st.SetActiveProject("p2")

	a := test.NewApp()
	w := a.NewWindow("Test")
	bApp := &BacklogApp{
		fyneApp: a,
		window:  w,
		store:   st,
	}

	bApp.buildUI()

	// Scenario 1: Switch to p1 (legacy project without specification)
	for _, p := range st.ListProjects() {
		if p.Name == "Project 1" {
			_ = st.SetActiveProject(p.Slug)
		}
	}
	bApp.refreshAll()

	// Scenario 2: Switch back to p2, set active, then delete p2
	for _, p := range st.ListProjects() {
		if p.Name == "Project 2" {
			_ = st.SetActiveProject(p.Slug)
		}
	}
	bApp.refreshAll()

	// Delete p2 while it's active
	err = st.DeleteProject("p2")
	if err != nil {
		t.Fatal(err)
	}
	bApp.refreshAll()

	// Delete p1 as well (now zero projects)
	err = st.DeleteProject("p1")
	if err != nil {
		t.Fatal(err)
	}
	bApp.refreshAll()
}

func TestRealUserDataProjects(t *testing.T) {
	home, _ := os.UserHomeDir()
	realConfigDir := filepath.Join(home, ".config", "backlog")
	if _, err := os.Stat(filepath.Join(realConfigDir, "config.json")); os.IsNotExist(err) {
		t.Skip("skipping test: no JSON data in ~/.config/backlog (not found or already migrated to SQLite)")
	}

	tmpDir, err := os.MkdirTemp("", "backlog-user-data-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Copy config and projects
	_ = os.MkdirAll(filepath.Join(tmpDir, "projects"), 0755)
	configData, _ := os.ReadFile(filepath.Join(realConfigDir, "config.json"))
	_ = os.WriteFile(filepath.Join(tmpDir, "config.json"), configData, 0644)

	entries, _ := os.ReadDir(filepath.Join(realConfigDir, "projects"))
	for _, e := range entries {
		data, _ := os.ReadFile(filepath.Join(realConfigDir, "projects", e.Name()))
		_ = os.WriteFile(filepath.Join(tmpDir, "projects", e.Name()), data, 0644)
	}

	st, err := store.NewStore(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	a := test.NewApp()
	a.Settings().SetTheme(theme.DefaultTheme())
	w := a.NewWindow("Real User Test")
	bApp := &BacklogApp{
		fyneApp: a,
		window:  w,
		store:   st,
	}

	bApp.buildUI()

	// Switch to books
	for _, p := range st.ListProjects() {
		if p.Name == "Altenwald Books" {
			_ = st.SetActiveProject(p.Slug)
		}
	}
	bApp.refreshAll()

	// Click through all tasks in books to render their markdown descriptions and resolutions
	for i := range bApp.displayedTasks {
		bApp.tasksList.Select(i)
	}

	// Switch to Conta
	for _, p := range st.ListProjects() {
		if p.Name == "Conta" {
			_ = st.SetActiveProject(p.Slug)
		}
	}
	bApp.refreshAll()

	// Switch back to books (legacy, no spec)
	for _, p := range st.ListProjects() {
		if p.Name == "Altenwald Books" {
			_ = st.SetActiveProject(p.Slug)
		}
	}
	bApp.refreshAll()

	// Create p2, set active, delete p2
	_, err = st.CreateProject("p2", "p2", "Test Project 2")
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range st.ListProjects() {
		if p.Name == "p2" {
			_ = st.SetActiveProject(p.Slug)
		}
	}
	bApp.refreshAll()

	err = st.DeleteProject("p2")
	if err != nil {
		t.Fatal(err)
	}
	bApp.refreshAll()
}
