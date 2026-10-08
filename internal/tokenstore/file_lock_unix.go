//go:build unix

//declscope:namespace file

package tokenstore

import (
	"errors"
	"os"
	"syscall"
)

// tryLockFile takes an exclusive lock on f without waiting. It reports
// false when another process holds the lock.
func tryLockFile(f *os.File) (bool, error) {
	err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if errors.Is(err, syscall.EWOULDBLOCK) {
		return false, nil
	}

	return err == nil, err
}

func unlockFile(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
}
