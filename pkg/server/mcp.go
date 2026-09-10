package server

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/altenwald/backlog/pkg/model"
	"github.com/altenwald/backlog/pkg/store"
	"github.com/altenwald/backlog/pkg/version"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// BacklogCoreInstructions is the immutable part of the MCP system prompt.
// It describes the Backlog tool protocol and MUST NOT be modified by users.
const BacklogCoreInstructions = `You are connected to Backlog, an issue and task tracking management system for software engineering.
Follow this standard protocol when interacting with Backlog:

0. MULTI-PROJECT REQUIREMENT:
   - Backlog manages tasks strictly scoped by project.
   - The 'project' parameter is MANDATORY for all task and summary operations (e.g. 'list_tasks', 'add_task', 'get_summary', 'assign_task', 'complete_task', 'update_task', 'delete_task').
   - Call 'list_projects()' to discover existing project slugs in the workspace.

1. INITIAL ON-CONNECT HANDSHAKE:
   - As soon as you connect to Backlog or start a conversation, determine the target project (via 'list_projects()' or user instruction), then call 'list_tasks(project="<project-slug>", assignee="<your-handle>", done=false)' (where <your-handle> is your agent handle like 'claude', 'antigravity', etc.) to inspect any pending tasks currently assigned to you by the user or team.
   - If you have assigned tasks, report them to the user and prioritize working on them before picking up unassigned work.

2. PERIODIC ASSIGNMENT CHECKING:
   - While working in the session, between tasks or when completing a milestone, periodically check 'list_tasks(project="<project-slug>", assignee="<your-handle>", done=false)' to discover if the user or another agent has assigned you new tasks in the GUI.

3. WORKFLOW LIFECYCLE:
   - Discover: If you have no assigned tasks, use 'get_top_priorities(project="<project-slug>")' or 'list_tasks(project="<project-slug>", assignee="unassigned", done=false)' to find pending work.
   - Claim & Assign: BEFORE starting work on a task, call 'assign_task(project="<project-slug>", task_id="<ID>", assignee="<your-handle>")'. This updates the Backlog GUI in real time and signals that the task is currently in progress.
   - Git Commit Requirement:
     * The work for every task MUST culminate in a Git commit once the implementation is complete.
   - Complete with Commit Hash:
     * Once committed, call 'complete_task(project="<project-slug>", task_id="<ID>", done=true, resolution="...")'.
     * The 'resolution' field MUST explicitly include:
       1) The Git commit hash created (e.g. 'Commit: abc1234').
       2) Summary of implementation details and architectural decisions.
       3) Files modified and verification results.

4. ESTIMATION AND PRIORITY TIERS:
   - Priority Tiers (1 to 5):
     * Tier 1 (Blocker): Critical issues, broken builds, fatal runtime crashes, or severe regressions blocking development or core functionality. Must be addressed immediately before any other work.
     * Tier 2 (Important): Key features, primary milestone deliverables, and high-impact bugs. Essential work planned for the current release or sprint cycle.
     * Tier 3 (Visual debt): UI/UX inconsistencies, styling issues, alignment/spacing glitches, and visual polish that degrade user experience without breaking application logic.
     * Tier 4 (Internal): Developer tooling, refactoring, test suite improvements, dependency maintenance, CI/CD automation, and internal infrastructure.
     * Tier 5 (Future): Nice-to-have suggestions, experimental ideas, or deferred feature proposals for future milestones.
   - Execution rule: Always address Tier 1 and Tier 2 tasks first.
   - Effort Sizes: XS, S, M, L, XL.
   - Task Hierarchy & Subtask Branching:
     * Tasks can be decomposed hierarchically: when breaking down a larger feature, design goal, or complex issue into smaller parts, create child tasks by providing 'parent_id="<parent-task-id>"'.
     * Subtasks can also branch into further subtasks.
     * When a parent task is deleted, all its descendant subtasks are automatically cascade deleted.
   - Dependencies & Blocking:
     * Tasks can declare dependencies via 'depends_on=["<task-id>"]' (or comma-separated string).
     * A task is BLOCKED until all its dependency tasks are marked as completed ('done=true').
     * CRITICAL AGENT RULE: Never pick or start work on a task that is BLOCKED. Always resolve the blocking dependencies first.

5. REPORTING:
   - Always inform the user when claiming a task, report implementation results, and report completion with the commit hash and resolution summary.`

