package mp4_test

import (
	"encoding/hex"
	"testing"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/Eyevinn/mp4ff/mp4"
)

func TestEncodeDecodeDOps(t *testing.T) {
	t.Run("mono-dops", func(t *testing.T) {
		dops := &mp4.DopsBox{
			Version:              0,
			OutputChannelCount:   1,
			PreSkip:              312,
			InputSampleRate:      48000,
			OutputGain:           0,
			ChannelMappingFamily: 0,
		}
		boxDiffAfterEncodeAndDecode(t, dops)
	})

	t.Run("stereo-dops", func(t *testing.T) {
		dops := &mp4.DopsBox{
			Version:              0,
			OutputChannelCount:   2,
			PreSkip:              312,
			InputSampleRate:      48000,
			OutputGain:           0,
			ChannelMappingFamily: 0,
		}
		boxDiffAfterEncodeAndDecode(t, dops)
	})

	t.Run("multichannel-dops", func(t *testing.T) {
		dops := &mp4.DopsBox{
			Version:              0,
			OutputChannelCount:   6,
			PreSkip:              312,
			InputSampleRate:      48000,
			OutputGain:           0,
			ChannelMappingFamily: 1,
			StreamCount:          4,
			CoupledCount:         2,
			ChannelMapping:       []byte{0, 4, 1, 2, 3, 5},
		}
		boxDiffAfterEncodeAndDecode(t, dops)
	})

	t.Run("high-samplerate-dops", func(t *testing.T) {
		dops := &mp4.DopsBox{
			Version:              0,
			OutputChannelCount:   2,
			PreSkip:              312,
			InputSampleRate:      96000,
			OutputGain:           256, // +1 dB in 8.8 fixed point
			ChannelMappingFamily: 0,
		}
		boxDiffAfterEncodeAndDecode(t, dops)
	})

	t.Run("negative-gain-dops", func(t *testing.T) {
		dops := &mp4.DopsBox{
			Version:              0,
			OutputChannelCount:   2,
			PreSkip:              312,
			InputSampleRate:      48000,
			OutputGain:           -512, // -2 dB in 8.8 fixed point
			ChannelMappingFamily: 0,
		}
		boxDiffAfterEncodeAndDecode(t, dops)
	})
}

func TestDOpsFromOpusFile(t *testing.T) {
	// Test with actual data from the opus.mp4 test file
	// This is the dOps box from the test file: version=0, outputChannelCount=2, preSkip=312,
	// inputSampleRate=48000, outputGain=0, channelMappingFamily=0
	// Extract from hex dump: 00 00 00 13 64 4f 70 73 00 02 01 38 00 00 bb 80 00 00 00
	dopsHex := "00000013644f7073000201380000bb80000000"
	dopsBytes, err := hex.DecodeString(dopsHex)
	if err != nil {
		t.Error(err)
	}
	sr := bits.NewFixedSliceReader(dopsBytes)
	box, err := mp4.DecodeBoxSR(0, sr)
	if err != nil {
		t.Error(err)
	}
	dops := box.(*mp4.DopsBox)

	expectedVersion := byte(0)
	if dops.Version != expectedVersion {
		t.Errorf("got version %d instead of %d", dops.Version, expectedVersion)
	}

	expectedOutputChannelCount := byte(2)
	if dops.OutputChannelCount != expectedOutputChannelCount {
		t.Errorf("got outputChannelCount %d instead of %d", dops.OutputChannelCount, expectedOutputChannelCount)
	}

	expectedPreSkip := uint16(312)
	if dops.PreSkip != expectedPreSkip {
		t.Errorf("got preSkip %d instead of %d", dops.PreSkip, expectedPreSkip)
	}

	expectedInputSampleRate := uint32(48000)
	if dops.InputSampleRate != expectedInputSampleRate {
		t.Errorf("got inputSampleRate %d instead of %d", dops.InputSampleRate, expectedInputSampleRate)
	}

	expectedOutputGain := int16(0)
	if dops.OutputGain != expectedOutputGain {
		t.Errorf("got outputGain %d instead of %d", dops.OutputGain, expectedOutputGain)
	}

	expectedChannelMappingFamily := byte(0)
	if dops.ChannelMappingFamily != expectedChannelMappingFamily {
		t.Errorf("got channelMappingFamily %d instead of %d", dops.ChannelMappingFamily, expectedChannelMappingFamily)
	}

	// For channelMappingFamily 0, there should be no channel mapping table
	if len(dops.ChannelMapping) != 0 {
		t.Errorf("got channelMapping length %d instead of 0", len(dops.ChannelMapping))
	}
}

func TestDOpsSize(t *testing.T) {
	tests := []struct {
		name         string
		dops         *mp4.DopsBox
		expectedSize uint64
	}{
		{
			name: "stereo-no-mapping",
			dops: &mp4.DopsBox{
				Version:              0,
				OutputChannelCount:   2,
				PreSkip:              312,
				InputSampleRate:      48000,
				OutputGain:           0,
				ChannelMappingFamily: 0,
			},
			expectedSize: 19, // 8 (header) + 11 (fixed fields)
		},
		{
			name: "multichannel-with-mapping",
			dops: &mp4.DopsBox{
				Version:              0,
				OutputChannelCount:   6,
				PreSkip:              312,
				InputSampleRate:      48000,
				OutputGain:           0,
				ChannelMappingFamily: 1,
				StreamCount:          4,
				CoupledCount:         2,
				ChannelMapping:       []byte{0, 4, 1, 2, 3, 5},
			},
			expectedSize: 27, // 8 (header) + 11 (fixed fields) + 2 (stream/coupled count) + 6 (channel mapping)
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			size := test.dops.Size()
			if size != test.expectedSize {
				t.Errorf("got size %d instead of %d", size, test.expectedSize)
			}
		})
	}
}
