# claudemux 🦥

A lazygit-style TUI for managing Claude Code sessions.

Navigate, rename, create, and switch between Claude Code conversations — all from a single terminal interface with vim keybindings.

## Features

- **Session sidebar** — See all your Claude sessions at a glance
- **Instant switching** — Jump between sessions with j/k + enter
- **Friendly names** — Name your sessions instead of memorizing IDs
- **Full PTY passthrough** — Claude runs natively, no output mangling
- **Vim keybindings** — If you know lazygit, you know claudemux

## Requirements

- [tmux](https://github.com/tmux/tmux) — `brew install tmux` / `apt install tmux`
- [claude CLI](https://claude.ai/code) — must be available in `$PATH`

## Quick Start

```bash
# Requires Go 1.22+
go install github.com/merlijnmacgillavry/claudemux/cmd/claudemux@latest

# Or build from source
git clone https://github.com/merlijnmacgillavry/claudemux.git
cd claudemux
go build -o claudemux ./cmd/claudemux
./claudemux
```

## Keybindings

| Key | Action |
|-----|--------|
| `j/k` | Navigate sessions |
| `enter` | Open session |
| `n` | New session |
| `r` | Rename session |
| `d` | Delete session |
| `alt+h/l` | Switch focus |
| `Esc` | Back to sidebar |
| `?` | Help |
| `q` | Quit |

## Config

Config lives at `~/.config/claudemux/config.json` and is created automatically on first run.

## Built With

- [Bubble Tea](https://github.com/charmbracelet/bubbletea) — TUI framework
- [Lip Gloss](https://github.com/charmbracelet/lipgloss) — Styling
- [tmux](https://github.com/tmux/tmux) — Session and process management

## License

MIT
