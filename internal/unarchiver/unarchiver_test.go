package unarchiver_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pddg/photon-container/internal/unarchiver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Unarchiver_Unarchive(t *testing.T) {
	t.Parallel()
	t.Run("normal", func(t *testing.T) {
		t.Parallel()
		// Setup
		archivePath := "testdata/data.tar.bz2"
		dest := t.TempDir()
		archive, err := os.Open(archivePath)
		require.NoError(t, err)
		defer archive.Close()
		u := unarchiver.NewUnarchiver()

		// Exercise
		err = u.Unarchive(t.Context(), archive, dest)

		// Verify
		require.NoError(t, err)
		got, err := os.ReadFile(filepath.Join(dest, "data", "hello.txt"))
		require.NoError(t, err)
		assert.Equal(t, "hello world!\n", string(got))
	})
}
