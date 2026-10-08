# Backlog 📋

**Backlog** is a high-performance visual task and priority manager engineered for developers and AI agents. It features a native **macOS / multiplatform desktop GUI** (powered by Fyne), an interactive **Burn-Up progress chart**, a resident **menu bar icon (System Tray)** with live status metrics, **multi-project** support, task **dependencies & blocking detection**, hierarchical subtask branching, a fast **CLI**, and a native **MCP (Model Context Protocol)** server.

Each project has its own **SQLite database** for portable backups. Share selected projects with other Backlog instances on the **local network**, with receiver-approved pairing and live updates across the GUI, CLI and MCP.

---

## Features

### 📈 Interactive Burn-Up Progress Chart
* **Chronological Timeline**: Visualizes total scope (blue) and completed tasks (green) over time.
* **Non-blocking Live Hover Indicator**: Hovering over any milestone smoothly displays the exact date, total scope, completed count, and pending workload (`📅 02 Sep: ● Total Scope: 12 · ● Completed: 8 · ⏳ Pending: 4`).
* **Direct Point Values**: Key milestones display numeric values directly on the plot.
* **Empty State Guidance**: Clear visual cues when initializing a new project.

### ⛔ Dependencies, Blocking Detection & Cycles
* **Task Prerequisites (`depends_on`)**: Declare task dependencies to enforce strict execution order. The desktop list groups dependent tasks under their first visible prerequisite, with indentation and vector branch indicators. Tasks with multiple prerequisites appear once, retaining all dependency references in their details. Filters keep matching tasks visible even when their prerequisites are hidden.
* **Automatic Blocked State**: Tasks remain marked as blocked (`⛔ [blocked by #X]`) until all prerequisite tasks are completed.
* **Cycle Prevention**: Circular dependencies (e.g. A → B → A) are detected and rejected.

### 🌳 Hierarchical Task Tree & Subtasks
* **Branching Breakdown (`parent_id`)**: Decompose complex features and epics into manageable subtasks.
* **Cascade Deletion**: Removing a parent task automatically and cleanly cascades to all its descendants.

### 🖥️ Native Desktop GUI (Fyne)
* **Master-Detail Layout**:
  * **Left Pane**: Searchable, filterable task list with hierarchical tree indentation, priority badges, effort sizes, blocking alerts, and assignee pills.
  * **Right Pane**: Tabs for task details, the Burn-Up progress chart, and the project specification.
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

### 🌐 LAN Project Sharing
* **Discovery and Pairing**: Find nearby Backlog instances with mDNS, approve incoming requests, and pair using a one-time code.
* **Open or Closed Projects**: Keep projects local by default, or open them for paired machines to read and modify.
* **Remote Project Switcher**: Browse projects grouped by their owning machine.
* **Live Updates and Reconnection**: Receive changes over TLS WebSockets and reload after reconnecting. Conflicting edits require a fresh read before reapplying the change.

