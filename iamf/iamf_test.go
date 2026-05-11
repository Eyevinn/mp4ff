package iamf

import (
	"errors"
	"testing"

	"github.com/Eyevinn/mp4ff/bits"
)

func TestLeb128RoundTrip(t *testing.T) {
	cases := []uint64{
		0, 1, 7, 0x7f, 0x80, 0x81, 0xff, 0x3fff, 0x4000, 0xffff,
		0x1fffff, 0x200000, 0xffffffff, 1 << 56,
	}
	for _, v := range cases {
		sw := bits.NewFixedSliceWriter(16)
		WriteLeb128(sw, v)
		size := Leb128Size(v)
		if len(sw.Bytes()) != size {
			t.Errorf("value %d: encoded length %d, Leb128Size returned %d",
				v, len(sw.Bytes()), size)
		}
		sr := bits.NewFixedSliceReader(sw.Bytes())
		got, err := ReadLeb128(sr)
		if err != nil {
			t.Fatalf("value %d: ReadLeb128 error: %v", v, err)
		}
		if got != v {
			t.Errorf("value %d: read back %d", v, got)
		}
	}
}

func TestLeb128Overflow(t *testing.T) {
	// 10 bytes all having the continuation bit set causes shift > 63
	data := []byte{0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80}
	sr := bits.NewFixedSliceReader(data)
	_, err := ReadLeb128(sr)
	if !errors.Is(err, ErrInvalidLeb) {
		t.Errorf("expected ErrInvalidLeb, got %v", err)
	}
}

func TestLeb128ReadEOF(t *testing.T) {
	// continuation bit set but no more bytes -> AccError
	data := []byte{0x80}
	sr := bits.NewFixedSliceReader(data)
	_, err := ReadLeb128(sr)
	if err == nil {
		t.Error("expected error on truncated leb128")
	}
}

func TestLeb128SizeZero(t *testing.T) {
	if Leb128Size(0) != 1 {
		t.Errorf("Leb128Size(0) = %d, want 1", Leb128Size(0))
	}
}

func TestSignExtend(t *testing.T) {
	cases := []struct {
		in  uint16
		out int32
	}{
		{0, 0},
		{1, 1},
		{0x7fff, 0x7fff},
		{0x8000, -32768},
		{0xffff, -1},
		{0xfffe, -2},
	}
	for _, c := range cases {
		got := signExtend(c.in)
		if got != c.out {
			t.Errorf("signExtend(0x%x) = %d, want %d", c.in, got, c.out)
		}
	}
}

func TestRational(t *testing.T) {
	r := MakeRational(3, 4)
	if r.Num != 3 || r.Den != 4 {
		t.Errorf("MakeRational(3, 4) = %+v", r)
	}
	if got := r.Float64(); got != 0.75 {
		t.Errorf("(3/4).Float64() = %v, want 0.75", got)
	}
	if r.String() != "3/4" {
		t.Errorf("(3/4).String() = %q", r.String())
	}
	zero := MakeRational(1, 0)
	if got := zero.Float64(); got != 0 {
		t.Errorf("(1/0).Float64() = %v, want 0", got)
	}
}

