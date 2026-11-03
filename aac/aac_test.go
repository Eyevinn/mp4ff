package aac

import (
	"bytes"
	"encoding/hex"
	"errors"
	"io"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/go-test/deep"
)

func encodeASC(t *testing.T, asc AudioSpecificConfig) []byte {
	t.Helper()
	buf := &bytes.Buffer{}
	if err := asc.Encode(buf); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestDecodeAudioSpecificConfigHeader(t *testing.T) {
	testCases := []struct {
		desc      string
		asc       []byte
		wanted    AudioSpecificConfigHeader
		wantedErr string
	}{
		{
			desc:   "AAC-LC 48000 stereo",
			asc:    []byte{0x11, 0x90},
			wanted: AudioSpecificConfigHeader{ObjectType: 2, SamplingFrequency: 48000, ChannelConfiguration: 2},
		},
		{
			desc: "HE-AACv1 24000 stereo (base frequency, not the extension)",
			asc: encodeASC(t, AudioSpecificConfig{ObjectType: HEAACv1, ChannelConfiguration: 2,
				SamplingFrequency: 24000, ExtensionFrequency: 48000, SBRPresentFlag: true}),
			wanted: AudioSpecificConfigHeader{ObjectType: 5, SamplingFrequency: 24000, ChannelConfiguration: 2},
		},
		{
			desc: "object type 39 (ER AAC ELD), unsupported by the full decoder",
			asc:  []byte{0xf8, 0xe8, 0x40},
			wanted: AudioSpecificConfigHeader{
				ObjectType: 39, SamplingFrequency: 44100, ChannelConfiguration: 2,
			},
		},
		{
			desc: "explicit 24-bit frequency",
			asc:  []byte{0x17, 0x80, 0x2e, 0x6c, 0x08},
			wanted: AudioSpecificConfigHeader{
				ObjectType: 2, SamplingFrequency: 23768, ChannelConfiguration: 1,
			},
		},
		{
			desc:      "invalid frequency index 13",
			asc:       []byte{0x16, 0x90},
			wantedErr: "strange frequency index",
		},
		{
			desc:      "truncated config",
			asc:       []byte{0x11},
			wantedErr: "audio specific config too short",
		},
		{
			desc:      "truncated inside the object type escape",
			asc:       []byte{0xf8},
			wantedErr: "audio specific config too short",
		},
		{
			desc:      "truncated inside the 24-bit frequency",
			asc:       []byte{0x17, 0x80},
			wantedErr: "audio specific config too short",
		},
		{
			desc:      "truncated at the channel configuration",
			asc:       []byte{0xf8, 0xe8},
			wantedErr: "audio specific config too short",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			hdr, err := DecodeAudioSpecificConfigHeader(bytes.NewReader(tc.asc))
			if tc.wantedErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantedErr) {
					t.Errorf("wanted error %q, got %v", tc.wantedErr, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if diff := deep.Equal(*hdr, tc.wanted); diff != nil {
				t.Error(diff)
			}
		})
	}
}

func TestAudioSpecificConfigEncodeDecode(t *testing.T) {

	testCases := []AudioSpecificConfig{
		{
			ObjectType:           AAClc,
			ChannelConfiguration: 2,
			SamplingFrequency:    48000,
			ExtensionFrequency:   0,
			SBRPresentFlag:       false,
			PSPresentFlag:        false,
		},
		{
			ObjectType:           HEAACv1,
			ChannelConfiguration: 2,
			SamplingFrequency:    24000,
			ExtensionFrequency:   48000,
			SBRPresentFlag:       true,
			PSPresentFlag:        false,
		},
		{
			ObjectType:           HEAACv2,
			ChannelConfiguration: 1,
			SamplingFrequency:    24000,
			ExtensionFrequency:   48000,
			SBRPresentFlag:       true,
			PSPresentFlag:        true,
		},
	}

	for _, asc := range testCases {

		buf := &bytes.Buffer{}
		err := asc.Encode(buf)
		if err != nil {
			t.Error(err)
		}
		ascBytes := buf.Bytes()
		t.Logf("ASC: %s\n", hex.EncodeToString(ascBytes))

		readBuf := bytes.NewBuffer(ascBytes)
		gotAsc, err := DecodeAudioSpecificConfig(readBuf)
		if err != nil {
			t.Error(err)
		}
		diff := deep.Equal(*gotAsc, asc)
		if diff != nil {
			t.Errorf("Diff %v for %+v", diff, asc)
		}
	}
}

func TestVariousInputs(t *testing.T) {

	testCases := []struct {
		desc          string
		data          []byte
		expectedError string
	}{
		{
			desc:          "unsupported object type",
			data:          []byte{0x1f, 0x00},
			expectedError: "unsupported object type: 3",
		},
		{
			desc:          "bad frequency index",
			data:          []byte{0x17, 0x30},
			expectedError: "strange frequency index",
		},
		{
			desc:          "too short extended frequency",
			data:          []byte{0x17, 0x80, 0x40},
			expectedError: "audio specific config too short",
		},
	}

	for _, c := range testCases {
		t.Run(c.desc, func(t *testing.T) {
			readBuf := bytes.NewBuffer(c.data)
			_, err := DecodeAudioSpecificConfig(readBuf)
			if err == nil || !strings.Contains(err.Error(), c.expectedError) {
				t.Errorf("Expected error: %s", c.expectedError)
			}
		})
	}

	t.Run("32768Hz", func(t *testing.T) {
		data := []byte{0x17, 0x80, 0x40, 0x00, 0x08} // 32768 Hz, channel configuration 1.
		readBuf := bytes.NewBuffer(data)
		gotAsc, err := DecodeAudioSpecificConfig(readBuf)
		if err != nil {
			t.Fatal(err)
		}
		if gotAsc.SamplingFrequency != 32768 {
			t.Errorf("Expected 32768Hz, got %d", gotAsc.SamplingFrequency)
		}
	})
}

func TestNrChannels(t *testing.T) {
	want := map[byte]int{0: 0, 1: 1, 2: 2, 3: 3, 4: 4, 5: 5, 6: 6, 7: 8, 8: 0, 9: 0, 10: 0,
		11: 7, 12: 8, 13: 24, 14: 8, 15: 0, 16: 0}
	for cc, n := range want {
		if got := NrChannels(cc); got != n {
			t.Errorf("NrChannels(%d) = %d, want %d", cc, got, n)
		}
	}
}

func mustHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// The PCE test vectors are AudioSpecificConfigs of FFmpeg FATE suite files (fate-suite/aac).
var (
	// am00_88.mp4: AAC Main 88200 Hz, mono via a PCE.
	pceMainMono = &ProgramConfigElement{ObjectType: 0, SamplingFrequencyIndex: 1,
		FrontElements: []PCEChannelElement{{}}}
	// al22_chCfg0PCE_44.mp4: 7.1 via a PCE whose comment holds a height_extension_element.
	pce71Height = &ProgramConfigElement{ObjectType: 1, SamplingFrequencyIndex: 4,
		FrontElements:  []PCEChannelElement{{}, {IsCPE: true}, {IsCPE: true, TagSelect: 2}},
		BackElements:   []PCEChannelElement{{IsCPE: true, TagSelect: 1}},
		LFEElementTags: []byte{0}, AssocDataElementTags: []byte{0},
		Comment: []byte{0xac, 0x04, 0x2f}}
	// am05_44.mp4: AAC Main 5.1 via a PCE with a coupling channel and a comment.
	pceMain51Comment = &ProgramConfigElement{ObjectType: 0, SamplingFrequencyIndex: 4,
		FrontElements:  []PCEChannelElement{{}, {IsCPE: true}},
		BackElements:   []PCEChannelElement{{IsCPE: true, TagSelect: 1}},
		LFEElementTags: []byte{0}, CCElements: []PCECouplingElement{{IsIndSw: true}},
		Comment: []byte("Encoded by Dolby Laboratories, December 2001")}
	// al_sbr_cm_48_5.1.mp4: 5.1 via a PCE, followed by an SBR sync extension.
	pce51 = &ProgramConfigElement{ObjectType: 1, SamplingFrequencyIndex: 6,
		FrontElements:  []PCEChannelElement{{}, {IsCPE: true}, {IsCPE: true, TagSelect: 1}},
		LFEElementTags: []byte{0}}
	// Explicit HE-AAC with a PCE, constructed so that the PCE starts at bit 25 and the
	// byte alignment before the comment, relative to the start of the config, adds no bits.
	pceStereoComment = &ProgramConfigElement{ObjectType: 1, SamplingFrequencyIndex: 6,
		FrontElements: []PCEChannelElement{{IsCPE: true}}, Comment: []byte("hi")}
)

func TestDecodeAudioSpecificConfig(t *testing.T) {
	testCases := []struct {
		desc           string
		asc            string
		want           AudioSpecificConfig
		wantNrChannels int
		wantEncoded    string // If empty, the same as asc
	}{
		{
			desc:           "AAC Main 48000 stereo",
			asc:            "0990",
			want:           AudioSpecificConfig{ObjectType: AACMain, ChannelConfiguration: 2, SamplingFrequency: 48000},
			wantNrChannels: 2,
		},
		{
			desc: "AAC-LC with 960 samples per frame",
			asc:  "1194",
			want: AudioSpecificConfig{ObjectType: AAClc, ChannelConfiguration: 2, SamplingFrequency: 48000,
				FrameLengthFlag: true},
			wantNrChannels: 2,
		},
		{
			desc: "AAC-LC depending on a core coder with delay 1000",
			asc:  "11921f40",
			want: AudioSpecificConfig{ObjectType: AAClc, ChannelConfiguration: 2, SamplingFrequency: 48000,
				DependsOnCoreCoder: true, CoreCoderDelay: 1000},
			wantNrChannels: 2,
		},
		{
			desc: "SBR sync extension with SBR off",
			asc:  "121056e500",
			want: AudioSpecificConfig{ObjectType: AAClc, ChannelConfiguration: 2, SamplingFrequency: 44100,
				SyncExtensionPresent: true},
			wantNrChannels: 2,
		},
		{
			desc: "SBR sync extension with SBR on (implicit HE-AAC v1)",
			asc:  "139056e5a0",
			want: AudioSpecificConfig{ObjectType: AAClc, ChannelConfiguration: 2, SamplingFrequency: 22050,
				ExtensionFrequency: 44100, SBRPresentFlag: true, SyncExtensionPresent: true},
			wantNrChannels: 2,
		},
		{
			desc: "SBR and PS sync extensions (implicit HE-AAC v2)",
			asc:  "138856e5a54880",
			want: AudioSpecificConfig{ObjectType: AAClc, ChannelConfiguration: 1, SamplingFrequency: 22050,
				ExtensionFrequency: 44100, SBRPresentFlag: true, PSPresentFlag: true,
				SyncExtensionPresent: true, PSSyncExtensionPresent: true},
			wantNrChannels: 1,
		},
		{
			desc: "PS sync extension after SBR off",
			asc:  "121056e55488",
			want: AudioSpecificConfig{ObjectType: AAClc, ChannelConfiguration: 2, SamplingFrequency: 44100,
				PSPresentFlag: true, SyncExtensionPresent: true, PSSyncExtensionPresent: true},
			wantNrChannels: 2,
		},
		{
			desc: "AAC Main with a mono PCE",
			asc:  "0880004400000000",
			want: AudioSpecificConfig{ObjectType: AACMain, ChannelConfiguration: 0, SamplingFrequency: 88200,
				PCE: pceMainMono},
			wantNrChannels: 1,
		},
		{
			desc: "7.1 PCE with height extension",
			asc:  "1200050c05200109440003ac042f",
			want: AudioSpecificConfig{ObjectType: AAClc, ChannelConfiguration: 0, SamplingFrequency: 44100,
				PCE: pce71Height},
			wantNrChannels: 8,
		},
		{
			desc: "AAC Main 5.1 PCE with coupling channel and comment",
			asc: "0a0001080502010884002c456e636f64656420627920446f6c6279204c61626f7261746f72696573" +
				"2c20446563656d6265722032303031",
			want: AudioSpecificConfig{ObjectType: AACMain, ChannelConfiguration: 0, SamplingFrequency: 44100,
				PCE: pceMain51Comment},
			wantNrChannels: 6,
		},
		{
			desc: "5.1 PCE followed by an SBR sync extension",
			asc:  "1300058c01000108800056e598",
			want: AudioSpecificConfig{ObjectType: AAClc, ChannelConfiguration: 0, SamplingFrequency: 24000,
				ExtensionFrequency: 48000, SBRPresentFlag: true, SyncExtensionPresent: true, PCE: pce51},
			wantNrChannels: 6,
		},
		{
			desc: "explicit HE-AAC with a PCE aligned relative to the config start",
			asc:  "2b018802c2000010026869",
			want: AudioSpecificConfig{ObjectType: HEAACv1, ChannelConfiguration: 0, SamplingFrequency: 24000,
				ExtensionFrequency: 48000, SBRPresentFlag: true, PCE: pceStereoComment},
			wantNrChannels: 2,
		},
		{
			desc:           "trailing zero byte after AAC-LC",
			asc:            "121000",
			want:           AudioSpecificConfig{ObjectType: AAClc, ChannelConfiguration: 2, SamplingFrequency: 44100},
			wantNrChannels: 2,
			wantEncoded:    "1210",
		},
		{
			desc: "trailing zero byte after explicit HE-AAC",
			asc:  "2b8a080000",
			want: AudioSpecificConfig{ObjectType: HEAACv1, ChannelConfiguration: 1, SamplingFrequency: 22050,
				ExtensionFrequency: 44100, SBRPresentFlag: true},
			wantNrChannels: 1,
			wantEncoded:    "2b8a0800",
		},
		{
			desc: "trailing zero bytes after an SBR sync extension with SBR off",
			asc:  "121056e5000000",
			want: AudioSpecificConfig{ObjectType: AAClc, ChannelConfiguration: 2, SamplingFrequency: 44100,
				SyncExtensionPresent: true},
			wantNrChannels: 2,
			wantEncoded:    "121056e500",
		},
		{
			desc:           "trailing byte too short for a sync extension",
			asc:            "1210ff",
			want:           AudioSpecificConfig{ObjectType: AAClc, ChannelConfiguration: 2, SamplingFrequency: 44100},
			wantNrChannels: 2,
			wantEncoded:    "1210",
		},
		{
			desc:           "trailing bits with an unknown sync word",
			asc:            "1210ffff",
			want:           AudioSpecificConfig{ObjectType: AAClc, ChannelConfiguration: 2, SamplingFrequency: 44100},
			wantNrChannels: 2,
			wantEncoded:    "1210",
		},
		{
			desc:           "SBR sync word with an unknown extension object type",
			asc:            "121056e000",
			want:           AudioSpecificConfig{ObjectType: AAClc, ChannelConfiguration: 2, SamplingFrequency: 44100},
			wantNrChannels: 2,
			wantEncoded:    "1210",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			data := mustHex(t, tc.asc)
			asc, err := DecodeAudioSpecificConfig(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if diff := deep.Equal(*asc, tc.want); diff != nil {
				t.Error(diff)
			}
			if got := asc.NrChannels(); got != tc.wantNrChannels {
				t.Errorf("got %d channels, want %d", got, tc.wantNrChannels)
			}
			wantEncoded := tc.wantEncoded
			if wantEncoded == "" {
				wantEncoded = tc.asc
			}
			if got := hex.EncodeToString(encodeASC(t, *asc)); got != wantEncoded {
				t.Errorf("encoded %s, want %s", got, wantEncoded)
			}
		})
	}
}

func TestDecodeAudioSpecificConfigErrors(t *testing.T) {
	testCases := []struct {
		desc      string
		asc       string
		wantedErr string
		wantAsc   AudioSpecificConfig // The fields decoded before the error
	}{
		{
			desc:      "unsupported escaped object type",
			asc:       "f8e64000",
			wantedErr: "unsupported object type: 39",
			wantAsc:   AudioSpecificConfig{ObjectType: 39},
		},
		{
			desc:      "explicit HE-AAC on AAC Main",
			asc:       "2b1184",
			wantedErr: "base audioObjectType is 1 instead of AAC-LC (2)",
			wantAsc: AudioSpecificConfig{ObjectType: HEAACv1, ChannelConfiguration: 2, SamplingFrequency: 24000,
				ExtensionFrequency: 48000, SBRPresentFlag: true},
		},
		{
			desc:      "truncated PCE",
			asc:       "120005",
			wantedErr: "program config element too short",
			wantAsc: AudioSpecificConfig{ObjectType: AAClc, SamplingFrequency: 44100,
				PCE: &ProgramConfigElement{ObjectType: 1}},
		},
		{
			desc:      "extensionFlag3 set",
			asc:       "119180",
			wantedErr: "extensionFlag3",
			wantAsc: AudioSpecificConfig{ObjectType: AAClc, ChannelConfiguration: 2, SamplingFrequency: 48000,
				ExtensionFlag: true, ExtensionFlag3: true},
		},
		{
			desc:      "truncated core coder delay",
			asc:       "1192",
			wantedErr: "GA specific config too short",
			wantAsc: AudioSpecificConfig{ObjectType: AAClc, ChannelConfiguration: 2, SamplingFrequency: 48000,
				DependsOnCoreCoder: true},
		},
		{
			desc:      "SBR sync extension without the SBR flag",
			asc:       "121056e5",
			wantedErr: "sync extension too short",
			wantAsc: AudioSpecificConfig{ObjectType: AAClc, ChannelConfiguration: 2, SamplingFrequency: 44100,
				SyncExtensionPresent: true},
		},
		{
			desc:      "SBR sync extension with truncated 24-bit extension frequency",
			asc:       "121056e5f8",
			wantedErr: "audio specific config too short",
			wantAsc: AudioSpecificConfig{ObjectType: AAClc, ChannelConfiguration: 2, SamplingFrequency: 44100,
				SBRPresentFlag: true, SyncExtensionPresent: true},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			asc, err := DecodeAudioSpecificConfig(bytes.NewReader(mustHex(t, tc.asc)))
			if err == nil || !strings.Contains(err.Error(), tc.wantedErr) {
				t.Fatalf("wanted error %q, got %v", tc.wantedErr, err)
			}
			if diff := deep.Equal(*asc, tc.wantAsc); diff != nil {
				t.Error(diff)
			}
		})
	}

	t.Run("read error", func(t *testing.T) {
		asc, err := DecodeAudioSpecificConfig(iotest.ErrReader(io.ErrUnexpectedEOF))
		if !errors.Is(err, io.ErrUnexpectedEOF) || asc != nil {
			t.Errorf("got %v, %v, want nil and the read error", asc, err)
		}
	})
	t.Run("too large", func(t *testing.T) {
		data := make([]byte, maxConfigSize+1)
		data[0], data[1] = 0x12, 0x10
		_, err := DecodeAudioSpecificConfig(bytes.NewReader(data))
		if err == nil || !strings.Contains(err.Error(), "exceeds") {
			t.Errorf("wanted size error, got %v", err)
		}
	})
}

