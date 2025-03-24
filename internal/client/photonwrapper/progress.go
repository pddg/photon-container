package photonwrapper

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/dustin/go-humanize"
)

// Progress is a simple struct that tracks the progress of a download.
// It implements the io.Writer interface.
// You can use this with the io.Copy and io.TeeReader functions to track the progress of a download.
type Progress struct {
	totalBytes int64
	stopCh     chan struct{}
	reader     io.Reader

	mutex     sync.Mutex
	bytesRead int64
}

func NewProgress(
	ctx context.Context,
	reader io.Reader,
	totalBytes int64,
	interval time.Duration,
	logger *slog.Logger,
) *Progress {
	p := &Progress{
		totalBytes: totalBytes,
		reader:     reader,
		stopCh:     make(chan struct{}),
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-p.stopCh:
				return
			case <-ticker.C:
				p.mutex.Lock()
				if p.bytesRead == p.totalBytes {
					return
				}
				bytesRead := humanize.Bytes(uint64(p.bytesRead))
				totalBytes := humanize.Bytes(uint64(p.totalBytes))
				logger.InfoContext(ctx, "upload progress", "bytes_read", bytesRead, "total_bytes", totalBytes, "percentage", float64(p.bytesRead)/float64(p.totalBytes)*100)
				p.mutex.Unlock()
			}
		}
	}()
	return p
}

// Read implements the io.Reader interface.
// It updates the BytesRead field with the number of bytes read.
func (p *Progress) Read(buf []byte) (int, error) {
	n, err := p.reader.Read(buf)
	if err != nil {
		return n, err
	}
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.bytesRead += int64(n)
	return n, nil
}

func (p *Progress) Stop() {
	close(p.stopCh)
}
