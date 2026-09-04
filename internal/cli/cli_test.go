package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/eos175/vigia/internal/pidfile"
)

func TestRunNoArgsShowsUsage(t *testing.T) {
	if code := Run(nil); code != 1 {
		t.Fatalf("Run(nil) = %d, want 1", code)
	}
}

func TestRunVersion(t *testing.T) {
	if code := Run([]string{"version"}); code != 0 {
		t.Fatalf("Run(version) = %d, want 0", code)
	}
}

func TestRunRejectsNegativeMaxRestarts(t *testing.T) {
	if code := Run([]string{"--max-restarts", "-1", "sleep", "1"}); code != 1 {
		t.Fatalf("Run(negative max-restarts) = %d, want 1", code)
	}
}

func TestRunRejectsEmptyPidfile(t *testing.T) {
	if code := Run([]string{"--pidfile", "", "sleep", "1"}); code != 1 {
		t.Fatalf("Run(empty pidfile) = %d, want 1", code)
	}
}

func TestRunReloadMissingPidfile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "absent.pid")
	if code := Run([]string{"reload", "--pidfile", path}); code != 1 {
		t.Fatalf("Run(reload missing) = %d, want 1", code)
	}
}

func TestRunReloadStalePidfile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stale.pid")
	if err := pidfile.Write(path, 2000000000); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(path)
	if code := Run([]string{"reload", "--pidfile", path}); code != 1 {
		t.Fatalf("Run(reload stale) = %d, want 1", code)
	}
}
