package mp4_test

import (
	"bytes"
	"encoding/binary"
	"math"
	"strings"
	"testing"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/Eyevinn/mp4ff/mp4"
)

func TestWriteReadOfAudioSampleEntry(t *testing.T) {
	ascBytes := []byte{0x11, 0x90}
	esds := mp4.CreateEsdsBox(ascBytes)
	ase := mp4.CreateAudioSampleEntryBox("mp4a", 2, 16, 48000, esds)
	boxDiffAfterEncodeAndDecode(t, ase)
	_, err := ase.RemoveEncryption()
	expectedErrMsg := "is not encrypted: mp4a"
	if err == nil || err.Error() != expectedErrMsg {
		t.Errorf("expected error with message: %q", err.Error())
	}
}

func TestQuickTimeV1AudioSampleEntry(t *testing.T) {
	ascBytes := []byte{0x11, 0x90}
	esds := mp4.CreateEsdsBox(ascBytes)
	ase := mp4.CreateAudioSampleEntryBox("mp4a", 2, 16, 48000, esds)
	ase.QuickTimeVersion = 1
	ase.QuickTimeRevisionLevel = 1
	ase.QuickTimeVendor = binary.BigEndian.Uint32([]byte("appl"))
	ase.QuickTimeV1 = &mp4.QuickTimeV1SoundDescription{
		SamplesPerPacket: 1024,
		BytesPerPacket:   4,
		BytesPerFrame:    8,
		BytesPerSample:   2,
	}
	boxDiffAfterEncodeAndDecode(t, ase)
}

func TestQuickTimeV2AudioSampleEntry(t *testing.T) {
	ascBytes := []byte{0x11, 0x90}
	esds := mp4.CreateEsdsBox(ascBytes)
	ase := mp4.CreateAudioSampleEntryBox("mp4a", 3, 16, 1, esds)
	ase.QuickTimeVersion = 2
	ase.CompressionID = -2
	ase.QuickTimeV2 = &mp4.QuickTimeV2SoundDescription{
		SizeOfStructOnly:              72,
		AudioSampleRate:               96000,
		NumAudioChannels:              2,
		Always7F000000:                mp4.QuickTimeV2Marker,
		ConstLPCMFramesPerAudioPacket: 1024,
	}
	boxDiffAfterEncodeAndDecode(t, ase)

	// Info reports the effective values from the version 2 struct, not the
	// mandated constants in the fixed fields.
	var infoBuf bytes.Buffer
	if err := ase.Info(&infoBuf, "", "", "  "); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"channel_count: 2", "sample_rate: 96000"} {
		if !strings.Contains(infoBuf.String(), want) {
			t.Errorf("Info output lacks %q:\n%s", want, infoBuf.String())
		}
	}
}

// quickTimeSoundDescriptionBytes builds an entry with the QuickTime
// version-specific extension bytes between the fixed fields and the children.
// Nonzero revision level and 'appl' vendor bytes pin their round trip.
func quickTimeSoundDescriptionBytes(name string, version uint16, extension, children []byte) []byte {
	body := make([]byte, 0, 36+len(extension)+len(children))
	body = append(body, make([]byte, 6)...) // reserved
	body = append(body, 0, 1)               // data reference index
	body = binary.BigEndian.AppendUint16(body, version)
	body = append(body, 0, 1)               // revision level
	body = append(body, []byte("appl")...)  // vendor
	body = append(body, 0, 2)               // channel count
	body = append(body, 0, 16)              // sample size
	body = append(body, make([]byte, 4)...) // compression id + packet size
	body = append(body, 0xbb, 0x80, 0, 0)   // sample rate 48000 << 16
	body = append(body, extension...)
	body = append(body, children...)
	box := make([]byte, 0, 8+len(body))
	box = binary.BigEndian.AppendUint32(box, uint32(8+len(body)))
	box = append(box, []byte(name)...)
	return append(box, body...)
}

