package mp4

import (
	"bytes"
	"io/ioutil"
	"testing"
)

// TestBoxDumper versus golden file. Can be regenerated with -update
func TestBoxDumper(t *testing.T) {
	goldenAssetPath := "testdata/trun_dump.golden"
	trun := CreateTrun()
	trun.DataOffset = 314159
	fs := FullSample{
		Sample: Sample{
			Flags: SyncSampleFlags,
			Dur:   1024,
			Size:  4,
			Cto:   -512,
		},
		DecodeTime: 1024,
		Data:       []byte{0, 1, 2, 3},
	}
	trun.AddFullSample(&fs)

	specificBoxLevels := "trun:1"
	buf := bytes.Buffer{}
	err := trun.Dump(&buf, specificBoxLevels, "", "  ")
	if err != nil {
		t.Error(err)
	}
	if *update {
		err = writeGolden(t, goldenAssetPath, buf.Bytes())
		if err != nil {
			t.Error(err)
		}
		return
	}
	got := buf.String()
	golden, err := ioutil.ReadFile(goldenAssetPath)
	if err != nil {
		t.Error(err)
	}
	want := string(golden)
	if got != want {
		t.Errorf("Got %s instead of %s", got, want)
	}
}
