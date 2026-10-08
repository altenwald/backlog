package server

import (
	"github.com/altenwald/backlog/pkg/client"
	"github.com/altenwald/backlog/pkg/model"
	"github.com/altenwald/backlog/pkg/store"
)

type ProjectSummaryItem struct {
	Slug       string `json:"slug"`
	Name       string `json:"name"`
	OpenTasks  int    `json:"open_tasks"`
	TotalTasks int    `json:"total_tasks"`
}

type Backend interface {
	ListProjects() ([]ProjectSummaryItem, error)
	CreateProject(slug, name, desc string) (*model.Project, error)
	GetProject(slug string) (*model.Project, error)
	GetSummary(slug string) (*model.Summary, error)
	GetTopPriorities(slug string, limit int) ([]model.Task, error)
	ListTasks(slug string, filter model.TaskFilter) ([]model.Task, error)
	AddTask(slug string, task model.Task) (*model.Task, error)
	UpdateTask(slug string, task model.TaskUpdate) (*model.Task, error)
	CompleteTask(slug string, taskID string, done bool, resolution string) (*model.Task, error)
	AssignTask(slug string, taskID string, assignee string) (*model.Task, error)
	DeleteTask(slug string, taskID string) error
	DeleteProject(slug string) error
	DeprecateTask(slug string, taskID string, deprecated bool) (*model.Task, error)
	GetProjectSpecification(slug string) (string, error)
	UpdateProjectSpecification(slug string, spec string) error
	ListSpecSections(slug string) ([]model.SpecSectionInfo, error)
	GetSpecSections(slug string, ids []string) ([]model.SpecSection, error)
	AddSpecSection(slug, id, title, body string, position int) (*model.SpecSection, error)
	UpdateSpecSection(slug, id string, title, body *string) (*model.SpecSection, error)
	DeleteSpecSection(slug, id string) error
	MoveSpecSection(slug, id string, position int) error
	GetSettings() (string, error)
	UpdateSettings(mcpUserInstructions string) error
}

// storeBackend adapts store.Backend to Backend
type storeBackend struct {
	st store.Backend
}

func NewStoreBackend(st store.Backend) Backend {
	return &storeBackend{st: st}
}

func (b *storeBackend) ListProjects() ([]ProjectSummaryItem, error) {
	projects := b.st.ListProjects()
	var items []ProjectSummaryItem
	for _, p := range projects {
		sum, _ := b.st.GetSummary(p.Slug)
		openTasks, totalTasks := 0, 0
		if sum != nil {
			openTasks = sum.OpenTasks
			totalTasks = sum.TotalTasks
		}
		items = append(items, ProjectSummaryItem{
			Slug:       p.Slug,
			Name:       p.Name,
			OpenTasks:  openTasks,
			TotalTasks: totalTasks,
		})
	}
	return items, nil
}

func (b *storeBackend) CreateProject(slug, name, desc string) (*model.Project, error) {
	return b.st.CreateProject(slug, name, desc)
}

func (b *storeBackend) GetProject(slug string) (*model.Project, error) {
	return b.st.GetProject(slug)
}

func (b *storeBackend) GetSummary(slug string) (*model.Summary, error) {
	return b.st.GetSummary(slug)
}

func (b *storeBackend) GetTopPriorities(slug string, limit int) ([]model.Task, error) {
	return b.st.GetTopPriorities(slug, limit)
}

func (b *storeBackend) ListTasks(slug string, filter model.TaskFilter) ([]model.Task, error) {
	return b.st.ListTasks(slug, filter)
}

func (b *storeBackend) AddTask(slug string, task model.Task) (*model.Task, error) {
	return b.st.AddTask(slug, task)
}

func (b *storeBackend) UpdateTask(slug string, task model.TaskUpdate) (*model.Task, error) {
	return b.st.UpdateTask(slug, task)
}

func (b *storeBackend) CompleteTask(slug string, taskID string, done bool, resolution string) (*model.Task, error) {
	return b.st.CompleteTask(slug, taskID, done, resolution)
}

func (b *storeBackend) AssignTask(slug string, taskID string, assignee string) (*model.Task, error) {
	return b.st.AssignTask(slug, taskID, assignee)
}

func (b *storeBackend) DeleteTask(slug string, taskID string) error {
	return b.st.DeleteTask(slug, taskID)
}

func (b *storeBackend) DeleteProject(slug string) error {
	return b.st.DeleteProject(slug)
}

func (b *storeBackend) DeprecateTask(slug string, taskID string, deprecated bool) (*model.Task, error) {
	return b.st.DeprecateTask(slug, taskID, deprecated)
}

func (b *storeBackend) GetProjectSpecification(slug string) (string, error) {
	return b.st.GetProjectSpecification(slug)
}

func (b *storeBackend) UpdateProjectSpecification(slug string, spec string) error {
	return b.st.UpdateProjectSpecification(slug, spec)
}

func (b *storeBackend) ListSpecSections(slug string) ([]model.SpecSectionInfo, error) {
	return b.st.ListSpecSections(slug)
}

func (b *storeBackend) GetSpecSections(slug string, ids []string) ([]model.SpecSection, error) {
	return b.st.GetSpecSections(slug, ids)
}

