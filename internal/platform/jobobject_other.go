//go:build !windows

package platform

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
)

// runContained is the non-Windows fallback: timeout and output limits only.
// ponytail: the PoC targets Windows 11 only — this exists so the package
// builds and so sidecar tests can run on a dev machine. Memory limits are not
// enforced here; add cgroups/rlimit only if another OS becomes a target.
func runContained(ctx context.Context, exe string, args, env []string, lim Limits) ([]byte, error) {
	if lim.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, lim.Timeout)
		defer cancel()
	}
	// #nosec G204 -- exe is a verified absolute path; args are resolved paths.
	cmd := exec.CommandContext(ctx, exe, args...)
	cmd.Env = env
	cmd.Cancel = func() error { return cmd.Process.Kill() }
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("wiring sidecar stdout: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting sidecar: %w", err)
	}

	limit := lim.OutputMax
	if limit <= 0 {
		limit = DefaultLimits.OutputMax
	}
	out, readErr := io.ReadAll(io.LimitReader(stdout, limit+1))
	overflow := int64(len(out)) > limit
	if overflow {
		_ = cmd.Process.Kill()
	}
	waitErr := cmd.Wait()

	switch {
	case overflow:
		return nil, fmt.Errorf("%w: over %d bytes of stdout", ErrOutputLimit, limit)
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		return nil, fmt.Errorf("%w after %s", ErrTimeout, lim.Timeout)
	case ctx.Err() != nil:
		return nil, ctx.Err()
	case readErr != nil:
		return nil, fmt.Errorf("reading sidecar stdout: %w", readErr)
	case waitErr != nil:
		return nil, fmt.Errorf("%w: %w", ErrExtract, waitErr)
	}
	return out, nil
}
