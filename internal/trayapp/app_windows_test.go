//go:build windows

package trayapp

import (
	"context"
	"path/filepath"
	"testing"
)

func TestMissingConfigStopsWithoutSilentRetry(t *testing.T) {
	a := New(filepath.Join(t.TempDir(), "missing.toml"), t.TempDir(), nil)
	if err := a.runClient(context.Background()); err == nil {
		t.Fatal("missing config accepted")
	}
}

func TestSuspendPreservesIntentButManualLogoutDoesNot(t *testing.T) {
	a := New("", "", nil)
	ctx, cancel := context.WithCancel(context.Background())
	a.cancel = cancel
	a.desired = true
	a.powerEvent(4)
	if ctx.Err() == nil || !a.desired || !a.suspended {
		t.Fatal("suspend state incorrect")
	}
	a.powerEvent(18)
	if a.suspended || !a.desired {
		t.Fatal("resume state incorrect")
	}
	a.stopClient()
	if a.desired {
		t.Fatal("logout should clear reconnect intent")
	}
}

func TestTrayActivation(t *testing.T) {
	if isTrayActivation(wmMouseMove) || !isTrayActivation(ninSelect) {
		t.Fatal("activation mapping")
	}
}
