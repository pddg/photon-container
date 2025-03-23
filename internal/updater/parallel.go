package updater

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/pddg/photon-container/internal/logging"
	"github.com/pddg/photon-container/internal/photondata"
)

type ParallelUpdater struct {
	downloader   Downloader
	unarchiver   Unarchiver
	photonServer PhotonServer
	migrator     ReplaceMigrator

	photonDataDir string
}

// NewParallelUpdater creates a new ParallelUpdater.
func NewParallelUpdater(
	downloader Downloader,
	unarchiver Unarchiver,
	photonServer PhotonServer,
	migrator ReplaceMigrator,
	photonDataDir string,
) *ParallelUpdater {
	return &ParallelUpdater{
		downloader:    downloader,
		unarchiver:    unarchiver,
		photonServer:  photonServer,
		migrator:      migrator,
		photonDataDir: photonDataDir,
	}
}

func (u *ParallelUpdater) UpdateByLocalArchive(ctx context.Context, archive photondata.Archive) error {
	logger := logging.FromContext(ctx).With("strategy", UpdateStrategyParallel)

	logger.InfoContext(ctx, "step 1/1: download Photon database")
	archivePath := filepath.Join(u.photonDataDir, "photon-db.tar.bz2")
	if err := u.downloader.Download(ctx, archive, archivePath); err != nil {
		return fmt.Errorf("updater.ParallelUpdater.UpdateByLocalArchive: failed to download %q to %q: %w", archive, archivePath, err)
	}
	defer func() {
		if err := os.RemoveAll(archivePath); err != nil {
			logger.WarnContext(ctx, "failed to remove archive", "path", archivePath, "error", err)
		}
	}()

	logger.InfoContext(ctx, "step 2/3: unarchive Photon database")
	archiveFile, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("updater.ParallelUpdater.UpdateByLocalArchive: failed to open %q: %w", archivePath, err)
	}
	defer archiveFile.Close()
	tempDir := filepath.Join(u.photonDataDir, "temp")
	if err := u.unarchiver.Unarchive(ctx, archiveFile, tempDir); err != nil {
		return fmt.Errorf("updater.ParallelUpdater.UpdateByLocalArchive: failed to unarchive to %q: %w", tempDir, err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			logger.WarnContext(ctx, "failed to remove temp directory", "path", tempDir, "error", err)
		}
	}()

	logger.InfoContext(ctx, "step 3/3: replace archive and restart Photon server")
	if err := u.restartPhotonServer(ctx, tempDir); err != nil {
		return fmt.Errorf("updater.ParallelUpdater.UpdateByLocalArchive: failed to restart Photon server: %w", err)
	}
	logger.InfoContext(ctx, "update complete")
	return nil
}

func (u *ParallelUpdater) UpdateAsync(ctx context.Context, archive io.Reader) error {
	logger := logging.FromContext(ctx).With("strategy", UpdateStrategyParallel)
	logger.InfoContext(ctx, "step 1/2: unarchive Photon database")
	tempDir := filepath.Join(u.photonDataDir, "temp")
	if err := u.unarchiver.Unarchive(ctx, archive, tempDir); err != nil {
		return fmt.Errorf("updater.ParallelUpdater.UpdateAsync: failed to unarchive to %q: %w", tempDir, err)
	}
	go func() {
		defer func() {
			if err := os.RemoveAll(tempDir); err != nil {
				logger.WarnContext(ctx, "failed to remove temp directory", "path", tempDir, "error", err)
			}
		}()
		logger.InfoContext(ctx, "step 2/2: replace archive and restart Photon server")
		if err := u.restartPhotonServer(ctx, tempDir); err != nil {
			logger.ErrorContext(ctx, "failed to restart Photon server", "error", err)
			return
		}
		logger.InfoContext(ctx, "update complete")
	}()
	return nil
}

func (u *ParallelUpdater) restartPhotonServer(ctx context.Context, unarchived string) error {
	if err := u.photonServer.Stop(ctx); err != nil {
		return fmt.Errorf("failed to stop Photon server: %w", err)
	}
	if err := u.migrator.MigrateByReplace(ctx, unarchived); err != nil {
		return fmt.Errorf("failed to replace existing database: %w", err)
	}
	if err := u.photonServer.Start(ctx); err != nil {
		return fmt.Errorf("failed to start Photon server: %w", err)
	}
	return nil
}
