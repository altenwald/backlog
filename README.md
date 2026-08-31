# Backlog 📋

**Backlog** is a visual task and priority manager for developers featuring a **Fyne desktop GUI**, a resident **menu bar icon (System Tray)** displaying real-time `[Project] Open/Total` ratios, **multi-project** support, a fast **CLI**, and a native **MCP (Model Context Protocol)** server.

---

## Features

* **System Tray / Menu Bar Resident Icon**:
  * Displays active project and open task ratio at all times: `[Dymmer] 78/119`.
  * Quick-access context menu with **Top 5 highest priority tasks** (T1 -> T2).
  * Fast project switcher and direct shortcut to "+ New Task".
* **Full Desktop GUI (Fyne)**:
  * **Summary Metrics Bar**: Total/open story points and breakdown by effort size (`XL`, `L`, `M`, `S`, `XS`).
  * **Priority Tier Filters**:
    * `T1 · Blocker` (Red)
    * `T2 · Important` (Orange)
    * `T3 · Visual debt` (Teal)
    * `T4 · Internal` (Purple)
    * `T5 · Future` (Gray)
  * Real-time search query filtering.
  * Tasks grouped cleanly by category (*Monetization*, *Domains*, *Bugs*, *Infrastructure*, etc.).
  * Single-click task completion with live recalculations.
* **Multi-Project Support**:
  * Isolate tasks and metrics across multiple independent projects (*Dymmer*, *Conta*, etc.).
  * Select project via `--project` (`-p`) flag, `BACKLOG_PROJECT` environment variable, or GUI dropdown.
* **Embedded Daemon with Live Reactivity**:
  * Central daemon runs on `127.0.0.1:8484`.
  * Any task mutation from the CLI or MCP updates the desktop UI and System Tray icon in real time.
* **MCP (Model Context Protocol) Integration**:
  * Native HTTP SSE endpoint on `http://127.0.0.1:8485/sse`.
  * Stdio mode for local client execution (`backlog mcp`).
  * Exposed tools: `list_projects`, `list_tasks`, `get_summary`, `get_top_priorities`, `add_task`, `complete_task`, `update_task`, `delete_task`, `set_active_project`.

---

## Installation & Build

```bash
# Build the binary
go build -o bin/backlog ./cmd/backlog

# Or install to your $GOPATH/bin
go install ./cmd/backlog
```

---

## Usage

### 1. Start Desktop App & Background Server
```bash
backlog
# or explicitly:
backlog start --port 8484
```
* The app docks into your macOS/Linux/Windows System Tray with active project stats (`[Dymmer] 78/119`).
* Closing the main window keeps the application resident in the menu bar.

### 2. CLI Commands

```bash
# Set default project for current shell session (optional)
export BACKLOG_PROJECT=dymmer

# List open tasks (with optional filters)
backlog list
backlog list --tier 1
backlog list -g "Monetization"
backlog list -p conta --tier 2

# Display metrics summary and points
backlog summary

# Add a new task
backlog add --title "Renew SSL certificates" --tier 1 --size M --group "Infrastructure"

# Complete or reopen tasks
backlog done 1
backlog undone 1

# Project management
backlog projects
backlog project use conta
backlog project new mobile-app --name "Mobile App"
```

---

### 3. MCP (Model Context Protocol) Setup

#### A. HTTP SSE Connection (Recommended when `backlog` is running)
In your MCP client configuration:

```json
{
  "mcpServers": {
    "backlog": {
      "url": "http://127.0.0.1:8485/sse"
    }
  }
}
```

#### B. Stdio Connection (On-demand execution)
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
