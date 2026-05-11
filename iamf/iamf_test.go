package iamf

import (
	"encoding/hex"
	"errors"
	"strings"
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
		{SoundSystemC_2_5_0.String(), "Sound System C (2.5.0)"},
		{SoundSystemD_4_5_0.String(), "Sound System D (4.5.0)"},
		{SoundSystemE_4_5_1.String(), "Sound System E (4.5.1)"},
		{SoundSystemF_3_7_0.String(), "Sound System F (3.7.0)"},
		{SoundSystemG_4_9_0.String(), "Sound System G (4.9.0)"},
		{SoundSystemH_9_10_3.String(), "Sound System H (9.10.3)"},
		{SoundSystemI_0_7_0.String(), "Sound System I (0.7.0)"},
		{SoundSystemJ_4_7_0.String(), "Sound System J (4.7.0)"},
		{SoundSystem10_2_7_0.String(), "Sound System I + Ltf + Rtf (10.2.7.0)"},
		{SoundSystem11_2_3_0.String(), "Sound System J Front Subset (11.2.3.0)"},
		{SoundSystem12_0_1_0.String(), "Mono (12.0.1.0)"},
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
	cases := []struct {
		format    byte
		bits      byte
		wantCodec string
	}{
		{0, 16, "pcm_s16be"},
		{0, 24, "pcm_s24be"},
		{0, 32, "pcm_s32be"},
		{1, 16, "pcm_s16le"},
		{1, 24, "pcm_s24le"},
		{1, 32, "pcm_s32le"},
	}
	for _, c := range cases {
		data := []byte{c.format, c.bits, 0x00, 0x00, 0xbb, 0x80}
		sr := bits.NewFixedSliceReader(data)
		cc := &IamfCodecConfig{AudioRollDistance: 0}
		if err := PcmDecoderConfig(sr, cc); err != nil {
			t.Fatalf("PcmDecoderConfig(%d,%d): %v", c.format, c.bits, err)
		}
		if cc.SampleRate != 48000 {
			t.Errorf("SampleRate = %d, want 48000", cc.SampleRate)
		}
		if cc.CodecID != c.wantCodec {
			t.Errorf("CodecID = %q, want %q", cc.CodecID, c.wantCodec)
		}
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

func TestParseObuSRWithTrimming(t *testing.T) {
	// header byte: type=1 (AudioElement), trimming=1, extension=0
	//   bits MSB-first: type[5]=00001 redundant[1]=0 trimming[1]=1 extension[1]=0
	//   => 0b00001010 = 0x0A
	// then leb128 obuSize = 5
	// then 2 leb128s for trimming counts (samples_to_trim_at_end, _at_start)
	// then 5 bytes of payload
	data := []byte{0x0A, 0x05, 0x00, 0x00, 1, 2, 3, 4, 5}
	sr := bits.NewFixedSliceReader(data)
	obu, err := parseObuSR(sr)
	if err != nil {
		t.Fatalf("parseObuSR: %v", err)
	}
	if obu.Type != ObuTypeAudioElement {
		t.Errorf("Type = %v, want AudioElement", obu.Type)
	}
}

func TestParseObuSRWithExtension(t *testing.T) {
	// header byte: type=0 (CodecConfig), trimming=0, extension=1
	//   bits: 00000 0 0 1 => 0x01
	// then leb128 obuSize = 4
	// then leb128 extensionBytes = 2
	// then 2 extension bytes (skipped)
	// then 4 payload bytes
	data := []byte{0x01, 0x04, 0x02, 0xaa, 0xbb, 1, 2, 3, 4}
	sr := bits.NewFixedSliceReader(data)
	obu, err := parseObuSR(sr)
	if err != nil {
		t.Fatalf("parseObuSR: %v", err)
	}
	if obu.Type != ObuTypeCodecConfig {
		t.Errorf("Type = %v, want CodecConfig", obu.Type)
	}
}

func TestParseObuSRNoPayload(t *testing.T) {
	// header with declared obuSize but no remaining bytes
	data := []byte{0x00, 0x05}
	sr := bits.NewFixedSliceReader(data)
	if _, err := parseObuSR(sr); err == nil {
		t.Error("expected error for missing payload")
	}
}

func TestReadObuReturnsNilForFrame(t *testing.T) {
	// Audio frame OBU types (>= ObuTypeParameterBlock=3, < ObuTypeSequenceHeader=31)
	// type=3 (ParameterBlock) -> ReadObu should return nil obu, nil err
	// byte: 00011 0 0 0 = 0x18
	data := []byte{0x18, 0x01, 0xff}
	r := NewObuReader(data, 100)
	obu, err := r.ReadObu()
	if err != nil {
		t.Fatalf("ReadObu: %v", err)
	}
	if obu != nil {
		t.Errorf("expected nil obu for non-descriptor type, got %+v", obu)
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

// opusDescriptors is the same sample IA Sequence Data used in the iacb
// integration tests in mp4/iamf_test.go: an IA Sequence Header followed by
// a Codec Config OBU (Opus), an Audio Element OBU (channel layout) and a
// Mix Presentation OBU.
const opusDescriptors = "" +
	"f80669616d6601010014004f707573c007fffc010201380000bb800000000829" +
	"ac02200010000102030405060708090a0b0c0d0e0f0000101000010203040506" +
	"0708090a0b0c0d0e0f080bad0200000110002010010110772a01656e2d757300" +
	"44656661756c74204d69782050726573656e746174696f6e000102ac02334f41" +
	"20617564696f20656c656d656e74004000e70780f702800000ad027374657265" +
	"6f20617564696f20656c656d656e74004000e30780f702800000e60780f70280" +
	"0000028000ebe5ff85c00080008000"

func TestReadDescriptorsFullStream(t *testing.T) {
	data, err := hex.DecodeString(opusDescriptors)
	if err != nil {
		t.Fatal(err)
	}
	r := NewObuReader(data, len(data))
	var seenTypes []ObuType
	for {
		obu, err := r.ReadObu()
		if err != nil {
			t.Fatalf("ReadObu: %v", err)
		}
		if obu == nil {
			break
		}
		seenTypes = append(seenTypes, obu.Type)
		if _, err := obu.ReadDescriptors(&r); err != nil {
			t.Fatalf("ReadDescriptors(%s): %v", obu.Type, err)
		}
	}
	want := []ObuType{
		ObuTypeSequenceHeader,
		ObuTypeCodecConfig,
		ObuTypeAudioElement,
		ObuTypeAudioElement,
		ObuTypeMixPresentation,
	}
	if len(seenTypes) != len(want) {
		t.Fatalf("got types %v, want %v", seenTypes, want)
	}
	for i, ty := range want {
		if seenTypes[i] != ty {
			t.Errorf("seenTypes[%d] = %v, want %v", i, seenTypes[i], ty)
		}
	}

	ctx := r.Context()
	if ctx.NumCodecConfigs != 1 {
		t.Errorf("NumCodecConfigs = %d, want 1", ctx.NumCodecConfigs)
	}
	if ctx.NumAudioElements != 2 {
		t.Errorf("NumAudioElements = %d, want 2", ctx.NumAudioElements)
	}
	if ctx.NumMixPresentations != 1 {
		t.Errorf("NumMixPresentations = %d, want 1", ctx.NumMixPresentations)
	}

	// Render Info output - exercises ctx.Info, obu.Info, channel-layout
	// description formatting and various enum String() methods.
	var sb strings.Builder
	err = ctx.Info(func(level int, format string, p ...interface{}) {
		sb.WriteString(strings.Repeat("  ", level))
		sb.WriteString(format)
		sb.WriteString("\n")
	})
	if err != nil {
		t.Fatalf("Info: %v", err)
	}
	if sb.Len() == 0 {
		t.Error("Info produced no output")
	}
}

// makeCodecConfigPayload builds a Codec Config OBU payload (without OBU header)
// for the given codec ID and codec-specific config bytes.
func makeCodecConfigPayload(codecConfigID uint64, codecID string, numSamples uint64,
	rollDistance int16, codecConfig []byte) []byte {
	sw := bits.NewFixedSliceWriter(64 + len(codecConfig))
	WriteLeb128(sw, codecConfigID)
	sw.WriteBytes([]byte(codecID))
	WriteLeb128(sw, numSamples)
	sw.WriteUint16(uint16(rollDistance))
	sw.WriteBytes(codecConfig)
	return sw.Bytes()
}

func TestCodecConfigObuPcm(t *testing.T) {
	// PCM: sample_format=1 (LE), bits=16, sample_rate=48000, roll_distance=0
	pcmConfig := []byte{0x01, 16, 0x00, 0x00, 0xbb, 0x80}
	payload := makeCodecConfigPayload(1, "ipcm", 960, 0, pcmConfig)
	ctx := &IamfContext{}
	if err := codecConfigObu(bits.NewFixedSliceReader(payload), ctx); err != nil {
		t.Fatalf("codecConfigObu(pcm): %v", err)
	}
	if ctx.NumCodecConfigs != 1 {
		t.Fatalf("NumCodecConfigs = %d, want 1", ctx.NumCodecConfigs)
	}
	cc := ctx.CodecConfigs[0]
	if cc.CodecID != "pcm_s16le" {
		t.Errorf("CodecID = %q, want pcm_s16le", cc.CodecID)
	}
	if cc.SampleRate != 48000 {
		t.Errorf("SampleRate = %d, want 48000", cc.SampleRate)
	}
}

func TestCodecConfigObuFlac(t *testing.T) {
	// FLAC: 4-byte metadata header + 18-byte STREAMINFO, roll_distance must be 0
	flacConfig := make([]byte, 4+18)
	// Place 0x0BB800 at extradata offset 0 for sample_rate 48000 (per the parser's
	// current behavior — it reads sample_rate from the start of extradata)
	flacConfig[4] = 0x0B
	flacConfig[5] = 0xB8
	flacConfig[6] = 0x00
	payload := makeCodecConfigPayload(2, "fLaC", 4096, 0, flacConfig)
	ctx := &IamfContext{}
	if err := codecConfigObu(bits.NewFixedSliceReader(payload), ctx); err != nil {
		t.Fatalf("codecConfigObu(flac): %v", err)
	}
	if ctx.CodecConfigs[0].CodecID != "flac" {
		t.Errorf("CodecID = %q, want flac", ctx.CodecConfigs[0].CodecID)
	}
}

func TestCodecConfigObuAac(t *testing.T) {
	aacConfig := aacDescriptor(0x02, []byte{0x11, 0x90})
	payload := makeCodecConfigPayload(3, "mp4a", 1024, -1, aacConfig)
	ctx := &IamfContext{}
	if err := codecConfigObu(bits.NewFixedSliceReader(payload), ctx); err != nil {
		t.Fatalf("codecConfigObu(aac): %v", err)
	}
	if ctx.CodecConfigs[0].CodecID != "aac" {
		t.Errorf("CodecID = %q, want aac", ctx.CodecConfigs[0].CodecID)
	}
}

func TestCodecConfigObuUnknownCodec(t *testing.T) {
	// Unknown 4cc should map to "none" with no codec-specific parsing
	payload := makeCodecConfigPayload(4, "xxxx", 480, 0, nil)
	ctx := &IamfContext{}
	if err := codecConfigObu(bits.NewFixedSliceReader(payload), ctx); err != nil {
		t.Fatalf("codecConfigObu(unknown): %v", err)
	}
	if ctx.CodecConfigs[0].CodecID != "none" {
		t.Errorf("CodecID = %q, want none", ctx.CodecConfigs[0].CodecID)
	}
}

func TestCodecConfigObuDuplicateID(t *testing.T) {
	payload := makeCodecConfigPayload(7, "xxxx", 480, 0, nil)
	ctx := &IamfContext{
		CodecConfigs:    []*IamfCodecConfig{{CodecConfigID: 7}},
		NumCodecConfigs: 1,
	}
	if err := codecConfigObu(bits.NewFixedSliceReader(payload), ctx); err == nil {
		t.Error("expected duplicate codec config id error")
	}
}

func TestCodecConfigObuZeroSamples(t *testing.T) {
	// numSamples == 0 must fail
	payload := makeCodecConfigPayload(8, "xxxx", 0, 0, nil)
	ctx := &IamfContext{}
	if err := codecConfigObu(bits.NewFixedSliceReader(payload), ctx); err == nil {
		t.Error("expected zero sample count error")
	}
}

func TestAudioElementObuSceneAmbisonics(t *testing.T) {
	ctx := &IamfContext{
		CodecConfigs:    []*IamfCodecConfig{{CodecConfigID: 1, CodecID: "pcm"}},
		NumCodecConfigs: 1,
	}
	// audioElementID=1, type=Scene (1<<5=0x20), codecConfigID=1,
	// numSubstreams=4, substream IDs 0..3, numParameters=0,
	// ambisonics mode=0 (Mono), outputChannelCount=4, substreamCount=4,
	// channel map 0,1,2,3
	payload := []byte{
		0x01,                   // audioElementID
		0x20,                   // type = Scene
		0x01,                   // codecConfigID
		0x04,                   // numSubstreams
		0x00, 0x01, 0x02, 0x03, // substream IDs
		0x00,       // numParameters
		0x00,       // ambisonics mode = Mono
		0x04, 0x04, // outputChannelCount, substreamCount
		0x00, 0x01, 0x02, 0x03, // channel map
	}
	if err := audioElementObu(bits.NewFixedSliceReader(payload), ctx); err != nil {
		t.Fatalf("audioElementObu: %v", err)
	}
	if ctx.NumAudioElements != 1 {
		t.Errorf("NumAudioElements = %d, want 1", ctx.NumAudioElements)
	}
	ae := ctx.AudioElements[0]
	if ae.Element.AudioElementType != AudioElementTypeScene {
		t.Errorf("AudioElementType = %v, want Scene", ae.Element.AudioElementType)
	}
	if len(ae.Element.Layers) != 1 {
		t.Fatalf("Layers count = %d, want 1", len(ae.Element.Layers))
	}
	if ae.Element.Layers[0].AmbisonicsMode != AmbisonicsModeMono {
		t.Errorf("AmbisonicsMode = %v, want Mono", ae.Element.Layers[0].AmbisonicsMode)
	}
}

func TestAudioElementObuSceneAmbisonicsProjection(t *testing.T) {
	ctx := &IamfContext{
		CodecConfigs:    []*IamfCodecConfig{{CodecConfigID: 1, CodecID: "pcm"}},
		NumCodecConfigs: 1,
	}
	// First-order ambisonics with projection mode: 4 output channels,
	// 4 substreams, 0 coupled substream count, demixing matrix = 4*4*2 bytes
	demixing := make([]byte, 4*4*2)
	payload := append([]byte{
		0x02,                   // audioElementID
		0x20,                   // type = Scene
		0x01,                   // codecConfigID
		0x04,                   // numSubstreams
		0x00, 0x01, 0x02, 0x03, // substream IDs
		0x00,       // numParameters
		0x01,       // ambisonics mode = Projection
		0x04, 0x04, // outputChannelCount, substreamCount
		0x00, // coupledSubstreamCount
	}, demixing...)
	if err := audioElementObu(bits.NewFixedSliceReader(payload), ctx); err != nil {
		t.Fatalf("audioElementObu: %v", err)
	}
	if ctx.AudioElements[0].Element.Layers[0].AmbisonicsMode != AmbisonicsModeProjection {
		t.Errorf("AmbisonicsMode = %v, want Projection",
			ctx.AudioElements[0].Element.Layers[0].AmbisonicsMode)
	}
}

func TestAudioElementObuDuplicateID(t *testing.T) {
	ctx := &IamfContext{
		AudioElements:    []*IamfAudioElement{{AudioElementID: 5}},
		NumAudioElements: 1,
	}
	payload := []byte{0x05, 0x00, 0x01}
	if err := audioElementObu(bits.NewFixedSliceReader(payload), ctx); err == nil {
		t.Error("expected duplicate audio element ID error")
	}
}

func TestAudioElementObuMissingCodecConfig(t *testing.T) {
	ctx := &IamfContext{}
	// references codecConfigID=9 which is not in ctx
	payload := []byte{0x01, 0x00, 0x09, 0x01, 0x00}
	if err := audioElementObu(bits.NewFixedSliceReader(payload), ctx); err == nil {
		t.Error("expected missing codec config error")
	}
}

func TestAudioElementObuUnknownType(t *testing.T) {
	ctx := &IamfContext{
		CodecConfigs:    []*IamfCodecConfig{{CodecConfigID: 1}},
		NumCodecConfigs: 1,
	}
	// type byte 0x60 -> upper 3 bits = 011 = 3, which is > AudioElementTypeScene
	payload := []byte{0x01, 0x60, 0x01}
	if err := audioElementObu(bits.NewFixedSliceReader(payload), ctx); err == nil {
		t.Error("expected unknown audio element type error")
	}
}

func TestParamParseDemixing(t *testing.T) {
	ctx := &IamfContext{}
	ae := &IamfAudioElement{}
	// parameterID=1, parameterRate=48000, mode=0 (1 bit, top), duration=480,
	// constantSubblockDuration=480 (numSubblocks=1).
	// Then 1 demixing subblock = 2 bytes:
	//   byte 0: dmixp_mode (top 3 bits)
	//   byte 1: default_w (top 4 bits)
	// 48000 leb128: 0x80, 0xf7, 0x02
	// 480 leb128: 0xe0, 0x03
	// dmixp_mode=2 -> 010_00000 = 0x40
	// default_w=5 -> 0101_0000 = 0x50
	payload := []byte{
		0x01,             // parameterID
		0x80, 0xf7, 0x02, // parameterRate=48000
		0x00,       // mode
		0xe0, 0x03, // duration=480
		0xe0, 0x03, // constantSubblockDuration=480
		0x40, 0x50, // dmixp_mode=2, default_w=5
	}
	pd, err := paramParse(bits.NewFixedSliceReader(payload), ctx, ParamDefinitionDemixing, ae)
	if err != nil {
		t.Fatalf("paramParse(demixing): %v", err)
	}
	if pd.ParameterID != 1 {
		t.Errorf("ParameterID = %d, want 1", pd.ParameterID)
	}
	if pd.Duration != 480 {
		t.Errorf("Duration = %d, want 480", pd.Duration)
	}
	if pd.NumSubblocks != 1 {
		t.Errorf("NumSubblocks = %d, want 1", pd.NumSubblocks)
	}
	if ae.Element.DefaultW != 5 {
		t.Errorf("DefaultW = %d, want 5", ae.Element.DefaultW)
	}
}

func TestParamParseReconGain(t *testing.T) {
	ctx := &IamfContext{}
	payload := []byte{
		0x02,             // parameterID
		0x80, 0xf7, 0x02, // parameterRate=48000
		0x00,       // mode
		0xe0, 0x03, // duration=480
		0xe0, 0x03, // constantSubblockDuration=480
	}
	pd, err := paramParse(bits.NewFixedSliceReader(payload), ctx, ParamDefinitionReconGain, nil)
	if err != nil {
		t.Fatalf("paramParse(recon): %v", err)
	}
	if pd.Type != ParamDefinitionReconGain {
		t.Errorf("Type = %v, want ReconGain", pd.Type)
	}
}

func TestParamParseVariableSubblocks(t *testing.T) {
	ctx := &IamfContext{}
	// mode=0, duration=600, constantSubblockDuration=0 -> numSubblocks read, then per
	// subblock the duration is read.
	// numSubblocks=2, then 2 subblock durations: 200, 400
	payload := []byte{
		0x03,             // parameterID
		0x80, 0xf7, 0x02, // parameterRate=48000
		0x00,       // mode
		0xd8, 0x04, // duration=600
		0x00,       // constantSubblockDuration=0
		0x02,       // numSubblocks=2
		0xc8, 0x01, // subblock 0 duration=200
		0x90, 0x03, // subblock 1 duration=400
	}
	pd, err := paramParse(bits.NewFixedSliceReader(payload), ctx, ParamDefinitionMixGain, nil)
	if err != nil {
		t.Fatalf("paramParse(variable): %v", err)
	}
	if pd.NumSubblocks != 2 {
		t.Errorf("NumSubblocks = %d, want 2", pd.NumSubblocks)
	}
}

func TestParamParseDuplicateReuse(t *testing.T) {
	ctx := &IamfContext{
		ParamDefinitions: []*IamfParamDefinition{
			{Param: ParamDefinition{ParameterID: 7}},
		},
		NumParamDefinitions: 1,
	}
	// parameterID matches existing - should reuse without growing
	payload := []byte{
		0x07,             // parameterID matches
		0x80, 0xf7, 0x02, // parameterRate=48000
		0x00,       // mode
		0xe0, 0x03, // duration
		0xe0, 0x03, // constantSubblockDuration
	}
	if _, err := paramParse(bits.NewFixedSliceReader(payload), ctx, ParamDefinitionReconGain, nil); err != nil {
		t.Fatalf("paramParse(reuse): %v", err)
	}
	if ctx.NumParamDefinitions != 1 {
		t.Errorf("NumParamDefinitions = %d, want 1", ctx.NumParamDefinitions)
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
