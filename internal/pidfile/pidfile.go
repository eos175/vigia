package pidfile

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
)

func Write(path string, pid int) error {
	// Keep the pidfile human-readable and easy to inspect from the shell.
	return os.WriteFile(path, []byte(strconv.Itoa(pid)+"\n"), 0o644)
}

func Remove(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func Read(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, fmt.Errorf("invalid pidfile %s: %w", path, err)
	}

	return pid, nil
}

func Reload(path string) error {
	pid, err := Read(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("pidfile not found: %s", path)
		}
		return err
	}

	if err := syscall.Kill(pid, syscall.SIGUSR1); err != nil {
		if errors.Is(err, syscall.ESRCH) {
			return fmt.Errorf("stale pidfile: %s", path)
		}
		return fmt.Errorf("failed to signal pid %d: %w", pid, err)
	}

	return nil
}
