package compose

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

var ErrBusy = errors.New("compose worker is already running")

type Lock struct{ file *os.File }

// AcquireLock holds a process-wide compose slot until Release or process exit.
func AcquireLock(path string) (*Lock, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open compose lock: %w", err)
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		file.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, ErrBusy
		}
		return nil, fmt.Errorf("acquire compose lock: %w", err)
	}
	return &Lock{file: file}, nil
}

func (l *Lock) Release() error {
	err := syscall.Flock(int(l.file.Fd()), syscall.LOCK_UN)
	return errors.Join(err, l.file.Close())
}
