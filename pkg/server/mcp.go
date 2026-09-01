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

const BacklogInstructions = `You are connected to Backlog, an issue and task tracking management system for software engineering.
Follow this standard protocol when interacting with Backlog:

1. INITIAL ON-CONNECT HANDSHAKE:
   - As soon as you connect to Backlog or start a conversation, immediately call 'list_tasks(assignee="<your-handle>", done=false)' (where <your-handle> is your agent handle like 'claude', 'antigravity', etc.) to inspect any pending tasks currently assigned to you by the user or team.
   - If you have assigned tasks, report them to the user and prioritize working on them before picking up unassigned work.

2. PERIODIC ASSIGNMENT CHECKING:
   - While working in the session, between tasks or when completing a milestone, periodically check 'list_tasks(assignee="<your-handle>", done=false)' to discover if the user or another agent has assigned you new tasks in the GUI.

3. WORKFLOW LIFECYCLE & STRICT TDD REQUIREMENT:
   - Discover: If you have no assigned tasks, use 'get_top_priorities' or 'list_tasks(assignee="unassigned", done=false)' to find pending work.
   - Claim & Assign: BEFORE starting work on a task, call 'assign_task(task_id="<ID>", assignee="<your-handle>")'. This updates the Backlog GUI in real time and signals that the task is currently in progress.
   - Strict TDD (Test-Driven Development):
     * Always develop following a strict TDD methodology: write or update tests FIRST to specify the expected behavior.
     * Implement the code changes to satisfy the tests.
     * Maximize test coverage: ensure thorough coverage for all new or modified code paths.
     * ZERO COVERAGE REGRESSION: The overall project test coverage percentage MUST NOT decrease with any new commit.
   - Git Commit Requirement:
     * The work for every task MUST culminate in a Git commit once tests pass and coverage is verified.
   - Complete with Commit Hash:
     * Once committed, call 'complete_task(task_id="<ID>", done=true, resolution="...")'.
     * The 'resolution' field MUST explicitly include:
       1) The Git commit hash created (e.g. 'Commit: abc1234').
       2) Summary of implementation details and architectural decisions.
       3) Files modified and test verification / coverage results.

4. ESTIMATION AND PRIORITIES:
   - Priority Tiers: 1 (Blocker) -> 2 (Important) -> 3 (Visual debt) -> 4 (Internal) -> 5 (Future). Always address Tier 1 and 2 tasks first.
   - Effort Sizes: XS (1 pt), S (2 pts), M (3 pts), L (5 pts), XL (8 pts).

5. REPORTING:
   - Always inform the user when claiming a task, report test coverage results, and report completion with the commit hash and resolution summary.`

