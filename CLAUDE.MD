# claudemux

A lazygit-style TUI for managing Claude Code sessions.

## Vision

Think lazygit but for Claude Code conversations. A terminal UI where you can see all your sessions in a sidebar, rename them, switch between them instantly, spawn new ones, and interact with the active session — all with vim-style keybindings.

## Tech Stack

- **Language**: Go 1.22+
- **TUI framework**: [Bubble Tea](https://github.com/charmbracelet/bubbletea) (same framework lazygit uses via gocui, but bubbletea is the modern Charm stack)
- **Styling**: [Lip Gloss](https://github.com/charmbracelet/lipgloss)
- **Components**: [Bubbles](https://github.com/charmbracelet/bubbles) (list, textinput, viewport, help)
- **Terminal rendering**: [glamour](https://github.com/charmbracelet/glamour) for markdown rendering in the output pane
- **PTY management**: [creack/pty](https://github.com/creack/pty) for running interactive Claude processes
- **Config storage**: JSON file via [adrg/xdg](https://github.com/adrg/xdg) (`~/.config/claudemux/config.json`)

## Architecture

```
cmd/claudemux/main.go          # Entry point, creates tea.Program
internal/
  ui/
    model.go                     # Root bubbletea model, orchestrates everything
    sidebar.go                   # Session list panel (left side)
    mainpane.go                  # Active session output/input (right side)
    statusbar.go                 # Bottom bar: keybinding hints, mode indicator
    dialog.go                    # Modal dialogs (rename, confirm delete, new session)
    styles.go                    # Lip Gloss style definitions
    theme.go                     # Color themes (dark/light)
  session/
    manager.go                   # Discovers, creates, tracks sessions
    store.go                     # Persists session metadata (names, tags, last-used)
  claude/
    process.go                   # PTY wrapper for interactive claude processes
    detect.go                    # Finds claude binary, detects version/capabilities
  config/
    config.go                    # App config: keybindings, UI prefs, claude binary path
pkg/
  keys/
    keys.go                      # Keybinding definitions (vim-style defaults)
```

## Core Features (MVP)

### 1. Session List Sidebar
- Displays all known sessions with friendly names
- Shows status indicator: ● running, ○ stopped, ◌ stale
- Sorted by last activity (most recent first)
- Vim navigation (j/k) to move selection
- Press `enter` to open/resume a session
- Press `n` to create a new session
- Press `r` to rename the selected session (opens inline text input)
- Press `d` to delete (with confirmation dialog)

### 2. Main Pane (Session View)
- Renders the active Claude session's terminal output
- Full PTY passthrough — you type, Claude responds, just like normal
- When focused, all keystrokes go to the Claude process
- Scrollback buffer (512KB ring buffer)
- Scroll with mouse or ctrl+u/ctrl+d when in scroll mode

### 3. Focus Model
- Two panes: sidebar (left) and main (right)
- `Tab` or `Ctrl+w` to swap focus between panes
- When sidebar is focused: vim keys navigate sessions, global shortcuts work
- When main pane is focused: all input goes to the active Claude process
- `Escape` always returns focus to sidebar

### 4. Session Discovery
- On startup, scan for existing Claude Code sessions
- Check `~/.claude/` directory structure for conversation data
- Also support `claude conversations list` if/when the CLI supports it
- Track sessions we've created ourselves in local store

### 5. Session Persistence
- Store in `~/.config/claudemux/config.json`:
  - Session ID → friendly name mapping
  - Session ID → tags (future)
  - Session ID → working directory
  - Last active session ID
  - UI preferences
- Store is just a JSON file, dead simple

### 6. Status Bar
- Shows current mode (NORMAL / INSERT / DIALOG)
- Shows keybinding hints contextual to current focus
- Shows active session name and status

## Keybindings

### Sidebar focused (NORMAL mode)
| Key | Action |
|-----|--------|
| `j` / `↓` | Move selection down |
| `k` / `↑` | Move selection up |
| `enter` | Open/resume selected session |
| `n` | New session |
| `r` | Rename selected session |
| `d` | Delete selected session |
| `/` | Search/filter sessions |
| `tab` | Focus main pane |
| `?` | Toggle help |
| `q` | Quit |

### Main pane focused (INSERT mode)
| Key | Action |
|-----|--------|
| `Escape` | Return focus to sidebar |
| All other keys | Passed through to Claude process |

### Dialogs
| Key | Action |
|-----|--------|
| `enter` | Confirm |
| `Escape` | Cancel |

## Implementation Plan

Work through these phases in order. Each phase should compile and be testable.

### Phase 1: Project Setup
1. Initialize go module (`github.com/lord/claudemux` or whatever repo name)
2. Set up the directory structure above
3. Add all dependencies
4. Create a minimal `main.go` that opens an alt-screen bubbletea program showing "claudemux" centered

### Phase 2: Layout & Styles
1. Build the root model with a two-pane layout (sidebar + main) using lipgloss
2. Implement responsive sizing (handle terminal resize)
3. Create the style definitions (borders, colors, active/inactive states)
4. Status bar at the bottom with placeholder text
5. At this point: app shows two bordered panes side by side, resizes correctly

### Phase 3: Session Store & Config
1. Implement config.go — load/save JSON, XDG paths
2. Implement store.go — session metadata CRUD
3. Implement detect.go — find claude binary, check version
4. On startup: load config, detect claude, log status

### Phase 4: Session List
1. Build the sidebar component using bubbles/list
2. Populate with mock sessions first, then wire up to store
3. Implement j/k navigation, selection highlighting
4. Show session name, status icon, last-used timestamp
5. Handle empty state ("No sessions. Press n to create one.")

### Phase 5: Session Creation & Naming
1. Implement the "new session" dialog (text input for name + optional cwd)
2. Wire up to claude CLI — spawn a new session
3. Capture the session ID from the CLI response
4. Store the name mapping in config
5. Auto-focus the new session in the main pane

### Phase 6: PTY Process Management
1. Implement process.go — PTY wrapper with ring buffer
2. Start a claude process with `--resume <id>`
3. Pipe PTY output to the main pane viewport
4. Pipe keyboard input from main pane to PTY stdin
5. Handle process exit gracefully
6. Handle window resize → resize PTY

### Phase 7: Focus & Input Routing
1. Implement the focus model (sidebar vs main pane)
2. Tab/Ctrl+w to swap focus
3. When main pane focused: raw input passthrough to PTY
4. When sidebar focused: vim navigation keys
5. Escape always returns to sidebar
6. Status bar updates to show current mode

### Phase 8: Rename & Delete
1. Inline rename: press r, text input appears over session name, enter to confirm
2. Delete: press d, confirmation dialog appears, y to confirm
3. Both update the store and refresh the sidebar

### Phase 9: Polish
1. Add `?` help overlay showing all keybindings
2. Session search/filter with `/`
3. Mouse support (click session to select, scroll main pane)
4. Graceful shutdown (stop all Claude processes on quit)
5. Error handling and edge cases

## Design Principles

- **Lazygit is the north star**: If unsure about a UX decision, look at how lazygit does it.
- **Minimal config**: Should work out of the box with zero config. Smart defaults everywhere.
- **Fast startup**: Don't block on session discovery. Show UI immediately, populate async.
- **PTY-native**: Don't try to parse or reformat Claude's output. Pass through the raw terminal output. This means colors, spinners, markdown rendering from Claude all work naturally.
- **Crash-resistant**: A Claude process dying should not crash the TUI. Show the error in the pane and allow restarting.

## Testing

- Unit test the session store (CRUD operations)
- Unit test the ring buffer
- Unit test config load/save
- Integration test: spawn a mock process, verify PTY I/O
- Manual testing is primary — this is a TUI, visual verification matters

## Notes for Claude Code

- Start each phase by running the existing code to see current state
- Use `go run ./cmd/claudemux` to test
- Commit after each working phase
- The Charm ecosystem docs are excellent: https://charm.sh/docs/
- For PTY handling, creack/pty is the standard Go library
- Keep the codebase clean — run `gofmt` and `go vet` regularly
- If `claude` CLI behavior is unclear, run `claude --help` to check available flags
