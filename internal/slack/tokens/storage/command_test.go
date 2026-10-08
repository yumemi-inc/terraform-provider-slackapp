package storage_test

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/ymm-oss/terraform-provider-slackapp/internal/slack/tokens/storage"
)

// commandHelper writes a helper script with body, and returns its path.
func commandHelper(t *testing.T, body string) string {
	t.Helper()

	if runtime.GOOS == "windows" {
		t.Skip("the helper is a shell script")
	}

	helper := filepath.Join(t.TempDir(), "helper.sh")
	if err := os.WriteFile(helper, []byte("#!/bin/sh\n"+body), 0o700); err != nil {
		t.Fatal(err)
	}

	return helper
}

func TestCommandRoundTrip(t *testing.T) {
	t.Parallel()

	file := filepath.Join(t.TempDir(), "tokens.json")
	// The argument before the operation shows the configured arguments are
	// passed first.
	helper := commandHelper(t, fmt.Sprintf(`
[ "$1" = "--profile=test" ] || { echo "got $1" >&2; exit 2; }
case "$2" in
  get) cat %[1]q 2>/dev/null || true ;;
  store) cat > %[1]q ;;
esac
`, file))
	c := storage.NewCommand([]string{helper, "--profile=test"})
	ctx := t.Context()

	got, err := c.Load(ctx)
	if err != nil || got != nil {
		t.Fatalf("Load before Save = %q, %v; want nil, nil", got, err)
	}

	unlock, err := c.Lock(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()

	if err := c.Save(ctx, []byte(`{"a":1}`)); err != nil {
		t.Fatal(err)
	}

	got, err = c.Load(ctx)
	if err != nil || string(got) != `{"a":1}` {
		t.Fatalf("Load = %q, %v", got, err)
	}
}

// A failing helper's stderr is in the error, and its stdout, which may
// carry the tokens, is not.
func TestCommandFailure(t *testing.T) {
	t.Parallel()

	helper := commandHelper(t, `echo xoxe-secret; echo "no access to the vault" >&2; exit 3`)
	c := storage.NewCommand([]string{helper})

	_, err := c.Load(t.Context())
	if err == nil {
		t.Fatal("Load succeeded")
	}
	if msg := err.Error(); !strings.Contains(msg, "no access to the vault") || !strings.Contains(msg, " get: ") {
		t.Errorf("error = %q, want the operation and stderr", msg)
	}
	if strings.Contains(err.Error(), "xoxe-secret") {
		t.Errorf("the error quotes stdout: %v", err)
	}

	if err := c.Save(t.Context(), []byte("{}")); err == nil || !strings.Contains(err.Error(), " store: ") {
		t.Errorf("Save returned %v, want an error naming store", err)
	}
}

func TestCommandFailureWithoutStderr(t *testing.T) {
	t.Parallel()

	c := storage.NewCommand([]string{commandHelper(t, "exit 1")})

	if _, err := c.Load(t.Context()); err == nil || !strings.HasSuffix(err.Error(), "exit status 1") {
		t.Errorf("Load returned %v", err)
	}
}

func TestCommandLongStderr(t *testing.T) {
	t.Parallel()

	c := storage.NewCommand([]string{commandHelper(t, `head -c 5000 /dev/zero | tr '\0' x >&2; exit 1`)})

	_, err := c.Load(t.Context())
	if err == nil || !strings.HasSuffix(err.Error(), "...") || len(err.Error()) > 1200 {
		t.Errorf("Load returned an error of %d bytes, want stderr cut short", len(fmt.Sprint(err)))
	}
}

func TestCommandEmpty(t *testing.T) {
	t.Parallel()

	if _, err := storage.NewCommand(nil).Load(t.Context()); err == nil {
		t.Error("Load ran an empty command")
	}
}
