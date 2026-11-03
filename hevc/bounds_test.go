package hevc

import (
	"bytes"
	"runtime"
	"testing"
	"time"

	"github.com/Eyevinn/mp4ff/bits"
)

// The tests in this file give the parsers counts, sizes and bit depths that are out of range
// and check that the parsers return an error promptly, without panicking.

// nalBuilder writes the syntax elements of a NAL unit, inserting emulation prevention bytes.
type nalBuilder struct {
	buf bytes.Buffer
	w   *bits.EBSPWriter
}

func newNalBuilder(naluType NaluType) *nalBuilder {
	b := &nalBuilder{}
	b.w = bits.NewEBSPWriter(&b.buf)
	b.u(uint(naluType)<<9|1, 16) // nal_unit_header with nuh_temporal_id_plus1 = 1
	return b
}

// u writes v in n bits. n must be at most 32.
func (b *nalBuilder) u(v uint, n int) { b.w.Write(v, n) }

func (b *nalBuilder) flag(f bool) {
	if f {
		b.u(1, 1)
	} else {
		b.u(0, 1)
	}
}

func (b *nalBuilder) ue(v uint) { b.w.WriteExpGolomb(v) }

// alignWithOnes writes one bits up to the next byte boundary.
func (b *nalBuilder) alignWithOnes() {
	for b.w.NrBitsInBuffer() != 0 {
		b.u(1, 1)
	}
}

// trailingBits writes rbsp_trailing_bits and returns the NAL unit.
func (b *nalBuilder) trailingBits() []byte {
	b.w.WriteRbspTrailingBits()
	return b.buf.Bytes()
}

// truncated pads to a byte boundary with zero bits and returns the NAL unit.
func (b *nalBuilder) truncated() []byte {
	b.w.StuffByteWithZeros()
	return b.buf.Bytes()
}

func (b *nalBuilder) profileTierLevel() {
	b.u(0, 2)           // general_profile_space
	b.u(0, 1)           // general_tier_flag
	b.u(1, 5)           // general_profile_idc
	b.u(0x60000000, 32) // general_profile_compatibility_flags
	b.u(0x9000, 16)     // first 16 of the 48 constraint flag bits
	b.u(0, 32)          // last 32 constraint flag bits
	b.u(93, 8)          // general_level_idc
}

// spsUpToShortTermRefPicSets writes an SPS of a 64x64 picture with 16x16 CTBs up to,
// but not including, num_short_term_ref_pic_sets.
func spsUpToShortTermRefPicSets(chromaFormatIDC, bitDepthLumaMinus8 uint) *nalBuilder {
	b := newNalBuilder(NALU_SPS)
	b.u(0, 4)    // sps_video_parameter_set_id
	b.u(0, 3)    // sps_max_sub_layers_minus1
	b.flag(true) // sps_temporal_id_nesting_flag
	b.profileTierLevel()
	b.ue(0)                  // sps_seq_parameter_set_id
	b.ue(chromaFormatIDC)    // chroma_format_idc
	b.ue(64)                 // pic_width_in_luma_samples
	b.ue(64)                 // pic_height_in_luma_samples
	b.flag(false)            // conformance_window_flag
	b.ue(bitDepthLumaMinus8) // bit_depth_luma_minus8
	b.ue(0)                  // bit_depth_chroma_minus8
	b.ue(0)                  // log2_max_pic_order_cnt_lsb_minus4
	b.flag(true)             // sps_sub_layer_ordering_info_present_flag
	b.ue(4)                  // sps_max_dec_pic_buffering_minus1
	b.ue(0)                  // sps_max_num_reorder_pics
	b.ue(0)                  // sps_max_latency_increase_plus1
	b.ue(0)                  // log2_min_luma_coding_block_size_minus3
	b.ue(1)                  // log2_diff_max_min_luma_coding_block_size
	b.ue(0)                  // log2_min_luma_transform_block_size_minus2
	b.ue(0)                  // log2_diff_max_min_luma_transform_block_size
	b.ue(0)                  // max_transform_hierarchy_depth_inter
	b.ue(0)                  // max_transform_hierarchy_depth_intra
	b.flag(false)            // scaling_list_enabled_flag
	b.flag(false)            // amp_enabled_flag
	b.flag(false)            // sample_adaptive_offset_enabled_flag
	b.flag(false)            // pcm_enabled_flag
	return b
}

