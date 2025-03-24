package unarchiver

import (
	"archive/tar"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"

	"github.com/cosnicolaou/pbzip2"
	"github.com/fujiwara/shapeio"

	"github.com/pddg/photon-container/internal/logging"
)

type Unarchiver struct {
	unarchiveLimitBytesPerSec float64
}

func NewUnarchiver(options ...UnarchiverOption) *Unarchiver {
	u := &Unarchiver{
		unarchiveLimitBytesPerSec: math.MaxFloat64,
	}
	for _, option := range options {
		option(u)
	}
	return u
}

func (u *Unarchiver) Unarchive(ctx context.Context, archive io.Reader, destPath string, options ...UnarchiveOption) error {
	logger := logging.FromContext(ctx)
	logger.Info("Unarchive database", "dest", destPath)
	destStat, err := os.Stat(destPath)
	if err != nil {
		if err := os.MkdirAll(destPath, 0755); err != nil {
			return fmt.Errorf("unarchiver.Unarchiver.Unarchive: failed to create destination directory %q: %w", destPath, err)
		}
		logger.Info("Created destination directory", "dest", destPath)
	} else {
		if !destStat.IsDir() {
			return fmt.Errorf("unarchiver.Unarchiver.Unarchive: destination %q is not a directory", destPath)
		}
	}
	if err := u.unarchive(ctx, archive, destPath, options...); err != nil {
		if rmErr := os.RemoveAll(destPath); rmErr != nil {
			err = errors.Join(err, fmt.Errorf("unarchiver.Unarchiver.Unarchive: failed to remove destination directory %q: %w", destPath, rmErr))
		}
		return fmt.Errorf("unarchiver.Unarchiver.Unarchive: failed to unarchive to %q: %w", destPath, err)
	}
	return nil
}

type runtimeOption struct {
	// noCompression specifies whether to skip decompression.
	noCompression bool
}

func (u *Unarchiver) unarchive(ctx context.Context, archive io.Reader, destPath string, options ...UnarchiveOption) error {
	opt := &runtimeOption{}
	for _, option := range options {
		option(opt)
	}
	var r io.Reader
	if opt.noCompression {
		r = archive
	} else {
		r = pbzip2.NewReader(ctx, archive)
	}
	limited := shapeio.NewReaderWithContext(r, ctx)
	limited.SetRateLimit(u.unarchiveLimitBytesPerSec)
	untar := tar.NewReader(limited)
	for {
		header, err := untar.Next()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("unarchiver.Unarchiver.Unarchive: failed to read tar header: %w", err)
		}
		if header == nil {
			continue
		}
		target := filepath.Join(destPath, header.Name)
		switch header.Typeflag {
		case tar.TypeDir:
			if _, err := os.Stat(target); err != nil {
				if errors.Is(err, os.ErrNotExist) {
					if err := os.Mkdir(target, 0755); err != nil {
						return fmt.Errorf("unarchiver.Unarchiver.Unarchive: failed to create directory %q: %w", target, err)
					}
				} else {
					return fmt.Errorf("unarchiver.Unarchiver.Unarchive: failed to stat directory %q: %w", target, err)
				}
			}
			if err := os.Chtimes(target, header.ModTime, header.ModTime); err != nil {
				return fmt.Errorf("unarchiver.Unarchiver.Unarchive: failed to change modtime of directory %q: %w", target, err)
			}
		case tar.TypeReg:
			if err := atomicWrite(target, untar); err != nil {
				return fmt.Errorf("unarchiver.Unarchiver.Unarchive: failed to write file %q: %w", target, err)
			}
			if err := os.Chtimes(target, header.ModTime, header.ModTime); err != nil {
				return fmt.Errorf("unarchiver.Unarchiver.Unarchive: failed to change modtime of directory %q: %w", target, err)
			}
		}
	}
}

func atomicWrite(dest string, src io.Reader) error {
	tmpFile, err := os.CreateTemp(filepath.Dir(dest), "tmp-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer func() {
		_ = tmpFile.Close()
		_ = os.Remove(tmpFile.Name())
	}()
	if _, err := io.Copy(tmpFile, src); err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}
	if err := os.Rename(tmpFile.Name(), dest); err != nil {
		return fmt.Errorf("failed to rename temp file to %q: %w", dest, err)
	}
	return nil
}
