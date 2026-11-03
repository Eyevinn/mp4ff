package iamf

import (
	"runtime"
	"testing"
	"time"

	"github.com/Eyevinn/mp4ff/bits"
)

// leb returns the leb128 encoding of v.
func leb(v uint64) []byte {
	sw := bits.NewFixedSliceWriter(10)
	WriteLeb128(sw, v)
	return sw.Bytes()
}

// cat concatenates byte slices.
func cat(parts ...[]byte) []byte {
	var out []byte
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

// obu returns an OBU with a header without trimming or extension.
func obu(obuType ObuType, payload []byte) []byte {
	return cat([]byte{byte(obuType) << 3}, leb(uint64(len(payload))), payload)
}

// readAllDescriptors reads the descriptor OBUs of data and returns the first error.
func readAllDescriptors(data []byte) (*IamfContext, error) {
	r := NewObuReader(data, len(data))
	for {
		o, err := r.ReadObu()
		if err != nil {
			return nil, err
		}
		if o == nil {
			return r.Context(), nil
		}
		if _, err := o.ReadDescriptors(&r); err != nil {
			return nil, err
		}
	}
}

// maxParseAlloc is the most memory that the parser may allocate for one of the small inputs below.
const maxParseAlloc = 1 << 20

// mustFailPromptly reads the descriptors of data and fails the test if that panics, does not
// return an error, takes more than a second, or allocates more than maxParseAlloc bytes.
func mustFailPromptly(t *testing.T, data []byte) {
	t.Helper()
	type result struct {
		err      error
		panicked any
		alloc    uint64
	}
	done := make(chan result, 1)
	go func() {
		var res result
		var before, after runtime.MemStats
		defer func() {
			res.panicked = recover()
			done <- res
		}()
		runtime.ReadMemStats(&before)
		_, res.err = readAllDescriptors(data)
		runtime.ReadMemStats(&after)
		res.alloc = after.TotalAlloc - before.TotalAlloc
	}()
	select {
	case res := <-done:
		if res.panicked != nil {
			t.Fatalf("panic: %v", res.panicked)
		}
		if res.err == nil {
			t.Fatal("expected an error")
		}
		if res.alloc > maxParseAlloc {
			t.Fatalf("allocated %d bytes, more than %d", res.alloc, maxParseAlloc)
		}
		t.Logf("error: %v", res.err)
	case <-time.After(time.Second):
		t.Fatal("did not return within a second")
	}
}

// codecConfigObu returns a Codec Config OBU with codec_config_id 0 and an unknown codec.
func testCodecConfigObu() []byte {
	return obu(ObuTypeCodecConfig, cat(
		leb(0),             // codec_config_id
		[]byte("xxxx"),     // codec_id
		leb(960),           // num_samples_per_frame
		[]byte{0x00, 0x00}, // audio_roll_distance
	))
}

// outputMixConfig returns the param definition of output_mix_config with mode 0, followed by
// default_mix_gain.
func outputMixConfig(duration, constantSubblockDuration uint64, subblockDurations ...uint64) []byte {
	pd := cat(
		leb(1),       // parameter_id
		leb(48000),   // parameter_rate
		[]byte{0x00}, // param_definition_mode = 0
		leb(duration),
		leb(constantSubblockDuration),
	)
	if constantSubblockDuration == 0 {
		pd = cat(pd, leb(uint64(len(subblockDurations))))
		for _, d := range subblockDurations {
			pd = cat(pd, leb(d))
		}
	}
	return cat(pd, []byte{0x00, 0x00}) // default_mix_gain
}

// mixPresentation returns a Mix Presentation OBU with one sub-mix without audio elements and
// with the given output_mix_config, followed by the given layouts.
func mixPresentation(mixConfig []byte, numLayouts uint64, layouts []byte) []byte {
	return obu(ObuTypeMixPresentation, cat(
		leb(0), // mix_presentation_id
		leb(0), // count_label
		leb(1), // num_sub_mixes
		leb(0), // num_audio_elements
		mixConfig,
		leb(numLayouts),
		layouts,
	))
}

// stereoLayout is a loudspeaker layout of sound system A with loudness info.
var stereoLayout = []byte{
	0x80,       // layout_type = loudspeakers, sound_system = A
	0x00,       // info_type
	0x00, 0x00, // integrated_loudness
	0x00, 0x00, // digital_peak
}

func TestParamDefinitionSubblocks(t *testing.T) {
	cases := []struct {
		name                  string
		duration, constant    uint64
		coded                 []uint64
		wantConstant          uint32
		wantSubblockDurations []uint32
	}{
		{"constant equal to duration", 960, 960, nil, 960, []uint32{960}},
		{"constant that divides duration", 960, 480, nil, 480, []uint32{480, 480}},
		{"shorter last subblock", 10, 3, nil, 3, []uint32{3, 3, 3, 1}},
		{"constant longer than duration", 10, 30, nil, 30, []uint32{10}},
		{"coded subblock durations", 600, 0, []uint64{200, 400}, 0, []uint32{200, 400}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			data := mixPresentation(outputMixConfig(c.duration, c.constant, c.coded...), 1, stereoLayout)
			ctx, err := readAllDescriptors(data)
			if err != nil {
				t.Fatal(err)
			}
			pd := ctx.MixPresentations[0].Mix.Submixes[0].OutputMixConfig
			if pd.ConstantSubblockDuration != c.wantConstant {
				t.Errorf("ConstantSubblockDuration = %d, want %d", pd.ConstantSubblockDuration, c.wantConstant)
			}
			if int(pd.NumSubblocks) != len(c.wantSubblockDurations) {
				t.Fatalf("NumSubblocks = %d, want %d", pd.NumSubblocks, len(c.wantSubblockDurations))
			}
			subblocks := pd.Subblocks.([]interface{})
			for i, want := range c.wantSubblockDurations {
				if got := subblocks[i].(MixGain).SubblockDuration; got != want {
					t.Errorf("subblock %d duration = %d, want %d", i, got, want)
				}
			}
		})
	}
}

