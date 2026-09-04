package supervisor

import (
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"
)

func TestNextBackoff(t *testing.T) {
	if got := nextBackoff(initialBackoff, maxBackoff); got != 2*time.Second {
		t.Errorf("nextBackoff(1s) = %v, want 2s", got)
	}
	if got := nextBackoff(30*time.Second, 30*time.Second); got != 30*time.Second {
		t.Errorf("nextBackoff(30s) = %v, want 30s", got)
	}
	if got := nextBackoff(20*time.Second, 30*time.Second); got != 30*time.Second {
		t.Errorf("nextBackoff(20s) = %v, want capped 30s", got)
	}
}

func TestWaitForSignalOrTimeoutExpires(t *testing.T) {
	sig, ok := waitForSignalOrTimeout(10*time.Millisecond, make(chan os.Signal))
	if ok || sig != nil {
		t.Fatalf("got (%v, %v), want (nil, false)", sig, ok)
	}
}

func TestWaitForSignalOrTimeoutReturnsSignal(t *testing.T) {
	ch := make(chan os.Signal, 1)
	ch <- syscall.SIGTERM
	sig, ok := waitForSignalOrTimeout(10*time.Millisecond, ch)
	if !ok || sig != syscall.SIGTERM {
		t.Fatalf("got (%v, %v), want (SIGTERM, true)", sig, ok)
	}
}

func TestWaitForSignalOrTimeoutZeroDelay(t *testing.T) {
	sig, ok := waitForSignalOrTimeout(0, make(chan os.Signal))
	if ok || sig != nil {
		t.Fatalf("got (%v, %v), want (nil, false)", sig, ok)
	}
}

func TestExitCodeFromError(t *testing.T) {
	err := exec.Command("sh", "-c", "exit 7").Run()
	code, ok := exitCodeFromError(err)
	if !ok || code != 7 {
		t.Fatalf("got (%d, %v), want (7, true)", code, ok)
	}
}

func TestExitCodeFromSignaledStatus(t *testing.T) {
	err := exec.Command("sh", "-c", "kill -TERM $$").Run()
	code, ok := exitCodeFromError(err)
	if !ok || code != 128+int(syscall.SIGTERM) {
		t.Fatalf("got (%d, %v), want (%d, true)", code, ok, 128+int(syscall.SIGTERM))
	}
}

func TestSignalProcessGroupRejectsInvalidPid(t *testing.T) {
	if err := signalProcessGroup(0, syscall.SIGTERM); err == nil {
		t.Fatal("expected error for pid 0")
	}
}

// invalidSignal implements os.Signal without being a syscall.Signal, so it can
// exercise the "unsupported signal type" guard without ever reaching
// syscall.Kill. Never pass os.Interrupt here: its dynamic type is
// syscall.SIGINT, and combined with pid=1 that would broadcast SIGINT to every
// process the user owns.
type invalidSignal int

func (invalidSignal) String() string { return "invalid" }
func (invalidSignal) Signal()        {}

func TestSignalProcessGroupRejectsNonSignal(t *testing.T) {
	if err := signalProcessGroup(1, invalidSignal(0)); err == nil {
		t.Fatal("expected error for a type that is not syscall.Signal")
	}
}
