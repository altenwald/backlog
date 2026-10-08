package store

import (
	"github.com/altenwald/backlog/pkg/model"
	"time"
)

// Backend is the common project API used by the desktop, CLI server and network adapter.
type Backend interface {
	ExportProjectJSON(slug string) ([]byte, error)
	ExportProjectMarkdown(slug string) (string, error)
	GetProject(slug string) (*model.Project, error)
	ListTasks(projectSlug string, filter model.TaskFilter) ([]model.Task, error)
	GetTopPriorities(projectSlug string, limit int) ([]model.Task, error)
	GetSummary(projectSlug string) (*model.Summary, error)
	AddTask(projectSlug string, task model.Task) (*model.Task, error)
	CompleteTask(projectSlug string, taskID string, done bool, resolution ...string) (*model.Task, error)
	UpdateTask(projectSlug string, upd model.TaskUpdate) (*model.Task, error)
	AssignTask(projectSlug string, taskID string, assignee string) (*model.Task, error)
	DeleteTask(projectSlug string, taskID string, expected ...time.Time) error
	DeprecateTask(projectSlug string, taskID string, deprecated bool) (*model.Task, error)
	GetProjectSpecification(slug string) (string, error)
	UpdateProjectSpecification(slug, spec string) error
	ListSpecSections(slug string) ([]model.SpecSectionInfo, error)
	GetSpecSections(slug string, ids []string) ([]model.SpecSection, error)
	AddSpecSection(slug, id, title, body string, position int) (*model.SpecSection, error)
	UpdateSpecSection(slug, id string, title, body *string, expected ...time.Time) (*model.SpecSection, error)
	DeleteSpecSection(slug, id string, expected ...time.Time) error
	MoveSpecSection(slug, id string, position int) error
	ListProjects() []*model.Project
	CreateProject(slug, name, description string) (*model.Project, error)
	DeleteProject(slug string) error
	GetActiveProjectSlug() string
	SetActiveProject(slug string) error
	GetMCPUserInstructions() string
	SaveMCPUserInstructions(string) error
	Subscribe() <-chan Event
	Watch() (<-chan Event, func())
	SetProjectOpen(slug string, open bool) error
}
