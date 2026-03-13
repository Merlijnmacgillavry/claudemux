# claudemux

A lazygit-style TUI for managing Claude Code sessions.

## Vision

Think lazygit but for Claude Code conversations. A terminal UI where you can see all your sessions in a sidebar, rename them, switch between them instantly, spawn new ones, and interact with the active session — all with vim-style keybindings.

## Tech Stack

- **Language**: Go 1.22+
- **TUI framework**: [Bubble Tea](https://github.com/charmbracelet/bubbletea)
- **Styling**: [Lip Gloss](https://github.com/charmbracelet/lipgloss)
- **Components**: [Bubbles](https://github.com/charmbracelet/bubbles) (list, textinput, viewport)
- **Session management**: tmux (via CLI subprocess calls)
- **Config storage**: JSON file at `~/.config/claudemux/config.json`
- **Clipboard**: [atotto/clipboard](https://github.com/atotto/clipboard)

## Architecture

```
cmd/claudemux/main.go              # Entry point, creates tea.Program with mouse support
internal/
  ui/
    model.go                       # Root Bubble Tea model — orchestrates everything
    sidebar.go                     # Session list panel (left pane)
    mainpane.go                    # Active session output + scrollbar + text selection
    statusbar.go                   # Bottom bar: mode indicator, keybinding hints
    dialog.go                      # Modal dialogs: new session, dir picker, rename,
                                   #   delete, settings, help
    styles.go                      # Lip Gloss style definitions
    theme.go                       # Color theme (single dark theme currently)
    ringbuffer.go                  # Fixed-size ring buffer (used internally)
  session/
    store.go                       # Session metadata CRUD; wraps config.Config
  claude/
    detect.go                      # Finds claude binary in PATH
  tmux/
    client.go                      # All tmux CLI calls (new-window, send-keys,
                                   #   capture-pane, resize-window, etc.)
    keys.go                        # Maps Bubble Tea KeyMsg → tmux key strings
  config/
    config.go                      # Config struct, load/save, XDG path, constants
```

## Core Features

### Session Sidebar
- Lists all claudemux-owned tmux windows with friendly names
- Status indicators: ● running, ○ stopped; ◎ waiting for input (background-polled)
- `j`/`k` vim navigation; `/` to filter
- `enter` — open/resume; `n` — new; `r` — rename; `s` — settings; `d` — delete
- `[` / `]` — shrink/grow sidebar width (persisted to `UIPrefs.SidebarWidth`)

### Main Pane
- Displays tmux pane output via `capture-pane -e` (ANSI passthrough, no reformatting)
- Proportional scrollbar rendered alongside the viewport (1 col reserved)
- Click-and-drag text selection with ANSI-aware highlight injection; release copies to clipboard
- Scrollback depth is configurable per session (default 2000 lines, set via `s` dialog)
- Viewport height = `pane height - 1` (title row); width = `pane width - 1` (scrollbar col)

### Input Routing
- **NORMAL mode** (sidebar focused): vim nav + global shortcuts
- **INSERT mode** (main pane focused): all keystrokes forwarded to tmux via `send-keys`
  - `shift+enter` sends kitty-protocol `\x1b[13;2u` for multiline input
- **DIALOG mode**: modal dialogs intercept all keys
- `alt+h` / `alt+l` switch focus universally from any non-dialog mode
- `esc` always returns to sidebar; `ctrl+q` / `ctrl+c` quit from anywhere

### Tmux Integration
- One tmux session (`claudemux`), one window per Claude session
- Each window runs `claude` (or `claude resume <id>`) with `remain-on-exit on`
- On resize (`tea.WindowSizeMsg`) and on `openWindow()`: calls `resize-window` to match
  the visible content area — `mainWidth - 3` × `paneHeight - 3`
  (accounts for border×2 + scrollbar col / border×2 + title row)
- Dead pane detection via `IsPaneDead`; auto-respawn with backoff (max 3 in 30s)
- Background poll every 5s checks non-active windows for waiting-for-input state

### Session Persistence
`~/.config/claudemux/config.json`:
- `windows`: map of tmux window name → `{displayName, workingDir, claudeSessionID, createdAt, scrollback}`
- `uiPrefs.sidebarWidth`: persisted sidebar width (0 = use default formula `width/4` clamped [20,40])
- `defaultWorkingDir`: optional default cwd for new sessions

### New Session Dialog
1. **Name field** — friendly name (tab to advance)
2. **Scrollback field** — lines to capture (tab to advance; default 2000)
3. **Skip permissions checkbox** — passes `--dangerously-skip-permissions` to claude
4. → **Dir picker** — searchable subdirectory picker; `ctrl+h` toggles hidden dirs
5. On confirm: creates tmux window, stores metadata, opens window, fires `detectSessionID`

## Keybindings

### Sidebar (NORMAL mode)
| Key | Action |
|-----|--------|
| `j` / `↓` | Navigate down |
| `k` / `↑` | Navigate up |
| `enter` | Open / resume session |
| `n` | New session |
| `r` | Rename session |
| `s` | Session settings (scrollback) |
| `d` | Delete session |
| `/` | Search / filter sessions |
| `[` | Shrink sidebar by 2 cols |
| `]` | Grow sidebar by 2 cols |
| `?` | Help overlay |
| `q` | Quit |

### Pane navigation (any non-dialog mode)
| Key | Action |
|-----|--------|
| `alt+l` | Focus main pane (INSERT mode) |
| `alt+h` | Focus sidebar (NORMAL mode) |

### Main pane (INSERT mode)
| Key | Action |
|-----|--------|
| `esc` | Return to sidebar |
| `shift+enter` | Send newline without submitting |
| drag | Select text; release copies to clipboard |
| all other keys | Forwarded to Claude via tmux send-keys |

### Dialogs
| Key | Action |
|-----|--------|
| `enter` | Confirm |
| `esc` | Cancel |
| `tab` | Cycle input fields (new session) |
| `space` | Toggle checkbox (permissions field) |
| `ctrl+h` | Toggle hidden dirs (dir picker) |

## Sizing Logic

```
sidebarWidth  = cfg.UIPrefs.SidebarWidth if > 0, else width/4 clamped [20,40]
                (when set: clamped [16,60], main pane always gets ≥ 20 cols)
mainWidth     = termWidth - sidebarWidth
paneHeight    = termHeight - 1  (status bar)

viewport.Width  = mainWidth - 2 - 1   (border × 2, then -1 for scrollbar col)
viewport.Height = paneHeight - 2 - 1  (border × 2, then -1 for title row)

tmux resize-window width  = mainWidth - 3   (= border×2 + scrollbar)
tmux resize-window height = paneHeight - 3  (= border×2 + title)
```

## Polling Architecture

- **Active window**: tick chain started on `openWindow`; interval 100ms→250ms→1s based on `unchangedTicks`
- **Dead check**: fired every 5 unchanged ticks via `pollDeadCheck`
- **Background windows**: `bgPollTickMsg` fires every 5s; polls up to 3 non-active, non-waiting windows for the `❯\s*$` idle prompt pattern
- Tick generation (`tickGeneration uint64`) invalidates stale messages from previous active windows

## Design Principles

- **Lazygit is the north star**: If unsure about a UX decision, look at how lazygit does it.
- **Minimal config**: Should work out of the box with zero config. Smart defaults everywhere.
- **Fast startup**: Don't block on session discovery. Show UI immediately, populate async.
- **PTY-native**: Don't try to parse or reformat Claude's output. Pass through raw terminal output. Colours, spinners, and Claude's own markdown rendering all work naturally.
- **Crash-resistant**: A Claude process dying should not crash the TUI. Auto-respawn with backoff; show error if it keeps crashing.

## Notes for Claude Code

- Run with `go run ./cmd/claudemux` or `go build ./cmd/claudemux && ./claudemux`
- The app must run inside an existing tmux session (or it creates one named `claudemux`)
- `capture-pane` only returns what is visible in the tmux window — the scrollback depth
  controls how many lines back we request via `-S -<lines>`
- `resize-window` targets the window, not the pane — this is correct since each claudemux
  window has exactly one pane
- `remain-on-exit on` is set per-window so dead panes can be detected and respawned
- `gofmt` and `go vet` before committing; `go build ./cmd/claudemux` to verify
- The Charm ecosystem docs: https://charm.sh/libs/
