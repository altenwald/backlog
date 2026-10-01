package store_test

import (
	"os"
	"reflect"
	"testing"

	"github.com/altenwald/backlog/pkg/model"
	"github.com/altenwald/backlog/pkg/store"
)

func TestSpecPages(t *testing.T) {
	st, tmpDir := setupTestStore(t)
	defer os.RemoveAll(tmpDir)

	if _, err := st.CreateProject("sec", "Sections", ""); err != nil {
		t.Fatal(err)
	}

	// The main page of an empty specification is created on first write.
	intro := "Intro, see [design](spec:architecture)"
	if _, err := st.UpdateSpecSection("sec", model.SpecMainID, nil, &intro); err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddSpecSection("sec", "", "Architecture", "Go + Fyne\n\n```md\n## not a heading\n```\n\nSee [scope](spec:scope)", -1); err != nil {
		t.Fatal(err)
	}
	// A page created from a link keeps the linked id.
	if _, err := st.AddSpecSection("sec", "scope", "Alcance", "#1", -1); err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddSpecSection("sec", "scope", "Again", "", -1); err == nil {
		t.Fatal("expected duplicate id error")
	}
	if _, err := st.AddSpecSection("sec", "Bad ID", "Bad", "", -1); err == nil {
		t.Fatal("expected invalid id error")
	}
	dup, err := st.AddSpecSection("sec", "", "Architecture", "duplicate title", 0)
	if err != nil {
		t.Fatal(err)
	}
	if dup.ID != "architecture-2" {
		t.Fatalf("expected unique id, got %q", dup.ID)
	}

	infos, err := st.ListSpecSections("sec")
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, info := range infos {
		ids = append(ids, info.ID)
	}
	// Position 0 is reserved for the main page.
	if want := []string{"main", "architecture-2", "architecture", "scope"}; !reflect.DeepEqual(ids, want) {
		t.Fatalf("got %v, want %v", ids, want)
	}
	if !reflect.DeepEqual(infos[2].Links, []string{"scope"}) {
		t.Fatalf("expected links to be derived, got %v", infos[2].Links)
	}
	if got := model.SpecUnreachable(infos); !reflect.DeepEqual(got, map[string]bool{"architecture-2": true}) {
		t.Fatalf("unexpected unreachable pages: %v", got)
	}

	if err := st.DeleteSpecSection("sec", model.SpecMainID); err == nil {
		t.Fatal("expected main page deletion to fail")
	}
	if err := st.MoveSpecSection("sec", model.SpecMainID, 2); err == nil {
		t.Fatal("expected main page move to fail")
	}
	title := "Design"
	if _, err := st.UpdateSpecSection("sec", "architecture", &title, nil); err != nil {
		t.Fatal(err)
	}
	if err := st.MoveSpecSection("sec", "scope", 0); err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteSpecSection("sec", "architecture-2"); err != nil {
		t.Fatal(err)
	}

	// Changes persist and the ID survives the rename.
	st2, err := store.NewStore(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	pages, err := st2.GetSpecSections("sec", nil)
	if err != nil {
		t.Fatal(err)
	}
	var order []string
	for _, p := range pages {
		order = append(order, p.ID+":"+p.Title)
	}
	if want := []string{"main:Main", "scope:Alcance", "architecture:Design"}; !reflect.DeepEqual(order, want) {
		t.Fatalf("got %v, want %v", order, want)
	}
	if pages[2].Body != "Go + Fyne\n\n```md\n## not a heading\n```\n\nSee [scope](spec:scope)" {
		t.Fatalf("unexpected body: %q", pages[2].Body)
	}
}
