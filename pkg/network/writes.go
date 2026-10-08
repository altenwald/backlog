package network

import (
	"context"
	"github.com/altenwald/backlog/pkg/model"
	"github.com/altenwald/backlog/pkg/store"
	"time"
)

func requestContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 15*time.Second)
}
func emptyPrecondition() store.Precondition { return store.Precondition{} }
func (r *Remote) AddTask(projectSlug string, task model.Task) (*model.Task, error) {
	var out model.Task
	e := r.call("AddTask", projectSlug, "", []any{projectSlug, task}, nil, &out)
	if e != nil {
		return nil, e
	}
	return &out, nil
}
func (r *Remote) CompleteTask(projectSlug string, taskID string, done bool, resolution ...string) (*model.Task, error) {
	var out model.Task
	e := r.call("CompleteTask", projectSlug, taskID, []any{projectSlug, taskID, done, resolution}, nil, &out)
	if e != nil {
		return nil, e
	}
	return &out, nil
}
func (r *Remote) UpdateTask(projectSlug string, upd model.TaskUpdate) (*model.Task, error) {
	var out model.Task
	e := r.call("UpdateTask", projectSlug, upd.ID, []any{projectSlug, upd}, upd.ExpectedUpdatedAt, &out)
	if e != nil {
		return nil, e
	}
	return &out, nil
}
func (r *Remote) AssignTask(projectSlug string, taskID string, assignee string) (*model.Task, error) {
	var out model.Task
	e := r.call("AssignTask", projectSlug, taskID, []any{projectSlug, taskID, assignee}, nil, &out)
	if e != nil {
		return nil, e
	}
	return &out, nil
}
func (r *Remote) DeleteTask(projectSlug string, taskID string, expected ...time.Time) error {
	return r.call("DeleteTask", projectSlug, taskID, []any{projectSlug, taskID, expected}, firstVersion(expected), nil)
}
func (r *Remote) DeprecateTask(projectSlug string, taskID string, deprecated bool) (*model.Task, error) {
	var out model.Task
	e := r.call("DeprecateTask", projectSlug, taskID, []any{projectSlug, taskID, deprecated}, nil, &out)
	if e != nil {
		return nil, e
	}
	return &out, nil
}
func (r *Remote) UpdateProjectSpecification(slug, spec string) error {
	return r.call("UpdateProjectSpecification", slug, "", []any{slug, spec}, nil, nil)
}
func (r *Remote) AddSpecSection(slug, id, title, body string, position int) (*model.SpecSection, error) {
	var out model.SpecSection
	e := r.call("AddSpecSection", slug, "", []any{slug, id, title, body, position}, nil, &out)
	if e != nil {
		return nil, e
	}
	return &out, nil
}
func (r *Remote) UpdateSpecSection(slug, id string, title, body *string, expected ...time.Time) (*model.SpecSection, error) {
	var out model.SpecSection
	e := r.call("UpdateSpecSection", slug, id, []any{slug, id, title, body, expected}, firstVersion(expected), &out)
	if e != nil {
		return nil, e
	}
	return &out, nil
}
func (r *Remote) DeleteSpecSection(slug, id string, expected ...time.Time) error {
	return r.call("DeleteSpecSection", slug, id, []any{slug, id, expected}, firstVersion(expected), nil)
}
func (r *Remote) MoveSpecSection(slug, id string, position int) error {
	return r.call("MoveSpecSection", slug, "", []any{slug, id, position}, nil, nil)
}

func firstVersion(v []time.Time) *time.Time {
	if len(v) == 0 {
		return nil
	}
	return &v[0]
}
