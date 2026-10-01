package store_test

import (
	"os"
	"testing"

	"github.com/altenwald/backlog/pkg/store"
)

func TestSpecSections(t *testing.T) {
	st, tmpDir := setupTestStore(t)
	defer os.RemoveAll(tmpDir)

	if _, err := st.CreateProject("sec", "Sections", ""); err != nil {
		t.Fatal(err)
	}
	if err := st.UpdateProjectSpecification("sec", "Intro\n\n## Architecture\n\nGo + Fyne\n\n```md\n## not a heading\n```\n\n## Scope\n\n#1"); err != nil {
		t.Fatal(err)
	}

	infos, err := st.ListSpecSections("sec")
	if err != nil {
		t.Fatal(err)
	}
	if len(infos) != 3 || infos[0].ID != "overview" || infos[1].ID != "architecture" || infos[2].ID != "scope" {
		t.Fatalf("unexpected index: %+v", infos)
	}

	added, err := st.AddSpecSection("sec", "Architecture", "duplicate title", 1)
	if err != nil {
		t.Fatal(err)
	}
	if added.ID != "architecture-2" {
		t.Fatalf("expected unique id, got %q", added.ID)
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
	if _, err := st.GetSpecSections("sec", []string{"missing"}); err == nil {
		t.Fatal("expected error for unknown section")
	}

	// Changes persist and the ID survives the rename.
	st2, err := store.NewStore(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	sections, err := st2.GetSpecSections("sec", nil)
	if err != nil {
		t.Fatal(err)
	}
	var order []string
	for _, s := range sections {
		order = append(order, s.ID+":"+s.Title)
	}
	want := []string{"scope:Scope", "overview:Overview", "architecture:Design"}
	if len(order) != len(want) {
		t.Fatalf("got %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("got %v, want %v", order, want)
		}
	}
	if sections[2].Body != "Go + Fyne\n\n```md\n## not a heading\n```" {
		t.Fatalf("code fence heading was split: %q", sections[2].Body)
	}
}
