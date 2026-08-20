package mp4_test

import (
	"bytes"
	"encoding/binary"
	"math"
	"strings"
	"testing"

	"github.com/Eyevinn/mp4ff/aac"
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

func TestLegacyQuickTimeNamesFallBackToUnknown(t *testing.T) {
	// Truncated legacy entries decoded as UnknownBox before the names were
	// registered; they must keep doing so instead of failing the file.
	data := quickTimeSoundDescriptionBytes(".mp3", 0, nil, nil)[:20]
	binary.BigEndian.PutUint32(data[:4], uint32(len(data)))
	cmpAfterDecodeEncodeBox(t, data)
	box, err := mp4.DecodeBox(0, bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if _, isUnknown := box.(*mp4.UnknownBox); !isUnknown {
		t.Errorf("truncated .mp3 entry decoded as %T, wanted UnknownBox", box)
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

func TestQuickTimeAudioSampleEntryNames(t *testing.T) {
	for _, name := range []string{".mp3", "lpcm", "twos", "sowt"} {
		t.Run(name, func(t *testing.T) {
			data := quickTimeSoundDescriptionBytes(name, 0, nil, nil)
			cmpAfterDecodeEncodeBox(t, data)
			box, err := mp4.DecodeBox(0, bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			entry, ok := box.(*mp4.AudioSampleEntryBox)
			if !ok {
				t.Fatalf("%s decoded as %T, wanted AudioSampleEntryBox", name, box)
			}
			if entry.ChannelCount != 2 || entry.SampleSize != 16 || entry.SampleRate != 48000 {
				t.Errorf("fixed fields not decoded: %+v", entry)
			}
			stsd := mp4.NewStsdBox()
			stsd.AddChild(entry)
			switch name {
			case ".mp3":
				if stsd.Mp3 != entry {
					t.Error("stsd.Mp3 not set")
				}
			default:
				if stsd.QtPcm != entry {
					t.Error("stsd.QtPcm not set")
				}
			}
		})
	}

	// lpcm entries carry a version 2 sound description in practice.
	t.Run("lpcm version 2", func(t *testing.T) {
		v2Extension := make([]byte, 0, 36)
		v2Extension = binary.BigEndian.AppendUint32(v2Extension, 72)
		v2Extension = binary.BigEndian.AppendUint64(v2Extension, math.Float64bits(44100))
		for _, val := range []uint32{2, mp4.QuickTimeV2Marker, 16, 0, 4, 1} {
			v2Extension = binary.BigEndian.AppendUint32(v2Extension, val)
		}
		data := quickTimeSoundDescriptionBytes("lpcm", 2, v2Extension, nil)
		cmpAfterDecodeEncodeBox(t, data)
		box, err := mp4.DecodeBox(0, bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		entry := box.(*mp4.AudioSampleEntryBox)
		q := entry.QuickTimeV2
		if q == nil || q.AudioSampleRate != 44100 || q.NumAudioChannels != 2 || q.ConstBitsPerChannel != 16 {
			t.Errorf("quickTimeV2 fields not decoded: %+v", q)
		}
	})
}

func TestNormalizeQuickTime(t *testing.T) {
	esds := encodedEsdsBytes(t) // AAC-LC, 48000 Hz, 2 channels
	frma := waveChildBytes("frma", []byte("mp4a"))
	mp4aStub := waveChildBytes("mp4a", make([]byte, 4)) // 12-byte QuickTime stub
	terminator := waveChildBytes(string([]byte{0, 0, 0, 0}), nil)
	wave := waveBytes(frma, mp4aStub, esds, terminator)

	decodeEntry := func(t *testing.T, data []byte) *mp4.AudioSampleEntryBox {
		t.Helper()
		box, err := mp4.DecodeBox(0, bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		return box.(*mp4.AudioSampleEntryBox)
	}
	encodedBytes := func(t *testing.T, box mp4.Box) []byte {
		t.Helper()
		var buf bytes.Buffer
		if err := box.Encode(&buf); err != nil {
			t.Fatal(err)
		}
		return buf.Bytes()
	}
	v2Extension := func(sampleRate float64, channels, bitsPerChannel uint32) []byte {
		ext := make([]byte, 0, 36)
		ext = binary.BigEndian.AppendUint32(ext, 72)
		ext = binary.BigEndian.AppendUint64(ext, math.Float64bits(sampleRate))
		for _, val := range []uint32{channels, mp4.QuickTimeV2Marker, bitsPerChannel, 0, 0, 1024} {
			ext = binary.BigEndian.AppendUint32(ext, val)
		}
		return ext
	}

	t.Run("version 1 with wave-wrapped esds matches a fresh ISO entry", func(t *testing.T) {
		entry := decodeEntry(t, quickTimeSoundDescriptionBytes("mp4a", 1, make([]byte, 16), wave))
		if !entry.NormalizeQuickTime() {
			t.Fatal("entry not normalized")
		}
		if entry.QuickTimeVersion != 0 || entry.QuickTimeRevisionLevel != 0 || entry.QuickTimeVendor != 0 ||
			entry.QuickTimeV1 != nil || entry.Wave != nil {
			t.Errorf("QuickTime shape left after normalization: %+v", entry)
		}
		want := mp4.CreateAudioSampleEntryBox("mp4a", 2, 16, 48000, mp4.CreateEsdsBox([]byte{0x11, 0x90}))
		if !bytes.Equal(encodedBytes(t, entry), encodedBytes(t, want)) {
			t.Error("normalized entry does not match the freshly created ISO entry")
		}
	})

	t.Run("version 2 restores rate and channels", func(t *testing.T) {
		entry := decodeEntry(t, quickTimeSoundDescriptionBytes("mp4a", 2, v2Extension(48000, 6, 0), wave))
		if !entry.NormalizeQuickTime() {
			t.Fatal("entry not normalized")
		}
		if entry.ChannelCount != 6 || entry.SampleRate != 48000 || entry.SampleSize != 16 {
			t.Errorf("version 2 fields not restored: channels %d, rate %d, size %d",
				entry.ChannelCount, entry.SampleRate, entry.SampleSize)
		}
		if entry.Esds == nil {
			t.Error("esds not hoisted out of wave")
		}
	})

	t.Run("version 2 keeps the declared bits per channel", func(t *testing.T) {
		entry := decodeEntry(t, quickTimeSoundDescriptionBytes("mp4a", 2, v2Extension(48000, 2, 24), wave))
		if !entry.NormalizeQuickTime() {
			t.Fatal("entry not normalized")
		}
		if entry.SampleSize != 24 {
			t.Errorf("got sample size %d, wanted the declared 24 bits per channel", entry.SampleSize)
		}
	})

	t.Run("version 2 fractional rate rounds to nearest", func(t *testing.T) {
		entry := decodeEntry(t, quickTimeSoundDescriptionBytes("mp4a", 2, v2Extension(44099.9999, 2, 0), wave))
		if !entry.NormalizeQuickTime() {
			t.Fatal("entry not normalized")
		}
		if entry.SampleRate != 44100 {
			t.Errorf("got sample rate %d, wanted the rounded 44100", entry.SampleRate)
		}
	})

	t.Run("rate above 65535 Hz leaves the fixed field 0", func(t *testing.T) {
		esds96k := encodedBytes(t, mp4.CreateEsdsBox([]byte{0x10, 0x10})) // AAC-LC, 96000 Hz, 2 channels
		wave96k := waveBytes(frma, esds96k, terminator)
		entry := decodeEntry(t, quickTimeSoundDescriptionBytes("mp4a", 2, v2Extension(96000, 2, 0), wave96k))
		if !entry.NormalizeQuickTime() {
			t.Fatal("entry not normalized")
		}
		if entry.SampleRate != 0 || entry.ChannelCount != 2 {
			t.Errorf("got rate %d and channels %d, wanted the unrepresentable rate left 0",
				entry.SampleRate, entry.ChannelCount)
		}
		hdr, err := aac.DecodeAudioSpecificConfigHeader(
			bytes.NewReader(entry.Esds.DecConfigDescriptor.DecSpecificInfo.DecConfig))
		if err != nil {
			t.Fatal(err)
		}
		if hdr.SamplingFrequency != 96000 {
			t.Errorf("the hoisted esds carries %d Hz, wanted the authoritative 96000", hdr.SamplingFrequency)
		}
	})

	t.Run("direct esds beats a wave-wrapped esds", func(t *testing.T) {
		directASC := []byte{0x11, 0x90}
		waveEsds := waveBytes(frma, encodedBytes(t, mp4.CreateEsdsBox([]byte{0x10, 0x10})), terminator)
		children := append(encodedBytes(t, mp4.CreateEsdsBox(directASC)), waveEsds...)
		entry := decodeEntry(t, quickTimeSoundDescriptionBytes("mp4a", 1, make([]byte, 16), children))
		if !entry.NormalizeQuickTime() {
			t.Fatal("entry not normalized")
		}
		if len(entry.Children) != 1 {
			t.Errorf("got %d children, wanted only the kept esds", len(entry.Children))
		}
		if got := entry.Esds.DecConfigDescriptor.DecSpecificInfo.DecConfig; !bytes.Equal(got, directASC) {
			t.Errorf("got decoder config %x, wanted the direct esds to win over the wave-wrapped one", got)
		}
	})

	t.Run("version 1 without any esds is left alone", func(t *testing.T) {
		entry := decodeEntry(t, quickTimeSoundDescriptionBytes("mp4a", 1, make([]byte, 16), nil))
		if entry.NormalizeQuickTime() {
			t.Error("entry without an esds must not be normalized")
		}
		if entry.QuickTimeV1 == nil {
			t.Error("version 1 fields must be preserved")
		}
	})

	t.Run("wave without esds is left alone", func(t *testing.T) {
		entry := decodeEntry(t, quickTimeSoundDescriptionBytes("mp4a", 0, nil, waveBytes(frma, terminator)))
		if entry.NormalizeQuickTime() {
			t.Error("entry without a reachable esds must not be normalized")
		}
		if entry.Wave == nil {
			t.Error("wave box must be preserved")
		}
	})

	t.Run("wave with an unrecognized child is left alone", func(t *testing.T) {
		chanChild := waveChildBytes("chan", make([]byte, 12))
		data := quickTimeSoundDescriptionBytes("mp4a", 0, nil, waveBytes(frma, esds, chanChild, terminator))
		entry := decodeEntry(t, data)
		if entry.NormalizeQuickTime() {
			t.Error("wave content beyond the decoder-init atoms must not be dropped")
		}
		if !bytes.Equal(encodedBytes(t, entry), data) {
			t.Error("refused entry must stay byte-identical to its input")
		}
	})

	t.Run("wave with a nonzero RawTail is left alone", func(t *testing.T) {
		badTail := []byte{0, 0, 0, 1, 'j', 'u', 'n', 'k'} // size 1: malformed, nonzero tail
		data := quickTimeSoundDescriptionBytes("mp4a", 0, nil, waveBytes(frma, esds, badTail))
		entry := decodeEntry(t, data)
		if entry.Wave == nil || len(entry.Wave.RawTail) == 0 {
			t.Fatal("test setup: wave must decode with a nonzero RawTail")
		}
		if entry.NormalizeQuickTime() {
			t.Error("a nonzero wave RawTail must not be dropped")
		}
		if !bytes.Equal(encodedBytes(t, entry), data) {
			t.Error("refused entry must stay byte-identical to its input")
		}
	})

	t.Run("wave with an oversized same-name child is left alone", func(t *testing.T) {
		bigStub := waveChildBytes("mp4a", make([]byte, 24)) // 32 bytes: not the 12-byte stub
		data := quickTimeSoundDescriptionBytes("mp4a", 0, nil, waveBytes(frma, esds, bigStub, terminator))
		entry := decodeEntry(t, data)
		if entry.NormalizeQuickTime() {
			t.Error("a same-name wave child beyond the 12-byte stub must not be dropped")
		}
		if !bytes.Equal(encodedBytes(t, entry), data) {
			t.Error("refused entry must stay byte-identical to its input")
		}
	})

	t.Run("version 2 hostile rate leaves the fixed field 0", func(t *testing.T) {
		for _, rate := range []float64{math.NaN(), math.Inf(1), -44100} {
			entry := decodeEntry(t, quickTimeSoundDescriptionBytes("mp4a", 2, v2Extension(rate, 2, 0), wave))
			if !entry.NormalizeQuickTime() {
				t.Fatal("entry not normalized")
			}
			if entry.SampleRate != 0 {
				t.Errorf("rate %v: got sample rate %d, wanted 0", rate, entry.SampleRate)
			}
		}
	})

	t.Run("wave with a size-zero terminator tail still normalizes", func(t *testing.T) {
		sizeZeroTerminator := make([]byte, 8) // size 0, type 0: all-zero RawTail
		data := quickTimeSoundDescriptionBytes("mp4a", 0, nil, waveBytes(frma, esds, sizeZeroTerminator))
		entry := decodeEntry(t, data)
		if !entry.NormalizeQuickTime() {
			t.Fatal("entry not normalized")
		}
		if entry.Wave != nil || entry.Esds == nil {
			t.Error("esds not hoisted out of wave")
		}
	})

	t.Run("version 0 with QuickTime residue is cleaned", func(t *testing.T) {
		// The helper stamps revision level 1 and vendor "appl" into an
		// otherwise plain version 0 entry.
		entry := decodeEntry(t, quickTimeSoundDescriptionBytes("mp4a", 0, nil, esds))
		if !entry.NormalizeQuickTime() {
			t.Fatal("entry with QuickTime residue not normalized")
		}
		want := mp4.CreateAudioSampleEntryBox("mp4a", 2, 16, 48000, mp4.CreateEsdsBox([]byte{0x11, 0x90}))
		if !bytes.Equal(encodedBytes(t, entry), encodedBytes(t, want)) {
			t.Error("cleaned entry does not match the freshly created ISO entry")
		}
	})

	t.Run("ISO entry is a no-op", func(t *testing.T) {
		entry := mp4.CreateAudioSampleEntryBox("mp4a", 2, 16, 48000, mp4.CreateEsdsBox([]byte{0x11, 0x90}))
		if entry.NormalizeQuickTime() {
			t.Error("clean ISO version 0 entry must not change")
		}
	})
}