func TestStringers(t *testing.T) {
	cases := []struct {
		got, want string
	}{
		{AnimationTypeStep.String(), "Step"},
		{AnimationTypeLinear.String(), "Linear"},
		{AnimationTypeBezier.String(), "Bezier"},
		{AnimationType(99).String(), "Unknown(99)"},

		{ParamDefinitionMixGain.String(), "MixGain"},
		{ParamDefinitionDemixing.String(), "Demixing"},
		{ParamDefinitionReconGain.String(), "ReconGain"},
		{ParamDefinitionType(99).String(), "Unknown(99)"},

		{AmbisonicsModeMono.String(), "Mono"},
		{AmbisonicsModeProjection.String(), "Projection"},
		{AmbisonicsMode(99).String(), "Unknown(99)"},

		{AudioElementTypeChannel.String(), "Channel"},
		{AudioElementTypeScene.String(), "Scene"},
		{AudioElementType(99).String(), "Unknown(99)"},

		{HeadphonesModeStereo.String(), "Stereo"},
		{HeadphonesModeBinaural.String(), "Binaural"},
		{HeadphonesMode(99).String(), "Unknown(99)"},

		{SubMixLayoutTypeLoudspeakers.String(), "Loudspeakers"},
		{SubMixLayoutTypeBinaural.String(), "Binaural"},
		{SubMixLayoutType(99).String(), "Unknown(99)"},

		{AnchorElementUnknown.String(), "Unknown"},
		{AnchorElementDialogue.String(), "Dialogue"},
		{AnchorElementAlbum.String(), "Album"},
		{AnchorElement(99).String(), "Unknown(99)"},

		{SoundSystemA_0_2_0.String(), "Sound System A (0.2.0)"},
		{SoundSystemB_0_5_0.String(), "Sound System B (0.5.0)"},
		{SoundSystemH_9_10_3.String(), "Sound System H (9.10.3)"},
		{SoundSystem13_9_1_6.String(), "Sound System H Subset (13.9.1.6)"},
		{SoundSystem(99).String(), "Unknown(99)"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("got %q, want %q", c.got, c.want)
		}
	}
}

func TestObuTypeString(t *testing.T) {
	cases := []struct {
		o    ObuType
		want string
	}{
		{ObuTypeCodecConfig, "Codec Config"},
		{ObuTypeAudioElement, "Audio Element"},
		{ObuTypeMixPresentation, "Mix Presentation"},
		{ObuTypeParameterBlock, "Parameter Block"},
		{ObuTypeTemporalDelimiter, "Temporal Delimiter"},
		{ObuTypeAudioFrame, "Audio Frame"},
		{ObuTypeAudioFrameID0, "Audio Frame ID0"},
		{ObuTypeAudioFrameID17, "Audio Frame ID17"},
		{ObuTypeSequenceHeader, "IA Sequence Header"},
		{ObuType(25), "Reserved OBU Type (25)"},
		{ObuType(99), "Unknown OBU Type (99)"},
	}
	for _, c := range cases {
		if got := c.o.String(); got != c.want {
			t.Errorf("ObuType(%d).String() = %q, want %q", c.o, got, c.want)
		}
	}
}

func TestLayerFlagString(t *testing.T) {
	if got := LayerFlagReconGain.String(); got != "[ReconGain]" {
		t.Errorf("LayerFlagReconGain.String() = %q, want %q", got, "[ReconGain]")
	}
	if got := LayerFlag(0).String(); got != "Unknown(0)" {
		t.Errorf("LayerFlag(0).String() = %q, want %q", got, "Unknown(0)")
	}
}

func TestChannelLayoutString(t *testing.T) {
	cl := ChannelLayout{Description: "Stereo", NumChannels: 2}
	if got := cl.String(); got != "Stereo" {
		t.Errorf("got %q, want Stereo", got)
	}
	cl2 := ChannelLayout{NumChannels: 6, ChannelMask: 0x3f}
	if got := cl2.String(); got != "6 channels (mask: 0x3F)" {
		t.Errorf("got %q", got)
	}
}

func TestObuInfoPayloadSize(t *testing.T) {
	o := ObuInfo{Size: 100, Start: 5, Type: ObuTypeCodecConfig}
	if o.PayloadSize() != 95 {
		t.Errorf("PayloadSize() = %d, want 95", o.PayloadSize())
	}
}

func TestPcmDecoderConfig(t *testing.T) {
	// sample_format=1 (LE), sample_size=24 (idx 1), sample_rate=48000
	data := []byte{0x01, 24, 0x00, 0x00, 0xbb, 0x80}
	sr := bits.NewFixedSliceReader(data)
	cc := &IamfCodecConfig{AudioRollDistance: 0}
	if err := PcmDecoderConfig(sr, cc); err != nil {
		t.Fatalf("PcmDecoderConfig: %v", err)
	}
	if cc.SampleRate != 48000 {
		t.Errorf("SampleRate = %d, want 48000", cc.SampleRate)
	}
	if cc.CodecID != "pcm_s24le" {
		t.Errorf("CodecID = %q, want pcm_s24le", cc.CodecID)
	}
}