// BacklogDefaultUserInstructions is the default editable section appended to the core instructions.
// Users can replace or extend this text from Settings to match their team's methodology.
const BacklogDefaultUserInstructions = `
6. DEVELOPMENT METHODOLOGY:
   - Strict TDD (Test-Driven Development):
     * Always develop following a strict TDD methodology: write or update tests FIRST to specify the expected behavior.
     * Implement the code changes to satisfy the tests.
     * Maximize test coverage: ensure thorough coverage for all new or modified code paths.
     * ZERO COVERAGE REGRESSION: The overall project test coverage percentage MUST NOT decrease with any new commit.`

// BuildInstructions composes the full MCP system prompt from the immutable core and
// the user-supplied instructions. If userInstructions is empty, the default is used.
func BuildInstructions(userInstructions string) string {
	custom := strings.TrimSpace(userInstructions)
	if custom == "" {
		custom = BacklogDefaultUserInstructions
	}
	return BacklogCoreInstructions + "\n" + custom
}

// BacklogInstructions is kept for backwards compatibility (e.g. tests that reference it directly).
// It returns the full instructions with the default user section.
var BacklogInstructions = BuildInstructions("")

func NewMCPServer(st *store.Store) *server.MCPServer {
	return NewMCPServerWithBackend(NewStoreBackend(st))
}

func NewMCPServerWithBackend(be Backend) *server.MCPServer {
	userInst, _ := be.GetSettings()
	return newMCPServerWithInstructions(be, BuildInstructions(userInst))
}