// writeChainedShortTermRPSs writes numSets short-term RPSs. The first one has 16 negative and
// 16 positive pictures, and each of the others is predicted from the one before it, keeping all
// its pictures and adding one. Only the first bits of the last set are written if truncateLast.
func (b *nalBuilder) writeChainedShortTermRPSs(numSets int, truncateLast bool) {
	b.ue(16) // num_negative_pics
	b.ue(16) // num_positive_pics
	for i := 0; i < 32; i++ {
		b.ue(0)      // delta_poc_sX_minus1
		b.flag(true) // used_by_curr_pic_sX_flag
	}
	numDeltaPocs := 32
	for idx := 1; idx < numSets; idx++ {
		b.flag(true)  // inter_ref_pic_set_prediction_flag
		b.flag(false) // delta_rps_sign
		b.ue(0)       // abs_delta_rps_minus1
		if truncateLast && idx == numSets-1 {
			b.u(0xff, 8) // the first used_by_curr_pic_flag values
			return
		}
		for j := 0; j <= numDeltaPocs; j++ {
			b.flag(true) // used_by_curr_pic_flag
		}
		numDeltaPocs++
	}
}

// writeSPSTail writes the SPS from long_term_ref_pics_present_flag to the end, without extensions.
func (b *nalBuilder) writeSPSTail() []byte {
	b.flag(false) // long_term_ref_pics_present_flag
	b.flag(false) // sps_temporal_mvp_enabled_flag
	b.flag(false) // strong_intra_smoothing_enabled_flag
	b.flag(false) // vui_parameters_present_flag
	b.flag(false) // sps_extension_present_flag
	return b.trailingBits()
}

// vpsUpToNumProfileTierLevel writes a two-layer MV-HEVC VPS up to, but not including,
// vps_num_profile_tier_level_minus1 in vps_extension().
func vpsUpToNumProfileTierLevel() *nalBuilder {
	b := newNalBuilder(NALU_VPS)
	b.u(0, 4)       // vps_video_parameter_set_id
	b.flag(true)    // vps_base_layer_internal_flag
	b.flag(true)    // vps_base_layer_available_flag
	b.u(1, 6)       // vps_max_layers_minus1
	b.u(0, 3)       // vps_max_sub_layers_minus1
	b.flag(true)    // vps_temporal_id_nesting_flag
	b.u(0xffff, 16) // vps_reserved_0xffff_16bits
	b.profileTierLevel()
	b.flag(true)  // vps_sub_layer_ordering_info_present_flag
	b.ue(0)       // vps_max_dec_pic_buffering_minus1
	b.ue(0)       // vps_max_num_reorder_pics
	b.ue(0)       // vps_max_latency_increase_plus1
	b.u(1, 6)     // vps_max_layer_id
	b.ue(1)       // vps_num_layer_sets_minus1
	b.flag(true)  // layer_id_included_flag[1][0]
	b.flag(true)  // layer_id_included_flag[1][1]
	b.flag(false) // vps_timing_info_present_flag
	b.flag(true)  // vps_extension_flag
	b.alignWithOnes()
	b.u(93, 8)    // profile_tier_level(0, vps_max_sub_layers_minus1): general_level_idc
	b.flag(false) // splitting_flag
	for i := 0; i < 16; i++ {
		b.flag(i == 1) // scalability_mask_flag: view order index
	}
	b.u(0, 3)     // dimension_id_len_minus1[0]
	b.flag(false) // vps_nuh_layer_id_present_flag
	b.u(1, 1)     // dimension_id[1][0]
	b.u(0, 4)     // view_id_len
	b.flag(true)  // direct_dependency_flag[1][0]
	b.flag(false) // vps_sub_layers_max_minus1_present_flag
	b.flag(false) // max_tid_ref_present_flag
	b.flag(false) // default_ref_layers_active_flag
	return b
}

