package mp4

import (
	"bytes"
	"testing"

	"github.com/edgeware/mp4ff/bits"
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
	encData := buf.Bytes()
	encBuf := bytes.NewBuffer(encData)
	decodedBox, err := DecodeBox(0, encBuf)
	if err != nil {
		t.Error("Did not get a box back")
	}
	outAse := decodedBox.(*AudioSampleEntryBox)
	if outAse.SampleRate != ase.SampleRate {
		t.Errorf("Out sampled rate %d differs from in %d", outAse.SampleRate, ase.SampleRate)
	}

	// Read back from buffer
	sr := bits.NewFixedSliceReader(encData)
	decodedBoxSR, err := DecodeBoxSR(0, sr)
	if err != nil {
		t.Error("Did not get a box back")
	}
	outAse = decodedBoxSR.(*AudioSampleEntryBox)
	if outAse.SampleRate != ase.SampleRate {
		t.Errorf("Out sampled rate %d differs from in %d for SliceReader", outAse.SampleRate, ase.SampleRate)
	}
}