func TestParamDefinitionSubblockBounds(t *testing.T) {
	cases := []struct {
		name string
		data []byte
	}{
		{"coded num_subblocks beyond the data", mixPresentation(cat(
			leb(1), leb(48000), []byte{0x00}, // parameter_id, parameter_rate, param_definition_mode
			leb(1<<23), // duration
			leb(0),     // constant_subblock_duration
			leb(1<<23), // num_subblocks
		), 0, nil)},
		{"many derived subblocks", mixPresentation(outputMixConfig(1<<31, 1), 1, stereoLayout)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			mustFailPromptly(t, c.data)
		})
	}
}

func TestMixPresentationCountBounds(t *testing.T) {
	cases := []struct {
		name    string
		payload []byte
	}{
		{"count_label", cat(leb(0), leb(1<<25))},
		{"num_sub_mixes", cat(leb(0), leb(0), leb(1<<26))},
		{"num_audio_elements", cat(leb(0), leb(0), leb(1), leb(1<<26))},
		{"num_layouts", cat(leb(0), leb(0), leb(1), leb(0),
			leb(0), leb(0), []byte{0x80}, // output_mix_config with param_definition_mode = 1
			[]byte{0x00, 0x00}, // default_mix_gain
			leb(1<<26))},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			mustFailPromptly(t, obu(ObuTypeMixPresentation, c.payload))
		})
	}
}

func TestAudioElementCountBounds(t *testing.T) {
	cases := []struct {
		name    string
		payload []byte
	}{
		{"num_substreams", cat(
			leb(0),       // audio_element_id
			[]byte{0x00}, // audio_element_type = channel
			leb(0),       // codec_config_id
			leb(1<<26),   // num_substreams
		)},
		{"demixing_matrix beyond the data", cat(
			leb(0),              // audio_element_id
			[]byte{0x20},        // audio_element_type = scene
			leb(0),              // codec_config_id
			leb(1),              // num_substreams
			leb(0),              // audio_substream_id
			leb(0),              // num_parameters
			leb(1),              // ambisonics_mode = projection
			[]byte{225, 1, 255}, // output_channel_count, substream_count, coupled_substream_count
		)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			mustFailPromptly(t, cat(testCodecConfigObu(), obu(ObuTypeAudioElement, c.payload)))
		})
	}
}
