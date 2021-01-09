package mp4

import (
	"testing"
)

// TestTrunDump versus golden file. Can be regenerated with -update
func TestTrunInfo(t *testing.T) {
	goldenDumpPath := "testdata/golden_trun_dump.txt"
	trun := CreateTrun(0)
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

	err := compareOrUpdateInfo(t, trun, goldenDumpPath)
	if err != nil {
		t.Error(err)
	}
}
