package storage_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/ymm-oss/terraform-provider-slackapp/internal/slack/tokens/storage"
)

func TestFileRoundTrip(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "nested", "tokens.json")
	f := storage.NewFile(path)
	ctx := t.Context()

	got, err := f.Load(ctx)
	if err != nil || got != nil {
		t.Fatalf("Load before Save = %q, %v; want nil, nil", got, err)
	}

	for _, want := range []string{`{"a":1}`, `{"b":2}`} {
		if err := f.Save(ctx, []byte(want)); err != nil {
			t.Fatal(err)
		}

		got, err := f.Load(ctx)
		if err != nil || string(got) != want {
			t.Fatalf("Load = %q, %v; want %q", got, err, want)
		}
	}

	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if perm := info.Mode().Perm(); perm != 0o600 {
			t.Errorf("the file has mode %o, want 600", perm)
		}
	}

	// Save leaves no temporary file behind.
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("the directory holds %d entries, want only the token file", len(entries))
	}
}

func TestFileLoadError(t *testing.T) {
	t.Parallel()

	// A directory where the file should be cannot be read as one.
	if _, err := storage.NewFile(t.TempDir()).Load(t.Context()); err == nil {
		t.Error("Load read a directory")
	}
}

func TestFileSaveError(t *testing.T) {
	t.Parallel()

	// A file where the directory should be cannot hold the token file.
	parent := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(parent, nil, 0o600); err != nil {
		t.Fatal(err)
	}

	f := storage.NewFile(filepath.Join(parent, "tokens.json"))
	if err := f.Save(t.Context(), []byte("{}")); err == nil {
		t.Error("Save wrote under a file")
	}
	if _, err := f.Lock(t.Context()); err == nil {
		t.Error("Lock locked under a file")
	}
}

// A second holder waits for the first to unlock, and gives up when its
// context ends first.
func TestFileLock(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "tokens.json")
	first := storage.NewFile(path)
	second := storage.NewFile(path)

	unlock, err := first.Lock(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(t.Context(), 300*time.Millisecond)
	defer cancel()
	if _, err := second.Lock(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Lock while held returned %v, want the context's deadline", err)
	}

	locked := make(chan func())
	go func() {
		unlockSecond, err := second.Lock(t.Context())
		if err != nil {
			t.Error(err)
		}
		locked <- unlockSecond
	}()

	select {
	case <-locked:
		t.Fatal("Lock returned while the lock was held")
	case <-time.After(200 * time.Millisecond):
	}

	unlock()

	select {
	case unlockSecond := <-locked:
		unlockSecond()
	case <-time.After(5 * time.Second):
		t.Fatal("Lock did not return after the lock was released")
	}
}

// A directory the process may not write to holds neither the token file
// nor the lock file.
func TestFileReadOnlyDirectory(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs a directory the process cannot write to")
	}

	dir := t.TempDir()
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	f := storage.NewFile(filepath.Join(dir, "tokens.json"))
	if err := f.Save(t.Context(), []byte("{}")); err == nil {
		t.Error("Save wrote to a read-only directory")
	}
	if _, err := f.Lock(t.Context()); err == nil {
		t.Error("Lock created a lock file in a read-only directory")
	}
}