// ppsUpToExtensions writes a PPS up to and including pps_extension_4bits, with the given
// extension flags set.
func ppsUpToExtensions(rangeExt, multilayerExt, d3Ext, sccExt bool) *nalBuilder {
	b := newNalBuilder(NALU_PPS)
	b.ue(0)       // pps_pic_parameter_set_id
	b.ue(0)       // pps_seq_parameter_set_id
	b.flag(false) // dependent_slice_segments_enabled_flag
	b.flag(false) // output_flag_present_flag
	b.u(0, 3)     // num_extra_slice_header_bits
	b.flag(false) // sign_data_hiding_enabled_flag
	b.flag(false) // cabac_init_present_flag
	b.ue(0)       // num_ref_idx_l0_default_active_minus1
	b.ue(0)       // num_ref_idx_l1_default_active_minus1
	b.ue(0)       // init_qp_minus26 = 0
	b.flag(false) // constrained_intra_pred_flag
	b.flag(false) // transform_skip_enabled_flag
	b.flag(false) // cu_qp_delta_enabled_flag
	b.ue(0)       // pps_cb_qp_offset = 0
	b.ue(0)       // pps_cr_qp_offset = 0
	b.flag(false) // pps_slice_chroma_qp_offsets_present_flag
	b.flag(false) // weighted_pred_flag
	b.flag(false) // weighted_bipred_flag
	b.flag(false) // transquant_bypass_enabled_flag
	b.flag(false) // tiles_enabled_flag
	b.flag(false) // entropy_coding_sync_enabled_flag
	b.flag(false) // pps_loop_filter_across_slices_enabled_flag
	b.flag(false) // deblocking_filter_control_present_flag
	b.flag(false) // pps_scaling_list_data_present_flag
	b.flag(false) // lists_modification_present_flag
	b.ue(0)       // log2_parallel_merge_level_minus2
	b.flag(false) // slice_segment_header_extension_present_flag
	b.flag(true)  // pps_extension_present_flag
	b.flag(rangeExt)
	b.flag(multilayerExt)
	b.flag(d3Ext)
	b.flag(sccExt)
	b.u(0, 4) // pps_extension_4bits
	return b
}

// maxParseAlloc is the most memory that a parser may allocate for one of the small inputs below.
const maxParseAlloc = 1 << 20

// mustFailPromptly runs parse and fails the test if it panics, does not return an error,
// takes more than a second, or allocates more than maxParseAlloc bytes.
func mustFailPromptly(t *testing.T, parse func() error) {
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
		res.err = parse()
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

func TestSliceHeaderNumEntryPointOffsets(t *testing.T) {
	// A 64x64 picture with 16x16 CTBs has 4 CTB rows, so wavefront parallel processing
	// allows 3 entry points.
	sizedSPS := &SPS{PicWidthInLumaSamples: 64, PicHeightInLumaSamples: 64, Log2DiffMaxMinLumaCodingBlockSize: 1}
	wppPPS := &PPS{EntropyCodingSyncEnabledFlag: true}
	tilesPPS := &PPS{TilesEnabledFlag: true}
	slice := func(numEntryPointOffsets, offsetLenMinus1 uint) []byte {
		b := newNalBuilder(NALU_IDR_W_RADL)
		b.flag(true)  // first_slice_segment_in_pic_flag
		b.flag(false) // no_output_of_prior_pics_flag
		b.ue(0)       // slice_pic_parameter_set_id
		b.ue(2)       // slice_type I
		b.ue(0)       // slice_qp_delta = 0
		b.ue(numEntryPointOffsets)
		if numEntryPointOffsets > 0 {
			b.ue(offsetLenMinus1)
		}
		if numEntryPointOffsets < 16 && offsetLenMinus1 < 32 {
			for i := uint(0); i < numEntryPointOffsets; i++ {
				b.u(1, int(offsetLenMinus1)+1) // entry_point_offset_minus1
			}
		}
		return b.trailingBits() // byte_alignment()
	}
	cases := []struct {
		name                 string
		sps                  *SPS
		pps                  *PPS
		numEntryPointOffsets uint
		offsetLenMinus1      uint
		wantErr              bool
	}{
		{"one per CTB row", sizedSPS, wppPPS, 3, 7, false},
		{"more than CTB rows", sizedSPS, wppPPS, 4, 7, true},
		{"one per tile", sizedSPS, &PPS{TilesEnabledFlag: true, NumTileColumnsMinus1: 1}, 1, 7, false},
		{"more than tiles", sizedSPS, &PPS{TilesEnabledFlag: true, NumTileColumnsMinus1: 1}, 2, 7, true},
		{"more than the bits left", &SPS{}, tilesPPS, 1 << 27, 0, true},
		{"offset_len_minus1 out of range", &SPS{}, tilesPPS, 1, 255, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			data := slice(c.numEntryPointOffsets, c.offsetLenMinus1)
			spsMap := map[uint32]*SPS{0: c.sps}
			ppsMap := map[uint32]*PPS{0: c.pps}
			if !c.wantErr {
				sh, err := ParseSliceHeader(data, spsMap, ppsMap)
				if err != nil {
					t.Fatal(err)
				}
				if sh.NumEntryPointOffsets != c.numEntryPointOffsets {
					t.Errorf("got %d entry point offsets, want %d", sh.NumEntryPointOffsets, c.numEntryPointOffsets)
				}
				return
			}
			mustFailPromptly(t, func() error {
				_, err := ParseSliceHeader(data, spsMap, ppsMap)
				return err
			})
		})
	}
}