func TestQuickTimeSoundDescriptionByteLayout(t *testing.T) {
	esds := mp4.CreateEsdsBox([]byte{0x11, 0x90})
	var childBuf bytes.Buffer
	if err := esds.Encode(&childBuf); err != nil {
		t.Fatal(err)
	}
	v1Extension := make([]byte, 0, 16)
	for _, val := range []uint32{1024, 4, 8, 2} {
		v1Extension = binary.BigEndian.AppendUint32(v1Extension, val)
	}
	v2Extension := make([]byte, 0, 36)
	v2Extension = binary.BigEndian.AppendUint32(v2Extension, 72)
	v2Extension = binary.BigEndian.AppendUint64(v2Extension, math.Float64bits(44100))
	for _, val := range []uint32{2, mp4.QuickTimeV2Marker, 0, 0, 0, 1024} {
		v2Extension = binary.BigEndian.AppendUint32(v2Extension, val)
	}
	cases := []struct {
		desc    string
		version uint16
		ext     []byte
	}{
		{"version 0", 0, nil},
		{"version 1", 1, v1Extension},
		{"version 2", 2, v2Extension},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			data := quickTimeSoundDescriptionBytes("mp4a", c.version, c.ext, childBuf.Bytes())
			cmpAfterDecodeEncodeBox(t, data)
			box, err := mp4.DecodeBox(0, bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			entry := box.(*mp4.AudioSampleEntryBox)
			if entry.QuickTimeVersion != c.version {
				t.Errorf("got version %d, wanted %d", entry.QuickTimeVersion, c.version)
			}
			if entry.QuickTimeRevisionLevel != 1 {
				t.Errorf("got revision level %d, wanted 1", entry.QuickTimeRevisionLevel)
			}
			if vendor := string(binary.BigEndian.AppendUint32(nil, entry.QuickTimeVendor)); vendor != "appl" {
				t.Errorf("got vendor %q, wanted \"appl\"", vendor)
			}
			if entry.Esds == nil {
				t.Error("esds child not found after the version extension")
			}
			if c.version == 2 {
				if entry.QuickTimeV2 == nil || entry.QuickTimeV2.AudioSampleRate != 44100 {
					t.Errorf("quickTimeV2 fields not decoded: %+v", entry.QuickTimeV2)
				}
			}
		})
	}
}

// TestQuickTimeV2MarkerRequired pins that a version 2 extension without the
// defined always7F000000 marker is not accepted as a version 2 layout, and
// that the reported error names the claimed layout's problem, on both
// decode paths.
func TestQuickTimeV2MarkerRequired(t *testing.T) {
	esds := mp4.CreateEsdsBox([]byte{0x11, 0x90})
	var childBuf bytes.Buffer
	if err := esds.Encode(&childBuf); err != nil {
		t.Fatal(err)
	}
	v2Extension := make([]byte, 0, 36)
	v2Extension = binary.BigEndian.AppendUint32(v2Extension, 72)
	v2Extension = binary.BigEndian.AppendUint64(v2Extension, math.Float64bits(44100))
	for _, val := range []uint32{2, 0 /* not the marker */, 0, 0, 0, 1024} {
		v2Extension = binary.BigEndian.AppendUint32(v2Extension, val)
	}
	data := quickTimeSoundDescriptionBytes("mp4a", 2, v2Extension, childBuf.Bytes())
	_, err := mp4.DecodeBox(0, bytes.NewReader(data))
	if err == nil || !strings.Contains(err.Error(), "always7F000000") {
		t.Errorf("wanted the missing-marker error, got %v", err)
	}
	_, err = mp4.DecodeBoxSR(0, bits.NewFixedSliceReader(data))
	if err == nil || !strings.Contains(err.Error(), "always7F000000") {
		t.Errorf("wanted the missing-marker error from the SR path, got %v", err)
	}
}

// TestTooShortAudioSampleEntry pins that a body too short for the fixed
// fields is an error on both decode paths instead of a partly garbage box.
func TestTooShortAudioSampleEntry(t *testing.T) {
	data := quickTimeSoundDescriptionBytes("mp4a", 0, nil, nil)[:20]
	binary.BigEndian.PutUint32(data, uint32(len(data))) // consistent, too small, size
	if _, err := mp4.DecodeBox(0, bytes.NewReader(data)); err == nil {
		t.Error("wanted an error for a too short audio sample entry")
	}
	if _, err := mp4.DecodeBoxSR(0, bits.NewFixedSliceReader(data)); err == nil {
		t.Error("wanted an error from the SR path for a too short audio sample entry")
	}
}

// TestQuickTimeEncodeValidation pins that encoding fails loudly when the
// version halfword and the version-specific pointers disagree, since such
// an entry could not be decoded again.
func TestQuickTimeEncodeValidation(t *testing.T) {
	for _, c := range []struct {
		desc   string
		modify func(a *mp4.AudioSampleEntryBox)
	}{
		{"v1 pointer without version 1", func(a *mp4.AudioSampleEntryBox) {
			a.QuickTimeV1 = &mp4.QuickTimeV1SoundDescription{SamplesPerPacket: 1024}
		}},
		{"v2 pointer without version 2", func(a *mp4.AudioSampleEntryBox) {
			a.QuickTimeV2 = &mp4.QuickTimeV2SoundDescription{Always7F000000: mp4.QuickTimeV2Marker}
		}},
		{"v2 pointer without the marker", func(a *mp4.AudioSampleEntryBox) {
			a.QuickTimeVersion = 2
			a.QuickTimeV2 = &mp4.QuickTimeV2SoundDescription{}
		}},
		{"both pointers set", func(a *mp4.AudioSampleEntryBox) {
			a.QuickTimeVersion = 1
			a.QuickTimeV1 = &mp4.QuickTimeV1SoundDescription{}
			a.QuickTimeV2 = &mp4.QuickTimeV2SoundDescription{Always7F000000: mp4.QuickTimeV2Marker}
		}},
	} {
		t.Run(c.desc, func(t *testing.T) {
			ase := mp4.CreateAudioSampleEntryBox("mp4a", 2, 16, 48000, mp4.CreateEsdsBox([]byte{0x11, 0x90}))
			c.modify(ase)
			if err := ase.Encode(&bytes.Buffer{}); err == nil {
				t.Error("wanted an encode error for an inconsistent QuickTime setup")
			}
			sw := bits.NewFixedSliceWriter(int(ase.Size()))
			if err := ase.EncodeSW(sw); err == nil {
				t.Error("wanted an EncodeSW error for an inconsistent QuickTime setup")
			}
		})
	}
}

