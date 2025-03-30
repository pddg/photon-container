package e2e_test

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

type fileLogCommander struct {
	logDir string
	epoch  int32
}

func newFileLogCommander(logDir string) *fileLogCommander {
	return &fileLogCommander{
		logDir: logDir,
	}
}

func (c *fileLogCommander) newCmd(ctx context.Context, name string, arg ...string) (*exec.Cmd, func()) {
	cmd := exec.CommandContext(ctx, name, arg...)
	cmd.Cancel = func() error {
		return cmd.Process.Signal(syscall.SIGTERM)
	}
	cmd.WaitDelay = 10 * time.Second
	suffix := atomic.AddInt32(&c.epoch, 1)
	logFile := filepath.Join(c.logDir, fmt.Sprintf("%s-%d.log", name, suffix))
	stdout, stderr, cleanup, err := newFileLogger(logFile)
	if err != nil {
		panic(fmt.Errorf("failed to create logger: %w", err))
	}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd, cleanup
}

func (c *fileLogCommander) Run(ctx context.Context, name string, arg ...string) error {
	cmd, cleanup := c.newCmd(ctx, name, arg...)
	defer cleanup()
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return fmt.Errorf("command %q exited with status %d (stderr: %s)", cmd.String(), exitErr.ExitCode(), exitErr.Stderr)
		}
		return fmt.Errorf("failed to run command %q: %w", cmd.String(), err)
	}
	return nil
}

func (c *fileLogCommander) Start(ctx context.Context, name string, arg ...string) (func() error, error) {
	cmdCtx, cancel := context.WithCancel(ctx)
	cmd, cleanup := c.newCmd(cmdCtx, name, arg...)
	if err := cmd.Start(); err != nil {
		cancel()
		cleanup()
		return nil, fmt.Errorf("failed to start command %q: %w", cmd.String(), err)
	}
	return func() error {
		defer cleanup()
		cancel()
		if err := cmd.Wait(); err != nil {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				return fmt.Errorf("command %q exited with status %d (stderr: %s)", cmd.String(), exitErr.ExitCode(), exitErr.Stderr)
			}
			return fmt.Errorf("failed to run command %q: %w", cmd.String(), err)
		}
		return nil
	}, nil
}

type cmdFileLogger struct {
	logger *slog.Logger
	// stream is either "stdout" or "stderr"
	stream string
}

func newFileLogger(logFile string) (*cmdFileLogger, *cmdFileLogger, func(), error) {
	fp, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return nil, nil, nil, err
	}
	logger := slog.New(
		slog.NewJSONHandler(fp, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}),
	)
	cleanup := func() {
		_ = fp.Close()
	}
	return &cmdFileLogger{
			logger: logger,
			stream: "stdout",
		}, &cmdFileLogger{
			logger: logger,
			stream: "stderr",
		}, cleanup, nil
}

func (l *cmdFileLogger) Write(p []byte) (n int, err error) {
	l.logger.Info(string(p), "stream", l.stream)
	return len(p), nil
}

type testCommandLogger struct {
	t *testing.T
}

func (l *testCommandLogger) Write(p []byte) (n int, err error) {
	l.t.Log(string(p))
	return len(p), nil
}

func handleExitError(t *testing.T, err error, cmd string) {
	t.Helper()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		t.Fatalf("command %q exited with status %d (stderr: %s)", cmd, exitErr.ExitCode(), exitErr.Stderr)
	}
	t.Fatalf("failed to run command %q: %v", cmd, err)
}

func runCommand(t *testing.T, cmd *exec.Cmd) {
	t.Helper()
	t.Logf("Running command: %s", cmd.String())
	if err := cmd.Run(); err != nil {
		handleExitError(t, err, cmd.String())
	}
}

func getOutput(t *testing.T, name string, args ...string) string {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), name, args...)
	cmd.Stderr = &testCommandLogger{t}
	t.Logf("Running command: %s", cmd.String())
	output, err := cmd.Output()
	if err != nil {
		handleExitError(t, err, cmd.String())
	}
	return string(output)
}

func kubectlCmd(t *testing.T, args ...string) *exec.Cmd {
	t.Helper()
	preArgs := []string{"--kubeconfig", kubeConfig}
	args = append(preArgs, args...)
	cmd := exec.Command("kubectl", args...)
	cmd.Stderr = &testCommandLogger{t}
	return cmd
}

func kubectl(t *testing.T, args ...string) []byte {
	t.Helper()
	cmd := kubectlCmd(t, args...)
	t.Logf("Running command: %s", cmd.String())
	output, err := cmd.Output()
	if err != nil {
		handleExitError(t, err, "kubectl "+strings.Join(args, " "))
	}
	return output
}

func bastionExec(t *testing.T, args ...string) []byte {
	t.Helper()
	cmd := kubectlCmd(t, append([]string{"exec", "bastion", "--"}, args...)...)
	t.Logf("Running command: %s", cmd.String())
	output, err := cmd.Output()
	if err != nil {
		handleExitError(t, err, cmd.String())
	}
	return output
}

func unsafeBastionExec(t *testing.T, args ...string) ([]byte, error) {
	t.Helper()
	cmd := kubectlCmd(t, append([]string{"exec", "bastion", "--"}, args...)...)
	t.Logf("Running command: %s", cmd.String())
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to run command %q: %w", cmd.String(), err)
	}
	return output, nil
}
