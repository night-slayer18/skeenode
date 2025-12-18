package runner

import (
	"bytes"
	"context"
	"os/exec"
	"syscall"
	"time"
)

type ShellRunner struct{}

func NewShellRunner() *ShellRunner {
	return &ShellRunner{}
}

func (s *ShellRunner) Run(ctx context.Context, cmdStr string, args []string) Result {
	start := time.Now()
	
	// Create command
	// Note: We might want a shell wrapper (e.g. /bin/sh -c) depending on need.
	// For now, we assume cmdStr is the binary and args are arguments.
	// If the user sends "bash -c '...'", cmdStr="bash", args=["-c", "..."]
	cmd := exec.CommandContext(ctx, cmdStr, args...)
	
	// Create buffers for output
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf
	
	// Process Group Management:
	// Setpgid=true asks the OS to assign a new Process Group ID (PGID) to the child.
	// This allows us to kill the entire tree if needed (though CommandContext handles kill mostly).
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	err := cmd.Run()
	duration := time.Since(start)
	
	exitCode := 0
	if err != nil {
		// Try to get exit code
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			// Other error (e.g. failed to start, caught signal)
			exitCode = -1
		}
	}
	
	// Handle Context cancellation specifically if needed
	if ctx.Err() == context.DeadlineExceeded {
		// Ensure we note it was a timeout
		if exitCode == 0 { exitCode = -1 }
		// In a real robust system, we might ensure the PGID is killed here:
		// syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		// But exec.CommandContext usually kills the process.
	}

	return Result{
		ExitCode: exitCode,
		Stdout:   stdoutBuf.String(),
		Stderr:   stderrBuf.String(),
		Duration: duration,
		Error:    err,
	}
}
