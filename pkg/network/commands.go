package network

import (
	"github.com/altenwald/backlog/pkg/model"
	"github.com/altenwald/backlog/pkg/store"
	"time"
)

// Commands is the request-oriented adapter for CLI/MCP. GUI reads use Hub's
// in-memory snapshots; API reads always query the remote owner directly.
type Commands struct{ *Hub }

func NewCommands(h *Hub) *Commands { return &Commands{h} }
func (c *Commands) ExportProjectJSON(slug string) ([]byte, error) {
	b, raw, err := c.resolve(slug)
	if err != nil {
		return nil, err
	}
	r, remote := b.(*Remote)
	if !remote {
		return c.Hub.ExportProjectJSON(slug)
	}
	var out []byte
	err = r.command("ExportProjectJSON", []any{raw}, nil, &out)
	return out, err
}
func (c *Commands) ExportProjectMarkdown(slug string) (string, error) {
	b, raw, err := c.resolve(slug)
	if err != nil {
		return "", err
	}
	r, remote := b.(*Remote)
	if !remote {
		return c.Hub.ExportProjectMarkdown(slug)
	}
	var out string
	err = r.command("ExportProjectMarkdown", []any{raw}, nil, &out)
	return out, err
}
func (c *Commands) GetProject(slug string) (*model.Project, error) {
	b, raw, err := c.resolve(slug)
	if err != nil {
		return nil, err
	}
	r, remote := b.(*Remote)
	if !remote {
		return c.Hub.GetProject(slug)
	}
	var out model.Project
	err = r.command("GetProject", []any{raw}, nil, &out)
	if err != nil {
		return nil, err
	}
	out.Slug = slug
	return &out, nil
}
func (c *Commands) ListTasks(projectSlug string, filter model.TaskFilter) ([]model.Task, error) {
	b, raw, err := c.resolve(projectSlug)
	if err != nil {
		return nil, err
	}
	r, remote := b.(*Remote)
	if !remote {
		return c.Hub.ListTasks(projectSlug, filter)
	}
	var out []model.Task
	err = r.command("ListTasks", []any{raw, filter}, nil, &out)
	return out, err
}
func (c *Commands) GetTopPriorities(projectSlug string, limit int) ([]model.Task, error) {
	b, raw, err := c.resolve(projectSlug)
	if err != nil {
		return nil, err
	}
	r, remote := b.(*Remote)
	if !remote {
		return c.Hub.GetTopPriorities(projectSlug, limit)
	}
	var out []model.Task
	err = r.command("GetTopPriorities", []any{raw, limit}, nil, &out)
	return out, err
}
func (c *Commands) GetSummary(projectSlug string) (*model.Summary, error) {
	b, raw, err := c.resolve(projectSlug)
	if err != nil {
		return nil, err
	}
	r, remote := b.(*Remote)
	if !remote {
		return c.Hub.GetSummary(projectSlug)
	}
	var out model.Summary
	err = r.command("GetSummary", []any{raw}, nil, &out)
	if err != nil {
		return nil, err
	}
	out.ProjectSlug = projectSlug
	return &out, nil
}
func (c *Commands) AddTask(projectSlug string, task model.Task) (*model.Task, error) {
	b, raw, err := c.resolve(projectSlug)
	if err != nil {
		return nil, err
	}
	r, remote := b.(*Remote)
	if !remote {
		return c.Hub.AddTask(projectSlug, task)
	}
	var out model.Task
	err = r.command("AddTask", []any{raw, task}, nil, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}
func (c *Commands) CompleteTask(projectSlug string, taskID string, done bool, resolution ...string) (*model.Task, error) {
	b, raw, err := c.resolve(projectSlug)
	if err != nil {
		return nil, err
	}
	r, remote := b.(*Remote)
	if !remote {
		return c.Hub.CompleteTask(projectSlug, taskID, done, resolution...)
	}
	var out model.Task
	err = r.command("CompleteTask", []any{raw, taskID, done, resolution}, nil, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}
func (c *Commands) UpdateTask(projectSlug string, upd model.TaskUpdate) (*model.Task, error) {
	b, raw, err := c.resolve(projectSlug)
	if err != nil {
		return nil, err
	}
	r, remote := b.(*Remote)
	if !remote {
		return c.Hub.UpdateTask(projectSlug, upd)
	}
	var out model.Task
	err = r.command("UpdateTask", []any{raw, upd}, upd.ExpectedUpdatedAt, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}
func (c *Commands) AssignTask(projectSlug string, taskID string, assignee string) (*model.Task, error) {
	b, raw, err := c.resolve(projectSlug)
	if err != nil {
		return nil, err
	}
	r, remote := b.(*Remote)
	if !remote {
		return c.Hub.AssignTask(projectSlug, taskID, assignee)
	}
	var out model.Task
	err = r.command("AssignTask", []any{raw, taskID, assignee}, nil, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}
func (c *Commands) DeleteTask(projectSlug string, taskID string, expected ...time.Time) error {
	b, raw, err := c.resolve(projectSlug)
	if err != nil {
		return err
	}
	r, remote := b.(*Remote)
	if !remote {
		return c.Hub.DeleteTask(projectSlug, taskID, expected...)
	}
	return r.command("DeleteTask", []any{raw, taskID, expected}, firstVersion(expected), nil)
}
func (c *Commands) DeprecateTask(projectSlug string, taskID string, deprecated bool) (*model.Task, error) {
	b, raw, err := c.resolve(projectSlug)
	if err != nil {
		return nil, err
	}
	r, remote := b.(*Remote)
	if !remote {
		return c.Hub.DeprecateTask(projectSlug, taskID, deprecated)
	}
	var out model.Task
	err = r.command("DeprecateTask", []any{raw, taskID, deprecated}, nil, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}
func (c *Commands) GetProjectSpecification(slug string) (string, error) {
	b, raw, err := c.resolve(slug)
	if err != nil {
		return "", err
	}
	r, remote := b.(*Remote)
	if !remote {
		return c.Hub.GetProjectSpecification(slug)
	}
	var out string
	err = r.command("GetProjectSpecification", []any{raw}, nil, &out)
	return out, err
}
func (c *Commands) UpdateProjectSpecification(slug, spec string) error {
	b, raw, err := c.resolve(slug)
	if err != nil {
		return err
	}
	r, remote := b.(*Remote)
	if !remote {
		return c.Hub.UpdateProjectSpecification(slug, spec)
	}
	return r.command("UpdateProjectSpecification", []any{raw, spec}, nil, nil)
}
func (c *Commands) ListSpecSections(slug string) ([]model.SpecSectionInfo, error) {
	b, raw, err := c.resolve(slug)
	if err != nil {
		return nil, err
	}
	r, remote := b.(*Remote)
	if !remote {
		return c.Hub.ListSpecSections(slug)
	}
	var out []model.SpecSectionInfo
	err = r.command("ListSpecSections", []any{raw}, nil, &out)
	return out, err
}
func (c *Commands) GetSpecSections(slug string, ids []string) ([]model.SpecSection, error) {
	b, raw, err := c.resolve(slug)
	if err != nil {
		return nil, err
	}
	r, remote := b.(*Remote)
	if !remote {
		return c.Hub.GetSpecSections(slug, ids)
	}
	var out []model.SpecSection
	err = r.command("GetSpecSections", []any{raw, ids}, nil, &out)
	return out, err
}
func (c *Commands) AddSpecSection(slug, id, title, body string, position int) (*model.SpecSection, error) {
	b, raw, err := c.resolve(slug)
	if err != nil {
		return nil, err
	}
	r, remote := b.(*Remote)
	if !remote {
		return c.Hub.AddSpecSection(slug, id, title, body, position)
	}
	var out model.SpecSection
	err = r.command("AddSpecSection", []any{raw, id, title, body, position}, nil, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}
func (c *Commands) UpdateSpecSection(slug, id string, title, body *string, expected ...time.Time) (*model.SpecSection, error) {
	b, raw, err := c.resolve(slug)
	if err != nil {
		return nil, err
	}
	r, remote := b.(*Remote)
	if !remote {
		return c.Hub.UpdateSpecSection(slug, id, title, body, expected...)
	}
	var out model.SpecSection
	err = r.command("UpdateSpecSection", []any{raw, id, title, body, expected}, firstVersion(expected), &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}
func (c *Commands) DeleteSpecSection(slug, id string, expected ...time.Time) error {
	b, raw, err := c.resolve(slug)
	if err != nil {
		return err
	}
	r, remote := b.(*Remote)
	if !remote {
		return c.Hub.DeleteSpecSection(slug, id, expected...)
	}
	return r.command("DeleteSpecSection", []any{raw, id, expected}, firstVersion(expected), nil)
}
func (c *Commands) MoveSpecSection(slug, id string, position int) error {
	b, raw, err := c.resolve(slug)
	if err != nil {
		return err
	}
	r, remote := b.(*Remote)
	if !remote {
		return c.Hub.MoveSpecSection(slug, id, position)
	}
	return r.command("MoveSpecSection", []any{raw, id, position}, nil, nil)
}

func (r *Remote) command(method string, args []any, expected *time.Time, out any) error {
	ctx, stop := requestContext()
	defer stop()
	pre := store.Precondition{}
	switch method {
	case "AddTask", "UpdateTask", "CompleteTask", "AssignTask", "DeleteTask", "DeprecateTask", "UpdateProjectSpecification", "AddSpecSection", "UpdateSpecSection", "DeleteSpecSection", "MoveSpecSection":
		var p model.Project
		if e := r.request(ctx, "Snapshot", args[:1], pre, &p); e != nil {
			return e
		}
		pre.UpdatedAt = p.UpdatedAt
		id := ""
		switch method {
		case "UpdateTask":
			id = args[1].(model.TaskUpdate).ID
		case "CompleteTask", "AssignTask", "DeleteTask", "DeprecateTask", "UpdateSpecSection", "DeleteSpecSection":
			id = args[1].(string)
		}
		if method == "UpdateSpecSection" || method == "DeleteSpecSection" {
			pre.UpdatedAt = time.Time{}
			for _, s := range p.Spec {
				if s.ID == id {
					pre.UpdatedAt = s.UpdatedAt
				}
			}
		} else if id != "" {
			for _, t := range p.Tasks {
				if t.ID == id {
					pre.UpdatedAt = t.UpdatedAt
				}
			}
		}
		if expected != nil {
			pre.UpdatedAt = *expected
		}
	}
	e := r.request(ctx, method, args, pre, out)
	if e == nil {
		r.signal()
	}
	return e
}
