package hooks

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
)

// Notification is the JSON payload sent from the hook subprocess to the listener.
type Notification struct {
	Window string `json:"window"`
	Event  string `json:"event"`
}

// HookNotificationMsg is the Bubble Tea message dispatched when a hook fires.
type HookNotificationMsg struct {
	WindowName string
	Event      string
}

// Listener accepts connections on a Unix domain socket and dispatches
// HookNotificationMsg to a running Bubble Tea program.
type Listener struct {
	socketPath string
	ln         net.Listener
	done       chan struct{}
}

// NewListener creates a Listener. The socket directory is created if needed
// and any stale socket file from a previous run is removed.
func NewListener(socketPath string) (*Listener, error) {
	if err := os.MkdirAll(filepath.Dir(socketPath), 0700); err != nil {
		return nil, err
	}
	// Remove stale socket from a previous run.
	_ = os.Remove(socketPath)

	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		return nil, err
	}
	return &Listener{
		socketPath: socketPath,
		ln:         ln,
		done:       make(chan struct{}),
	}, nil
}

// Start begins accepting connections in a background goroutine and forwards
// each decoded Notification to p as a HookNotificationMsg.
func (l *Listener) Start(p *tea.Program) {
	go func() {
		defer close(l.done)
		for {
			conn, err := l.ln.Accept()
			if err != nil {
				return
			}
			go l.handleConn(conn, p)
		}
	}()
}

func (l *Listener) handleConn(conn net.Conn, p *tea.Program) {
	defer conn.Close()
	var n Notification
	if err := json.NewDecoder(conn).Decode(&n); err != nil {
		return
	}
	if n.Window == "" {
		return
	}
	p.Send(HookNotificationMsg{WindowName: n.Window, Event: n.Event})
}

// Stop closes the listener, waits for the accept goroutine to exit, then
// removes the socket file.
func (l *Listener) Stop() {
	_ = l.ln.Close()
	<-l.done
	_ = os.Remove(l.socketPath)
}
