package model_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/altenwald/backlog/pkg/model"
)

func TestSplitSpecBuildsLinkedMainPage(t *testing.T) {
	text := "Intro with [design](spec:architecture)\n\n## Architecture\n\nSee [scope](spec:scope)\n\n## Scope\n\n#1"
	pages := model.SplitSpec(text, time.Time{})
	if len(pages) != 3 || pages[0].ID != model.SpecMainID {
		t.Fatalf("unexpected pages: %+v", pages)
	}
	// Only pages not already linked from main are appended to it.
	if pages[0].Body != "Intro with [design](spec:architecture)\n\n- [Scope](spec:scope)" {
		t.Fatalf("unexpected main body %q", pages[0].Body)
	}
	if !reflect.DeepEqual(pages[0].Links, []string{"architecture", "scope"}) || !reflect.DeepEqual(pages[1].Links, []string{"scope"}) {
		t.Fatalf("unexpected links: %v / %v", pages[0].Links, pages[1].Links)
	}
	// Joining and splitting again is stable.
	again := model.SplitSpec(model.JoinSpec(pages), time.Time{})
	if model.JoinSpec(again) != model.JoinSpec(pages) {
		t.Fatalf("round trip changed the spec:\n%s\n---\n%s", model.JoinSpec(pages), model.JoinSpec(again))
	}
}

func TestNormalizeSpecMigratesOverview(t *testing.T) {
	pages := model.NormalizeSpec([]model.SpecSection{
		{ID: "overview", Title: "Overview", Body: "Intro"},
		{ID: "scope", Title: "Scope", Body: "#1"},
	}, time.Time{})
	if pages[0].ID != model.SpecMainID || pages[0].Body != "Intro\n\n- [Scope](spec:scope)" {
		t.Fatalf("unexpected main page: %+v", pages[0])
	}
}

func TestSpecGraph(t *testing.T) {
	index := []model.SpecSectionInfo{
		{ID: model.SpecMainID, Links: []string{"a"}},
		{ID: "a", Links: []string{"b", model.SpecMainID}},
		{ID: "b", Links: []string{"a"}},
		{ID: "orphan", Links: []string{"a"}},
	}
	if got := model.SpecUnreachable(index); !reflect.DeepEqual(got, map[string]bool{"orphan": true}) {
		t.Fatalf("unexpected unreachable pages: %v", got)
	}
	var back []string
	for _, info := range model.SpecBacklinks(index, "a") {
		back = append(back, info.ID)
	}
	if !reflect.DeepEqual(back, []string{model.SpecMainID, "b", "orphan"}) {
		t.Fatalf("unexpected backlinks: %v", back)
	}
}
