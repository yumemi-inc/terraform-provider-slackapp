package tokenstore

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"slices"
	"strings"
)

// commandStderrLimit is how much of a helper's stderr an error quotes.
const commandStderrLimit = 1024

// Command keeps the tokens wherever a helper program puts them: a secret
// manager, a bucket, anything the user can reach from a script. It suits a
// CI runner that starts each run on a clean disk.
//
// The helper is run with one more argument:
//
//   - get: write what store last received to stdout, or nothing when it
//     holds nothing yet.
//   - store: keep stdin, replacing what it held.
//
// Command takes no lock. The helper must keep two runs from rotating at the
// same time, or the user must not start them at once.
type Command struct {
	args []string
}

// NewCommand returns a store that runs args[0] with args[1:] before the
// argument it adds.
func NewCommand(args []string) *Command {
	return &Command{args: slices.Clone(args)}
}

// Lock does nothing: a helper process cannot hold a lock for its caller.
func (c *Command) Lock(context.Context) (func(), error) {
	return func() {}, nil
}

// Load runs the helper with get. It returns nil when the helper printed
// nothing.
func (c *Command) Load(ctx context.Context) ([]byte, error) {
	stdout, err := c.runCommand(ctx, "get", nil)
	if err != nil {
		return nil, err
	}

	if len(bytes.TrimSpace(stdout)) == 0 {
		return nil, nil
	}

	return stdout, nil
}

// Save runs the helper with store, writing data to its stdin.
func (c *Command) Save(ctx context.Context, data []byte) error {
	_, err := c.runCommand(ctx, "store", data)

	return err
}

func (c *Command) runCommand(ctx context.Context, operation string, stdin []byte) ([]byte, error) {
	if len(c.args) == 0 {
		return nil, errors.New("the token store command is empty")
	}

	cmd := exec.CommandContext(ctx, c.args[0], slices.Concat(c.args[1:], []string{operation})...)

	var stdout, stderr bytes.Buffer
	cmd.Stdin = bytes.NewReader(stdin)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// The error quotes stderr, never stdout: stdout carries the tokens.
	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if len(message) > commandStderrLimit {
			message = message[:commandStderrLimit] + "..."
		}
		if message != "" {
			return nil, fmt.Errorf("token store command %q %s: %w: %s", c.args[0], operation, err, message)
		}

		return nil, fmt.Errorf("token store command %q %s: %w", c.args[0], operation, err)
	}

	return stdout.Bytes(), nil
}
