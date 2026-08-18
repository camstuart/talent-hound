package platform

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// runContained runs exe under a Windows Job Object enforcing lim: a wall-clock
// timeout, a whole-tree memory limit, an output-size limit, and kill-on-close
// so no child outlives the job. It returns stdout on success.
func runContained(ctx context.Context, exe string, args, env []string, lim Limits) ([]byte, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, fmt.Errorf("creating job object: %w", err)
	}
	defer func() { _ = windows.CloseHandle(job) }()

	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if lim.MemoryMax > 0 {
		info.BasicLimitInformation.LimitFlags |= windows.JOB_OBJECT_LIMIT_JOB_MEMORY
		info.JobMemoryLimit = uintptr(lim.MemoryMax)
	}
	if _, err := windows.SetInformationJobObject(job,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
		return nil, fmt.Errorf("setting job limits: %w", err)
	}

	if lim.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, lim.Timeout)
		defer cancel()
	}

	// #nosec G204 -- exe is a verified absolute path inside the install
	// directory; args are absolute paths resolved by the caller.
	cmd := exec.Command(exe, args...)
	cmd.Env = env
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("wiring sidecar stdout: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting sidecar: %w", err)
	}
	// ponytail: assign immediately after Start rather than starting suspended.
	// A child spawned in the first microseconds would escape the job; switch to
	// CREATE_SUSPENDED + ResumeThread if that ever matters.
	proc, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(cmd.Process.Pid))
	if err != nil {
		_ = cmd.Process.Kill()
		return nil, fmt.Errorf("opening sidecar process: %w", err)
	}
	defer func() { _ = windows.CloseHandle(proc) }()
	if err := windows.AssignProcessToJobObject(job, proc); err != nil {
		_ = cmd.Process.Kill()
		return nil, fmt.Errorf("assigning sidecar to job: %w", err)
	}

	kill := func() { _ = windows.TerminateJobObject(job, 1) }
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			kill()
		case <-done:
		}
	}()

	limit := lim.OutputMax
	if limit <= 0 {
		limit = DefaultLimits.OutputMax
	}
	out, readErr := io.ReadAll(io.LimitReader(stdout, limit+1))
	overflow := int64(len(out)) > limit
	if overflow {
		kill()
	}
	waitErr := cmd.Wait()

	switch {
	case overflow:
		return nil, fmt.Errorf("%w: over %d bytes of stdout", ErrOutputLimit, limit)
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		return nil, fmt.Errorf("%w after %s", ErrTimeout, lim.Timeout)
	case ctx.Err() != nil:
		return nil, ctx.Err()
	}
	if lim.MemoryMax > 0 && peakJobMemory(job) >= uintptr(lim.MemoryMax) {
		return nil, fmt.Errorf("%w of %d bytes", ErrMemoryLimit, lim.MemoryMax)
	}
	if readErr != nil {
		return nil, fmt.Errorf("reading sidecar stdout: %w", readErr)
	}
	if waitErr != nil {
		return nil, fmt.Errorf("%w: %w", ErrExtract, waitErr)
	}
	return out, nil
}

// peakJobMemory reports the job's peak committed memory; 0 if unavailable.
func peakJobMemory(job windows.Handle) uintptr {
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	if err := windows.QueryInformationJobObject(job,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info)), nil); err != nil {
		return 0
	}
	return info.PeakJobMemoryUsed
}

// waitForExit reports whether pid is gone within d. Used by the gate tests to
// prove process-tree termination.
func waitForExit(pid int, d time.Duration) bool {
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
		if err != nil {
			return true
		}
		var code uint32
		err = windows.GetExitCodeProcess(h, &code)
		_ = windows.CloseHandle(h)
		if err == nil && code != 259 { // STILL_ACTIVE
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return false
}
