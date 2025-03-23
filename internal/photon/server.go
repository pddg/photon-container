package photon

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/pddg/photon-container/internal/logging"
)

type PhotonServer struct {
	jarPath       string
	photonDataDir string

	mutex        sync.Mutex
	photonServer *exec.Cmd
	stopServer   func()

	// additionalArgs are additional arguments to pass to the Photon server.
	additionalArgs []string
}

// NewPhotonServer creates a new PhotonServer.
func NewPhotonServer(
	ctx context.Context,
	jarPath string,
	photonDataDir string,
	options ...PhotonServerOption,
) *PhotonServer {
	ps := &PhotonServer{
		jarPath:       jarPath,
		photonDataDir: photonDataDir,
	}
	for _, option := range options {
		option(ps)
	}
	return ps
}

// Start starts the Photon server.
func (s *PhotonServer) Start(ctx context.Context) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	logger := logging.FromContext(ctx)
	if s.photonServer == nil {
		s.photonServer, s.stopServer = s.newServerCommand(ctx)
	}
	if s.photonServer.ProcessState != nil {
		if s.photonServer.ProcessState.Exited() {
			// The server has exited, so we need to create a new one.
			logger.DebugContext(ctx, "photon server has exited. creating a new server")
			s.photonServer, s.stopServer = s.newServerCommand(ctx)
		} else {
			// The server is already running.
			return nil
		}
	}
	logger.Info("starting photon server", "command", s.photonServer.String())
	if err := s.photonServer.Start(); err != nil {
		return fmt.Errorf("photon.PhotonServer.Start: failed to start Photon server: %w", err)
	}
	return nil
}

// Restart restarts the Photon server.
func (s *PhotonServer) Stop(ctx context.Context) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	logger := logging.FromContext(ctx)
	if s.photonServer == nil {
		logger.DebugContext(ctx, "skip stopping server. photon server is not running")
		return nil
	}
	logger.DebugContext(ctx, "sending SIGTERM to photon server")
	s.stopServer()
	logger.DebugContext(ctx, "waiting for photon server to stop")
	if err := s.photonServer.Wait(); err != nil {
		logger.WarnContext(ctx, "photon exited with error", "error", err)
		// No need to return an error here.
		// The server has stopped, and the error is logged.
		return nil
	}
	logger.InfoContext(ctx, "photon server stopped")
	return nil
}

// Running returns true if the Photon server is running.
func (s *PhotonServer) Running() bool {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if s.photonServer == nil {
		return false
	}
	return s.photonServer.ProcessState != nil && s.photonServer.ProcessState.Exited()
}

func (s *PhotonServer) newServerCommand(
	ctx context.Context,
) (*exec.Cmd, func()) {
	// Do not cancel the context within this function.
	// It is the caller's responsibility to cancel the context.
	serverCtx, cancel := context.WithCancel(ctx)

	args := []string{"-jar", s.jarPath, "-data-dir", s.photonDataDir}
	args = append(args, s.additionalArgs...)

	cmd := exec.CommandContext(serverCtx, "java", args...)

	// Send SIGTERM to the process when cancel is called.
	cmd.Cancel = func() error {
		return cmd.Process.Signal(syscall.SIGTERM)
	}
	// Wait for 10 seconds before sending a SIGKILL after SIGTERM.
	cmd.WaitDelay = 10 * time.Second

	// Redirect stdout and stderr
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd, cancel
}
