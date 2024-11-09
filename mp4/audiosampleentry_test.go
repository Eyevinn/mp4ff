package mp4

import (
	"testing"
)

func TestWriteReadOfAudioSampleEntry(t *testing.T) {
	ascBytes := []byte{0x11, 0x90}
	esds := CreateEsdsBox(ascBytes)
	ase := CreateAudioSampleEntryBox("mp4a", 2, 16, 48000, esds)
	boxDiffAfterEncodeAndDecode(t, ase)
	_, err := ase.RemoveEncryption()
	expectedErrMsg := "is not encrypted: mp4a"
	if err == nil || err.Error() != expectedErrMsg {
		t.Errorf("expected error with message: %q", err.Error())
	}
}