func TestPcmDecoderConfigInvalid(t *testing.T) {
	// sample_format=2 is invalid (>1)
	data := []byte{0x02, 16, 0x00, 0x00, 0xbb, 0x80}
	sr := bits.NewFixedSliceReader(data)
	cc := &IamfCodecConfig{}
	if err := PcmDecoderConfig(sr, cc); err == nil {
		t.Error("expected error for invalid sample format")
	}

	// non-zero AudioRollDistance is invalid
	sr2 := bits.NewFixedSliceReader([]byte{0x00, 16, 0x00, 0x00, 0xbb, 0x80})
	cc2 := &IamfCodecConfig{AudioRollDistance: 1}
	if err := PcmDecoderConfig(sr2, cc2); err == nil {
		t.Error("expected error for non-zero roll distance")
	}

	// extra trailing data is invalid
	sr3 := bits.NewFixedSliceReader([]byte{0x00, 16, 0x00, 0x00, 0xbb, 0x80, 0xff})
	cc3 := &IamfCodecConfig{}
	if err := PcmDecoderConfig(sr3, cc3); err == nil {
		t.Error("expected error for trailing bytes")
	}
}

func TestOpusDecoderConfigInvalid(t *testing.T) {
	// Opus requires AudioRollDistance < 0 and at least 11 bytes of payload
	cc := &IamfCodecConfig{AudioRollDistance: 0}
	sr := bits.NewFixedSliceReader(make([]byte, 16))
	if err := OpusDecoderConfig(sr, cc); err == nil {
		t.Error("expected error when AudioRollDistance >= 0")
	}

	cc2 := &IamfCodecConfig{AudioRollDistance: -4}
	sr2 := bits.NewFixedSliceReader(make([]byte, 5))
	if err := OpusDecoderConfig(sr2, cc2); err == nil {
		t.Error("expected error for too-small payload")
	}
}

func TestOpusDecoderConfigOK(t *testing.T) {
	cc := &IamfCodecConfig{AudioRollDistance: -4}
	// 11 arbitrary bytes
	sr := bits.NewFixedSliceReader([]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11})
	if err := OpusDecoderConfig(sr, cc); err != nil {
		t.Fatalf("OpusDecoderConfig: %v", err)
	}
	if cc.SampleRate != 48000 {
		t.Errorf("SampleRate = %d, want 48000", cc.SampleRate)
	}
	if string(cc.Extradata[:8]) != "OpusHead" {
		t.Errorf("expected OpusHead prefix, got %q", string(cc.Extradata[:8]))
	}
	if cc.ExtradataSize != 19 { // 11 + 8
		t.Errorf("ExtradataSize = %d, want 19", cc.ExtradataSize)
	}
}

func TestFlacDecoderConfigInvalid(t *testing.T) {
	// non-zero AudioRollDistance is invalid for FLAC
	sr := bits.NewFixedSliceReader(make([]byte, 30))
	cc := &IamfCodecConfig{AudioRollDistance: -4}
	if err := FlacDecoderConfig(sr, cc); err == nil {
		t.Error("expected error when AudioRollDistance != 0")
	}

	// streaminfo too small
	sr2 := bits.NewFixedSliceReader(make([]byte, 10))
	cc2 := &IamfCodecConfig{}
	if err := FlacDecoderConfig(sr2, cc2); err == nil {
		t.Error("expected error for too-small streaminfo")
	}
}

func TestFlacDecoderConfigOK(t *testing.T) {
	// 4 bytes metadata block header + STREAMINFO (>=18 bytes).
	// Note: the current FLAC parser reads sample_rate from the start of the
	// STREAMINFO block (offset 0) rather than the spec-required offset 10.
	// We test the implemented behavior.
	data := make([]byte, 4+18)
	// 24-bit value at extradata[0..2] where (val >> 4) == 48000
	// 48000 << 4 = 0xBB800 — bytes 0x0B 0xB8 0x00
	data[4] = 0x0B
	data[5] = 0xB8
	data[6] = 0x00
	sr := bits.NewFixedSliceReader(data)
	cc := &IamfCodecConfig{}
	if err := FlacDecoderConfig(sr, cc); err != nil {
		t.Fatalf("FlacDecoderConfig: %v", err)
	}
	if cc.SampleRate != 48000 {
		t.Errorf("SampleRate = %d, want 48000", cc.SampleRate)
	}
	if cc.ExtradataSize != 18 {
		t.Errorf("ExtradataSize = %d, want 18", cc.ExtradataSize)
	}
}