func TestEncodeValidation(t *testing.T) {
	lc := func(modify func(a *AudioSpecificConfig)) AudioSpecificConfig {
		a := AudioSpecificConfig{ObjectType: AAClc, ChannelConfiguration: 2, SamplingFrequency: 48000}
		modify(&a)
		return a
	}
	withPCE := func(modify func(p *ProgramConfigElement)) AudioSpecificConfig {
		p := &ProgramConfigElement{ObjectType: 1, SamplingFrequencyIndex: 3, FrontElements: []PCEChannelElement{{IsCPE: true}}}
		modify(p)
		return AudioSpecificConfig{ObjectType: AAClc, SamplingFrequency: 48000, PCE: p}
	}
	testCases := []struct {
		desc      string
		asc       AudioSpecificConfig
		wantedErr string
	}{
		{"unsupported object type", lc(func(a *AudioSpecificConfig) { a.ObjectType = 3 }), "audioObjectType 3 not supported"},
		{"channel configuration 16", lc(func(a *AudioSpecificConfig) { a.ChannelConfiguration = 16 }), "four bits"},
		{"channel configuration 0 without PCE", lc(func(a *AudioSpecificConfig) { a.ChannelConfiguration = 0 }),
			"needs a program config element"},
		{"PCE with channel configuration 2", lc(func(a *AudioSpecificConfig) { a.PCE = &ProgramConfigElement{} }),
			"needs channel configuration 0"},
		{"zero sampling frequency", lc(func(a *AudioSpecificConfig) { a.SamplingFrequency = 0 }), "invalid sampling frequency"},
		{"25-bit sampling frequency", lc(func(a *AudioSpecificConfig) { a.SamplingFrequency = 1 << 24 }),
			"invalid sampling frequency"},
		{"explicit HE-AAC with sync extension", lc(func(a *AudioSpecificConfig) {
			a.ObjectType, a.ExtensionFrequency, a.SyncExtensionPresent = HEAACv1, 96000, true
		}), "cannot also carry"},
		{"explicit HE-AAC without extension frequency", lc(func(a *AudioSpecificConfig) { a.ObjectType = HEAACv1 }),
			"invalid extension frequency"},
		{"extension frequency without SBR", lc(func(a *AudioSpecificConfig) { a.ExtensionFrequency = 96000 }),
			"extension frequency requires SBR"},
		{"SBR without sync extension", lc(func(a *AudioSpecificConfig) {
			a.SBRPresentFlag, a.ExtensionFrequency = true, 96000
		}), "SBR presence requires a sync extension"},
		{"PS sync extension without SBR sync extension", lc(func(a *AudioSpecificConfig) { a.PSSyncExtensionPresent = true }),
			"PS sync extension requires"},
		{"PS without PS signaling", lc(func(a *AudioSpecificConfig) { a.PSPresentFlag = true }), "PS presence requires"},
		{"core coder delay without core coder", lc(func(a *AudioSpecificConfig) { a.CoreCoderDelay = 1 }), "core coder delay"},
		{"15-bit core coder delay", lc(func(a *AudioSpecificConfig) {
			a.DependsOnCoreCoder, a.CoreCoderDelay = true, 0x4000
		}), "core coder delay"},
		{"extensionFlag3", lc(func(a *AudioSpecificConfig) { a.ExtensionFlag, a.ExtensionFlag3 = true, true }),
			"extensionFlag3"},
		{"PCE object type 4", withPCE(func(p *ProgramConfigElement) { p.ObjectType = 4 }), "object type 4 exceeds 3"},
		{"PCE with 16 front elements", withPCE(func(p *ProgramConfigElement) {
			p.FrontElements = make([]PCEChannelElement, 16)
		}), "number of front elements 16 exceeds 15"},
		{"PCE with 4 LFE elements", withPCE(func(p *ProgramConfigElement) { p.LFEElementTags = make([]byte, 4) }),
			"number of LFE elements 4 exceeds 3"},
		{"PCE with 256 comment bytes", withPCE(func(p *ProgramConfigElement) { p.Comment = make([]byte, 256) }),
			"comment length 256 exceeds 255"},
		{"PCE front element tag 16", withPCE(func(p *ProgramConfigElement) { p.FrontElements[0].TagSelect = 16 }),
			"element tag 16 exceeds 15"},
		{"PCE LFE tag 16", withPCE(func(p *ProgramConfigElement) { p.LFEElementTags = []byte{16} }),
			"element tag 16 exceeds 15"},
		{"PCE coupling element tag 16", withPCE(func(p *ProgramConfigElement) {
			p.CCElements = []PCECouplingElement{{TagSelect: 16}}
		}), "element tag 16 exceeds 15"},
		{"PCE mono mixdown element without flag", withPCE(func(p *ProgramConfigElement) { p.MonoMixdownElement = 1 }),
			"mono mixdown"},
		{"PCE stereo mixdown element without flag", withPCE(func(p *ProgramConfigElement) { p.StereoMixdownElement = 1 }),
			"stereo mixdown"},
		{"PCE pseudo surround without matrix mixdown", withPCE(func(p *ProgramConfigElement) { p.PseudoSurroundEnable = true }),
			"matrix mixdown"},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			err := tc.asc.Encode(&bytes.Buffer{})
			if err == nil || !strings.Contains(err.Error(), tc.wantedErr) {
				t.Errorf("wanted error %q, got %v", tc.wantedErr, err)
			}
		})
	}
}

