package e2e_test

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"testing"

	"github.com/pddg/photon-container/internal/logging"
)

const (
	logDir      = "logs"
	clusterName = "photon-e2e"
	kubeConfig  = "testdata/kubeconfig"
)

func TestMain(m *testing.M) {
	logger, err := logging.Configure("info", "text", os.Stdout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to configure logger: %v\n", err)
		os.Exit(1)
	}
	ctx, cancel := signal.NotifyContext(logging.NewContext(context.Background(), logger), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := testMain(ctx, m); err != nil {
		logger.ErrorContext(ctx, "failed", "error", err)
		os.Exit(1)
	}
}

func testMain(ctx context.Context, m *testing.M) error {
	logger := logging.FromContext(ctx)
	// Cleanup lodDir before running tests
	if err := os.RemoveAll(logDir); err != nil {
		return fmt.Errorf("failed to remove log directory: %w", err)
	}
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}
	cmd := newFileLogCommander(logDir)

	cleanupCluster, err := setupKindCluster(ctx, cmd)
	if err != nil {
		return fmt.Errorf("failed to setup kind cluster: %w", err)
	}
	defer cleanupCluster()

	if err := setupBastionPod(ctx, cmd); err != nil {
		return fmt.Errorf("failed to setup bastion Pod: %w", err)
	}

	logger.InfoContext(ctx, "run tests")
	if exitCode := m.Run(); exitCode != 0 {
		return fmt.Errorf("test failed with exit code %d", exitCode)
	}
	return nil
}

func setupKindCluster(ctx context.Context, cmd *fileLogCommander) (func(), error) {
	logger := logging.FromContext(ctx)
	logger.InfoContext(ctx, "delete old kind cluster")
	if err := cmd.Run(ctx, "kind", "delete", "cluster", "--name", clusterName); err != nil {
		return func() {}, fmt.Errorf("failed to delete old kind cluster: %w", err)
	}

	logger.InfoContext(ctx, "setup kubernetes cluster")
	if err := cmd.Run(ctx, "kind", "create", "cluster", "--name", clusterName, "--kubeconfig", kubeConfig); err != nil {
		return func() {}, fmt.Errorf("failed to create Kubernetes cluster: %w", err)
	}
	cleanup := func() {
		if os.Getenv("DEBUG") == "true" {
			logger.InfoContext(ctx, "debugging mode enabled, skipping cluster deletion")
			return
		}
		logger.InfoContext(ctx, "delete kubernetes cluster")
		if err := cmd.Run(context.WithoutCancel(ctx), "kind", "delete", "cluster", "--name", clusterName); err != nil {
			logger.WarnContext(ctx, "failed to delete Kubernetes cluster", "error", err)
		}
		if err := os.Remove(kubeConfig); err != nil {
			logger.WarnContext(ctx, "failed to remove kubeconfig", "error", err)
		}
	}

	logger.InfoContext(ctx, "load images into the cluster")
	if err := cmd.Run(ctx, "kind", "load", "docker-image", "ghcr.io/pddg/photon:latest", "--name", clusterName); err != nil {
		cleanup()
		return func() {}, fmt.Errorf("failed to load images into the cluster: %w", err)
	}
	return cleanup, nil
}

func setupBastionPod(ctx context.Context, cmd *fileLogCommander) error {
	logger := logging.FromContext(ctx)
	kubectl := func(args ...string) error {
		args = append([]string{"--kubeconfig", kubeConfig}, args...)
		return cmd.Run(ctx, "kubectl", args...)
	}
	logger.InfoContext(ctx, "deploy bastion Pod")
	if err := kubectl("apply", "-f", "testdata/manifests/bastion.yaml"); err != nil {
		return fmt.Errorf("failed to deploy bastion Pod: %w", err)
	}

	logger.InfoContext(ctx, "wait for bastion Pod to be ready")
	if err := kubectl("wait", "--for=condition=Ready", "pod", "bastion"); err != nil {
		return fmt.Errorf("failed to wait for bastion Pod to be ready: %w", err)
	}

	logger.InfoContext(ctx, "Copy photon-db-uploader to bastion Pod")
	if err := kubectl("cp", uploaderBinary(), "bastion:/bin/photon-db-uploader"); err != nil {
		return fmt.Errorf("failed to copy photon-db-uploader to bastion Pod: %w", err)
	}

	logger.InfoContext(ctx, "install curl in bastion Pod")
	if err := kubectl("exec", "-it", "bastion", "--", "apt-get", "update"); err != nil {
		return fmt.Errorf("failed to run apt-get update in bastion Pod: %w", err)
	}
	if err := kubectl("exec", "-it", "bastion", "--", "apt-get", "install", "-y", "--no-install-recommends",
		"curl",
		"ca-certificates",
		"bzip2",
	); err != nil {
		return fmt.Errorf("failed to install curl in bastion Pod: %w", err)
	}
	return nil
}

func uploaderBinary() string {
	binary := os.Getenv("PHOTON_DB_UPLOADER_BINARY")
	if binary == "" {
		binary = fmt.Sprintf("../build/photon-db-updater-linux-%s", runtime.GOARCH)
	}
	return binary
}
