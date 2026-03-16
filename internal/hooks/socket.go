package hooks

import (
	"fmt"
	"os"
	"path/filepath"
)

// SocketPath returns the Unix domain socket path for hook notifications.
// It is placed under /tmp/claudemux-<uid>/ so that it is user-specific
// and survives reboots without permission conflicts.
func SocketPath() string {
	uid := os.Getuid()
	return filepath.Join(fmt.Sprintf("/tmp/claudemux-%d", uid), "notify.sock")
}