func TestPCEMixdownRoundTrip(t *testing.T) {
	asc := AudioSpecificConfig{ObjectType: AAClc, SamplingFrequency: 48000, PCE: &ProgramConfigElement{
		ElementInstanceTag: 3, ObjectType: 1, SamplingFrequencyIndex: 3,
		FrontElements:        []PCEChannelElement{{TagSelect: 1}, {IsCPE: true, TagSelect: 2}},
		SideElements:         []PCEChannelElement{{IsCPE: true, TagSelect: 3}},
		BackElements:         []PCEChannelElement{{TagSelect: 4}},
		LFEElementTags:       []byte{5, 6},
		AssocDataElementTags: []byte{7},
		CCElements:           []PCECouplingElement{{IsIndSw: true, TagSelect: 8}, {TagSelect: 9}},
		MonoMixdownPresent:   true, MonoMixdownElement: 10,
		StereoMixdownPresent: true, StereoMixdownElement: 11,
		MatrixMixdownIdxPresent: true, MatrixMixdownIdx: 2, PseudoSurroundEnable: true,
	}}
	data := encodeASC(t, asc)
	got, err := DecodeAudioSpecificConfig(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if diff := deep.Equal(*got, asc); diff != nil {
		t.Error(diff)
	}
	if n := got.NrChannels(); n != 8 {
		t.Errorf("got %d channels, want 8", n)
	}
}

func TestAudioSpecificConfigNrChannels(t *testing.T) {
	testCases := []struct {
		asc  AudioSpecificConfig
		want int
	}{
		{AudioSpecificConfig{ChannelConfiguration: 2}, 2},
		{AudioSpecificConfig{ChannelConfiguration: 7}, 8},
		{AudioSpecificConfig{ChannelConfiguration: 8}, 0},
		{AudioSpecificConfig{ChannelConfiguration: 0}, 0},
		{AudioSpecificConfig{ChannelConfiguration: 0, PCE: pce71Height}, 8},
	}
	for _, tc := range testCases {
		if got := tc.asc.NrChannels(); got != tc.want {
			t.Errorf("NrChannels() of %+v = %d, want %d", tc.asc, got, tc.want)
		}
	}
}

func TestCoreSamplesPerFrame(t *testing.T) {
	for frameLengthFlag, want := range map[bool]uint32{false: 1024, true: 960} {
		asc := AudioSpecificConfig{FrameLengthFlag: frameLengthFlag}
		if got := asc.CoreSamplesPerFrame(); got != want {
			t.Errorf("FrameLengthFlag %v: got %d, want %d", frameLengthFlag, got, want)
		}
	}
}
