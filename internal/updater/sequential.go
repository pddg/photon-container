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

type SequentialUpdater struct {
	downloader   Downloader
	unarchiver   Unarchiver
	photonServer PhotonServer
	migrator     RemoveMigrator

	photonDataDir string
}

// NewSequentialUpdater creates a new SequentialUpdater.
func NewSequentialUpdater(
	downloader Downloader,
	unarchiver Unarchiver,
	photonServer PhotonServer,
	migrator RemoveMigrator,
	photonDataDir string,
) *SequentialUpdater {
	return &SequentialUpdater{
		downloader:    downloader,
		unarchiver:    unarchiver,
		photonServer:  photonServer,
		migrator:      migrator,
		photonDataDir: photonDataDir,
	}
}

func (u *SequentialUpdater) UpdateByLocalArchive(ctx context.Context, archive photondata.Archive) error {
	logger := logging.FromContext(ctx).With("strategy", UpdateStrategySequential)

	logger.InfoContext(ctx, "step 1/6: stop Photon server")
	tempDir := filepath.Join(u.photonDataDir, "temp")
	if err := u.photonServer.Stop(ctx); err != nil {
		return fmt.Errorf("updater.SequentialUpdater.UpdateByLocalArchive: failed to stop Photon server: %w", err)
	}

	logger.InfoContext(ctx, "step 2/6: remove existing database")
	runMigration, err := u.migrator.MigrateByRemoveFirst(ctx, tempDir)
	if err != nil {
		return fmt.Errorf("updater.SequentialUpdater.UpdateByLocalArchive: failed to remove existing database: %w", err)
	}

	logger.InfoContext(ctx, "step 3/6: download Photon database")
	archivePath := filepath.Join(u.photonDataDir, "photon-db.tar.bz2")
	if err := u.downloader.Download(ctx, archive, archivePath); err != nil {
		return fmt.Errorf("updater.SequentialUpdater.UpdateByLocalArchive: failed to download %q to %q: %w", archive, archivePath, err)
	}
	defer func() {
		if err := os.RemoveAll(archivePath); err != nil {
			logger.WarnContext(ctx, "failed to remove archive", "path", archivePath, "error", err)
		}
	}()

	logger.InfoContext(ctx, "step 4/6: unarchive Photon database")
	archiveFile, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("updater.SequentialUpdater.UpdateByLocalArchive: failed to open %q: %w", archivePath, err)
	}
	defer archiveFile.Close()
	if err := u.unarchiver.Unarchive(ctx, archiveFile, tempDir); err != nil {
		return fmt.Errorf("updater.SequentialUpdater.UpdateByLocalArchive: failed to unarchive %q to %q: %w", archivePath, tempDir, err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			logger.WarnContext(ctx, "failed to remove temp directory", "path", tempDir, "error", err)
		}
	}()

	logger.InfoContext(ctx, "step 5/6: replace existing database")
	if err := runMigration(); err != nil {
		return fmt.Errorf("updater.SequentialUpdater.UpdateByLocalArchive: failed to run migration: %w", err)
	}

	logger.InfoContext(ctx, "step 6/6: start Photon server")
	if err := u.photonServer.Start(ctx); err != nil {
		return fmt.Errorf("updater.SequentialUpdater.UpdateByLocalArchive: failed to start Photon server: %w", err)
	}

	logger.InfoContext(ctx, "update complete")
	return nil
}

func (u *SequentialUpdater) UpdateAsync(ctx context.Context, archive io.Reader) error {
	logger := logging.FromContext(ctx).With("strategy", UpdateStrategySequential)

	logger.InfoContext(ctx, "step 1/5: stop Photon server")
	tempDir := filepath.Join(u.photonDataDir, "temp")
	if err := u.photonServer.Stop(ctx); err != nil {
		return fmt.Errorf("updater.SequentialUpdater.UpdateAsync: failed to stop Photon server: %w", err)
	}

	logger.InfoContext(ctx, "step 2/5: remove existing database")
	runMigration, err := u.migrator.MigrateByRemoveFirst(ctx, tempDir)
	if err != nil {
		return fmt.Errorf("updater.SequentialUpdater.UpdateAsync: failed to remove existing database: %w", err)
	}

	logger.InfoContext(ctx, "step 3/5: unarchive Photon database")
	if err := u.unarchiver.Unarchive(ctx, archive, tempDir); err != nil {
		return fmt.Errorf("updater.SequentialUpdater.UpdateAsync: failed to unarchive to %q: %w", tempDir, err)
	}
	go func() {
		defer func() {
			if err := os.RemoveAll(tempDir); err != nil {
				logger.WarnContext(ctx, "failed to remove temp directory", "path", tempDir, "error", err)
			}
		}()
		logger.InfoContext(ctx, "step 4/5: replace existing database")
		if err := runMigration(); err != nil {
			logger.ErrorContext(ctx, "failed to run migration", "error", err)
			return
		}
		logger.InfoContext(ctx, "step 5/5: start Photon server")
		if err := u.photonServer.Start(ctx); err != nil {
			logger.ErrorContext(ctx, "failed to start Photon server", "error", err)
			return
		}
		logger.InfoContext(ctx, "update complete")
	}()
	return nil
}
