package compose

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestLockRejectsSecondProcess(t *testing.T) {
	path := filepath.Join(t.TempDir(), "worker.lock")
	lock, err := AcquireLock(path)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Release()

	command := exec.Command(os.Args[0], "-test.run=TestLockChild")
	command.Env = append(os.Environ(), "COMPOSE_TEST_LOCK="+path)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("child process failed: %v: %s", err, output)
	}
	if _, err := AcquireLock(path); !errors.Is(err, ErrBusy) {
		t.Fatalf("second lock error = %v, want ErrBusy", err)
	}
	if err := lock.Release(); err != nil {
		t.Fatal(err)
	}
	available, err := AcquireLock(path)
	if err != nil {
		t.Fatalf("lock after release: %v", err)
	}
	defer available.Release()
}

func TestLockChild(t *testing.T) {
	path := os.Getenv("COMPOSE_TEST_LOCK")
	if path == "" {
		return
	}
	_, err := AcquireLock(path)
	if !errors.Is(err, ErrBusy) {
		t.Fatalf("child lock error = %v, want ErrBusy", err)
	}
}
