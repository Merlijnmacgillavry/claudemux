package session

import (
	"testing"

	"github.com/merlijnmacgillavry/claudemux/internal/config"
)

func TestStoreSetWindow(t *testing.T) {
	cfg := config.DefaultConfig()
	store := NewStore(cfg)
	store.SetWindow("w-123", "My Great Session", "/home/user/project")

	meta, ok := store.GetWindow("w-123")
	if !ok {
		t.Fatal("GetWindow returned not-found")
	}
	if meta.DisplayName != "My Great Session" {
		t.Errorf("DisplayName: got %q, want %q", meta.DisplayName, "My Great Session")
	}
	if meta.WorkingDir != "/home/user/project" {
		t.Errorf("WorkingDir: got %q, want %q", meta.WorkingDir, "/home/user/project")
	}
	if meta.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set by SetWindow")
	}
}

func TestStoreRemoveWindow(t *testing.T) {
	cfg := config.DefaultConfig()
	store := NewStore(cfg)
	store.SetWindow("w-456", "Temp Session", "")
	store.RemoveWindow("w-456")

	_, ok := store.GetWindow("w-456")
	if ok {
		t.Error("window should have been removed")
	}
}

func TestStoreSetWindowPreservesCreatedAt(t *testing.T) {
	cfg := config.DefaultConfig()
	store := NewStore(cfg)
	store.SetWindow("w-789", "First Name", "")
	first, _ := store.GetWindow("w-789")

	// Updating display name should not reset CreatedAt.
	store.SetWindow("w-789", "Renamed", "")
	second, _ := store.GetWindow("w-789")

	if !second.CreatedAt.Equal(first.CreatedAt) {
		t.Error("SetWindow should not reset CreatedAt on update")
	}
}
