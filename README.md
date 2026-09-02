# Backlog 📋

**Backlog** is a high-performance visual task and priority manager engineered for developers and AI agents. It features a native **macOS / multiplatform desktop GUI** (powered by Fyne), an interactive **Burn-Up progress chart**, a resident **menu bar icon (System Tray)** with live status metrics, **multi-project** support, task **dependencies & blocking detection**, hierarchical subtask branching, a fast **CLI**, and a native **MCP (Model Context Protocol)** server.

---

## Features

### 📈 Interactive Burn-Up Progress Chart
* **Chronological Timeline**: Visualizes total scope (blue) and completed tasks (green) over time.
* **Non-blocking Live Hover Indicator**: Hovering over any milestone smoothly displays the exact date, total scope, completed count, and pending workload (`📅 02 Sep: ● Total Scope: 12 · ● Completed: 8 · ⏳ Pending: 4`).
* **Direct Point Values**: Key milestones display numeric values directly on the plot.
* **Empty State Guidance**: Clear visual cues when initializing a new project.

### ⛔ Dependencies, Blocking Detection & Cycles
* **Task Prerequisites (`depends_on`)**: Declare task dependencies to enforce strict execution order.
* **Automatic Blocked State**: Tasks remain marked as blocked (`⛔ [blocked by #X]`) until all prerequisite tasks are completed.
* **Cycle Prevention**: Circular dependencies (e.g. A → B → A) are detected and rejected.

### 🌳 Hierarchical Task Tree & Subtasks
* **Branching Breakdown (`parent_id`)**: Decompose complex features and epics into manageable subtasks.
* **Cascade Deletion**: Removing a parent task automatically and cleanly cascades to all its descendants.

### 🖥️ Native Desktop GUI (Fyne)
* **Master-Detail Layout**:
  * **Left Pane**: Searchable, filterable task list with hierarchical tree indentation, priority badges, effort sizes, blocking alerts, and assignee pills.
  * **Right Pane (Split View)**: Interactive Burn-Up chart on top, comprehensive Markdown detail inspector on the bottom.
* **Markdown Renderer**: Formatted headers, code blocks, lists, quotes, and links for descriptions and resolution notes.
* **Priority Tier Filters**:
  * `T1 · Blocker` (Red)
  * `T2 · Important` (Orange)
  * `T3 · Visual debt` (Teal)
  * `T4 · Internal` (Purple)
  * `T5 · Future` (Gray)
* **Effort Size Chips**: Filter by `XL`, `L`, `M`, `S`, `XS`.
* **Keyboard Navigation**: Use `↑` / `↓` arrow keys to quickly navigate between tasks.

### 🪟 Menu Bar / System Tray Resident App
* **Always-Visible Metric**: Displays the active project and pending ratio at all times (`[Project] 4/12`).
* **Instant Project Switcher**: Switch active projects directly from the menu bar.
* **Quick Task Creation**: Instant shortcut to launch the "New Task" dialog.

### 🤖 AI Agent & MCP Integration
* **Model Context Protocol (MCP)**:
  * SSE endpoint on `http://127.0.0.1:8485/sse` and stdio transport via `backlog mcp`.
  * Provides tools for autonomous agents to inspect tasks, manage priorities, track dependencies, assign work, and log implementation resolutions.
  * Built-in workflow resource (`backlog://workflow`).

---

## Installation & Build

### Requirements
* Go 1.22+
* macOS 11.0+ (Universal binary: Apple Silicon arm64 + Intel amd64) or Linux/Windows

### Build Commands
```bash
# Build universal macOS .app bundle
make bundle

# Build standalone CLI binary
make build

# Run all test suites
make test
```

The universal bundle will be available at `bin/Backlog.app`.

---

## CLI Usage

```bash
# Set default project for current session (optional)
export BACKLOG_PROJECT=my-project

# List open tasks (hierarchical tree display)
backlog list
backlog list --tier 1
backlog list --done=all
backlog list --blocked

# Add a root task
backlog add --title "Migrate authentication to OAuth2" --tier 1 --size L

# Add a dependent subtask
backlog add --title "Implement refresh token rotation" --parent 1 --depends 1 --size M --assignee manuel

# Assign or claim tasks
backlog assign 2 claude

# Mark task as completed with resolution notes
backlog done 2 -r "Implemented PKCE with automatic refresh rotation in commit abc1234"

# Project management
backlog projects
backlog project use web-app
backlog project new backend --name "Backend Service"
```

---

## MCP Server Configuration

### HTTP SSE Connection (Recommended when Backlog desktop is running)
```json
{
  "mcpServers": {
    "backlog": {
      "url": "http://127.0.0.1:8485/sse"
    }
  }
}
```

### Stdio Connection (Standalone execution)
```json
{
  "mcpServers": {
    "backlog": {
      "command": "backlog",
      "args": ["mcp"]
    }
  }
}
```

---

## License

MIT License — Altenwald
