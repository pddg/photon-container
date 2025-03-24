package updater

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/dustin/go-humanize"

	"github.com/pddg/photon-container/internal/logging"
	"github.com/pddg/photon-container/internal/photondata"
)

type UpdaterInterface interface {
	DownloadAndUpdate(ctx context.Context, archive photondata.Archive, options ...UpdateOption) error
	UpdateAsync(ctx context.Context, archive io.Reader, options ...UpdateOption) error
}

type Updater struct {
	updaterImpl UpdaterInterface
	migrator    Migrator
	downloader  Downloader
}

func New(
	strategy UpdateStrategy,
	downloader Downloader,
	unarchiver Unarchiver,
	photonServer PhotonServer,
	migrator Migrator,
	photonDataDir string,
) (*Updater, error) {
	var updaterImpl UpdaterInterface
	switch strategy {
	case UpdateStrategySequential:
		updaterImpl = NewSequentialUpdater(downloader, unarchiver, photonServer, migrator, photonDataDir)
	case UpdateStrategyParallel:
		updaterImpl = NewParallelUpdater(downloader, unarchiver, photonServer, migrator, photonDataDir)
	default:
		return nil, fmt.Errorf("updater.NewUpdater: unknown strategy %q", strategy)
	}
	return &Updater{
		updaterImpl: updaterImpl,
		migrator:    migrator,
		downloader:  downloader,
	}, nil
}

func (u *Updater) DownloadAndUpdate(ctx context.Context, archive photondata.Archive, options ...UpdateOption) error {
	opts := initOptions(options...)
	if opts.force {
		logging.FromContext(ctx).WarnContext(ctx, "force update initiated")
		u.migrator.ResetState(ctx)
	} else {
		archive = opts.getArchive(archive)
		migratable, err := u.checkMigratability(ctx, archive)
		if err != nil {
			return fmt.Errorf("updater.Updater.UpdateByLocalArchive: failed to check migratability: %w", err)
		}
		if !migratable {
			return fmt.Errorf("updater.Updater.UpdateByLocalArchive: database is up to date")
		}
	}
	return u.updaterImpl.DownloadAndUpdate(ctx, archive)
}

func (u *Updater) UpdateAsync(ctx context.Context, archive io.Reader, options ...UpdateOption) error {
	opts := initOptions(options...)
	if opts.force {
		logging.FromContext(ctx).WarnContext(ctx, "force update initiated")
		u.migrator.ResetState(ctx)
	}
	return u.updaterImpl.UpdateAsync(ctx, archive, options...)
}

func (u *Updater) checkMigratability(ctx context.Context, archive photondata.Archive) (bool, error) {
	_, importTime := u.migrator.State(ctx)
	lastModified, err := u.downloader.GetLastModified(ctx, archive)
	if err != nil {
		return false, fmt.Errorf("updater.Updater.Update: failed to get last modified time of %q: %w", archive, err)
	}
	logger := logging.FromContext(ctx)
	// The importTime returned by photon may be a few days before the time the database was uploaded (lastModified).
	// Therefore, simply comparing it with importTime will result in an update every time.
	diff := lastModified.Sub(importTime)
	if diff <= 7*24*time.Hour {
		logger.InfoContext(ctx, "database is up to date", "importTime", importTime, "lastModified", humanize.RelTime(importTime, lastModified, "ago", "from now"))
		return false, nil
	}
	return true, nil
}