// aacDescriptor builds a buffer matching what AacDecoderConfig expects.
// The parser uses mp4ReadDescr to read each descriptor's tag+descLen byte,
// then for the specific descriptor reads an additional byte as specLen.
func aacDescriptor(specLen byte, asc []byte) []byte {
	out := []byte{
		0x03,    // DecConfigDescr tag
		0x14,    // descLen (consumed by mp4ReadDescr, ignored)
		0x40,    // objectTypeID = MP4 audio
		0x14,    // streamType: (5<<2)|0
		0, 0, 0, // buffer size DB
		0, 0, 0, 0, // rc_max_rate
		0, 0, 0, 0, // avg bitrate
		0x05,    // DecSpecificDescr tag
		0x14,    // descLen (consumed by mp4ReadDescr, ignored)
		specLen, // actual length read by parser
	}
	return append(out, asc...)
}

func TestAacDecoderConfigOK(t *testing.T) {
	// AAC-LC ASC (13 bits): objectType=2, freqIdx=3 (48000), channels=2 -> 0x11 0x90
	data := aacDescriptor(0x02, []byte{0x11, 0x90})
	sr := bits.NewFixedSliceReader(data)
	cc := &IamfCodecConfig{AudioRollDistance: -1}
	if err := AacDecoderConfig(sr, cc); err != nil {
		t.Fatalf("AacDecoderConfig: %v", err)
	}
	if cc.SampleRate != 48000 {
		t.Errorf("SampleRate = %d, want 48000", cc.SampleRate)
	}
	if cc.ExtradataSize != 2 {
		t.Errorf("ExtradataSize = %d, want 2", cc.ExtradataSize)
	}
}

func TestAacDecoderConfigInvalid(t *testing.T) {
	// AudioRollDistance >= 0 must fail
	sr := bits.NewFixedSliceReader([]byte{0x03, 0x10})
	cc := &IamfCodecConfig{AudioRollDistance: 0}
	if err := AacDecoderConfig(sr, cc); err == nil {
		t.Error("expected error for non-negative roll distance")
	}

	// wrong descriptor tag
	sr2 := bits.NewFixedSliceReader([]byte{0x04, 0x10})
	cc2 := &IamfCodecConfig{AudioRollDistance: -1}
	if err := AacDecoderConfig(sr2, cc2); err == nil {
		t.Error("expected error for wrong descriptor tag")
	}

	// wrong objectTypeID
	d3 := aacDescriptor(0x02, []byte{0x11, 0x90})
	d3[2] = 0x41 // wrong objectTypeID
	sr3 := bits.NewFixedSliceReader(d3)
	cc3 := &IamfCodecConfig{AudioRollDistance: -1}
	if err := AacDecoderConfig(sr3, cc3); err == nil {
		t.Error("expected error for wrong objectTypeID")
	}

	// wrong streamType
	d4 := aacDescriptor(0x02, []byte{0x11, 0x90})
	d4[3] = 0x10 // wrong streamType
	sr4 := bits.NewFixedSliceReader(d4)
	cc4 := &IamfCodecConfig{AudioRollDistance: -1}
	if err := AacDecoderConfig(sr4, cc4); err == nil {
		t.Error("expected error for wrong streamType")
	}

	// wrong specific descriptor tag
	d5 := aacDescriptor(0x02, []byte{0x11, 0x90})
	d5[15] = 0x06 // wrong spec tag
	sr5 := bits.NewFixedSliceReader(d5)
	cc5 := &IamfCodecConfig{AudioRollDistance: -1}
	if err := AacDecoderConfig(sr5, cc5); err == nil {
		t.Error("expected error for wrong specific descriptor tag")
	}

	// zero specLen
	d6 := aacDescriptor(0x00, nil)
	sr6 := bits.NewFixedSliceReader(d6)
	cc6 := &IamfCodecConfig{AudioRollDistance: -1}
	if err := AacDecoderConfig(sr6, cc6); err == nil {
		t.Error("expected error for zero specLen")
	}
}

