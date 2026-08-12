package bot

import (
	"sync"
	"testing"
)

func TestSettingsGroupChatRegistration(t *testing.T) {
	settings := NewSettings()

	if settings.IsGroupChat(-1001234567890) {
		t.Error("IsGroupChat() = true for a chat that was never registered")
	}

	settings.RegisterGroupChat(-1001234567890)

	if !settings.IsGroupChat(-1001234567890) {
		t.Error("IsGroupChat() = false for a registered chat")
	}
	if settings.IsGroupChat(42) {
		t.Error("IsGroupChat() = true for a different chat")
	}

	// Registering twice is a no-op, not a change of state.
	settings.RegisterGroupChat(-1001234567890)
	if !settings.IsGroupChat(-1001234567890) {
		t.Error("IsGroupChat() = false after a repeated registration")
	}
}

func TestSettingsAreIndependent(t *testing.T) {
	// Each Settings owns its own chats, so tests (and a fork running more than one bot
	// in a process) cannot leak registrations into one another.
	first := NewSettings()
	second := NewSettings()

	first.RegisterGroupChat(1)

	if second.IsGroupChat(1) {
		t.Error("a chat registered on one Settings is visible on another")
	}
}

func TestSettingsConcurrentAccess(t *testing.T) {
	const goroutines = 50

	settings := NewSettings()

	var wg sync.WaitGroup
	wg.Add(goroutines * 2)
	for i := range goroutines {
		go func() {
			defer wg.Done()
			settings.RegisterGroupChat(int64(i))
		}()
		go func() {
			defer wg.Done()
			settings.IsGroupChat(int64(i))
		}()
	}
	wg.Wait()

	for i := range goroutines {
		if !settings.IsGroupChat(int64(i)) {
			t.Fatalf("IsGroupChat(%d) = false, want every registration to have landed", i)
		}
	}
}
