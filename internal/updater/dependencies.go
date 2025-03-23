package updater

import (
	"context"
	"io"
	"time"

	"github.com/pddg/photon-container/internal/photondata"
)

type Downloader interface {
	Download(ctx context.Context, archive photondata.Archive, dest string) error
	GetLastModified(ctx context.Context, archive photondata.Archive) (time.Time, error)
}

type Unarchiver interface {
	Unarchive(ctx context.Context, src io.Reader, dest string) error
}

type PhotonServer interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

type ReplaceMigrator interface {
	MigrateByReplace(ctx context.Context, unarchived string) error
}

type RemoveMigrator interface {
	MigrateByRemoveFirst(ctx context.Context, unarchived string) (func() error, error)
}

type Migrator interface {
	ReplaceMigrator
	RemoveMigrator
	State(ctx context.Context) (photondata.MigrationState, time.Time)
}
