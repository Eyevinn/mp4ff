package mp4_test

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/Eyevinn/mp4ff/mp4"
	"github.com/go-test/deep"
)

func compareOrUpdateInfo(t *testing.T, b mp4.Informer, path string) error {
	return compareOrUpdateInfoLevel(t, b, "all:1", path)
}

// compareOrUpdateInfo - compare box with golden dump or update it with -update flag set
func compareOrUpdateInfoLevel(t *testing.T, b mp4.Informer, specificLevels, path string) error {
	t.Helper()

	var dumpBuf bytes.Buffer
	err := b.Info(&dumpBuf, specificLevels, "", "  ")
	if err != nil {
		t.Error(err)
	}

	if *update { // Generate golden dump file
		err = writeGolden(t, path, dumpBuf.Bytes())
		if err != nil {
			t.Error(err)
		}
		return nil
	}

	// Compare with golden dump file
	golden, err := os.ReadFile(path)
	if err != nil {
		t.Error(err)
	}
	if strings.HasSuffix(path, ".txt") {
		// Replace \r\n with \n to handle accidental Windows line endings
		golden = bytes.ReplaceAll(golden, []byte{13, 10}, []byte{10})
	}
	diff := deep.Equal(golden, dumpBuf.Bytes())
	if diff != nil {
		return fmt.Errorf("Generated dump different from %s", path)
	}
	return nil
}
