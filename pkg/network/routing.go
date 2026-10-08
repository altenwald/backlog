package network

import (
	"github.com/altenwald/backlog/pkg/model"
	"time"
)

func (h *Hub) ExportProjectJSON(slug string) ([]byte, error) {
	b, raw, err := h.resolve(slug)
	if err != nil {
		return nil, err
	}
	return b.ExportProjectJSON(raw)
}
func (h *Hub) ExportProjectMarkdown(slug string) (string, error) {
	b, raw, err := h.resolve(slug)
	if err != nil {
		return "", err
	}
	return b.ExportProjectMarkdown(raw)
}
func (h *Hub) GetProject(slug string) (*model.Project, error) {
	b, raw, err := h.resolve(slug)
	if err != nil {
		return nil, err
	}
	p, e := b.GetProject(raw)
	if e == nil {
		p.Slug = slug
	}
	return p, e
}
func (h *Hub) ListTasks(projectSlug string, filter model.TaskFilter) ([]model.Task, error) {
	b, raw, err := h.resolve(projectSlug)
	if err != nil {
		return nil, err
	}
	return b.ListTasks(raw, filter)
}
func (h *Hub) GetTopPriorities(projectSlug string, limit int) ([]model.Task, error) {
	b, raw, err := h.resolve(projectSlug)
	if err != nil {
		return nil, err
	}
	return b.GetTopPriorities(raw, limit)
}
func (h *Hub) GetSummary(projectSlug string) (*model.Summary, error) {
	b, raw, err := h.resolve(projectSlug)
	if err != nil {
		return nil, err
	}
	v, e := b.GetSummary(raw)
	if e == nil {
		v.ProjectSlug = projectSlug
	}
	return v, e
}
func (h *Hub) AddTask(projectSlug string, task model.Task) (*model.Task, error) {
	b, raw, err := h.resolve(projectSlug)
	if err != nil {
		return nil, err
	}
	return b.AddTask(raw, task)
}
func (h *Hub) CompleteTask(projectSlug string, taskID string, done bool, resolution ...string) (*model.Task, error) {
	b, raw, err := h.resolve(projectSlug)
	if err != nil {
		return nil, err
	}
	return b.CompleteTask(raw, taskID, done, resolution...)
}
func (h *Hub) UpdateTask(projectSlug string, upd model.TaskUpdate) (*model.Task, error) {
	b, raw, err := h.resolve(projectSlug)
	if err != nil {
		return nil, err
	}
	return b.UpdateTask(raw, upd)
}
func (h *Hub) AssignTask(projectSlug string, taskID string, assignee string) (*model.Task, error) {
	b, raw, err := h.resolve(projectSlug)
	if err != nil {
		return nil, err
	}
	return b.AssignTask(raw, taskID, assignee)
}
func (h *Hub) DeleteTask(projectSlug string, taskID string, expected ...time.Time) error {
	b, raw, err := h.resolve(projectSlug)
	if err != nil {
		return err
	}
	return b.DeleteTask(raw, taskID, expected...)
}
func (h *Hub) DeprecateTask(projectSlug string, taskID string, deprecated bool) (*model.Task, error) {
	b, raw, err := h.resolve(projectSlug)
	if err != nil {
		return nil, err
	}
	return b.DeprecateTask(raw, taskID, deprecated)
}
func (h *Hub) GetProjectSpecification(slug string) (string, error) {
	b, raw, err := h.resolve(slug)
	if err != nil {
		return "", err
	}
	return b.GetProjectSpecification(raw)
}
func (h *Hub) UpdateProjectSpecification(slug, spec string) error {
	b, raw, err := h.resolve(slug)
	if err != nil {
		return err
	}
	return b.UpdateProjectSpecification(raw, spec)
}
func (h *Hub) ListSpecSections(slug string) ([]model.SpecSectionInfo, error) {
	b, raw, err := h.resolve(slug)
	if err != nil {
		return nil, err
	}
	return b.ListSpecSections(raw)
}
func (h *Hub) GetSpecSections(slug string, ids []string) ([]model.SpecSection, error) {
	b, raw, err := h.resolve(slug)
	if err != nil {
		return nil, err
	}
	return b.GetSpecSections(raw, ids)
}
func (h *Hub) AddSpecSection(slug, id, title, body string, position int) (*model.SpecSection, error) {
	b, raw, err := h.resolve(slug)
	if err != nil {
		return nil, err
	}
	return b.AddSpecSection(raw, id, title, body, position)
}
func (h *Hub) UpdateSpecSection(slug, id string, title, body *string, expected ...time.Time) (*model.SpecSection, error) {
	b, raw, err := h.resolve(slug)
	if err != nil {
		return nil, err
	}
	return b.UpdateSpecSection(raw, id, title, body, expected...)
}
func (h *Hub) DeleteSpecSection(slug, id string, expected ...time.Time) error {
	b, raw, err := h.resolve(slug)
	if err != nil {
		return err
	}
	return b.DeleteSpecSection(raw, id, expected...)
}
func (h *Hub) MoveSpecSection(slug, id string, position int) error {
	b, raw, err := h.resolve(slug)
	if err != nil {
		return err
	}
	return b.MoveSpecSection(raw, id, position)
}
