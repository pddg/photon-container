package downloader_test

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pddg/photon-container/internal/downloader"
	"github.com/pddg/photon-container/internal/photondata"
)

func Test_Downloader_Download(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name    string
		options []downloader.DownloaderOption
	}{
		{
			name:    "default",
			options: nil,
		},
		{
			name:    "without progress",
			options: []downloader.DownloaderOption{downloader.WithoutProgress()},
		},
		{
			name: "with progress interval",
			options: []downloader.DownloaderOption{
				downloader.WithProgressInterval(1 * time.Second),
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			// Setup
			dest := filepath.Join(t.TempDir(), "test")
			want := []byte("hello, world")
			requestPathWant := "/test"
			var requestPathGot string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requestPathGot = r.URL.Path
				if strings.HasSuffix(r.URL.Path, ".md5") {
					// Return the md5 hash of the file.
					w.WriteHeader(http.StatusOK)
					hash := md5.Sum(want)
					body := fmt.Sprintf("%s  test", hex.EncodeToString(hash[:]))
					w.Write([]byte(body))
					return
				}
				w.WriteHeader(http.StatusOK)
				w.Write(want)
			}))
			defer srv.Close()
			archive := photondata.NewArchive(srv.URL, photondata.WithArchiveName("test"))
			d := downloader.New(srv.Client(), tc.options...)

			// Exercise
			err := d.Download(t.Context(), archive, dest)

			// Verify
			require.NoError(t, err)
			assert.Equal(t, requestPathWant, requestPathGot, "requested path is wrong")
			require.FileExists(t, dest, "downloaded file does not exist")
			got, err := os.ReadFile(dest)
			require.NoError(t, err)
			assert.Equal(t, want, got, "downloaded file content is wrong")
		})
	}
	t.Run("md5 mismatch", func(t *testing.T) {
		t.Parallel()
		// Setup
		dest := filepath.Join(t.TempDir(), "test")
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, ".md5") {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("wrong md5"))
				return
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("hello, world"))
		}))
		defer srv.Close()
		archive := photondata.NewArchive(srv.URL, photondata.WithArchiveName("test"))
		d := downloader.New(srv.Client())

		// Exercise
		err := d.Download(t.Context(), archive, dest)

		// Verify
		require.Error(t, err)
	})
}

func Test_Downloader_GetLastModified(t *testing.T) {
	t.Parallel()
	// Setup
	want := time.Now().UTC().Truncate(time.Second)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Last-Modified", want.Format(http.TimeFormat))
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	archive := photondata.NewArchive(srv.URL, photondata.WithArchiveName("test"))
	d := downloader.New(srv.Client())

	// Exercise
	got, err := d.GetLastModified(t.Context(), archive)

	// Verify
	require.NoError(t, err)
	assert.Equal(t, want, got, "last modified date is wrong")
}
