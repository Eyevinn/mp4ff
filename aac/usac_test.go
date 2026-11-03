package aac

import (
	"bytes"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/go-test/deep"
)

// xheTargetLevel is the AudioSpecificConfig of fate-suite/aac/usac/xhe_target_level.m4a
// from the FFmpeg FATE suite.
const xheTargetLevel = "f94643221cc05852002000a04046d0b800"

func TestDecodeUSACConfig(t *testing.T) {
	testCases := []struct {
		desc              string
		asc               string
		wantFrequency     int
		wantChannelConfig byte
		wantUSAC          USACConfig // Without Data, which must equal the bits after the header
		headerBits        int
		wantNrChannels    int
		wantCoreFrame     uint32
		wantOutputFrame   int
	}{
		{
			desc: "xHE-AAC stereo 48 kHz", asc: xheTargetLevel, wantFrequency: 48000, wantChannelConfig: 2,
			wantUSAC:   USACConfig{SamplingFrequency: 48000, CoreSbrFrameLengthIndex: 1, ChannelConfigurationIndex: 2},
			headerBits: 19, wantNrChannels: 2, wantCoreFrame: 1024, wantOutputFrame: 1024,
		},
		{
			desc: "escaped USAC frequency, SBR 2:1, 32 channels in UsacChannelConfig", asc: "f9461f00bb8060f808",
			wantFrequency: 48000, wantChannelConfig: 0,
			wantUSAC: USACConfig{SamplingFrequency: 48000, CoreSbrFrameLengthIndex: 3, ChannelConfigurationIndex: 0,
				NumOutChannels: 32},
			headerBits: 19, wantNrChannels: 32, wantCoreFrame: 1024, wantOutputFrame: 2048,
		},
		{
			desc: "escaped ASC frequency, USAC-only frequency index, SBR 4:1, 1+1 channels", asc: "f95e01c2004f88",
			wantFrequency: 57600, wantChannelConfig: 2,
			wantUSAC:   USACConfig{SamplingFrequency: 57600, CoreSbrFrameLengthIndex: 4, ChannelConfigurationIndex: 8},
			headerBits: 43, wantNrChannels: 2, wantCoreFrame: 1024, wantOutputFrame: 4096,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			data := mustHex(t, tc.asc)
			asc, err := DecodeAudioSpecificConfig(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if asc.ObjectType != USAC || asc.SamplingFrequency != tc.wantFrequency ||
				asc.ChannelConfiguration != tc.wantChannelConfig {
				t.Errorf("got object type %d, frequency %d, channel configuration %d", asc.ObjectType,
					asc.SamplingFrequency, asc.ChannelConfiguration)
			}
			if asc.USAC == nil {
				t.Fatal("no USAC config")
			}
			if asc.USAC.NrBits != 8*len(data)-tc.headerBits {
				t.Errorf("got %d bits, want %d", asc.USAC.NrBits, 8*len(data)-tc.headerBits)
			}
			got := *asc.USAC
			got.Data, got.NrBits = nil, 0
			if diff := deep.Equal(got, tc.wantUSAC); diff != nil {
				t.Error(diff)
			}
			if n := asc.NrChannels(); n != tc.wantNrChannels {
				t.Errorf("got %d channels, want %d", n, tc.wantNrChannels)
			}
			if n := asc.CoreSamplesPerFrame(); n != tc.wantCoreFrame {
				t.Errorf("got %d core samples per frame, want %d", n, tc.wantCoreFrame)
			}
			if n := asc.USAC.OutputFrameLength(); n != tc.wantOutputFrame {
				t.Errorf("got output frame length %d, want %d", n, tc.wantOutputFrame)
			}
			if enc := hex.EncodeToString(encodeASC(t, *asc)); enc != tc.asc {
				t.Errorf("encoded %s, want %s", enc, tc.asc)
			}
		})
	}
}