func TestSliceHeaderNumLongTermPics(t *testing.T) {
	slice := func(numLongTermSps, numLongTermPics uint, complete bool) []byte {
		b := newNalBuilder(NALU_TRAIL_R)
		b.flag(true) // first_slice_segment_in_pic_flag
		b.ue(0)      // slice_pic_parameter_set_id
		b.ue(2)      // slice_type I
		b.u(0, 4)    // slice_pic_order_cnt_lsb
		b.flag(true) // short_term_ref_pic_set_sps_flag
		b.ue(numLongTermSps)
		b.ue(numLongTermPics)
		if !complete {
			return b.truncated()
		}
		for i := uint(0); i < numLongTermSps+numLongTermPics; i++ {
			if i >= numLongTermSps {
				b.u(i, 4)    // poc_lsb_lt
				b.flag(true) // used_by_curr_pic_lt_flag
			}
			b.flag(false) // delta_poc_msb_present_flag
		}
		b.ue(0) // slice_qp_delta = 0
		return b.trailingBits()
	}
	sps := &SPS{LongTermRefPicsPresentFlag: true, NumLongTermRefPics: 1, LongTermRefPicSets: make([]LongTermRPS, 1)}
	spsMap := map[uint32]*SPS{0: sps}
	ppsMap := map[uint32]*PPS{0: {}}
	sh, err := ParseSliceHeader(slice(1, 2, true), spsMap, ppsMap)
	if err != nil {
		t.Fatal(err)
	}
	if len(sh.LongTermRefPicSets) != 3 {
		t.Errorf("got %d long-term pictures, want 3", len(sh.LongTermRefPicSets))
	}
	cases := []struct {
		name                            string
		numLongTermSps, numLongTermPics uint
	}{
		{"num_long_term_sps beyond the SPS", 2, 0},
		{"too many num_long_term_pics", 0, 1 << 23},
		{"too many in all", 1, maxRPSEntries},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			data := slice(c.numLongTermSps, c.numLongTermPics, false)
			mustFailPromptly(t, func() error {
				_, err := ParseSliceHeader(data, spsMap, ppsMap)
				return err
			})
		})
	}
}

func TestSliceHeaderShortTermRPS(t *testing.T) {
	// The SPS is built rather than parsed, so its last RPS can have more pictures than a
	// parsed SPS is allowed.
	sps := &SPS{
		NumShortTermRefPicSets: 2,
		ShortTermRefPicSets:    []ShortTermRPS{{}, {NumDeltaPocs: 255}},
	}
	spsMap := map[uint32]*SPS{0: sps}
	ppsMap := map[uint32]*PPS{0: {}}
	slice := func(deltaIdxMinus1 uint) []byte {
		b := newNalBuilder(NALU_TRAIL_R)
		b.flag(true)  // first_slice_segment_in_pic_flag
		b.ue(0)       // slice_pic_parameter_set_id
		b.ue(2)       // slice_type I
		b.u(0, 4)     // slice_pic_order_cnt_lsb
		b.flag(false) // short_term_ref_pic_set_sps_flag
		b.flag(true)  // inter_ref_pic_set_prediction_flag
		b.ue(deltaIdxMinus1)
		b.flag(false) // delta_rps_sign
		b.ue(0)       // abs_delta_rps_minus1
		for i := 0; i < 40; i++ {
			b.u(0xff, 8) // used_by_curr_pic_flag
		}
		return b.truncated()
	}
	cases := []struct {
		name           string
		deltaIdxMinus1 uint
	}{
		{"predicted from a large RPS", 0},
		{"delta_idx_minus1 equal to the number of RPSs", 2},
		{"delta_idx_minus1 that wraps in a byte", 255},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			data := slice(c.deltaIdxMinus1)
			mustFailPromptly(t, func() error {
				_, err := ParseSliceHeader(data, spsMap, ppsMap)
				return err
			})
		})
	}
}

