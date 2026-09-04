package pidfile

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// farAwayPid can never belong to a live process, so signalling it always
// returns ESRCH without touching anything real.
const farAwayPid = 2000000000

func TestWriteReadRemove(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.pid")

	if err := Write(path, 1234); err != nil {
		t.Fatalf("Write: %v", err)
	}
	got, err := Read(path)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got != 1234 {
		t.Fatalf("Read = %d, want 1234", got)
	}

	if err := Remove(path); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, err := Read(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Read after Remove = %v, want IsNotExist", err)
	}
}

func TestRemoveMissingIsNoError(t *testing.T) {
	if err := Remove(filepath.Join(t.TempDir(), "absent.pid")); err != nil {
		t.Fatalf("Remove(missing) = %v, want nil", err)
	}
}

func TestReadMissing(t *testing.T) {
	if _, err := Read(filepath.Join(t.TempDir(), "absent.pid")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Read(missing) = %v, want IsNotExist", err)
	}
}

func TestReadInvalidContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.pid")
	if err := os.WriteFile(path, []byte("not-a-number\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(path); err == nil || !strings.Contains(err.Error(), "invalid pidfile") {
		t.Fatalf("Read(invalid) = %v, want invalid pidfile error", err)
	}
}

func TestReloadMissingPidfile(t *testing.T) {
	err := Reload(filepath.Join(t.TempDir(), "absent.pid"))
	if err == nil || !strings.Contains(err.Error(), "pidfile not found") {
		t.Fatalf("Reload(missing) = %v, want pidfile not found error", err)
	}
}

func TestReloadStalePidfile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stale.pid")
	if err := Write(path, farAwayPid); err != nil {
		t.Fatal(err)
	}
	err := Reload(path)
	if err == nil || !strings.Contains(err.Error(), "stale pidfile") {
		t.Fatalf("Reload(stale) = %v, want stale pidfile error", err)
	}
}

func TestKillFarAwayPidIsESRCH(t *testing.T) {
	if err := syscall.Kill(farAwayPid, syscall.SIGUSR1); !errors.Is(err, syscall.ESRCH) {
		t.Fatalf("kill far-away pid = %v, want ESRCH", err)
	}
}
