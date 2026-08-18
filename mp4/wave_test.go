package mp4_test

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"

	"github.com/Eyevinn/mp4ff/mp4"
)

func waveChildBytes(name string, payload []byte) []byte {
	child := make([]byte, 0, 8+len(payload))
	child = binary.BigEndian.AppendUint32(child, uint32(8+len(payload)))
	child = append(child, []byte(name)...)
	return append(child, payload...)
}

func waveBytes(children ...[]byte) []byte {
	size := 8
	for _, child := range children {
		size += len(child)
	}
	box := make([]byte, 0, size)
	box = binary.BigEndian.AppendUint32(box, uint32(size))
	box = append(box, []byte("wave")...)
	for _, child := range children {
		box = append(box, child...)
	}
	return box
}

func encodedEsdsBytes(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := mp4.CreateEsdsBox([]byte{0x11, 0x90}).Encode(&buf); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestWaveBox(t *testing.T) {
	esds := encodedEsdsBytes(t)
	frma := waveChildBytes("frma", []byte("mp4a"))
	mp4aStub := waveChildBytes("mp4a", make([]byte, 4)) // 12-byte QuickTime stub
	terminator := waveChildBytes(string([]byte{0, 0, 0, 0}), nil)

	data := waveBytes(frma, mp4aStub, esds, terminator)
	cmpAfterDecodeEncodeBox(t, data)

	box, err := mp4.DecodeBox(0, bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	wave := box.(*mp4.WaveBox)
	if wave.Frma == nil || wave.Frma.DataFormat != "mp4a" {
		t.Errorf("frma not decoded: %+v", wave.Frma)
	}
	if wave.Esds == nil || wave.Esds.DecConfigDescriptor == nil {
		t.Error("esds not decoded inside wave")
	}
	if len(wave.Children) != 4 {
		t.Errorf("got %d children, wanted 4", len(wave.Children))
	}
}

func TestWaveBoxIsContainer(t *testing.T) {
	wave := &mp4.WaveBox{}
	var container mp4.ContainerBox = wave // wave must satisfy the container interface
	frma := &mp4.FrmaBox{DataFormat: "mp4a"}
	wave.AddChild(frma)
	children := container.GetChildren()
	if len(children) != 1 || children[0] != frma {
		t.Errorf("GetChildren gave %v, wanted the single added frma", children)
	}
}

func TestWaveBoxTerminatorInInfo(t *testing.T) {
	terminator := waveChildBytes(string([]byte{0, 0, 0, 0}), nil)
	box, err := mp4.DecodeBox(0, bytes.NewReader(waveBytes(terminator)))
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := box.Info(&buf, "all:1", "", "  "); err != nil {
		t.Fatal(err)
	}
	info := buf.String()
	if strings.ContainsRune(info, 0) {
		t.Errorf("info output has a raw NUL byte in a box type: %q", info)
	}
	if !strings.Contains(info, `[\x00\x00\x00\x00]`) {
		t.Errorf("terminator box type not escaped in info output: %q", info)
	}
}

func TestWaveBoxMalformedEsds(t *testing.T) {
	badEsds := waveChildBytes("esds", []byte{0, 0}) // truncated esds body
	data := waveBytes(badEsds)
	cmpAfterDecodeEncodeBox(t, data)

	box, err := mp4.DecodeBox(0, bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	wave := box.(*mp4.WaveBox)
	if wave.Esds != nil {
		t.Error("malformed esds must not become a typed reference")
	}
	if len(wave.Children) != 1 {
		t.Errorf("got %d children, wanted 1 preserved unknown child", len(wave.Children))
	}
}

func TestWaveBoxMalformedTail(t *testing.T) {
	esds := encodedEsdsBytes(t)
	sizeZeroTerminator := make([]byte, 8) // size 0, type 0: not a well-formed box

	data := waveBytes(esds, sizeZeroTerminator)
	cmpAfterDecodeEncodeBox(t, data)

	box, err := mp4.DecodeBox(0, bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	wave := box.(*mp4.WaveBox)
	if wave.Esds == nil {
		t.Error("esds not decoded before the malformed tail")
	}
	if len(wave.RawTail) != 8 {
		t.Errorf("got %d raw tail bytes, wanted 8", len(wave.RawTail))
	}
}

func TestWaveBoxRoundTrip(t *testing.T) {
	wave := &mp4.WaveBox{}
	wave.AddChild(&mp4.FrmaBox{DataFormat: "mp4a"})
	wave.AddChild(mp4.CreateEsdsBox([]byte{0x11, 0x90}))
	wave.AddChild(mp4.CreateUnknownBox(string([]byte{0, 0, 0, 0}), 8, nil))
	boxDiffAfterEncodeAndDecode(t, wave)
}

func TestWaveWrappedEsdsInAudioSampleEntry(t *testing.T) {
	esds := encodedEsdsBytes(t)
	frma := waveChildBytes("frma", []byte("mp4a"))
	terminator := waveChildBytes(string([]byte{0, 0, 0, 0}), nil)
	v1Extension := make([]byte, 16)
	binary.BigEndian.PutUint32(v1Extension, 1024) // samples per packet

	data := quickTimeSoundDescriptionBytes("mp4a", 1, v1Extension, waveBytes(frma, esds, terminator))
	cmpAfterDecodeEncodeBox(t, data)

	box, err := mp4.DecodeBox(0, bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	entry := box.(*mp4.AudioSampleEntryBox)
	if entry.Wave == nil || entry.Wave.Esds == nil {
		t.Fatal("wave-wrapped esds not reachable from the audio sample entry")
	}
	if entry.Esds != nil {
		t.Error("wave-wrapped esds should not be a direct esds reference")
	}
}