// TestDirtyReservedBytesKeepDecoding pins the fallback: entries whose
// reserved bytes carry an unknown version, or claim a QuickTime version the
// layout does not match, decode with the plain ISO layout like before, and
// the halfword round-trips.
func TestDirtyReservedBytesKeepDecoding(t *testing.T) {
	esds := mp4.CreateEsdsBox([]byte{0x11, 0x90})
	var childBuf bytes.Buffer
	if err := esds.Encode(&childBuf); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		desc    string
		version uint16
	}{
		{"unknown version 3", 3},
		{"version 1 claimed but ISO layout", 1},
		{"version 2 claimed but ISO layout", 2},
	} {
		t.Run(c.desc, func(t *testing.T) {
			data := quickTimeSoundDescriptionBytes("mp4a", c.version, nil, childBuf.Bytes())
			cmpAfterDecodeEncodeBox(t, data)
			box, err := mp4.DecodeBox(0, bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			entry := box.(*mp4.AudioSampleEntryBox)
			if entry.QuickTimeVersion != c.version || entry.QuickTimeV1 != nil || entry.QuickTimeV2 != nil {
				t.Errorf("wanted ISO-layout fallback with version %d preserved, got %+v", c.version, entry)
			}
			if entry.Esds == nil {
				t.Error("esds child not found at the ISO offset")
			}
		})
	}
}

// TestQuickTimeLpcmFormatFlags pins the defined CoreAudio/QTFF linear-PCM
// format flag values and the accessors on QuickTimeV2SoundDescription.
func TestQuickTimeLpcmFormatFlags(t *testing.T) {
	definedFlags := []struct {
		name   string
		flag   uint32
		wanted uint32
	}{
		{"IsFloat", mp4.QuickTimeFormatFlagIsFloat, 0x1},
		{"IsBigEndian", mp4.QuickTimeFormatFlagIsBigEndian, 0x2},
		{"IsSignedInteger", mp4.QuickTimeFormatFlagIsSignedInteger, 0x4},
		{"IsPacked", mp4.QuickTimeFormatFlagIsPacked, 0x8},
		{"IsAlignedHigh", mp4.QuickTimeFormatFlagIsAlignedHigh, 0x10},
		{"IsNonInterleaved", mp4.QuickTimeFormatFlagIsNonInterleaved, 0x20},
		{"IsNonMixable", mp4.QuickTimeFormatFlagIsNonMixable, 0x40},
	}
	for _, d := range definedFlags {
		if d.flag != d.wanted {
			t.Errorf("QuickTimeFormatFlag%s is %#x, wanted %#x", d.name, d.flag, d.wanted)
		}
	}
	cases := []struct {
		desc            string
		flags           uint32
		isFloat         bool
		isBigEndian     bool
		isSignedInteger bool
	}{
		{"zero flags", 0, false, false, false},
		{"packed float32", mp4.QuickTimeFormatFlagIsFloat | mp4.QuickTimeFormatFlagIsPacked,
			true, false, false},
		{"big-endian signed int16", mp4.QuickTimeFormatFlagIsBigEndian | mp4.QuickTimeFormatFlagIsSignedInteger,
			false, true, true},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			q := mp4.QuickTimeV2SoundDescription{FormatSpecificFlags: c.flags}
			if q.IsFloat() != c.isFloat {
				t.Errorf("IsFloat() is %t, wanted %t", q.IsFloat(), c.isFloat)
			}
			if q.IsBigEndian() != c.isBigEndian {
				t.Errorf("IsBigEndian() is %t, wanted %t", q.IsBigEndian(), c.isBigEndian)
			}
			if q.IsSignedInteger() != c.isSignedInteger {
				t.Errorf("IsSignedInteger() is %t, wanted %t", q.IsSignedInteger(), c.isSignedInteger)
			}
		})
	}
}