func TestSPSShortTermRPSChain(t *testing.T) {
	cases := []struct {
		name         string
		numSets      int
		truncateLast bool
	}{
		{"predicted RPS with too many pictures", 2, false},
		{"more RPSs than allowed", 225, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			b := spsUpToShortTermRefPicSets(1, 0)
			b.ue(uint(c.numSets)) // num_short_term_ref_pic_sets
			b.writeChainedShortTermRPSs(c.numSets, c.truncateLast)
			var data []byte
			if c.truncateLast {
				data = b.truncated()
			} else {
				data = b.writeSPSTail()
			}
			mustFailPromptly(t, func() error {
				_, err := ParseSPSNALUnit(data)
				return err
			})
		})
	}
}

func TestSPSCountBounds(t *testing.T) {
	cases := []struct {
		name  string
		build func() []byte
	}{
		{"bit_depth_luma_minus8 out of range", func() []byte {
			return spsUpToShortTermRefPicSets(1, 9).truncated()
		}},
		{"num_long_term_ref_pics_sps out of range", func() []byte {
			b := spsUpToShortTermRefPicSets(1, 0)
			b.ue(0)      // num_short_term_ref_pic_sets
			b.flag(true) // long_term_ref_pics_present_flag
			b.ue(256)    // num_long_term_ref_pics_sps
			return b.truncated()
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			data := c.build()
			mustFailPromptly(t, func() error {
				_, err := ParseSPSNALUnit(data)
				return err
			})
		})
	}
}

func TestSPSSccPaletteBounds(t *testing.T) {
	build := func(bitDepthLumaMinus8, paletteMaxSize, numInitializersMinus1 uint) []byte {
		b := spsUpToShortTermRefPicSets(0, bitDepthLumaMinus8)
		b.ue(0)       // num_short_term_ref_pic_sets
		b.flag(false) // long_term_ref_pics_present_flag
		b.flag(false) // sps_temporal_mvp_enabled_flag
		b.flag(false) // strong_intra_smoothing_enabled_flag
		b.flag(false) // vui_parameters_present_flag
		b.flag(true)  // sps_extension_present_flag
		b.flag(false) // sps_range_extension_flag
		b.flag(false) // sps_multilayer_extension_flag
		b.flag(false) // sps_3d_extension_flag
		b.flag(true)  // sps_scc_extension_flag
		b.u(0, 4)     // sps_extension_4bits
		b.flag(false) // sps_curr_pic_ref_enabled_flag
		b.flag(true)  // palette_mode_enabled_flag
		b.ue(paletteMaxSize)
		b.ue(0)      // delta_palette_max_predictor_size
		b.flag(true) // sps_palette_predictor_initializers_present_flag
		b.ue(numInitializersMinus1)
		return b.truncated()
	}
	cases := []struct {
		name                                                  string
		bitDepthLumaMinus8, paletteMaxSize, numInitializersM1 uint
	}{
		{"bit depth that wraps in a byte", 248, 64, 1<<24 - 1},
		{"palette_max_size out of range", 0, 65, 0},
		{"more initializers than PaletteMaxPredictorSize", 0, 64, 64},
		{"many initializers", 0, 64, 1<<24 - 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			data := build(c.bitDepthLumaMinus8, c.paletteMaxSize, c.numInitializersM1)
			mustFailPromptly(t, func() error {
				_, err := ParseSPSNALUnit(data)
				return err
			})
		})
	}
}

