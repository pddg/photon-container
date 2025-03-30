package unarchiver_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pddg/photon-container/internal/unarchiver"
)

func Test_Unarchiver_Unarchive(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name    string
		input   string
		options []unarchiver.UnarchiveOption
	}{
		{
			name:  "compressed",
			input: "testdata/data.tar.bz2",
		},
		{
			name:  "uncompressed",
			input: "testdata/data.tar",
			options: []unarchiver.UnarchiveOption{
				unarchiver.NoCompression(),
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			// Setup
			dest := t.TempDir()
			archive, err := os.Open(tc.input)
			require.NoError(t, err)
			defer archive.Close()
			u := unarchiver.NewUnarchiver()

			// Exercise
			err = u.Unarchive(t.Context(), archive, dest, tc.options...)

			// Verify
			require.NoError(t, err)
			got, err := os.ReadFile(filepath.Join(dest, "data", "hello.txt"))
			require.NoError(t, err)
			assert.Equal(t, "hello world!\n", string(got))
		})
	}
}
