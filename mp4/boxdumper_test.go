package mp4

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"testing"

	"github.com/go-test/deep"
)

// compareOrUpdateDump - compare box with golden dump or update it with -update flag set
func compareOrUpdateDump(t *testing.T, b Dumper, path string) error {
	t.Helper()

	var dumpBuf bytes.Buffer
	err := b.Dump(&dumpBuf, "all:1", "", "  ")
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
	golden, err := ioutil.ReadFile(path)
	if err != nil {
		t.Error(err)
	}
	diff := deep.Equal(golden, dumpBuf.Bytes())
	if diff != nil {
		return fmt.Errorf("Generated dump different from %s", path)
	}
	return nil
}