func TestPPSExtensionBounds(t *testing.T) {
	cases := []struct {
		name  string
		build func() []byte
	}{
		{"pps_num_palette_predictor_initializers out of range", func() []byte {
			b := ppsUpToExtensions(false, false, false, true)
			b.flag(false) // pps_curr_pic_ref_enabled_flag
			b.flag(false) // residual_adaptive_colour_transform_enabled_flag
			b.flag(true)  // pps_palette_predictor_initializers_present_flag
			b.ue(1 << 24) // pps_num_palette_predictor_initializers
			b.flag(true)  // monochrome_palette_flag
			b.ue(0)       // luma_bit_depth_entry_minus8
			return b.truncated()
		}},
		{"luma_bit_depth_entry_minus8 out of range", func() []byte {
			b := ppsUpToExtensions(false, false, false, true)
			b.flag(false) // pps_curr_pic_ref_enabled_flag
			b.flag(false) // residual_adaptive_colour_transform_enabled_flag
			b.flag(true)  // pps_palette_predictor_initializers_present_flag
			b.ue(128)     // pps_num_palette_predictor_initializers
			b.flag(true)  // monochrome_palette_flag
			b.ue(1 << 31) // luma_bit_depth_entry_minus8
			return b.truncated()
		}},
		{"pps_bit_depth_for_depth_layers_minus8 out of range", func() []byte {
			b := ppsUpToExtensions(false, false, true, false)
			b.flag(true) // dlts_present_flag
			b.u(63, 6)   // pps_depth_layers_minus1
			b.u(15, 4)   // pps_bit_depth_for_depth_layers_minus8
			for i := 0; i < 64; i++ {
				b.flag(true)  // dlt_flag
				b.flag(false) // dlt_pred_flag
				b.flag(true)  // dlt_val_flags_present_flag
			}
			return b.truncated()
		}},
		{"dlt_value_flag beyond the data", func() []byte {
			b := ppsUpToExtensions(false, false, true, false)
			b.flag(true)  // dlts_present_flag
			b.u(63, 6)    // pps_depth_layers_minus1
			b.u(8, 4)     // pps_bit_depth_for_depth_layers_minus8
			b.flag(true)  // dlt_flag
			b.flag(false) // dlt_pred_flag
			b.flag(true)  // dlt_val_flags_present_flag
			return b.truncated()
		}},
		{"chroma_qp_offset_list_len_minus1 out of range", func() []byte {
			b := ppsUpToExtensions(true, false, false, false)
			b.flag(false) // cross_component_prediction_enabled_flag
			b.flag(true)  // chroma_qp_offset_list_enabled_flag
			b.ue(0)       // diff_cu_chroma_qp_offset_depth
			b.ue(6)       // chroma_qp_offset_list_len_minus1
			return b.truncated()
		}},
		{"num_cm_ref_layers_minus1 out of range", func() []byte {
			b := ppsUpToExtensions(false, true, false, false)
			b.flag(false) // poc_reset_info_present_flag
			b.flag(false) // pps_infer_scaling_list_flag
			b.ue(0)       // num_ref_loc_offsets
			b.flag(true)  // colour_mapping_enabled_flag
			b.ue(255)     // num_cm_ref_layers_minus1
			return b.truncated()
		}},
	}
	spsMap := map[uint32]*SPS{0: {}}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			data := c.build()
			mustFailPromptly(t, func() error {
				_, err := ParsePPSNALUnit(data, spsMap)
				return err
			})
		})
	}
}

func TestVPSExtensionCountBounds(t *testing.T) {
	cases := []struct {
		name  string
		build func() []byte
	}{
		{"vps_num_profile_tier_level_minus1 out of range", func() []byte {
			b := vpsUpToNumProfileTierLevel()
			b.ue(1 << 21) // vps_num_profile_tier_level_minus1
			return b.truncated()
		}},
		{"num_add_olss out of range", func() []byte {
			b := vpsUpToNumProfileTierLevel()
			b.ue(0)       // vps_num_profile_tier_level_minus1
			b.ue(1 << 30) // num_add_olss
			b.u(0, 2)     // default_output_layer_idc
			return b.truncated()
		}},
		{"num_add_olss beyond the data", func() []byte {
			b := vpsUpToNumProfileTierLevel()
			b.ue(0)             // vps_num_profile_tier_level_minus1
			b.ue(maxNumAddOlss) // num_add_olss
			b.u(0, 2)           // default_output_layer_idc
			return b.truncated()
		}},
		{"vps_num_rep_formats_minus1 out of range", func() []byte {
			b := vpsUpToNumProfileTierLevel()
			b.ue(0)       // vps_num_profile_tier_level_minus1
			b.ue(0)       // num_add_olss
			b.u(0, 2)     // default_output_layer_idc
			b.ue(1 << 22) // vps_num_rep_formats_minus1
			return b.truncated()
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			data := c.build()
			mustFailPromptly(t, func() error {
				_, err := ParseVPSNALUnit(data)
				return err
			})
		})
	}
}