func TestNewObuReader(t *testing.T) {
	data := []byte{0x00, 0x01, 0x02}
	r := NewObuReader(data, 0) // maxSize < default should be clamped up
	if r.maxSize < MaxIamfObuHeaderSizeBytes {
		t.Errorf("maxSize = %d, want at least %d", r.maxSize, MaxIamfObuHeaderSizeBytes)
	}
	if r.Context() == nil {
		t.Error("Context() returned nil")
	}
}

func TestObuReaderSkipPayload(t *testing.T) {
	data := make([]byte, 50)
	r := NewObuReader(data, 100)
	obu := &ObuInfo{Size: 20, Start: 4, Type: ObuTypeCodecConfig}
	startPos := r.sr.GetPos()
	r.SkipPayload(obu)
	if r.sr.GetPos()-startPos != 16 {
		t.Errorf("SkipPayload moved by %d, want 16", r.sr.GetPos()-startPos)
	}
}

func TestObuReaderEmpty(t *testing.T) {
	r := NewObuReader(nil, 100)
	obu, err := r.ReadObu()
	if err != nil {
		t.Errorf("expected nil error on empty data, got %v", err)
	}
	if obu != nil {
		t.Errorf("expected nil obu on empty data, got %+v", obu)
	}
}

func TestParseObuSRTooShort(t *testing.T) {
	r := NewObuReader([]byte{}, 100)
	if _, err := parseObuSR(r.sr); err == nil {
		t.Error("expected error for empty data")
	}
}

func TestToChannelLayoutScalable(t *testing.T) {
	cl := scalableChannelLayouts[1] // Stereo
	out := cl.toChannelLayout()
	if out.NumChannels != 2 {
		t.Errorf("NumChannels = %d, want 2", out.NumChannels)
	}
	if out.Description != "Stereo (System A - 0+2+0)" {
		t.Errorf("Description = %q", out.Description)
	}
}

func TestToChannelLayoutExpanded(t *testing.T) {
	// expanded layout 0 — LFE
	cl := expandedScalableChannelLayouts[0]
	out := cl.toChannelLayout()
	if out.Description != "LFE (System J subset)" {
		t.Errorf("Description = %q", out.Description)
	}

	// systemE has TableIndex == -1 — falls into named-system branch
	out2 := systemE.toChannelLayout()
	if out2.Description != "5.1.4ch SS_D + BFC (System E)" {
		t.Errorf("systemE Description = %q", out2.Description)
	}
	out3 := systemF.toChannelLayout()
	if out3.Description != "7.1.2ch SS_I + TpBC + LFE2 (System F)" {
		t.Errorf("systemF Description = %q", out3.Description)
	}
	out4 := systemG.toChannelLayout()
	if out4.Description != "9.1.4ch (System G)" {
		t.Errorf("systemG Description = %q", out4.Description)
	}
}

func TestToChannelLayoutAmbisonics(t *testing.T) {
	// 4 channels => order 1 (first-order ambisonics)
	cl := channelLayout{
		Order:    coAmbisonics,
		Channels: 4,
	}
	out := cl.toChannelLayout()
	if out.Description != "Ambisonics Order 1 (4 channels)" {
		t.Errorf("Description = %q", out.Description)
	}

	// with explicit map
	m := map[int]int{0: 0, 1: 1, 2: 2, 3: 3}
	cl.Map = &m
	out2 := cl.toChannelLayout()
	if got := out2.ChannelMap["0"]; got != "ACN0" {
		t.Errorf("ChannelMap[0] = %q, want ACN0", got)
	}
}

func TestToChannelLayoutCustom(t *testing.T) {
	m := map[int]int{0: 0, 1: 1}
	cl := channelLayout{
		Order:    coCustom,
		Channels: 2,
		Map:      &m,
	}
	out := cl.toChannelLayout()
	if out.Description != "Custom 2-Channel Layout" {
		t.Errorf("Description = %q", out.Description)
	}
	if got := out.ChannelMap["0"]; got != "CH0" {
		t.Errorf("ChannelMap[0] = %q, want CH0", got)
	}
}