func (b *storeBackend) AddSpecSection(slug, id, title, body string, position int) (*model.SpecSection, error) {
	return b.st.AddSpecSection(slug, id, title, body, position)
}

func (b *storeBackend) UpdateSpecSection(slug, id string, title, body *string) (*model.SpecSection, error) {
	return b.st.UpdateSpecSection(slug, id, title, body)
}

func (b *storeBackend) DeleteSpecSection(slug, id string) error {
	return b.st.DeleteSpecSection(slug, id)
}

func (b *storeBackend) MoveSpecSection(slug, id string, position int) error {
	return b.st.MoveSpecSection(slug, id, position)
}

func (b *storeBackend) GetSettings() (string, error) {
	return b.st.GetMCPUserInstructions(), nil
}

func (b *storeBackend) UpdateSettings(instructions string) error {
	return b.st.SaveMCPUserInstructions(instructions)
}

// clientBackend adapts *client.Client to Backend
type clientBackend struct {
	c *client.Client
}

func NewClientBackend(c *client.Client) Backend {
	return &clientBackend{c: c}
}

func (b *clientBackend) ListProjects() ([]ProjectSummaryItem, error) {
	projects, err := b.c.ListProjects()
	if err != nil {
		return nil, err
	}
	var items []ProjectSummaryItem
	for _, p := range projects {
		openTasks, totalTasks := 0, 0
		if p.Summary != nil {
			openTasks = p.Summary.OpenTasks
			totalTasks = p.Summary.TotalTasks
		}
		items = append(items, ProjectSummaryItem{
			Slug:       p.Slug,
			Name:       p.Name,
			OpenTasks:  openTasks,
			TotalTasks: totalTasks,
		})
	}
	return items, nil
}

func (b *clientBackend) CreateProject(slug, name, desc string) (*model.Project, error) {
	return b.c.CreateProject(slug, name, desc)
}

func (b *clientBackend) GetProject(slug string) (*model.Project, error) {
	return b.c.GetProject(slug)
}

func (b *clientBackend) GetSummary(slug string) (*model.Summary, error) {
	return b.c.GetSummary(slug)
}

func (b *clientBackend) GetTopPriorities(slug string, limit int) ([]model.Task, error) {
	return b.c.GetTopPriorities(slug, limit)
}

func (b *clientBackend) ListTasks(slug string, filter model.TaskFilter) ([]model.Task, error) {
	return b.c.ListTasks(slug, filter)
}

func (b *clientBackend) AddTask(slug string, task model.Task) (*model.Task, error) {
	return b.c.AddTask(slug, task)
}

func (b *clientBackend) UpdateTask(slug string, task model.TaskUpdate) (*model.Task, error) {
	return b.c.UpdateTask(slug, task)
}

func (b *clientBackend) CompleteTask(slug string, taskID string, done bool, resolution string) (*model.Task, error) {
	return b.c.CompleteTask(slug, taskID, done, resolution)
}

func (b *clientBackend) AssignTask(slug string, taskID string, assignee string) (*model.Task, error) {
	return b.c.AssignTask(slug, taskID, assignee)
}

func (b *clientBackend) DeleteTask(slug string, taskID string) error {
	return b.c.DeleteTask(slug, taskID)
}

func (b *clientBackend) DeleteProject(slug string) error {
	return b.c.DeleteProject(slug)
}

func (b *clientBackend) DeprecateTask(slug string, taskID string, deprecated bool) (*model.Task, error) {
	return b.c.DeprecateTask(slug, taskID, deprecated)
}

func (b *clientBackend) GetProjectSpecification(slug string) (string, error) {
	return b.c.GetProjectSpecification(slug)
}

func (b *clientBackend) UpdateProjectSpecification(slug string, spec string) error {
	return b.c.UpdateProjectSpecification(slug, spec)
}

func (b *clientBackend) ListSpecSections(slug string) ([]model.SpecSectionInfo, error) {
	return b.c.ListSpecSections(slug)
}

func (b *clientBackend) GetSpecSections(slug string, ids []string) ([]model.SpecSection, error) {
	return b.c.GetSpecSections(slug, ids)
}

func (b *clientBackend) AddSpecSection(slug, id, title, body string, position int) (*model.SpecSection, error) {
	return b.c.AddSpecSection(slug, id, title, body, position)
}

func (b *clientBackend) UpdateSpecSection(slug, id string, title, body *string) (*model.SpecSection, error) {
	return b.c.UpdateSpecSection(slug, id, title, body)
}

func (b *clientBackend) DeleteSpecSection(slug, id string) error {
	return b.c.DeleteSpecSection(slug, id)
}

func (b *clientBackend) MoveSpecSection(slug, id string, position int) error {
	return b.c.MoveSpecSection(slug, id, position)
}

func (b *clientBackend) GetSettings() (string, error) {
	s, err := b.c.GetSettings()
	if err != nil {
		return "", err
	}
	return s.MCPUserInstructions, nil
}

func (b *clientBackend) UpdateSettings(instructions string) error {
	_, err := b.c.UpdateSettings(client.SettingsInfo{
		MCPUserInstructions: instructions,
	})
	return err
}
