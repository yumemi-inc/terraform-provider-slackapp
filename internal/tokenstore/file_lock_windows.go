//go:build windows

//declscope:namespace file

package tokenstore

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

// tryLockFile takes an exclusive lock on f without waiting. It reports
// false when another process holds the lock.
func tryLockFile(f *os.File) (bool, error) {
	err := windows.LockFileEx(
		windows.Handle(f.Fd()),
		windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY,
		0, 1, 0,
		new(windows.Overlapped),
	)
	if errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
		return false, nil
	}

	return err == nil, err
}

func unlockFile(f *os.File) error {
	return windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, new(windows.Overlapped))
}
