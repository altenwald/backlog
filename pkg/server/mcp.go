package server

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/altenwald/backlog/pkg/model"
	"github.com/altenwald/backlog/pkg/store"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func NewMCPServer(st *store.Store) *server.MCPServer {
	s := server.NewMCPServer("backlog", "1.0.0", server.WithLogging())

	// Tool: list_projects
	s.AddTool(
		mcp.NewTool(
			"list_projects",
			mcp.WithDescription("List all registered projects in Backlog with metrics, open tasks, points, and active status."),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			projects := st.ListProjects()
			active := st.GetActiveProjectSlug()

			type ProjectItem struct {
				Slug        string `json:"slug"`
				Name        string `json:"name"`
				Active      bool   `json:"active"`
				OpenTasks   int    `json:"open_tasks"`
				TotalTasks  int    `json:"total_tasks"`
				OpenPoints  int    `json:"open_points"`
				TotalPoints int    `json:"total_points"`
			}

			var items []ProjectItem
			for _, p := range projects {
				sum, _ := st.GetSummary(p.Slug)
				items = append(items, ProjectItem{
					Slug:        p.Slug,
					Name:        p.Name,
					Active:      p.Slug == active,
					OpenTasks:   sum.OpenTasks,
					TotalTasks:  sum.TotalTasks,
					OpenPoints:  sum.OpenPoints,
					TotalPoints: sum.TotalPoints,
				})
			}

			data, _ := json.MarshalIndent(items, "", "  ")
			return mcp.NewToolResultText(string(data)), nil
		},
	)

	// Tool: get_active_project
	s.AddTool(
		mcp.NewTool(
			"get_active_project",
			mcp.WithDescription("Get the currently active project in the GUI and server."),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			active := st.GetActiveProjectSlug()
			p, err := st.GetProject(active)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			sum, _ := st.GetSummary(active)
			resp := map[string]any{
				"active_project": active,
				"name":           p.Name,
				"description":    p.Description,
				"summary":        sum,
			}
			data, _ := json.MarshalIndent(resp, "", "  ")
			return mcp.NewToolResultText(string(data)), nil
		},
	)

	// Tool: set_active_project
	s.AddTool(
		mcp.NewTool(
			"set_active_project",
			mcp.WithDescription("Switch the active project focused in the GUI and System Tray."),
			mcp.WithString("project", mcp.Description("Project slug to activate (e.g. 'dymmer', 'conta')"), mcp.Required()),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			project, ok := req.Params.Arguments["project"].(string)
			if !ok || project == "" {
				return mcp.NewToolResultError("parameter 'project' is required"), nil
			}
			if err := st.SetActiveProject(project); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(fmt.Sprintf("Active project switched to '%s'", project)), nil
		},
	)

	// Tool: list_tasks
	s.AddTool(
		mcp.NewTool(
			"list_tasks",
			mcp.WithDescription("List tasks in a project with optional filters by Tier (1 to 5), Group, Status (open/completed), or search query."),
			mcp.WithString("project", mcp.Description("Project slug (optional; defaults to active project)")),
			mcp.WithNumber("tier", mcp.Description("Filter by priority Tier: 1=Blocker, 2=Important, 3=Visual debt, 4=Internal, 5=Future")),
			mcp.WithString("group", mcp.Description("Filter by category/group (e.g. 'Monetization', 'Domains', 'Bugs')")),
			mcp.WithBoolean("done", mcp.Description("Filter by status: true=completed, false=open")),
			mcp.WithString("search", mcp.Description("Text search term")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			project, _ := req.Params.Arguments["project"].(string)
			if project == "" {
				project = st.GetActiveProjectSlug()
			}

			filter := model.TaskFilter{}
			if tierVal, ok := req.Params.Arguments["tier"].(float64); ok && tierVal > 0 {
				t := model.Tier(int(tierVal))
				filter.Tier = &t
			}
			if groupVal, ok := req.Params.Arguments["group"].(string); ok && groupVal != "" {
				filter.Group = &groupVal
			}
			if doneVal, ok := req.Params.Arguments["done"].(bool); ok {
				filter.Done = &doneVal
			}
			if searchVal, ok := req.Params.Arguments["search"].(string); ok {
				filter.Search = searchVal
			}

			tasks, err := st.ListTasks(project, filter)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			data, _ := json.MarshalIndent(tasks, "", "  ")
			return mcp.NewToolResultText(string(data)), nil
		},
	)

	// Tool: get_summary
	s.AddTool(
		mcp.NewTool(
			"get_summary",
			mcp.WithDescription("Get metric summary, total/open points, and breakdown by size and tier for a project."),
			mcp.WithString("project", mcp.Description("Project slug (optional; defaults to active project)")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			project, _ := req.Params.Arguments["project"].(string)
			if project == "" {
				project = st.GetActiveProjectSlug()
			}

			sum, err := st.GetSummary(project)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			data, _ := json.MarshalIndent(sum, "", "  ")
			return mcp.NewToolResultText(string(data)), nil
		},
	)

	// Tool: get_top_priorities
	s.AddTool(
		mcp.NewTool(
			"get_top_priorities",
			mcp.WithDescription("Get the highest priority pending tasks (T1 -> T2) for a project."),
			mcp.WithString("project", mcp.Description("Project slug (optional)")),
			mcp.WithNumber("limit", mcp.Description("Number of tasks to return (default 5)")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			project, _ := req.Params.Arguments["project"].(string)
			if project == "" {
				project = st.GetActiveProjectSlug()
			}

			limit := 5
			if lVal, ok := req.Params.Arguments["limit"].(float64); ok && lVal > 0 {
				limit = int(lVal)
			}

			tasks, err := st.GetTopPriorities(project, limit)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			data, _ := json.MarshalIndent(tasks, "", "  ")
			return mcp.NewToolResultText(string(data)), nil
		},
	)

	// Tool: add_task
	s.AddTool(
		mcp.NewTool(
			"add_task",
			mcp.WithDescription("Add a new task to the project backlog."),
			mcp.WithString("title", mcp.Description("Concise title of the task"), mcp.Required()),
			mcp.WithString("description", mcp.Description("Detailed description or context")),
			mcp.WithString("project", mcp.Description("Project slug (optional; defaults to active project)")),
			mcp.WithString("group", mcp.Description("Group or category (e.g. 'Monetization', 'Domains', 'Bugs', 'General')")),
			mcp.WithString("size", mcp.Description("Effort size: 'XS', 'S', 'M', 'L', 'XL' (default 'M')")),
			mcp.WithNumber("tier", mcp.Description("Priority tier: 1 (Blocker) to 5 (Future). Default 3")),
			mcp.WithString("tag", mcp.Description("Tag or reference label (e.g. 'TODO', 'spec 08-24')")),
			mcp.WithString("resolution", mcp.Description("Summary of implementation details or resolution (optional)")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			title, ok := req.Params.Arguments["title"].(string)
			if !ok || strings.TrimSpace(title) == "" {
				return mcp.NewToolResultError("parameter 'title' is required"), nil
			}

			project, _ := req.Params.Arguments["project"].(string)
			if project == "" {
				project = st.GetActiveProjectSlug()
			}

			desc, _ := req.Params.Arguments["description"].(string)
			group, _ := req.Params.Arguments["group"].(string)
			sizeStr, _ := req.Params.Arguments["size"].(string)
			tag, _ := req.Params.Arguments["tag"].(string)
			resolution, _ := req.Params.Arguments["resolution"].(string)

			tier := model.Tier3
			if tierVal, ok := req.Params.Arguments["tier"].(float64); ok && tierVal >= 1 && tierVal <= 5 {
				tier = model.Tier(int(tierVal))
			}

			if sizeStr == "" {
				sizeStr = "M"
			}

			task, err := st.AddTask(project, model.Task{
				Title:       title,
				Description: desc,
				Group:       group,
				Size:        model.Size(strings.ToUpper(sizeStr)),
				Tier:        tier,
				Tag:         tag,
				Resolution:  resolution,
			})
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			sum, _ := st.GetSummary(project)
			return mcp.NewToolResultText(fmt.Sprintf("✔ Task created in '%s': #%s [%s] [%s] %s\nProject status: %d/%d open (%d pts)",
				project, task.ID, task.Size, task.Tier.ShortLabel(), task.Title, sum.OpenTasks, sum.TotalTasks, sum.OpenPoints)), nil
		},
	)

	// Tool: complete_task
	s.AddTool(
		mcp.NewTool(
			"complete_task",
			mcp.WithDescription("Mark a task as completed with optional implementation details / resolution summary (or reopen if done: false)."),
			mcp.WithString("task_id", mcp.Description("Numeric ID of the task"), mcp.Required()),
			mcp.WithString("project", mcp.Description("Project slug (optional)")),
			mcp.WithBoolean("done", mcp.Description("Completed status: true (default) or false")),
			mcp.WithString("resolution", mcp.Description("Summary of implementation details, architectural decisions, and resolution (Markdown supported)")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			taskIDRaw := req.Params.Arguments["task_id"]
			taskID := ""
			switch v := taskIDRaw.(type) {
			case string:
				taskID = v
			case float64:
				taskID = strconv.Itoa(int(v))
			}

			if taskID == "" {
				return mcp.NewToolResultError("parameter 'task_id' is required"), nil
			}

			project, _ := req.Params.Arguments["project"].(string)
			if project == "" {
				project = st.GetActiveProjectSlug()
			}

			done := true
			if doneVal, ok := req.Params.Arguments["done"].(bool); ok {
				done = doneVal
			}
			resolution, _ := req.Params.Arguments["resolution"].(string)

			task, err := st.CompleteTask(project, taskID, done, resolution)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			sum, _ := st.GetSummary(project)
			statusStr := "completed"
			if !done {
				statusStr = "marked as pending"
			}
			resInfo := ""
			if task.Resolution != "" {
				resInfo = fmt.Sprintf("\nResolution: %s", task.Resolution)
			}
			return mcp.NewToolResultText(fmt.Sprintf("✔ Task #%s %s in '%s': %s%s\nRemaining status: %d/%d open (%d pts)",
				task.ID, statusStr, project, task.Title, resInfo, sum.OpenTasks, sum.TotalTasks, sum.OpenPoints)), nil
		},
	)

	// Tool: update_task
	s.AddTool(
		mcp.NewTool(
			"update_task",
			mcp.WithDescription("Update fields of an existing task."),
			mcp.WithString("task_id", mcp.Description("ID of task to update"), mcp.Required()),
			mcp.WithString("project", mcp.Description("Project slug")),
			mcp.WithString("title", mcp.Description("New title")),
			mcp.WithString("description", mcp.Description("New description")),
			mcp.WithString("group", mcp.Description("New category/group")),
			mcp.WithString("size", mcp.Description("New effort size ('XS', 'S', 'M', 'L', 'XL')")),
			mcp.WithNumber("tier", mcp.Description("New Tier (1..5)")),
			mcp.WithString("tag", mcp.Description("New tag")),
			mcp.WithString("resolution", mcp.Description("New resolution / implementation details summary")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			taskIDRaw := req.Params.Arguments["task_id"]
			taskID := ""
			switch v := taskIDRaw.(type) {
			case string:
				taskID = v
			case float64:
				taskID = strconv.Itoa(int(v))
			}

			if taskID == "" {
				return mcp.NewToolResultError("parameter 'task_id' is required"), nil
			}

			project, _ := req.Params.Arguments["project"].(string)
			if project == "" {
				project = st.GetActiveProjectSlug()
			}

			update := model.Task{ID: taskID}
			if t, ok := req.Params.Arguments["title"].(string); ok {
				update.Title = t
			}
			if d, ok := req.Params.Arguments["description"].(string); ok {
				update.Description = d
			}
			if g, ok := req.Params.Arguments["group"].(string); ok {
				update.Group = g
			}
			if s, ok := req.Params.Arguments["size"].(string); ok && s != "" {
				update.Size = model.Size(strings.ToUpper(s))
			}
			if tierVal, ok := req.Params.Arguments["tier"].(float64); ok && tierVal > 0 {
				update.Tier = model.Tier(int(tierVal))
			}
			if tagVal, ok := req.Params.Arguments["tag"].(string); ok {
				update.Tag = tagVal
			}
			if resVal, ok := req.Params.Arguments["resolution"].(string); ok {
				update.Resolution = resVal
			}

			task, err := st.UpdateTask(project, update)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			return mcp.NewToolResultText(fmt.Sprintf("✔ Task #%s updated in '%s': %s", task.ID, project, task.Title)), nil
		},
	)

	// Tool: delete_task
	s.AddTool(
		mcp.NewTool(
			"delete_task",
			mcp.WithDescription("Permanently delete a task from the backlog."),
			mcp.WithString("task_id", mcp.Description("ID of task to delete"), mcp.Required()),
			mcp.WithString("project", mcp.Description("Project slug")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			taskIDRaw := req.Params.Arguments["task_id"]
			taskID := ""
			switch v := taskIDRaw.(type) {
			case string:
				taskID = v
			case float64:
				taskID = strconv.Itoa(int(v))
			}

			if taskID == "" {
				return mcp.NewToolResultError("parameter 'task_id' is required"), nil
			}

			project, _ := req.Params.Arguments["project"].(string)
			if project == "" {
				project = st.GetActiveProjectSlug()
			}

			if err := st.DeleteTask(project, taskID); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			return mcp.NewToolResultText(fmt.Sprintf("✔ Task #%s deleted from '%s'", taskID, project)), nil
		},
	)

	return s
}
