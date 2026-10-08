// Package tokenstore keeps Slack's rotated tokens between Terraform runs.
//
// Slack's refresh token works once: tooling.tokens.rotate answers with a new
// one and voids the old. A store keeps the newest, so that the next process
// can rotate again. The stores here hold the bytes internal/slack/tokens gives
// them and know nothing of what they mean.
package tokenstore

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// fileLockPollInterval is how long File.Lock waits before it tries the lock
// again.
const fileLockPollInterval = 100 * time.Millisecond

// File keeps the tokens in a file on the local disk. It suits a machine
// whose disk outlives a Terraform run, such as an Atlantis server.
//
// Processes that share the file take turns through a lock file beside it,
// so that two of them never rotate the same refresh token.
type File struct {
	path string
}

func NewFile(path string) *File {
	return &File{path: path}
}

// Lock waits until no other process holds the file, or until ctx is done.
func (f *File) Lock(ctx context.Context) (func(), error) {
	if err := os.MkdirAll(filepath.Dir(f.path), 0o700); err != nil {
		return nil, err
	}

	// The lock is on a file of its own: Save replaces the token file by
	// renaming another over it, which would drop a lock held on it.
	lockFile, err := os.OpenFile(f.path+".lock", os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}

	for {
		locked, err := tryLockFile(lockFile)
		if err != nil {
			_ = lockFile.Close()

			return nil, fmt.Errorf("locking %s: %w", lockFile.Name(), err)
		}

		if locked {
			return func() {
				_ = unlockFile(lockFile)
				_ = lockFile.Close()
			}, nil
		}

		select {
		case <-ctx.Done():
			_ = lockFile.Close()

			return nil, fmt.Errorf("waiting for the lock on %s: %w", lockFile.Name(), ctx.Err())
		case <-time.After(fileLockPollInterval):
		}
	}
}

// Load returns what the file holds, or nil when there is no file yet.
func (f *File) Load(context.Context) ([]byte, error) {
	data, err := os.ReadFile(f.path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}

	return data, err
}

// Save replaces the file with data. It writes another file and renames it
// over the old one, so that a crash leaves the old tokens whole, never a
// file half written.
func (f *File) Save(_ context.Context, data []byte) error {
	dir := filepath.Dir(f.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}

	// os.CreateTemp creates the file readable by its owner only.
	tmp, err := os.CreateTemp(dir, filepath.Base(f.path)+".tmp-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()

	_, err = tmp.Write(data)
	if err == nil {
		err = tmp.Sync()
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}

	return os.Rename(tmp.Name(), f.path)
}