func TestDecodeUSACConfigErrors(t *testing.T) {
	testCases := []struct {
		desc      string
		asc       string
		wantedErr string
	}{
		{"reserved USAC frequency index 13", "f9464d22", "reserved or zero sampling frequency, index 13"},
		{"UsacConfig ends after the frequency index", "f94643", "USAC config too short"},
		{"no UsacConfig", "f946", "audio specific config too short"},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			asc, err := DecodeAudioSpecificConfig(bytes.NewReader(mustHex(t, tc.asc)))
			if err == nil || !strings.Contains(err.Error(), tc.wantedErr) {
				t.Fatalf("wanted error %q, got %v", tc.wantedErr, err)
			}
			if asc == nil || asc.ObjectType != USAC {
				t.Errorf("wanted the partly decoded config, got %+v", asc)
			}
		})
	}
}

func TestUSACNrChannelsAndFrameLengths(t *testing.T) {
	wantChannels := map[byte]int{1: 1, 2: 2, 6: 6, 7: 8, 8: 2, 9: 3, 10: 4, 11: 7, 12: 8, 13: 24, 14: 0, 31: 0}
	for index, want := range wantChannels {
		u := USACConfig{ChannelConfigurationIndex: index}
		if got := u.NrChannels(); got != want {
			t.Errorf("channelConfigurationIndex %d: got %d channels, want %d", index, got, want)
		}
	}
	wantLengths := map[byte][2]int{0: {768, 768}, 1: {1024, 1024}, 2: {768, 2048}, 3: {1024, 2048},
		4: {1024, 4096}, 5: {0, 0}, 7: {0, 0}}
	for index, want := range wantLengths {
		u := USACConfig{CoreSbrFrameLengthIndex: index}
		if got := [2]int{u.CoreCoderFrameLength(), u.OutputFrameLength()}; got != want {
			t.Errorf("coreSbrFrameLengthIndex %d: got %v, want %v", index, got, want)
		}
	}
}

func TestEncodeUSACValidation(t *testing.T) {
	valid := func(modify func(a *AudioSpecificConfig)) AudioSpecificConfig {
		a := AudioSpecificConfig{ObjectType: USAC, ChannelConfiguration: 2, SamplingFrequency: 48000,
			USAC: &USACConfig{SamplingFrequency: 48000, CoreSbrFrameLengthIndex: 1, ChannelConfigurationIndex: 2,
				Data: []byte{0x19, 0x10}, NrBits: 13}} // 48 kHz, index 1, stereo
		modify(&a)
		return a
	}
	asc := valid(func(a *AudioSpecificConfig) {})
	got, err := DecodeAudioSpecificConfig(bytes.NewReader(encodeASC(t, asc)))
	if err != nil {
		t.Fatal(err)
	}
	if diff := deep.Equal(got, &asc); diff != nil {
		t.Error(diff)
	}
	testCases := []struct {
		desc      string
		asc       AudioSpecificConfig
		wantedErr string
	}{
		{"object type USAC without UsacConfig", valid(func(a *AudioSpecificConfig) { a.USAC = nil }),
			"needs a USAC config"},
		{"UsacConfig with object type AAC-LC", valid(func(a *AudioSpecificConfig) { a.ObjectType = AAClc }),
			"USAC config needs object type 42, not 2"},
		{"USAC with SBR flag", valid(func(a *AudioSpecificConfig) { a.SBRPresentFlag = true }),
			"fields of other object types"},
		{"USAC with PCE", valid(func(a *AudioSpecificConfig) { a.PCE = &ProgramConfigElement{} }),
			"fields of other object types"},
		{"no bits", valid(func(a *AudioSpecificConfig) { a.USAC.NrBits = 0 }), "0 bits in 2 bytes"},
		{"more bits than data", valid(func(a *AudioSpecificConfig) { a.USAC.NrBits = 17 }), "17 bits in 2 bytes"},
		{"unused data byte", valid(func(a *AudioSpecificConfig) { a.USAC.NrBits = 8 }), "8 bits in 2 bytes"},
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
