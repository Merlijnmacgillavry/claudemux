# claudemux 🦥

A lazygit-style TUI for managing Claude Code sessions.

Navigate, rename, create, and switch between Claude Code conversations — all from a single terminal interface with vim keybindings.

## Features

- **Session sidebar** — See all your Claude sessions at a glance with live status indicators
- **Instant switching** — Jump between sessions with j/k + enter; Claude resumes automatically
- **Friendly names** — Name your sessions instead of memorising IDs
- **Full PTY passthrough** — Claude runs natively in tmux; no output mangling, colours and spinners work
- **Mouse support** — Click to switch sessions, click-and-drag to select and copy text
- **Scrollbar** — Proportional scrollbar on the main pane
- **Sidebar resize** — `[` / `]` to adjust sidebar width; persisted across restarts
- **Per-session scrollback** — Configure how many lines to capture per session (`s` key)
- **Shift+Enter** — Send a newline without submitting (kitty keyboard protocol)
- **Vim keybindings** — If you know lazygit, you know claudemux

## Requirements

- [tmux](https://github.com/tmux/tmux) — `brew install tmux` / `apt install tmux`
- [claude CLI](https://claude.ai/code) — must be available in `$PATH`

## Install

### Homebrew (macOS / Linux)

```bash
brew install Merlijnmacgillavry/tap/claudemux
```

### Go install

```bash
go install github.com/merlijnmacgillavry/claudemux/cmd/claudemux@latest
```

### Build from source

```bash
git clone https://github.com/merlijnmacgillavry/claudemux.git
cd claudemux
go build -o claudemux ./cmd/claudemux
./claudemux
```

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
| `[` | Shrink sidebar |
| `]` | Grow sidebar |
| `?` | Help overlay |
| `q` | Quit |

### Pane navigation

| Key | Action |
|-----|--------|
| `alt+l` | Focus main pane |
| `alt+h` | Focus sidebar |
| click | Switch pane / open session |

### Main pane (INSERT mode)

| Key | Action |
|-----|--------|
| `esc` | Return to sidebar |
| `shift+enter` | Send newline (multiline input) |
| drag | Select and copy text |
| all other keys | Passed through to Claude |

### Dialogs

| Key | Action |
|-----|--------|
| `enter` | Confirm |
| `esc` | Cancel |
| `tab` | Cycle fields (new session dialog) |
| `space` | Toggle checkbox (permissions field) |
| `ctrl+h` | Toggle hidden dirs (dir picker) |

## Config

Config lives at `~/.config/claudemux/config.json` and is created automatically on first run. Notable fields:

```json
{
  "uiPrefs": {
    "sidebarWidth": 30
  },
  "windows": {
    "w-1234567890": {
      "displayName": "my project",
      "workingDir": "/path/to/project",
      "scrollback": 5000
    }
  }
}
```

`sidebarWidth` is updated automatically when you press `[` / `]`. `scrollback` is set per session via the `s` dialog (default 2000 lines).

## Built With

- [Bubble Tea](https://github.com/charmbracelet/bubbletea) — TUI framework
- [Lip Gloss](https://github.com/charmbracelet/lipgloss) — Styling
- [Bubbles](https://github.com/charmbracelet/bubbles) — List, textinput, viewport components
- [tmux](https://github.com/tmux/tmux) — Session and process management
- [atotto/clipboard](https://github.com/atotto/clipboard) — Cross-platform clipboard

## License

MIT
