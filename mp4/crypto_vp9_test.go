package mp4_test

import (
	"bytes"
	"encoding/hex"
	"os"
	"testing"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/Eyevinn/mp4ff/ivf"
	"github.com/Eyevinn/mp4ff/mp4"
	"github.com/go-test/deep"
)

// vp9TestSample returns a VP9 sample of n bytes, including superframeIndex at the end. It starts
// with an inter frame header (frame_marker 2, profile 0, frame_type 1, show_frame 1).
func vp9TestSample(n int, superframeIndex []byte) []byte {
	s := make([]byte, n)
	for i := range s {
		s[i] = byte(i % 97)
	}
	s[0] = 0x86
	copy(s[n-len(superframeIndex):], superframeIndex)
	return s
}

// vp9TestKeyFrame returns an n-byte VP9 key frame (profile 0, 8-bit 4:2:0, 64x48).
func vp9TestKeyFrame(n int) []byte {
	s := vp9TestSample(n, nil)
	copy(s, []byte{0x82, 0x49, 0x83, 0x42, 0x00, 0x03, 0xf0, 0x02, 0xf0})
	return s
}

func TestGetVP9ProtectRanges(t *testing.T) {
	twoFrameSample := append(vp9TestSample(128, nil), vp9TestSample(128, nil)...)
	twoFrameSample = append(twoFrameSample, 0xc1, 0x80, 0x80, 0xc1)
	shortSuperframe := append(vp9TestSample(20, nil), vp9TestSample(20, nil)...)
	shortSuperframe = append(shortSuperframe, 0xc1, 0x14, 0x14, 0xc1)
	clearFirstFrame := append(vp9TestSample(20, nil), vp9TestSample(128, nil)...)
	clearFirstFrame = append(clearFirstFrame, 0xc1, 0x14, 0x80, 0xc1)
	clearLastFrame := append(vp9TestSample(128, nil), vp9TestSample(20, nil)...)
	clearLastFrame = append(clearLastFrame, 0xc1, 0x80, 0x14, 0xc1)
	invalidSecondFrame := bytes.Clone(twoFrameSample)
	invalidSecondFrame[128] = 0
	eightFrames := []byte{}
	for i := 0; i < 8; i++ {
		eightFrames = append(eightFrames, vp9TestSample(20, nil)...)
	}
	eightFrames = append(eightFrames, 0xc7, 20, 20, 20, 20, 20, 20, 20, 20, 0xc7)
	testCases := []struct {
		name    string
		sample  []byte
		scheme  string
		want    []mp4.SubSamplePattern
		wantErr bool
	}{
		{name: "shorter than clear prefix", sample: vp9TestSample(50, nil), scheme: "cenc",
			want: []mp4.SubSamplePattern{{50, 0}}},
		{name: "exactly clear prefix", sample: vp9TestSample(96, nil), scheme: "cenc",
			want: []mp4.SubSamplePattern{{96, 0}}},
		{name: "less than one block after prefix", sample: vp9TestSample(111, nil), scheme: "cenc",
			want: []mp4.SubSamplePattern{{111, 0}}},
		{name: "one block after prefix", sample: vp9TestSample(112, nil), scheme: "cenc",
			want: []mp4.SubSamplePattern{{96, 16}}},
		{name: "partial block joins clear prefix", sample: vp9TestSample(300, nil), scheme: "cenc",
			want: []mp4.SubSamplePattern{{108, 192}}},
		{name: "key frame", sample: vp9TestKeyFrame(1000), scheme: "cenc",
			want: []mp4.SubSamplePattern{{104, 896}}},
		{name: "superframe headers and index stay clear", sample: twoFrameSample, scheme: "cenc",
			want: []mp4.SubSamplePattern{{96, 32}, {96, 32}, {4, 0}}},
		{name: "short superframe", sample: shortSuperframe, scheme: "cenc",
			want: []mp4.SubSamplePattern{{44, 0}}},
		{name: "clear first frame", sample: clearFirstFrame, scheme: "cenc",
			want: []mp4.SubSamplePattern{{116, 32}, {4, 0}}},
		{name: "clear last frame", sample: clearLastFrame, scheme: "cenc",
			want: []mp4.SubSamplePattern{{96, 32}, {24, 0}}},
		{name: "eight frames", sample: eightFrames, scheme: "cenc",
			want: []mp4.SubSamplePattern{{170, 0}}},
		{name: "three-byte frame size", sample: vp9TestSample(117, []byte{0xd0, 112, 0, 0, 0xd0}), scheme: "cenc",
			want: []mp4.SubSamplePattern{{96, 16}, {5, 0}}},
		{name: "four-byte frame size", sample: vp9TestSample(118, []byte{0xd8, 112, 0, 0, 0, 0xd8}), scheme: "cenc",
			want: []mp4.SubSamplePattern{{96, 16}, {6, 0}}},
		{name: "invalid second frame header", sample: invalidSecondFrame, scheme: "cenc", wantErr: true},
		{name: "superframe sizes exceed payload", sample: vp9TestSample(200, []byte{0xc1, 0x80, 0x80, 0xc1}),
			scheme: "cenc", wantErr: true},
		{name: "superframe sizes leave unused payload", sample: vp9TestSample(200, []byte{0xc1, 0x40, 0x40, 0xc1}),
			scheme: "cenc", wantErr: true},
		{name: "zero size superframe", sample: vp9TestSample(200, []byte{0xc1, 0x00, 0xc4, 0xc1}),
			scheme: "cenc", wantErr: true},
		{name: "superframe size exceeds int32", sample: vp9TestSample(200,
			[]byte{0xd8, 0xff, 0xff, 0xff, 0xff, 0xd8}), scheme: "cenc", wantErr: true},
		{name: "mismatched superframe markers", sample: vp9TestSample(200, []byte{0xc2, 0x40, 0x40, 0xc1}),
			scheme: "cenc", want: []mp4.SubSamplePattern{{104, 96}}},
		{name: "large sample", sample: vp9TestSample(70000, nil), scheme: "cenc",
			want: []mp4.SubSamplePattern{{96, 69904}}},
		{name: "invalid frame marker", sample: append([]byte{0x00}, vp9TestSample(199, nil)...),
			scheme: "cenc", wantErr: true},
		{name: "empty sample", sample: nil, scheme: "cenc", wantErr: true},
		{name: "cbcs", sample: vp9TestSample(300, nil), scheme: "cbcs", wantErr: true},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := mp4.GetVP9ProtectRanges(tc.sample, tc.scheme)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %v", got)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if diff := deep.Equal(got, tc.want); diff != nil {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func vp9TestSuperframe(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile("testdata/vp9_superframe.ivf")
	if err != nil {
		t.Fatal(err)
	}
	rd, err := ivf.NewReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	frame, err := rd.ReadFrame()
	if err != nil {
		t.Fatal(err)
	}
	return frame.Data
}

func TestVP9SuperframeHeadersStayClear(t *testing.T) {
	sample := vp9TestSuperframe(t)
	// The superframe contains a 5931-byte hidden key frame, a 2287-byte displayed frame
	// and a six-byte index. Each frame's uncompressed header must stay clear.
	patterns, err := mp4.GetVP9ProtectRanges(sample, "cenc")
	if err != nil {
		t.Fatal(err)
	}
	want := []mp4.SubSamplePattern{{107, 5824}, {111, 2176}, {6, 0}}
	if diff := deep.Equal(patterns, want); diff != nil {
		t.Errorf("subsample patterns: %v", diff)
	}
	key, _ := hex.DecodeString("00112233445566778899aabbccddeeff")
	iv, _ := hex.DecodeString("ffeeddccbbaa99887766554433221100")
	encrypted := bytes.Clone(sample)
	if err := mp4.CryptSampleCenc(encrypted, key, iv, patterns); err != nil {
		t.Fatal(err)
	}
	for _, start := range []int{0, 5931} {
		if !bytes.Equal(encrypted[start:start+96], sample[start:start+96]) {
			t.Errorf("frame header at offset %d was encrypted", start)
		}
	}
	if !bytes.Equal(encrypted[8218:], sample[8218:]) {
		t.Error("superframe index was encrypted")
	}
	for i, ss := range patterns {
		if ss.BytesOfProtectedData%16 != 0 {
			t.Errorf("subsample %d: protected bytes are not block-aligned", i)
		}
	}
}

func vp9TestInit(t *testing.T) *mp4.InitSegment {
	t.Helper()
	init := mp4.CreateEmptyInit()
	init.AddEmptyTrack(90000, "video", "und")
	vpcC := &mp4.VppCBox{Version: 1, Profile: 0, Level: 10, BitDepth: 8, ChromaSubsampling: 1,
		ColourPrimaries: 1, TransferCharacteristics: 1, MatrixCoefficients: 1}
	if err := init.Moov.Trak.SetVPxDescriptor("vp09", vpcC, 64, 48); err != nil {
		t.Fatal(err)
	}
	return init
}

func TestInitProtectVP9(t *testing.T) {
	key, _ := hex.DecodeString("00112233445566778899aabbccddeeff")
	iv, _ := hex.DecodeString("7766554433221100")
	kid, _ := mp4.NewUUIDFromString("11112222333344445555666677778888")

	init := vp9TestInit(t)
	ipd, err := mp4.InitProtect(init, key, iv, "cenc", kid, nil)
	if err != nil {
		t.Fatal(err)
	}
	se := init.Moov.Trak.Mdia.Minf.Stbl.Stsd.Children[0].(*mp4.VisualSampleEntryBox)
	if se.Type() != "encv" {
		t.Errorf("sample entry type %s, want encv", se.Type())
	}
	if se.Sinf == nil || se.Sinf.Frma == nil || se.Sinf.Frma.DataFormat != "vp09" {
		t.Fatal("missing frma with data format vp09")
	}
	if se.Sinf.Schm == nil || se.Sinf.Schm.SchemeType != "cenc" {
		t.Error("missing schm with scheme cenc")
	}
	tenc := se.Sinf.Schi.Tenc
	if tenc == nil || tenc.DefaultIsProtected != 1 || tenc.DefaultPerSampleIVSize != 8 || !bytes.Equal(tenc.DefaultKID, kid) {
		t.Errorf("unexpected tenc %+v", tenc)
	}

	var buf bytes.Buffer
	if err := init.Encode(&buf); err != nil {
		t.Fatal(err)
	}
	decodedIO, err := mp4.DecodeFile(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := mp4.DecodeFileSR(bits.NewFixedSliceReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if diff := deep.Equal(decodedIO.Init, decoded.Init); diff != nil {
		t.Errorf("decode paths differ: %v", diff)
	}
	extracted, err := mp4.ExtractInitProtectData(decoded.Init)
	if err != nil {
		t.Fatal(err)
	}
	if extracted.Scheme != "cenc" {
		t.Errorf("extracted scheme %s, want cenc", extracted.Scheme)
	}
	if diff := deep.Equal(extracted.Tenc, ipd.Tenc); diff != nil {
		t.Errorf("extracted tenc differs: %v", diff)
	}
	if _, err := extracted.NewFragmentEncryptor(key, iv); err != nil {
		t.Errorf("extracted protection data cannot encrypt: %v", err)
	}
}

func TestInitProtectVP9RejectsCbcs(t *testing.T) {
	key, _ := hex.DecodeString("00112233445566778899aabbccddeeff")
	iv, _ := hex.DecodeString("7766554433221100")
	kid, _ := mp4.NewUUIDFromString("11112222333344445555666677778888")
	if _, err := mp4.InitProtect(vp9TestInit(t), key, iv, "cbcs", kid, nil); err == nil {
		t.Error("expected error for vp09 with scheme cbcs")
	}

	init := vp9TestInit(t)
	if _, err := mp4.InitProtect(init, key, iv, "cenc", kid, nil); err != nil {
		t.Fatal(err)
	}
	se := init.Moov.Trak.Mdia.Minf.Stbl.Stsd.Children[0].(*mp4.VisualSampleEntryBox)
	se.Sinf.Schm.SchemeType = "cbcs"
	if _, err := mp4.ExtractInitProtectData(init); err == nil {
		t.Error("expected error extracting vp09 protection data with scheme cbcs")
	}
}

func TestEncryptDecryptVP9Fragment(t *testing.T) {
	key, _ := hex.DecodeString("00112233445566778899aabbccddeeff")
	kid, _ := mp4.NewUUIDFromString("11112222333344445555666677778888")
	ivs := map[string]string{
		"8-byte iv":  "7766554433221100",
		"16-byte iv": "ffeeddccbbaa99887766554433221100",
	}
	samples := [][]byte{
		vp9TestKeyFrame(1000),
		vp9TestSample(500, nil),
		append(append(vp9TestSample(128, nil), vp9TestSample(128, nil)...), 0xc1, 0x80, 0x80, 0xc1),
		vp9TestSuperframe(t),
		vp9TestSample(40, nil),
	}
	for name, ivHex := range ivs {
		t.Run(name, func(t *testing.T) {
			iv, _ := hex.DecodeString(ivHex)
			init := vp9TestInit(t)
			ipd, err := mp4.InitProtect(init, key, iv, "cenc", kid, nil)
			if err != nil {
				t.Fatal(err)
			}
			frag, err := mp4.CreateFragment(1, init.Moov.Trak.Tkhd.TrackID)
			if err != nil {
				t.Fatal(err)
			}
			for i, s := range samples {
				frag.AddFullSample(mp4.FullSample{
					Sample:     mp4.NewSample(mp4.SyncSampleFlags, 3000, uint32(len(s)), 0),
					DecodeTime: uint64(i * 3000),
					Data:       bytes.Clone(s),
				})
			}
			if _, err := mp4.EncryptFragment(frag, key, iv, ipd); err != nil {
				t.Fatal(err)
			}

			senc := frag.Moof.Traf.Senc
			if senc == nil || len(senc.SubSamples) != len(samples) {
				t.Fatal("expected one senc subsample entry per sample")
			}
			encSamples, err := frag.GetFullSamples(ipd.Trex)
			if err != nil {
				t.Fatal(err)
			}
			for i, s := range samples {
				want, err := mp4.GetVP9ProtectRanges(s, "cenc")
				if err != nil {
					t.Fatal(err)
				}
				if diff := deep.Equal(senc.SubSamples[i], want); diff != nil {
					t.Errorf("sample %d: subsamples %v, want %v", i, senc.SubSamples[i], want)
				}
				pos := 0
				for _, ss := range want {
					clearEnd := pos + int(ss.BytesOfClearData)
					protEnd := clearEnd + int(ss.BytesOfProtectedData)
					if !bytes.Equal(encSamples[i].Data[pos:clearEnd], s[pos:clearEnd]) {
						t.Errorf("sample %d: clear bytes [%d:%d] changed", i, pos, clearEnd)
					}
					if ss.BytesOfProtectedData > 0 && bytes.Equal(encSamples[i].Data[clearEnd:protEnd], s[clearEnd:protEnd]) {
						t.Errorf("sample %d: protected bytes [%d:%d] not encrypted", i, clearEnd, protEnd)
					}
					pos = protEnd
				}
			}

			var initBuf, fragBuf bytes.Buffer
			if err := init.Encode(&initBuf); err != nil {
				t.Fatal(err)
			}
			if err := frag.Encode(&fragBuf); err != nil {
				t.Fatal(err)
			}
			encInit, err := mp4.DecodeFileSR(bits.NewFixedSliceReader(initBuf.Bytes()))
			if err != nil {
				t.Fatal(err)
			}
			decInfo, err := mp4.DecryptInit(encInit.Init)
			if err != nil {
				t.Fatal(err)
			}
			encFile, err := mp4.DecodeFileSR(bits.NewFixedSliceReader(fragBuf.Bytes()))
			if err != nil {
				t.Fatal(err)
			}
			decFrag := encFile.Segments[0].Fragments[0]
			if err := mp4.DecryptFragment(decFrag, decInfo, key); err != nil {
				t.Fatal(err)
			}
			decSamples, err := decFrag.GetFullSamples(decInfo.TrackInfos[0].Trex)
			if err != nil {
				t.Fatal(err)
			}
			for i, s := range samples {
				if !bytes.Equal(decSamples[i].Data, s) {
					t.Errorf("sample %d differs after encryption and decryption", i)
				}
			}
		})
	}
}
