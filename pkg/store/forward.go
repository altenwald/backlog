package store

import (
	"github.com/altenwald/backlog/pkg/model"
	"time"
)

func (s *Store) ExportProjectJSON(slug string) ([]byte, error) {
	d, err := s.openProject(slug)
	if err != nil {
		return nil, err
	}
	defer d.Close()
	return d.ExportProjectJSON(slug)
}
func (s *Store) ExportProjectMarkdown(slug string) (string, error) {
	d, err := s.openProject(slug)
	if err != nil {
		return "", err
	}
	defer d.Close()
	return d.ExportProjectMarkdown(slug)
}
func (s *Store) GetProject(slug string) (*model.Project, error) {
	d, err := s.openProject(slug)
	if err != nil {
		return nil, err
	}
	defer d.Close()
	return d.GetProject(slug)
}
func (s *Store) ListTasks(projectSlug string, filter model.TaskFilter) ([]model.Task, error) {
	d, err := s.openProject(projectSlug)
	if err != nil {
		return nil, err
	}
	defer d.Close()
	return d.ListTasks(projectSlug, filter)
}
func (s *Store) GetTopPriorities(projectSlug string, limit int) ([]model.Task, error) {
	d, err := s.openProject(projectSlug)
	if err != nil {
		return nil, err
	}
	defer d.Close()
	return d.GetTopPriorities(projectSlug, limit)
}
func (s *Store) GetSummary(projectSlug string) (*model.Summary, error) {
	d, err := s.openProject(projectSlug)
	if err != nil {
		return nil, err
	}
	defer d.Close()
	return d.GetSummary(projectSlug)
}
func (s *Store) AddTask(projectSlug string, task model.Task) (*model.Task, error) {
	d, err := s.openProject(projectSlug)
	if err != nil {
		return nil, err
	}
	defer d.Close()
	return d.AddTask(projectSlug, task)
}
func (s *Store) CompleteTask(projectSlug string, taskID string, done bool, resolution ...string) (*model.Task, error) {
	d, err := s.openProject(projectSlug)
	if err != nil {
		return nil, err
	}
	defer d.Close()
	return d.CompleteTask(projectSlug, taskID, done, resolution...)
}
func (s *Store) UpdateTask(projectSlug string, upd model.TaskUpdate) (*model.Task, error) {
	d, err := s.openProject(projectSlug)
	if err != nil {
		return nil, err
	}
	defer d.Close()
	return d.UpdateTask(projectSlug, upd)
}
func (s *Store) AssignTask(projectSlug string, taskID string, assignee string) (*model.Task, error) {
	d, err := s.openProject(projectSlug)
	if err != nil {
		return nil, err
	}
	defer d.Close()
	return d.AssignTask(projectSlug, taskID, assignee)
}
func (s *Store) DeleteTask(projectSlug string, taskID string, expected ...time.Time) error {
	d, err := s.openProject(projectSlug)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.DeleteTask(projectSlug, taskID, expected...)
}
func (s *Store) DeprecateTask(projectSlug string, taskID string, deprecated bool) (*model.Task, error) {
	d, err := s.openProject(projectSlug)
	if err != nil {
		return nil, err
	}
	defer d.Close()
	return d.DeprecateTask(projectSlug, taskID, deprecated)
}
func (s *Store) GetProjectSpecification(slug string) (string, error) {
	d, err := s.openProject(slug)
	if err != nil {
		return "", err
	}
	defer d.Close()
	return d.GetProjectSpecification(slug)
}
func (s *Store) UpdateProjectSpecification(slug, spec string) error {
	d, err := s.openProject(slug)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.UpdateProjectSpecification(slug, spec)
}
func (s *Store) ListSpecSections(slug string) ([]model.SpecSectionInfo, error) {
	d, err := s.openProject(slug)
	if err != nil {
		return nil, err
	}
	defer d.Close()
	return d.ListSpecSections(slug)
}
func (s *Store) GetSpecSections(slug string, ids []string) ([]model.SpecSection, error) {
	d, err := s.openProject(slug)
	if err != nil {
		return nil, err
	}
	defer d.Close()
	return d.GetSpecSections(slug, ids)
}
func (s *Store) AddSpecSection(slug, id, title, body string, position int) (*model.SpecSection, error) {
	d, err := s.openProject(slug)
	if err != nil {
		return nil, err
	}
	defer d.Close()
	return d.AddSpecSection(slug, id, title, body, position)
}
func (s *Store) UpdateSpecSection(slug, id string, title, body *string, expected ...time.Time) (*model.SpecSection, error) {
	d, err := s.openProject(slug)
	if err != nil {
		return nil, err
	}
	defer d.Close()
	return d.UpdateSpecSection(slug, id, title, body, expected...)
}
func (s *Store) DeleteSpecSection(slug, id string, expected ...time.Time) error {
	d, err := s.openProject(slug)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.DeleteSpecSection(slug, id, expected...)
}
func (s *Store) MoveSpecSection(slug, id string, position int) error {
	d, err := s.openProject(slug)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.MoveSpecSection(slug, id, position)
}