See [Sharing projects on the LAN](#sharing-projects-on-the-lan) for setup and access rules, and [Local storage, migration and portable project backups](#local-storage-migration-and-portable-project-backups) for upgrading existing installations.

---

## Installation & Build

### Requirements
* Go 1.26.6+ (as specified in `go.mod`)
* macOS 11.0+ (Universal binary: Apple Silicon arm64 + Intel amd64) or Linux/Windows

### Build Commands
```bash
# Build universal macOS .app bundle
make bundle

# Install DMG packaging tools once (Python 3.10+)
make dmg-deps

# Build macOS .dmg installer
make dmg

# Build standalone CLI binary
make build

# Run all test suites
make test
```

The universal bundle will be available at `bin/Backlog.app` and the disk image installer at `bin/Backlog.dmg`.

The DMG opens with a custom background, large app and Applications icons, and a
simple drag-to-install layout. Local builds and GitHub releases use the same
packaging script. Packaging uses [dmgbuild](https://dmgbuild.readthedocs.io/),
which writes the Finder layout without automating Finder, plus Pillow to render
the background. Dependencies live in `bin/dmg-venv`; they are build tools only.
If the system Python is older than 3.10, use e.g.
`make dmg-deps PYTHON=/opt/homebrew/bin/python3`.
To package an already built or signed bundle without rebuilding it, run
`make dmg-package`. The artwork and layout sources are in `build/macos/dmg/`.


---

## CLI Usage

Start the desktop app or run `backlog start` before using commands that connect to the local daemon.

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
backlog project new backend --name "Backend Service"

# Settings & MCP instructions
backlog settings get
backlog settings set "Custom team rules, TDD requirements, and workflow guidelines"
backlog settings reset
```

> **Note:** All task commands require a project. Set it via `--project <slug>` or the `BACKLOG_PROJECT` environment variable.
> The active project switcher (tray menu) is only available in the desktop GUI.

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

### Stdio Connection (Through the local daemon)

Keep the desktop app or `backlog start` running. `backlog mcp` bridges stdio requests to that daemon.

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

## Local storage, migration and portable project backups

Backlog now uses `~/.config/backlog/catalog.db` for local preferences, pairing
credentials and the project catalog, and `projects/<id>/project.db` for each
project's tasks and specification. `--data-dir` selects another directory.
Project database handles are opened per operation, so inactive projects do not
retain open connections. The original `backlog.db`, when present, is a migration
source and is no longer the active database.

On the first start, Backlog migrates either the old `config.json` plus
`projects/*.json` layout, or the single multi-project `backlog.db`. Stop the old
application before upgrading. The migration works from a SQLite snapshot or the
original JSON files, checks the new databases, and activates the complete catalog
in one transaction. Original sources are retained unchanged. Invalid references
or unreadable projects stop the migration rather than silently dropping data;
repair the original and retry. An interrupted migration can be restarted without
duplicating projects. Do not run an older binary against a migrated directory:
its retained source files no longer receive updates.

```sh
# Consistent standalone SQLite backup of one LOCAL project, even while in use
backlog --project backend backup ./backend.db

# Copy backend.db to another machine, then import it there
backlog import ./backend.db

# A single-file backup containing all local projects and the legacy preferences
backlog backup ./all-projects.db

# JSON imports and exports remain supported
backlog --project backend export --format json > backend.json
backlog import ./backend.json
```

Imports reject an existing project slug and start closed to network access.
Restore a complete backup by placing it as `backlog.db` in a fresh data directory;
the first start splits it into individual project databases. Pairing identities
and credentials are machine-local and are not transferred in these backups.
Use `backup` when the app is running: copying only a live `project.db` can miss
committed changes still in its `-wal` file. A deleted project is removed from the
catalog; its independent database remains available for recovery.

## Sharing projects on the LAN

Each machine owns its projects and SQLite files. There is no database replication
or offline write queue. Remote clients send requests to the owner and receive
change notifications through an authenticated TLS WebSocket.

1. Start Backlog on both machines. Open **Network**, wait for discovery, and
   refresh the list. mDNS uses `_backlog._tcp.local.`. **Connect by address** is
   available if multicast is blocked.
2. Select **Connect**. The receiving machine displays the requesting name and
   address and asks for approval. After acceptance it shows a random one-time
   code; enter it on the initiating machine. Requests expire after two minutes
   and permit at most five proof attempts. The code is not sent over the network:
   both sides prove knowledge of it, bound to the TLS identity and a fresh nonce.
3. On the owner, select a project and use **Sharing** to open it. Projects are
   closed by default. Opening grants read/write access to all paired incoming
   clients; closing removes remote access, including existing connections.
4. The project switcher groups remote projects under their machine names.
   **Network** can revoke incoming access or forget an outgoing connection.

The LAN listener uses `--port + 2` (8486 by default); REST and MCP remain bound to
loopback on 8484/8485. Allow the LAN TCP port and mDNS UDP 5353 through the local
firewall. The connection is directional: pairing A to B lets A use B's open
projects. Pair in the other direction to expose A's projects to B as well.

Remote GUI reads use an in-memory snapshot. Notifications refresh that snapshot;
reconnection uses exponential backoff and reloads the catalog and selected
project, recovering missed events. Disconnected projects cannot accept writes.
CLI/MCP through the local daemon can also address a remote project using the
`<machine-id>::<project-slug>` identifier returned by project listing, even when
it is not selected in the GUI.

Every remote modification carries the prior `updated_at` of its task, page, or
project. The owner checks the version and sharing permission in the same SQL
transaction as the write. A `stale` response requires reading again and explicitly
reapplying the change; GUI task/page drafts are retained. Task edits through REST
and MCP may supply `expected_updated_at` to preserve the version read by their
caller. CLI-style operations without an explicit version read the owner's state
before sending their conditional operation. Remote request IDs are recorded in
the same transaction, preventing a repeated request from being executed twice.
An interrupted request is not automatically replayed: reload to determine whether
it succeeded before issuing another operation.

The initial protocol reloads the selected project's complete snapshot after
changes (64 MiB maximum WebSocket message). Incremental/paginated snapshots and
per-client project permissions are future extensions.

---

## License

MIT License — Altenwald
