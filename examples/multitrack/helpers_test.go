package main

import (
	"flag"
	"os"
	"testing"
)

// Helpers to tests. By including t.Helper(), the right failing line in the test
// itself is reported.

var (
	update = flag.Bool("update", false, "update the golden files of this test")
)

func assertNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Errorf("Got error %s but expected none", err)
	}
}

func assertError(t *testing.T, err error, msg string) {
	t.Helper()
	if err == nil {
		t.Errorf(msg)
	}
}

// writeGolden - write golden file that to be used for later tests
func writeGolden(t *testing.T, goldenAssetPath string, data []byte) error {
	t.Helper()
	fd, err := os.Create(goldenAssetPath)
	if err != nil {
		return err
	}
	_, err = fd.Write(data)
	if err != nil {
		return err
	}
	return nil
}

// TestMain is to set flags for tests. In particular, the update flag to update golden files.
func TestMain(m *testing.M) {
	flag.Parse()
	os.Exit(m.Run())
}
