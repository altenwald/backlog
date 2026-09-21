package ui

import "github.com/altenwald/backlog/pkg/model"

type taskPlacement struct {
	Depth    int
	AnchorID string
}

// arrangeTaskOutline projects the visible dependency graph into a stable forest.
// A task appears once, under its first visible prerequisite. Other prerequisites
// remain in its metadata. ParentID is used only when no prerequisite is visible.
// Filtered-out prerequisites never suppress matching tasks or create empty rows.
func arrangeTaskOutline(tasks []model.Task) ([]model.Task, map[string]taskPlacement) {
	visible := make(map[string]bool, len(tasks))
	for _, task := range tasks {
		visible[task.ID] = true
	}
	anchors := make(map[string]string, len(tasks))
	children := make(map[string][]model.Task, len(tasks))
	var roots []model.Task
	for _, task := range tasks {
		candidates := append([]string(nil), task.DependsOn...)
		if task.ParentID != "" {
			candidates = append(candidates, task.ParentID)
		}
		for _, candidate := range candidates {
			if !visible[candidate] || candidate == task.ID {
				continue
			}
			// Imported data or mixed parent/dependency relations can contain cycles.
			cyclic := false
			for ancestor := candidate; ancestor != ""; ancestor = anchors[ancestor] {
				if ancestor == task.ID {
					cyclic = true
					break
				}
			}
			if !cyclic {
				anchors[task.ID] = candidate
				break
			}
		}
		if anchor := anchors[task.ID]; anchor != "" {
			children[anchor] = append(children[anchor], task)
		} else {
			roots = append(roots, task)
		}
	}
	ordered := make([]model.Task, 0, len(tasks))
	placements := make(map[string]taskPlacement, len(tasks))
	var visit func(model.Task, int)
	visit = func(task model.Task, depth int) {
		ordered = append(ordered, task)
		placements[task.ID] = taskPlacement{Depth: depth, AnchorID: anchors[task.ID]}
		for _, child := range children[task.ID] {
			visit(child, depth+1)
		}
	}
	for _, root := range roots {
		visit(root, 0)
	}
	return ordered, placements
}
