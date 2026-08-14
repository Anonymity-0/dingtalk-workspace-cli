package upgrade

import (
	"errors"
	"os/user"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/testseam"
)

// TestCrossPlatformCoverageSystemHomeIndependentOfHOME pins that the app-bundle
// detection gate compares against a HOME-independent system home: ResolveSystemHomeDir
// prefers the OS user record (getpwuid on Unix) and only falls back to $HOME when that
// record cannot be resolved. Without this, allowSystemApps = homeDir == systemHome is a
// no-op whenever $HOME is overridden, because both sides would honor the same override.
func TestCrossPlatformCoverageSystemHomeIndependentOfHOME(t *testing.T) {
	// Success path: the OS user database home is returned as-is.
	testseam.Swap(t, &upgradeCurrentUser, func() (*user.User, error) {
		return &user.User{HomeDir: "/tmp/dws-real-system-home"}, nil
	})
	got, err := ResolveSystemHomeDir()
	if err != nil || got != "/tmp/dws-real-system-home" {
		t.Fatalf("ResolveSystemHomeDir success = (%q, %v), want /tmp/dws-real-system-home", got, err)
	}

	// Fallback path: when the user record is unavailable, fall back to $HOME.
	testseam.Swap(t, &upgradeCurrentUser, func() (*user.User, error) {
		return nil, errors.New("user record unavailable")
	})
	got, err = ResolveSystemHomeDir()
	if err != nil {
		t.Fatalf("ResolveSystemHomeDir fallback returned error: %v", err)
	}
	if got == "" {
		t.Fatalf("ResolveSystemHomeDir fallback returned empty home")
	}

	// Empty HomeDir is also treated as "unresolved" and falls back.
	testseam.Swap(t, &upgradeCurrentUser, func() (*user.User, error) {
		return &user.User{HomeDir: ""}, nil
	})
	got, err = ResolveSystemHomeDir()
	if err != nil {
		t.Fatalf("ResolveSystemHomeDir empty-home fallback returned error: %v", err)
	}
	if got == "" {
		t.Fatalf("ResolveSystemHomeDir empty-home fallback returned empty home")
	}
}
