package mp4

import (
	"bytes"
	"testing"
)

func TestWriteReadOfAudioSampleEntry(t *testing.T) {
	ase := CreateAudioSampleEntryBox("mp4a", 2, 16, 48000, nil)

	// Write to a buffer so that we can read and check
	var buf bytes.Buffer
	err := ase.Encode(&buf)
	if err != nil {
		t.Fatal(err)
	}

	// Read back from buffer
	decodedBox, err := DecodeBox(0, &buf)
	if err != nil {
		t.Error("Did not get a box back")
	}
	outAse := decodedBox.(*AudioSampleEntryBox)
	if outAse.SampleRate != ase.SampleRate {
		t.Errorf("Out sampled rate %d differs from in %d", outAse.SampleRate, ase.SampleRate)
	}
}
