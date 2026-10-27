package mp4_test

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/Eyevinn/mp4ff/mp4"
)

// alacSampleEntries are alac sample entries from files written by afconvert
// (stereo, and 5.1 with a chan box next to the cookie) and by ffmpeg (a
// QuickTime version 1 entry with the cookie inside a wave box). cookieIndent
// is the indent of the cookie in Info output with indentStep "  ".
var alacSampleEntries = []struct {
	name         string
	hexData      string
	inWave       bool
	cookieIndent string
	numChannels  byte
	sampleRate   uint32
	maxFrameSize uint32
}{
	{
		name: "afconvert stereo m4a",
		hexData: "00000048616c6163000000000000000100000000000000000002001000000000ac44000000000024616c616300000000" +
			"000010000010280a0e0200ff000006b300022fbd0000ac44",
		cookieIndent: "  ",
		numChannels:  2,
		sampleRate:   44100,
		maxFrameSize: 1715,
	},
	{
		name: "afconvert 5.1 m4a",
		hexData: "00000060616c6163000000000000000100000000000000000002001000000000ac44000000000024616c616300000000" +
			"000010000010280a0e0600ff00001b200009a8cf0000bb80000000186368616e00000000007c00060000000000000000",
		cookieIndent: "  ",
		numChannels:  6,
		sampleRate:   48000,
		maxFrameSize: 6944,
	},
	{
		name: "ffmpeg stereo mov",
		hexData: "0000008c616c61630000000000000001000100000000000000020010fffe0000ac440000000010000000000000000000" +
			"0000000200000040776176650000000c66726d61616c616300000024616c616300000000000010000010280a0e020000" +
			"00004004001588800000ac440000000800000000000000186368616e00000000006500020000000000000000",
		inWave:       true,
		cookieIndent: "    ",
		numChannels:  2,
		sampleRate:   44100,
		maxFrameSize: 16388,
	},
}

func TestAlacSampleEntries(t *testing.T) {
	for _, tc := range alacSampleEntries {
		t.Run(tc.name, func(t *testing.T) {
			data, err := hex.DecodeString(tc.hexData)
			if err != nil {
				t.Fatal(err)
			}
			cmpAfterDecodeEncodeBox(t, data)
			decodes := []struct {
				name   string
				decode func() (mp4.Box, error)
			}{
				{"DecodeBox", func() (mp4.Box, error) { return mp4.DecodeBox(0, bytes.NewReader(data)) }},
				{"DecodeBoxSR", func() (mp4.Box, error) { return mp4.DecodeBoxSR(0, bits.NewFixedSliceReader(data)) }},
			}
			for _, d := range decodes {
				box, err := d.decode()
				if err != nil {
					t.Fatalf("%s: %v", d.name, err)
				}
				entry, ok := box.(*mp4.AudioSampleEntryBox)
				if !ok {
					t.Fatalf("%s: got %T, wanted AudioSampleEntryBox", d.name, box)
				}
				alac := entry.Alac
				if tc.inWave {
					if entry.Wave == nil {
						t.Fatalf("%s: no wave box", d.name)
					}
					alac = entry.Wave.Alac
				}
				if alac == nil {
					t.Fatalf("%s: alac magic cookie not decoded", d.name)
				}
				if alac.FrameLength != 4096 || alac.BitDepth != 16 || alac.NumChannels != tc.numChannels ||
					alac.SampleRate != tc.sampleRate || alac.MaxFrameBytes != tc.maxFrameSize {
					t.Errorf("%s: wrong cookie values %+v", d.name, alac)
				}
				stsd := mp4.NewStsdBox()
				stsd.AddChild(entry)
				if stsd.Alac != entry {
					t.Errorf("%s: stsd.Alac not set", d.name)
				}
				var info bytes.Buffer
				if err := entry.Info(&info, "", "", "  "); err != nil {
					t.Fatalf("%s: %v", d.name, err)
				}
				wantLines := fmt.Sprintf("%[1]s[alac] size=36 version=0 flags=000000\n"+
					"%[1]s - frameLength: 4096\n"+
					"%[1]s - compatibleVersion: 0\n"+
					"%[1]s - bitDepth: 16\n"+
					"%[1]s - pb: 40, mb: 10, kb: 14\n"+
					"%[1]s - numChannels: %[2]d\n", tc.cookieIndent, tc.numChannels)
				if !strings.Contains(info.String(), wantLines) || strings.Contains(info.String(), "channelLayoutInfo") {
					t.Errorf("%s: info lacks the cookie lines\n%s\nor has channelLayoutInfo:\n%s",
						d.name, wantLines, info.String())
				}
			}
		})
	}
}