func newMCPServerWithInstructions(be Backend, instructions string) *server.MCPServer {
	s := server.NewMCPServer(
		"backlog",
		version.Version,
		server.WithLogging(),
		server.WithInstructions(instructions),
	)

	// Resource: backlog://workflow
	s.AddResource(
		mcp.NewResource("backlog://workflow", "Backlog AI Workflow Guidelines", mcp.WithMIMEType("text/markdown")),
		func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			currentInst := instructions
			if userInst, err := be.GetSettings(); err == nil {
				currentInst = BuildInstructions(userInst)
			}
			return []mcp.ResourceContents{
				mcp.TextResourceContents{
					URI:      "backlog://workflow",
					MIMEType: "text/markdown",
					Text:     currentInst,
				},
			}, nil
		},
	)

	// Prompt: pick_next_task
	s.AddPrompt(
		mcp.NewPrompt(
			"pick_next_task",
			mcp.WithPromptDescription("Guide the AI to find the highest-priority pending task, assign it to itself, and plan implementation."),
			mcp.WithArgument("project", mcp.ArgumentDescription("Project slug (required)"), mcp.RequiredArgument()),
			mcp.WithArgument("agent", mcp.ArgumentDescription("Your agent handle (e.g. 'claude', 'antigravity')"), mcp.RequiredArgument()),
		),
		func(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			project := strings.TrimSpace(req.Params.Arguments["project"])
			if project == "" {
				return nil, fmt.Errorf("argument 'project' is required")
			}
			agent := strings.TrimSpace(req.Params.Arguments["agent"])

			promptText := fmt.Sprintf(`Please perform the following workflow in Backlog:
1. Check for tasks assigned to you: call 'list_tasks(project="%s", assignee="%s", done=false)'.
2. If you already have assigned open tasks, pick the highest priority one and proceed.
3. If no tasks are assigned to you, call 'get_top_priorities(project="%s", limit=5)' or 'list_tasks(project="%s", assignee="unassigned", done=false)'.
4. Claim the task: call 'assign_task(project="%s", task_id="<ID>", assignee="%s")' so it shows assigned to you in the Backlog GUI.
5. Plan implementation following TDD (write tests first, ensure coverage does not decrease).`, project, agent, project, project, project, agent)

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
			mcp.WithArgument("project", mcp.ArgumentDescription("Project slug (required)"), mcp.RequiredArgument()),
			mcp.WithArgument("task_id", mcp.ArgumentDescription("ID of the task completed"), mcp.RequiredArgument()),
			mcp.WithArgument("commit_hash", mcp.ArgumentDescription("Git commit hash created for this task"), mcp.RequiredArgument()),
			mcp.WithArgument("resolution", mcp.ArgumentDescription("Markdown summary of implementation details and test coverage"), mcp.RequiredArgument()),
		),
		func(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			project := strings.TrimSpace(req.Params.Arguments["project"])
			if project == "" {
				return nil, fmt.Errorf("argument 'project' is required")
			}
			taskID := strings.TrimSpace(req.Params.Arguments["task_id"])
			commitHash := strings.TrimSpace(req.Params.Arguments["commit_hash"])
			res := strings.TrimSpace(req.Params.Arguments["resolution"])

			promptText := fmt.Sprintf(`Follow the completion protocol:
1. Ensure all tests pass and overall test coverage has not decreased.
2. Verify commit '%s' exists in git history.
3. Call 'complete_task(project="%s", task_id="%s", done=true, resolution="Commit: %s\n\n%s")'.
4. Summarize the resolution to the user with the commit hash and test results.`, commitHash, project, taskID, commitHash, res)

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
			mcp.WithDescription("List all registered projects in Backlog with metrics and open tasks."),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			items, err := be.ListProjects()
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			data, _ := json.MarshalIndent(items, "", "  ")
			return mcp.NewToolResultText(string(data)), nil
		},
	)

	// Tool: create_project
	s.AddTool(
		mcp.NewTool(
			"create_project",
			mcp.WithDescription("Create a new project in Backlog."),
			mcp.WithString("slug", mcp.Description("Unique identifier slug for the project (e.g. 'my-app')"), mcp.Required()),
			mcp.WithString("name", mcp.Description("Human-readable display name (e.g. 'My Application')")),
			mcp.WithString("description", mcp.Description("Project description or notes")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			slug := req.GetString("slug", "")
			if slug == "" {
				return mcp.NewToolResultError("parameter 'slug' is required"), nil
			}
			name := req.GetString("name", "")
			desc := req.GetString("description", "")
			p, err := be.CreateProject(slug, name, desc)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(fmt.Sprintf("✔ Project '%s' (%s) created successfully", p.Slug, p.Name)), nil
		},
	)

	// Tool: delete_project
	s.AddTool(
		mcp.NewTool(
			"delete_project",
			mcp.WithDescription("Permanently delete a project and all its tasks from Backlog."),
			mcp.WithString("project", mcp.Description("Unique identifier slug of the project to delete (e.g. 'test')"), mcp.Required()),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			slug := req.GetString("project", "")
			if slug == "" {
				slug = req.GetString("slug", "")
			}
			if slug == "" {
				return mcp.NewToolResultError("parameter 'project' is required"), nil
			}
			if err := be.DeleteProject(slug); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(fmt.Sprintf("✔ Project '%s' deleted successfully", slug)), nil
		},
	)

	// Tool: list_tasks
	s.AddTool(
		mcp.NewTool(
			"list_tasks",
			mcp.WithDescription("List tasks in a project with optional filters by Tier (1 to 5), parent task ID, Status (open/completed), Size, or search query."),
			mcp.WithString("project", mcp.Description("Project slug (required)"), mcp.Required()),
			mcp.WithNumber("tier", mcp.Description("Filter by priority Tier: 1=Blocker, 2=Important, 3=Visual debt, 4=Internal, 5=Future")),
			mcp.WithString("parent_id", mcp.Description("Filter by parent task ID (optional; pass task ID to list subtasks)")),
			mcp.WithString("depends_on", mcp.Description("Filter tasks that depend on this specific task ID (optional)")),
			mcp.WithBoolean("blocked", mcp.Description("Filter by blocked state: true=only blocked tasks, false=only unblocked/actionable tasks")),
			mcp.WithString("size", mcp.Description("Filter by size: 'XS', 'S', 'M', 'L', 'XL'")),
			mcp.WithBoolean("done", mcp.Description("Filter by status: true=completed, false=open")),
			mcp.WithString("search", mcp.Description("Text search term")),
			mcp.WithString("assignee", mcp.Description("Filter by assignee (e.g. 'claude', 'manuel', 'unassigned')")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			project := strings.TrimSpace(req.GetString("project", ""))
			if project == "" {
				return mcp.NewToolResultError("parameter 'project' is required"), nil
			}

			filter := model.TaskFilter{}
			tierVal := req.GetInt("tier", 0)
			if tierVal >= 1 && tierVal <= 5 {
				t := model.Tier(tierVal)
				filter.Tier = &t
			}
			if pVal := req.GetString("parent_id", ""); pVal != "" {
				filter.ParentID = &pVal
			}
			if depVal := req.GetString("depends_on", ""); depVal != "" {
				filter.DependsOn = &depVal
			}
			if sVal := req.GetString("size", ""); sVal != "" {
				sz := model.Size(strings.ToUpper(sVal))
				filter.Size = &sz
			}
			if rawArgs := req.GetArguments(); rawArgs != nil {
				if doneVal, ok := rawArgs["done"].(bool); ok {
					filter.Done = &doneVal
				}
				if blockedVal, ok := rawArgs["blocked"].(bool); ok {
					filter.Blocked = &blockedVal
				}
			}
			if searchVal := req.GetString("search", ""); searchVal != "" {
				filter.Search = searchVal
			}
			if assigneeVal := req.GetString("assignee", ""); assigneeVal != "" {
				filter.Assignee = &assigneeVal
			}

			tasks, err := be.ListTasks(project, filter)
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
			mcp.WithDescription("Get metric summary, open/total tasks, and breakdown by size and tier for a project."),
			mcp.WithString("project", mcp.Description("Project slug (required)"), mcp.Required()),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			project := strings.TrimSpace(req.GetString("project", ""))
			if project == "" {
				return mcp.NewToolResultError("parameter 'project' is required"), nil
			}

			sum, err := be.GetSummary(project)
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
			mcp.WithString("project", mcp.Description("Project slug (required)"), mcp.Required()),
			mcp.WithNumber("limit", mcp.Description("Number of tasks to return (default 5)")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			project := strings.TrimSpace(req.GetString("project", ""))
			if project == "" {
				return mcp.NewToolResultError("parameter 'project' is required"), nil
			}

			limit := req.GetInt("limit", 5)
			if limit <= 0 {
				limit = 5
			}

			tasks, err := be.GetTopPriorities(project, limit)
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
			mcp.WithString("project", mcp.Description("Project slug (required)"), mcp.Required()),
			mcp.WithString("title", mcp.Description("Concise title of the task"), mcp.Required()),
			mcp.WithString("description", mcp.Description("Detailed description or context")),
			mcp.WithString("parent_id", mcp.Description("Parent task ID if this task is a subtask/branch (optional)")),
			mcp.WithString("depends_on", mcp.Description("Comma-separated task IDs this task depends on / is blocked by (optional)")),
			mcp.WithString("size", mcp.Description("Effort size: 'XS', 'S', 'M', 'L', 'XL' (default 'M')")),
			mcp.WithNumber("tier", mcp.Description("Priority tier: 1 (Blocker) to 5 (Future). Default 3")),
			mcp.WithString("resolution", mcp.Description("Summary of implementation details or resolution (optional)")),
			mcp.WithString("assignee", mcp.Description("Assignee name/handle (e.g. 'claude', 'manuel')")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			project := strings.TrimSpace(req.GetString("project", ""))
			if project == "" {
				return mcp.NewToolResultError("parameter 'project' is required"), nil
			}

			title := strings.TrimSpace(req.GetString("title", ""))
			if title == "" {
				return mcp.NewToolResultError("parameter 'title' is required"), nil
			}

			desc := req.GetString("description", "")
			parentID := req.GetString("parent_id", "")
			sizeStr := req.GetString("size", "M")
			resolution := req.GetString("resolution", "")
			assignee := req.GetString("assignee", "")

			var dependsOn []string
			if rawArgs := req.GetArguments(); rawArgs != nil {
				if deps, ok := rawArgs["depends_on"].([]any); ok {
					for _, d := range deps {
						if s, ok := d.(string); ok && strings.TrimSpace(s) != "" {
							dependsOn = append(dependsOn, strings.TrimSpace(s))
						}
					}
				}
			}
			if len(dependsOn) == 0 {
				if depStr := req.GetString("depends_on", ""); depStr != "" {
					for _, part := range strings.Split(depStr, ",") {
						if s := strings.TrimSpace(part); s != "" {
							dependsOn = append(dependsOn, s)
						}
					}
				}
			}

			tier := model.Tier3
			tierVal := req.GetInt("tier", 3)
			if tierVal >= 1 && tierVal <= 5 {
				tier = model.Tier(tierVal)
			}

			task, err := be.AddTask(project, model.Task{
				Title:       title,
				Description: desc,
				ParentID:    parentID,
				DependsOn:   dependsOn,
				Size:        model.Size(strings.ToUpper(sizeStr)),
				Tier:        tier,
				Resolution:  resolution,
				Assignee:    assignee,
			})
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			sum, _ := be.GetSummary(project)
			return mcp.NewToolResultText(fmt.Sprintf("✔ Task created in '%s': #%s [%s] [%s] %s\nProject status: %d/%d open",
				project, task.ID, task.Size, task.Tier.ShortLabel(), task.Title, sum.OpenTasks, sum.TotalTasks)), nil
		},
	)

	// Tool: assign_task
	s.AddTool(
		mcp.NewTool(
			"assign_task",
			mcp.WithDescription("Assign a task to an agent (e.g. 'claude', 'antigravity') or person, or unassign (empty string). Call this before starting work on a task."),
			mcp.WithString("project", mcp.Description("Project slug (required)"), mcp.Required()),
			mcp.WithString("task_id", mcp.Description("Numeric ID of the task"), mcp.Required()),
			mcp.WithString("assignee", mcp.Description("Agent or user handle to assign to (e.g. 'claude', 'manuel'). Pass empty string to unassign."), mcp.Required()),
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

			project := strings.TrimSpace(req.GetString("project", ""))
			if project == "" {
				return mcp.NewToolResultError("parameter 'project' is required"), nil
			}

			assignee := req.GetString("assignee", "")

			task, err := be.AssignTask(project, taskID, assignee)
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
			mcp.WithString("project", mcp.Description("Project slug (required)"), mcp.Required()),
			mcp.WithString("task_id", mcp.Description("Numeric ID of the task"), mcp.Required()),
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

			project := strings.TrimSpace(req.GetString("project", ""))
			if project == "" {
				return mcp.NewToolResultError("parameter 'project' is required"), nil
			}

			done := true
			if rawArgs := req.GetArguments(); rawArgs != nil {
				if dVal, ok := rawArgs["done"].(bool); ok {
					done = dVal
				}
			}
			resolution := req.GetString("resolution", "")

			if aVal := req.GetString("assignee", ""); aVal != "" {
				_, _ = be.AssignTask(project, taskID, aVal)
			}

			task, err := be.CompleteTask(project, taskID, done, resolution)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			sum, _ := be.GetSummary(project)
			statusStr := "completed"
			if !done {
				statusStr = "marked as pending"
			}
			resInfo := ""
			if task.Resolution != "" {
				resInfo = fmt.Sprintf("\nResolution: %s", task.Resolution)
			}
			return mcp.NewToolResultText(fmt.Sprintf("✔ Task #%s %s in '%s': %s%s\nRemaining status: %d/%d open",
				task.ID, statusStr, project, task.Title, resInfo, sum.OpenTasks, sum.TotalTasks)), nil
		},
	)

	// Tool: update_task
	s.AddTool(
		mcp.NewTool(
			"update_task",
			mcp.WithDescription("Update fields of an existing task."),
			mcp.WithString("project", mcp.Description("Project slug (required)"), mcp.Required()),
			mcp.WithString("task_id", mcp.Description("ID of task to update"), mcp.Required()),
			mcp.WithString("title", mcp.Description("New title")),
			mcp.WithString("description", mcp.Description("New description")),
			mcp.WithString("parent_id", mcp.Description("New parent task ID (or 'none'/'0' to detach/unparent)")),
			mcp.WithString("depends_on", mcp.Description("New comma-separated dependency task IDs (or 'none'/'clear' to clear dependencies)")),
			mcp.WithString("size", mcp.Description("New effort size ('XS', 'S', 'M', 'L', 'XL')")),
			mcp.WithNumber("tier", mcp.Description("New Tier (1..5)")),
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

			project := strings.TrimSpace(req.GetString("project", ""))
			if project == "" {
				return mcp.NewToolResultError("parameter 'project' is required"), nil
			}

			update := model.Task{ID: taskID}
			if t := req.GetString("title", ""); t != "" {
				update.Title = t
			}
			if d := req.GetString("description", ""); d != "" {
				update.Description = d
			}
			if p := req.GetString("parent_id", ""); p != "" {
				update.ParentID = p
			}
			if rawArgs := req.GetArguments(); rawArgs != nil {
				if deps, ok := rawArgs["depends_on"].([]any); ok {
					var list []string
					for _, d := range deps {
						if s, ok := d.(string); ok && strings.TrimSpace(s) != "" {
							list = append(list, strings.TrimSpace(s))
						}
					}
					update.DependsOn = list
				}
			}
			if len(update.DependsOn) == 0 {
				if depStr := req.GetString("depends_on", ""); depStr != "" {
					if depStr == "none" || depStr == "clear" {
						update.DependsOn = []string{}
					} else {
						for _, part := range strings.Split(depStr, ",") {
							if s := strings.TrimSpace(part); s != "" {
								update.DependsOn = append(update.DependsOn, s)
							}
						}
					}
				}
			}
			if s := req.GetString("size", ""); s != "" {
				update.Size = model.Size(strings.ToUpper(s))
			}
			if tierVal := req.GetInt("tier", 0); tierVal > 0 {
				update.Tier = model.Tier(tierVal)
			}
			if resVal := req.GetString("resolution", ""); resVal != "" {
				update.Resolution = resVal
			}
			if assignVal := req.GetString("assignee", ""); assignVal != "" {
				update.Assignee = assignVal
			}

			task, err := be.UpdateTask(project, update)
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
			mcp.WithString("project", mcp.Description("Project slug (required)"), mcp.Required()),
			mcp.WithString("task_id", mcp.Description("ID of task to delete"), mcp.Required()),
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

			project := strings.TrimSpace(req.GetString("project", ""))
			if project == "" {
				return mcp.NewToolResultError("parameter 'project' is required"), nil
			}

			if err := be.DeleteTask(project, taskID); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			return mcp.NewToolResultText(fmt.Sprintf("✔ Task #%s deleted from '%s'", taskID, project)), nil
		},
	)

	// Tool: get_settings
	s.AddTool(
		mcp.NewTool(
			"get_settings",
			mcp.WithDescription("Retrieve current Backlog settings, including immutable core instructions, customizable user instructions, and effective prompt."),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			custom, err := be.GetSettings()
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			isDefault := strings.TrimSpace(custom) == ""
			effectiveCustom := custom
			if isDefault {
				effectiveCustom = strings.TrimSpace(BacklogDefaultUserInstructions)
			}

			res := map[string]any{
				"core_instructions":      BacklogCoreInstructions,
				"custom_instructions":    custom,
				"using_default":          isDefault,
				"default_instructions":   strings.TrimSpace(BacklogDefaultUserInstructions),
				"effective_instructions": BacklogCoreInstructions + "\n" + effectiveCustom,
			}
			data, _ := json.MarshalIndent(res, "", "  ")
			return mcp.NewToolResultText(string(data)), nil
		},
	)

	// Tool: update_settings
	s.AddTool(
		mcp.NewTool(
			"update_settings",
			mcp.WithDescription("Update customizable Backlog settings (e.g. user instructions, testing methodology, team conventions). The core Backlog protocol is immutable and cannot be changed."),
			mcp.WithString("mcp_user_instructions", mcp.Description("Custom instructions text to append after the core protocol.")),
			mcp.WithString("instructions", mcp.Description("Alias for mcp_user_instructions.")),
			mcp.WithBoolean("reset_to_default", mcp.Description("If true, resets custom instructions to the built-in default.")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			reset := false
			if rawArgs := req.GetArguments(); rawArgs != nil {
				if rVal, ok := rawArgs["reset_to_default"].(bool); ok {
					reset = rVal
				}
			}
			text := req.GetString("mcp_user_instructions", "")
			if text == "" {
				text = req.GetString("instructions", "")
			}

			if reset || strings.TrimSpace(text) == "default" {
				text = ""
			}

			if err := be.UpdateSettings(text); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			msg := "✔ Settings updated successfully. Custom instructions will take effect on next MCP connection."
			if text == "" {
				msg = "✔ Settings reset to default. Built-in instructions will take effect on next MCP connection."
			}
			return mcp.NewToolResultText(msg), nil
		},
	)

	// Tool: deprecate_task
	s.AddTool(
		mcp.NewTool(
			"deprecate_task",
			mcp.WithDescription("Mark a task as deprecated (and completed) because it is no longer applicable based on the composite project specification."),
			mcp.WithString("project", mcp.Description("Project slug (required)"), mcp.Required()),
			mcp.WithString("task_id", mcp.Description("ID of task to deprecate (required)"), mcp.Required()),
			mcp.WithBoolean("deprecated", mcp.Description("True to deprecate (default), false to undeprecate")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			project := strings.TrimSpace(req.GetString("project", ""))
			if project == "" {
				return mcp.NewToolResultError("parameter 'project' is required"), nil
			}

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

			deprecated := true
			if rawArgs := req.GetArguments(); rawArgs != nil {
				if dVal, ok := rawArgs["deprecated"].(bool); ok {
					deprecated = dVal
				}
			}

			task, err := be.DeprecateTask(project, taskID, deprecated)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			statusMsg := "deprecated"
			if !task.Deprecated {
				statusMsg = "undeprecated"
			}
			return mcp.NewToolResultText(fmt.Sprintf("✔ Task #%s in '%s' is now %s", task.ID, project, statusMsg)), nil
		},
	)

	// Tool: get_project_spec
	s.AddTool(
		mcp.NewTool(
			"get_project_spec",
			mcp.WithDescription("Get the composite project specification / definition document for a project."),
			mcp.WithString("project", mcp.Description("Project slug (required)"), mcp.Required()),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			project := strings.TrimSpace(req.GetString("project", ""))
			if project == "" {
				return mcp.NewToolResultError("parameter 'project' is required"), nil
			}

			spec, err := be.GetProjectSpecification(project)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			res := map[string]string{
				"project":       project,
				"specification": spec,
			}
			data, _ := json.MarshalIndent(res, "", "  ")
			return mcp.NewToolResultText(string(data)), nil
		},
	)

	// Tool: update_project_spec
	s.AddTool(
		mcp.NewTool(
			"update_project_spec",
			mcp.WithDescription("Update the composite project specification / definition document for a project (describing all project scope and referencing ticket IDs)."),
			mcp.WithString("project", mcp.Description("Project slug (required)"), mcp.Required()),
			mcp.WithString("specification", mcp.Description("Markdown content of the composite project specification"), mcp.Required()),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			project := strings.TrimSpace(req.GetString("project", ""))
			if project == "" {
				return mcp.NewToolResultError("parameter 'project' is required"), nil
			}

			spec := req.GetString("specification", "")
			if err := be.UpdateProjectSpecification(project, spec); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			return mcp.NewToolResultText(fmt.Sprintf("✔ Project specification for '%s' updated successfully.", project)), nil
		},
	)

	return s
}