func NewMCPServer(st *store.Store) *server.MCPServer {
	s := server.NewMCPServer(
		"backlog",
		"1.0.0",
		server.WithLogging(),
		server.WithInstructions(BacklogInstructions),
	)

	// Resource: backlog://workflow
	s.AddResource(
		mcp.NewResource("backlog://workflow", "Backlog AI Workflow Guidelines", mcp.WithMIMEType("text/markdown")),
		func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			return []mcp.ResourceContents{
				mcp.TextResourceContents{
					URI:      "backlog://workflow",
					MIMEType: "text/markdown",
					Text:     BacklogInstructions,
				},
			}, nil
		},
	)

	// Prompt: pick_next_task
	s.AddPrompt(
		mcp.NewPrompt(
			"pick_next_task",
			mcp.WithPromptDescription("Guide the AI to find the highest-priority pending task, assign it to itself, and plan implementation."),
			mcp.WithArgument("agent", mcp.ArgumentDescription("Your agent handle (e.g. 'claude', 'antigravity')"), mcp.RequiredArgument()),
			mcp.WithArgument("project", mcp.ArgumentDescription("Project slug (optional, defaults to active)")),
		),
		func(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			agent := req.Params.Arguments["agent"]
			project := req.Params.Arguments["project"]
			if project == "" {
				project = st.GetActiveProjectSlug()
			}

			promptText := fmt.Sprintf(`Please perform the following workflow in Backlog:
1. Check for tasks assigned to you: call 'list_tasks(assignee="%s", done=false, project="%s")'.
2. If you already have assigned open tasks, pick the highest priority one and proceed.
3. If no tasks are assigned to you, call 'get_top_priorities(project="%s", limit=5)' or 'list_tasks(assignee="unassigned", done=false, project="%s")'.
4. Claim the task: call 'assign_task(task_id="<ID>", assignee="%s", project="%s")' so it shows assigned to you in the Backlog GUI.
5. Plan implementation following TDD (write tests first, ensure coverage does not decrease).`, agent, project, project, project, agent, project)

			return mcp.NewGetPromptResult(
				"Pick Next Task Workflow",
				[]mcp.PromptMessage{
					mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent(promptText)),
				},
			), nil
		},
	)

	// Prompt: complete_task_workflow
	s.AddPrompt(
		mcp.NewPrompt(
			"complete_task_workflow",
			mcp.WithPromptDescription("Guide the AI to mark a task as completed with structured resolution details including Git commit hash and test coverage."),
			mcp.WithArgument("task_id", mcp.ArgumentDescription("ID of the task completed"), mcp.RequiredArgument()),
			mcp.WithArgument("commit_hash", mcp.ArgumentDescription("Git commit hash created for this task"), mcp.RequiredArgument()),
			mcp.WithArgument("resolution", mcp.ArgumentDescription("Markdown summary of implementation details and test coverage"), mcp.RequiredArgument()),
		),
		func(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			taskID := req.Params.Arguments["task_id"]
			commitHash := req.Params.Arguments["commit_hash"]
			res := req.Params.Arguments["resolution"]

			promptText := fmt.Sprintf(`Follow the completion protocol:
1. Ensure all tests pass and overall test coverage has not decreased.
2. Verify commit '%s' exists in git history.
3. Call 'complete_task(task_id="%s", done=true, resolution="Commit: %s\n\n%s")'.
4. Summarize the resolution to the user with the commit hash and test results.`, commitHash, taskID, commitHash, res)

			return mcp.NewGetPromptResult(
				"Complete Task Workflow",
				[]mcp.PromptMessage{
					mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent(promptText)),
				},
			), nil
		},
	)

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
			mcp.WithString("project", mcp.Description("Project slug to activate (e.g. 'my-project')"), mcp.Required()),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			project := req.GetString("project", "")
			if project == "" {
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
			mcp.WithDescription("List tasks in a project with optional filters by Tier (1 to 5), Group, Status (open/completed), Size, or search query."),
			mcp.WithString("project", mcp.Description("Project slug (optional; defaults to active project)")),
			mcp.WithNumber("tier", mcp.Description("Filter by priority Tier: 1=Blocker, 2=Important, 3=Visual debt, 4=Internal, 5=Future")),
			mcp.WithString("group", mcp.Description("Filter by category/group (e.g. 'Monetization', 'Domains', 'Bugs')")),
			mcp.WithString("size", mcp.Description("Filter by size: 'XS', 'S', 'M', 'L', 'XL'")),
			mcp.WithBoolean("done", mcp.Description("Filter by status: true=completed, false=open")),
			mcp.WithString("search", mcp.Description("Text search term")),
			mcp.WithString("assignee", mcp.Description("Filter by assignee (e.g. 'claude', 'manuel', 'unassigned')")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			project := req.GetString("project", "")
			if project == "" {
				project = st.GetActiveProjectSlug()
			}

			filter := model.TaskFilter{}
			tierVal := req.GetInt("tier", 0)
			if tierVal >= 1 && tierVal <= 5 {
				t := model.Tier(tierVal)
				filter.Tier = &t
			}
			if gVal := req.GetString("group", ""); gVal != "" {
				filter.Group = &gVal
			}
			if sVal := req.GetString("size", ""); sVal != "" {
				sz := model.Size(strings.ToUpper(sVal))
				filter.Size = &sz
			}
			if rawArgs := req.GetArguments(); rawArgs != nil {
				if doneVal, ok := rawArgs["done"].(bool); ok {
					filter.Done = &doneVal
				}
			}
			if searchVal := req.GetString("search", ""); searchVal != "" {
				filter.Search = searchVal
			}
			if assigneeVal := req.GetString("assignee", ""); assigneeVal != "" {
				filter.Assignee = &assigneeVal
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
			project := req.GetString("project", "")
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
			project := req.GetString("project", "")
			if project == "" {
				project = st.GetActiveProjectSlug()
			}

			limit := req.GetInt("limit", 5)
			if limit <= 0 {
				limit = 5
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
			mcp.WithString("assignee", mcp.Description("Assignee name/handle (e.g. 'claude', 'manuel')")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			title := strings.TrimSpace(req.GetString("title", ""))
			if title == "" {
				return mcp.NewToolResultError("parameter 'title' is required"), nil
			}

			project := req.GetString("project", "")
			if project == "" {
				project = st.GetActiveProjectSlug()
			}

			desc := req.GetString("description", "")
			group := req.GetString("group", "")
			sizeStr := req.GetString("size", "M")
			tag := req.GetString("tag", "")
			resolution := req.GetString("resolution", "")
			assignee := req.GetString("assignee", "")

			tier := model.Tier3
			tierVal := req.GetInt("tier", 3)
			if tierVal >= 1 && tierVal <= 5 {
				tier = model.Tier(tierVal)
			}

			task, err := st.AddTask(project, model.Task{
				Title:       title,
				Description: desc,
				Group:       group,
				Size:        model.Size(strings.ToUpper(sizeStr)),
				Tier:        tier,
				Tag:         tag,
				Resolution:  resolution,
				Assignee:    assignee,
			})
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			sum, _ := st.GetSummary(project)
			return mcp.NewToolResultText(fmt.Sprintf("✔ Task created in '%s': #%s [%s] [%s] %s\nProject status: %d/%d open (%d pts)",
				project, task.ID, task.Size, task.Tier.ShortLabel(), task.Title, sum.OpenTasks, sum.TotalTasks, sum.OpenPoints)), nil
		},
	)

	// Tool: assign_task
	s.AddTool(
		mcp.NewTool(
			"assign_task",
			mcp.WithDescription("Assign a task to an agent (e.g. 'claude', 'antigravity') or person, or unassign (empty string). Call this before starting work on a task."),
			mcp.WithString("task_id", mcp.Description("Numeric ID of the task"), mcp.Required()),
			mcp.WithString("assignee", mcp.Description("Agent or user handle to assign to (e.g. 'claude', 'manuel'). Pass empty string to unassign."), mcp.Required()),
			mcp.WithString("project", mcp.Description("Project slug (optional)")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			taskID := req.GetString("task_id", "")
			if taskID == "" {
				if rawArgs := req.GetArguments(); rawArgs != nil {
					if v, ok := rawArgs["task_id"].(float64); ok {
						taskID = strconv.Itoa(int(v))
					}
				}
			}

			if taskID == "" {
				return mcp.NewToolResultError("parameter 'task_id' is required"), nil
			}

			assignee := req.GetString("assignee", "")
			project := req.GetString("project", "")
			if project == "" {
				project = st.GetActiveProjectSlug()
			}

			task, err := st.AssignTask(project, taskID, assignee)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			if task.Assignee != "" {
				return mcp.NewToolResultText(fmt.Sprintf("✔ Task #%s assigned to @%s in '%s': %s", task.ID, strings.TrimPrefix(task.Assignee, "@"), project, task.Title)), nil
			}
			return mcp.NewToolResultText(fmt.Sprintf("✔ Task #%s unassigned in '%s': %s", task.ID, project, task.Title)), nil
		},
	)

	// Tool: complete_task
	s.AddTool(
		mcp.NewTool(
			"complete_task",
			mcp.WithDescription("Mark a task as completed. WORKFLOW REQUIREMENT: Develop following strict TDD with high test coverage (zero coverage regression). Work must culminate in a Git commit. The 'resolution' argument MUST include the commit hash created (e.g. 'Commit: abc1234') along with implementation details and test verification summary."),
			mcp.WithString("task_id", mcp.Description("Numeric ID of the task"), mcp.Required()),
			mcp.WithString("project", mcp.Description("Project slug (optional)")),
			mcp.WithBoolean("done", mcp.Description("Completed status: true (default) or false")),
			mcp.WithString("resolution", mcp.Description("Summary of implementation details, files changed, test verification, and MUST include the Git commit hash (e.g. 'Commit: a1b2c3d')")),
			mcp.WithString("assignee", mcp.Description("Agent or user handle that resolved the task (optional)")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			taskID := req.GetString("task_id", "")
			if taskID == "" {
				if rawArgs := req.GetArguments(); rawArgs != nil {
					if v, ok := rawArgs["task_id"].(float64); ok {
						taskID = strconv.Itoa(int(v))
					}
				}
			}

			if taskID == "" {
				return mcp.NewToolResultError("parameter 'task_id' is required"), nil
			}

			project := req.GetString("project", "")
			if project == "" {
				project = st.GetActiveProjectSlug()
			}

			done := true
			if rawArgs := req.GetArguments(); rawArgs != nil {
				if dVal, ok := rawArgs["done"].(bool); ok {
					done = dVal
				}
			}
			resolution := req.GetString("resolution", "")

			if aVal := req.GetString("assignee", ""); aVal != "" {
				_, _ = st.AssignTask(project, taskID, aVal)
			}

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
			mcp.WithString("assignee", mcp.Description("New assignee handle (or empty to unassign)")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			taskID := req.GetString("task_id", "")
			if taskID == "" {
				if rawArgs := req.GetArguments(); rawArgs != nil {
					if v, ok := rawArgs["task_id"].(float64); ok {
						taskID = strconv.Itoa(int(v))
					}
				}
			}

			if taskID == "" {
				return mcp.NewToolResultError("parameter 'task_id' is required"), nil
			}

			project := req.GetString("project", "")
			if project == "" {
				project = st.GetActiveProjectSlug()
			}

			update := model.Task{ID: taskID}
			if t := req.GetString("title", ""); t != "" {
				update.Title = t
			}
			if d := req.GetString("description", ""); d != "" {
				update.Description = d
			}
			if g := req.GetString("group", ""); g != "" {
				update.Group = g
			}
			if s := req.GetString("size", ""); s != "" {
				update.Size = model.Size(strings.ToUpper(s))
			}
			if tierVal := req.GetInt("tier", 0); tierVal > 0 {
				update.Tier = model.Tier(tierVal)
			}
			if tagVal := req.GetString("tag", ""); tagVal != "" {
				update.Tag = tagVal
			}
			if resVal := req.GetString("resolution", ""); resVal != "" {
				update.Resolution = resVal
			}
			if assignVal := req.GetString("assignee", ""); assignVal != "" {
				update.Assignee = assignVal
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
			taskID := req.GetString("task_id", "")
			if taskID == "" {
				if rawArgs := req.GetArguments(); rawArgs != nil {
					if v, ok := rawArgs["task_id"].(float64); ok {
						taskID = strconv.Itoa(int(v))
					}
				}
			}

			if taskID == "" {
				return mcp.NewToolResultError("parameter 'task_id' is required"), nil
			}

			project := req.GetString("project", "")
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