func TestEncodeDecodeAlac(t *testing.T) {
	cookie := &mp4.AlacBox{
		FrameLength:   4096,
		BitDepth:      24,
		Pb:            40,
		Mb:            10,
		Kb:            14,
		NumChannels:   2,
		MaxRun:        255,
		MaxFrameBytes: 24588,
		AvgBitRate:    2304000,
		SampleRate:    96000,
	}
	entry := mp4.CreateAudioSampleEntryBox("alac", 2, 24, 0, cookie)
	boxDiffAfterEncodeAndDecode(t, entry)

	withLayout := *cookie
	withLayout.NumChannels = 6
	withLayout.ChannelLayoutInfo = []byte{
		0, 0, 0, 24, 'c', 'h', 'a', 'n', 0, 0, 0, 0, 0, 124, 0, 6, 0, 0, 0, 0, 0, 0, 0, 0,
	}
	entry = mp4.CreateAudioSampleEntryBox("alac", 6, 24, 0, &withLayout)
	boxDiffAfterEncodeAndDecode(t, entry)
	if withLayout.Size() != 60 {
		t.Errorf("alac with channel layout info has size %d, wanted 60", withLayout.Size())
	}

	var info bytes.Buffer
	if err := entry.Info(&info, "", "", "  "); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"frameLength: 4096", "numChannels: 6", "sampleRate: 96000",
		"channelLayoutInfo: 000000186368616e00000000007c00060000000000000000"} {
		if !strings.Contains(info.String(), want) {
			t.Errorf("info lacks %q:\n%s", want, info.String())
		}
	}
}

func TestDecodeAlac(t *testing.T) {
	payload, err := hex.DecodeString("00000000000010000010280a0e0200ff000006b300022fbd0000ac44")
	if err != nil {
		t.Fatal(err)
	}
	hdr := mp4.BoxHeader{Name: "alac", Size: uint64(8 + len(payload)), Hdrlen: 8}
	box, err := mp4.DecodeAlac(hdr, 0, bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	if alac := box.(*mp4.AlacBox); alac.FrameLength != 4096 || alac.AvgBitRate != 143293 {
		t.Errorf("wrong cookie values %+v", alac)
	}
	hdr.Size = 8 + 20
	if _, err := mp4.DecodeAlac(hdr, 0, bytes.NewReader(payload[:20])); err == nil {
		t.Error("no error for a cookie shorter than ALACSpecificConfig")
	}
}

// TestShortAlacCookieKeepsBytes checks that a cookie too short for
// ALACSpecificConfig makes the sample entry, or the wave child, an
// UnknownBox, so that the bytes survive the round trip.
func TestShortAlacCookieKeepsBytes(t *testing.T) {
	shortCookie := make([]byte, 0, 20)
	shortCookie = binary.BigEndian.AppendUint32(shortCookie, 20)
	shortCookie = append(shortCookie, []byte("alac")...)
	shortCookie = append(shortCookie, make([]byte, 12)...)

	t.Run("in sample entry", func(t *testing.T) {
		data := quickTimeSoundDescriptionBytes("alac", 0, nil, shortCookie)
		cmpAfterDecodeEncodeBox(t, data)
		box, err := mp4.DecodeBox(0, bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := box.(*mp4.UnknownBox); !ok {
			t.Errorf("entry with short cookie decoded as %T, wanted UnknownBox", box)
		}
	})

	t.Run("in wave", func(t *testing.T) {
		wave := make([]byte, 0, 8+len(shortCookie))
		wave = binary.BigEndian.AppendUint32(wave, uint32(8+len(shortCookie)))
		wave = append(wave, []byte("wave")...)
		wave = append(wave, shortCookie...)
		data := quickTimeSoundDescriptionBytes("alac", 0, nil, wave)
		cmpAfterDecodeEncodeBox(t, data)
		box, err := mp4.DecodeBoxSR(0, bits.NewFixedSliceReader(data))
		if err != nil {
			t.Fatal(err)
		}
		entry := box.(*mp4.AudioSampleEntryBox)
		if entry.Wave == nil || entry.Wave.Alac != nil || len(entry.Wave.Children) != 1 {
			t.Fatalf("wrong wave box %+v", entry.Wave)
		}
		if _, ok := entry.Wave.Children[0].(*mp4.UnknownBox); !ok {
			t.Errorf("short cookie in wave decoded as %T, wanted UnknownBox", entry.Wave.Children[0])
		}
	})
}
