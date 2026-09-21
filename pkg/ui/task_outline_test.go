package ui

import (
	"github.com/altenwald/backlog/pkg/model"
	"reflect"
	"testing"
)

func TestTaskOutline(t *testing.T) {
	cases := []struct {
		name   string
		tasks  []model.Task
		ids    []string
		depths []int
	}{
		{"chain with prerequisites appearing later", []model.Task{{ID: "3", DependsOn: []string{"2"}}, {ID: "1"}, {ID: "4"}, {ID: "2", DependsOn: []string{"1"}}}, []string{"1", "2", "3", "4"}, []int{0, 1, 2, 0}},
		{"siblings kept together", []model.Task{{ID: "1"}, {ID: "2", DependsOn: []string{"1"}}, {ID: "3"}, {ID: "4", DependsOn: []string{"1"}}}, []string{"1", "2", "4", "3"}, []int{0, 1, 1, 0}},
		{"multiple prerequisites appear once", []model.Task{{ID: "1"}, {ID: "2"}, {ID: "3", DependsOn: []string{"2", "1"}}}, []string{"1", "2", "3"}, []int{0, 0, 1}},
		{"filtered prerequisite keeps matching descendants", []model.Task{{ID: "2", DependsOn: []string{"1"}}, {ID: "3", DependsOn: []string{"2"}}}, []string{"2", "3"}, []int{0, 1}},
		{"first visible prerequisite", []model.Task{{ID: "1"}, {ID: "3", DependsOn: []string{"missing", "1"}}}, []string{"1", "3"}, []int{0, 1}},
		{"parent grouping", []model.Task{{ID: "2", ParentID: "1"}, {ID: "1"}}, []string{"1", "2"}, []int{0, 1}},
		{"dependencies take precedence over parent", []model.Task{{ID: "1"}, {ID: "2"}, {ID: "3", ParentID: "1", DependsOn: []string{"2"}}}, []string{"1", "2", "3"}, []int{0, 0, 1}},
		{"mixed relation cycle", []model.Task{{ID: "1", ParentID: "2"}, {ID: "2", DependsOn: []string{"1"}}}, []string{"2", "1"}, []int{0, 1}},
		{"imported dependency cycle", []model.Task{{ID: "1", DependsOn: []string{"2"}}, {ID: "2", DependsOn: []string{"1"}}}, []string{"2", "1"}, []int{0, 1}},
		{"self dependency", []model.Task{{ID: "1", DependsOn: []string{"1"}}}, []string{"1"}, []int{0}},
		{"empty", nil, nil, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			before := append([]model.Task(nil), tc.tasks...)
			got, placements := arrangeTaskOutline(tc.tasks)
			var ids []string
			var depths []int
			for _, task := range got {
				ids = append(ids, task.ID)
				depths = append(depths, placements[task.ID].Depth)
			}
			if !reflect.DeepEqual(ids, tc.ids) || !reflect.DeepEqual(depths, tc.depths) {
				t.Fatalf("outline = %v depths %v; want %v depths %v", ids, depths, tc.ids, tc.depths)
			}
			if !reflect.DeepEqual(before, tc.tasks) {
				t.Fatal("outline changed source task order")
			}
		})
	}
}
