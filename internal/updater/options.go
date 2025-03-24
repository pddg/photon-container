package updater

import (
	"github.com/pddg/photon-container/internal/photondata"
	"github.com/pddg/photon-container/internal/unarchiver"
)

type UpdateOption func(*updateOptions)

// WithUnarchiveOptions sets the options for the unarchiver.
// If called multiple times, the options will be appended.
// No deduplication is performed.
func WithUnarchiveOptions(opts ...unarchiver.UnarchiveOption) UpdateOption {
	return func(o *updateOptions) {
		if o.unarchiveOpts == nil {
			o.unarchiveOpts = &unarchiveOptions{
				options: opts,
			}
			return
		}
		if o.unarchiveOpts.options == nil {
			o.unarchiveOpts.options = opts
			return
		}
		o.unarchiveOpts.options = append(o.unarchiveOpts.options, opts...)
	}
}

// WithForceUpdate forces the update process to start.
// This will reset the migration state.
func WithForceUpdate() UpdateOption {
	return func(o *updateOptions) {
		o.force = true
	}
}

// WithArchiveName sets the name of the archive to be downloaded.
func WithArchiveName(name string) UpdateOption {
	return func(o *updateOptions) {
		o.archiveName = name
	}
}

type updateOptions struct {
	force         bool
	archiveName   string
	unarchiveOpts *unarchiveOptions
}

func initOptions(opts ...UpdateOption) *updateOptions {
	o := &updateOptions{
		unarchiveOpts: newUnarchiveOptions(),
	}
	for _, opt := range opts {
		opt(o)
	}
	return o
}

func (uo *updateOptions) getArchive(orig photondata.Archive) photondata.Archive {
	if uo.archiveName != "" {
		return orig.FromArchiveName(uo.archiveName)
	}
	return orig
}

func (uo *updateOptions) getUnarchiveOptions() []unarchiver.UnarchiveOption {
	return uo.unarchiveOpts.options
}

type unarchiveOptions struct {
	options []unarchiver.UnarchiveOption
}

func newUnarchiveOptions() *unarchiveOptions {
	return &unarchiveOptions{}
}
