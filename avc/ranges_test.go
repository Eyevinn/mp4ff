package avc

import (
	"bytes"
	"math"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/go-test/deep"
)

// spsFields are the SPS syntax elements varied by the range tests. The SPS is
// High profile with frame_mbs_only_flag 1, and has no scaling matrix and no VUI
// unless the fields ask for them.
type spsFields struct {
	seqParameterSetID         uint
	chromaFormatIDC           uint
	bitDepthLumaMinus8        uint
	bitDepthChromaMinus8      uint
	log2MaxFrameNumMinus4     uint
	log2MaxPocLsbMinus4       uint
	picWidthInMbsMinus1       uint
	picHeightInMapUnitsMinus1 uint
	crop                      []uint // left, right, top, bottom offsets, if cropping
	nalHrdCpbCntMinus1        *uint  // cpb_cnt_minus1 of a VUI with NAL HRD parameters, if set
	scalingList0Deltas        []int  // delta_scale values of the first scaling list, if a scaling matrix
	picOrderCntType           uint
}

func defaultSPSFields() spsFields {
	return spsFields{chromaFormatIDC: 1} // 4:2:0, 8-bit, 16x16 pixels
}

func writeSPS(t *testing.T, f spsFields) []byte {
	t.Helper()
	buf := bytes.Buffer{}
	w := bits.NewEBSPWriter(&buf)
	w.Write(0x67, 8) // NAL header: nal_ref_idc=3, nal_unit_type=7 (SPS)
	w.Write(100, 8)  // profile_idc (High)
	w.Write(0, 8)    // constraint_set flags
	w.Write(40, 8)   // level_idc
	w.WriteExpGolomb(f.seqParameterSetID)
	w.WriteExpGolomb(f.chromaFormatIDC)
	if f.chromaFormatIDC == 3 {
		w.Write(0, 1) // separate_colour_plane_flag
	}
	w.WriteExpGolomb(f.bitDepthLumaMinus8)
	w.WriteExpGolomb(f.bitDepthChromaMinus8)
	w.Write(0, 1) // qpprime_y_zero_transform_bypass_flag
	if f.scalingList0Deltas != nil {
		w.Write(1, 1) // seq_scaling_matrix_present_flag
		w.Write(1, 1) // seq_scaling_list_present_flag[0]
		writeScalingList(w, f.scalingList0Deltas, 16)
		w.Write(0, 7) // seq_scaling_list_present_flag[1..7]
	} else {
		w.Write(0, 1) // seq_scaling_matrix_present_flag
	}
	w.WriteExpGolomb(f.log2MaxFrameNumMinus4)
	w.WriteExpGolomb(f.picOrderCntType)
	if f.picOrderCntType == 0 {
		w.WriteExpGolomb(f.log2MaxPocLsbMinus4)
	}
	w.WriteExpGolomb(1) // max_num_ref_frames
	w.Write(0, 1)       // gaps_in_frame_num_value_allowed_flag
	w.WriteExpGolomb(f.picWidthInMbsMinus1)
	w.WriteExpGolomb(f.picHeightInMapUnitsMinus1)
	w.Write(1, 1) // frame_mbs_only_flag
	w.Write(1, 1) // direct_8x8_inference_flag
	if f.crop != nil {
		w.Write(1, 1) // frame_cropping_flag
		for _, offset := range f.crop {
			w.WriteExpGolomb(offset)
		}
	} else {
		w.Write(0, 1)
	}
	if f.nalHrdCpbCntMinus1 != nil {
		w.Write(1, 1) // vui_parameters_present_flag
		w.Write(0, 5) // aspect_ratio_info, overscan_info, video_signal_type, chroma_loc_info, timing_info present flags
		w.Write(1, 1) // nal_hrd_parameters_present_flag
		cpbCntMinus1 := *f.nalHrdCpbCntMinus1
		w.WriteExpGolomb(cpbCntMinus1)
		w.Write(0, 4) // bit_rate_scale
		w.Write(0, 4) // cpb_size_scale
		// Write at most 32 entries, since the parser must stop at a larger count.
		for i := uint(0); i <= min(cpbCntMinus1, 31); i++ {
			w.WriteExpGolomb(0) // bit_rate_value_minus1
			w.WriteExpGolomb(0) // cpb_size_value_minus1
			w.Write(0, 1)       // cbr_flag
		}
		w.Write(23, 5) // initial_cpb_removal_delay_length_minus1
		w.Write(23, 5) // cpb_removal_delay_length_minus1
		w.Write(23, 5) // dpb_output_delay_length_minus1
		w.Write(24, 5) // time_offset_length
		w.Write(0, 1)  // vcl_hrd_parameters_present_flag
		w.Write(0, 1)  // low_delay_hrd_flag
		w.Write(0, 1)  // pic_struct_present_flag
		w.Write(0, 1)  // bitstream_restriction_flag
	} else {
		w.Write(0, 1) // vui_parameters_present_flag
	}
	w.WriteRbspTrailingBits()
	if w.AccError() != nil {
		t.Fatal(w.AccError())
	}
	return buf.Bytes()
}

// ppsFields are the PPS syntax elements varied by the range tests. The PPS has
// deblocking_filter_control_present_flag set.
type ppsFields struct {
	seqParameterSetID              uint
	entropyCodingModeFlag          bool
	numSliceGroupsMinus1           uint
	sliceGroups                    func(w *bits.EBSPWriter) // writes slice_group_map_type and its parameters
	numRefIdxL0DefaultActiveMinus1 uint
	numRefIdxL1DefaultActiveMinus1 uint
	weightedPredFlag               bool
	weightedBipredIDC              uint
	picInitQpMinus26               int
	picInitQsMinus26               int
	chromaQpIndexOffset            int
	secondChromaQpIndexOffset      *int  // written with the High profile extension if set
	scalingList0Deltas             []int // delta_scale values of the first 4x4 list of a scaling matrix in the extension
}

func writePPS(t *testing.T, f ppsFields) []byte {
	t.Helper()
	buf := bytes.Buffer{}
	w := bits.NewEBSPWriter(&buf)
	w.Write(0x68, 8)    // NAL header: nal_ref_idc=3, nal_unit_type=8 (PPS)
	w.WriteExpGolomb(0) // pic_parameter_set_id
	w.WriteExpGolomb(f.seqParameterSetID)
	if f.entropyCodingModeFlag {
		w.Write(1, 1)
	} else {
		w.Write(0, 1)
	}
	w.Write(0, 1) // bottom_field_pic_order_in_frame_present_flag
	w.WriteExpGolomb(f.numSliceGroupsMinus1)
	if f.numSliceGroupsMinus1 > 0 {
		f.sliceGroups(w)
	}
	w.WriteExpGolomb(f.numRefIdxL0DefaultActiveMinus1)
	w.WriteExpGolomb(f.numRefIdxL1DefaultActiveMinus1)
	if f.weightedPredFlag {
		w.Write(1, 1)
	} else {
		w.Write(0, 1)
	}
	w.Write(f.weightedBipredIDC, 2)
	writeSignedGolomb(w, f.picInitQpMinus26)
	writeSignedGolomb(w, f.picInitQsMinus26)
	writeSignedGolomb(w, f.chromaQpIndexOffset)
	w.Write(1, 1) // deblocking_filter_control_present_flag
	w.Write(0, 1) // constrained_intra_pred_flag
	w.Write(0, 1) // redundant_pic_cnt_present_flag
	if f.secondChromaQpIndexOffset != nil || f.scalingList0Deltas != nil {
		w.Write(0, 1) // transform_8x8_mode_flag
		if f.scalingList0Deltas != nil {
			w.Write(1, 1) // pic_scaling_matrix_present_flag
			w.Write(1, 1) // pic_scaling_list_present_flag[0]
			writeScalingList(w, f.scalingList0Deltas, 16)
			w.Write(0, 5) // pic_scaling_list_present_flag[1..5]
		} else {
			w.Write(0, 1) // pic_scaling_matrix_present_flag
		}
		secondChromaQpIndexOffset := 0
		if f.secondChromaQpIndexOffset != nil {
			secondChromaQpIndexOffset = *f.secondChromaQpIndexOffset
		}
		writeSignedGolomb(w, secondChromaQpIndexOffset)
	}
	w.WriteRbspTrailingBits()
	if w.AccError() != nil {
		t.Fatal(w.AccError())
	}
	return buf.Bytes()
}

// writeScalingList writes the delta_scale values of a scaling_list() of size entries,
// followed by zero deltas, as many as the parser reads.
func writeScalingList(w *bits.EBSPWriter, deltas []int, size int) {
	lastScale, nextScale := 8, 8
	for j := 0; j < size; j++ {
		if nextScale != 0 {
			delta := 0
			if j < len(deltas) {
				delta = deltas[j]
			}
			writeSignedGolomb(w, delta)
			nextScale = (lastScale + delta + 256) % 256
		}
		if nextScale != 0 {
			lastScale = nextScale
		}
	}
}

// paramSetMaps parses an SPS and a PPS into maps for ParseSliceHeader.
func paramSetMaps(t *testing.T, sf spsFields, pf ppsFields) (map[uint32]*SPS, map[uint32]*PPS) {
	t.Helper()
	sps, err := ParseSPSNALUnit(writeSPS(t, sf), false)
	if err != nil {
		t.Fatalf("parse SPS: %v", err)
	}
	spsMap := map[uint32]*SPS{sps.ParameterID: sps}
	pps, err := ParsePPSNALUnit(writePPS(t, pf), spsMap)
	if err != nil {
		t.Fatalf("parse PPS: %v", err)
	}
	return spsMap, map[uint32]*PPS{pps.PicParameterSetID: pps}
}

// writeIDRSlice writes an IDR I slice NAL unit with the given slice_qp_delta,
// matching the parameter sets above, followed by one byte of slice data.
func writeIDRSlice(t *testing.T, sliceQPDelta int) []byte {
	t.Helper()
	return writeIDRSliceDeblocking(t, sliceQPDelta, deblockingFields{disableDeblockingFilterIDC: 1})
}

// deblockingFields are the deblocking filter syntax elements of a slice header.
// The offsets are only written if disableDeblockingFilterIDC is not 1.
type deblockingFields struct {
	disableDeblockingFilterIDC uint
	sliceAlphaC0OffsetDiv2     int
	sliceBetaOffsetDiv2        int
}

// writeIDRSliceDeblocking is writeIDRSlice with the given deblocking filter
// syntax elements.
func writeIDRSliceDeblocking(t *testing.T, sliceQPDelta int, d deblockingFields) []byte {
	t.Helper()
	buf := bytes.Buffer{}
	w := bits.NewEBSPWriter(&buf)
	w.Write(0x65, 8)    // NAL header: nal_ref_idc=3, nal_unit_type=5 (IDR)
	w.WriteExpGolomb(0) // first_mb_in_slice
	w.WriteExpGolomb(7) // slice_type (I)
	w.WriteExpGolomb(0) // pic_parameter_set_id
	w.Write(0, 4)       // frame_num
	w.WriteExpGolomb(0) // idr_pic_id
	w.Write(0, 4)       // pic_order_cnt_lsb
	w.Write(0, 1)       // no_output_of_prior_pics_flag
	w.Write(0, 1)       // long_term_reference_flag
	writeSignedGolomb(w, sliceQPDelta)
	w.WriteExpGolomb(d.disableDeblockingFilterIDC)
	if d.disableDeblockingFilterIDC != 1 {
		writeSignedGolomb(w, d.sliceAlphaC0OffsetDiv2)
		writeSignedGolomb(w, d.sliceBetaOffsetDiv2)
	}
	w.Write(0xff, 8) // slice data
	w.WriteRbspTrailingBits()
	if w.AccError() != nil {
		t.Fatal(w.AccError())
	}
	return buf.Bytes()
}

// writePSlice writes a P slice NAL unit that overrides num_ref_idx_l0_active_minus1.
// With weighted_pred_flag set in the PPS, it is followed by a pred_weight_table
// with numRefIdxL0ActiveMinus1 + 1 entries.
func writePSlice(t *testing.T, numRefIdxL0ActiveMinus1 uint, predWeightTable bool) []byte {
	t.Helper()
	buf := bytes.Buffer{}
	w := bits.NewEBSPWriter(&buf)
	w.Write(0x41, 8)    // NAL header: nal_ref_idc=2, nal_unit_type=1 (non-IDR)
	w.WriteExpGolomb(0) // first_mb_in_slice
	w.WriteExpGolomb(0) // slice_type (P)
	w.WriteExpGolomb(0) // pic_parameter_set_id
	w.Write(1, 4)       // frame_num
	w.Write(2, 4)       // pic_order_cnt_lsb
	w.Write(1, 1)       // num_ref_idx_active_override_flag
	w.WriteExpGolomb(numRefIdxL0ActiveMinus1)
	w.Write(0, 1) // ref_pic_list_modification_flag_l0
	if predWeightTable {
		w.WriteExpGolomb(0) // luma_log2_weight_denom
		w.WriteExpGolomb(0) // chroma_log2_weight_denom
		for i := uint(0); i <= numRefIdxL0ActiveMinus1; i++ {
			w.Write(0, 1) // luma_weight_l0_flag
			w.Write(0, 1) // chroma_weight_l0_flag
		}
		w.Write(0, 1)           // adaptive_ref_pic_marking_mode_flag
		writeSignedGolomb(w, 0) // slice_qp_delta
		w.WriteExpGolomb(1)     // disable_deblocking_filter_idc
		w.Write(0xff, 8)        // slice data
	}
	w.WriteRbspTrailingBits()
	if w.AccError() != nil {
		t.Fatal(w.AccError())
	}
	return buf.Bytes()
}

// checkErr fails unless err is nil when wantErr is empty, or contains wantErr.
func checkErr(t *testing.T, err error, wantErr string) {
	t.Helper()
	switch {
	case wantErr == "" && err != nil:
		t.Errorf("unexpected error: %v", err)
	case wantErr != "" && err == nil:
		t.Errorf("expected an error containing %q", wantErr)
	case wantErr != "" && !strings.Contains(err.Error(), wantErr):
		t.Errorf("expected an error containing %q, got %q", wantErr, err.Error())
	}
}

// TestSPSParserRanges verifies that SPS values outside the ranges of
// ISO/IEC 14496-10 Section 7.4.2.1.1 are rejected. Without the checks, a
// chroma_format_idc of 257 was truncated to 1, and frame cropping larger than
// the picture made Width and Height wrap around to huge values.
func TestSPSParserRanges(t *testing.T) {
	cases := []struct {
		name    string
		modify  func(f *spsFields)
		wantErr string
	}{
		{"valid", func(f *spsFields) {}, ""},
		{"chroma_format_idc 4", func(f *spsFields) { f.chromaFormatIDC = 4 }, "chroma_format_idc"},
		{"chroma_format_idc 257", func(f *spsFields) { f.chromaFormatIDC = 257 }, "chroma_format_idc"},
		{"bit_depth_luma_minus8 6", func(f *spsFields) { f.bitDepthLumaMinus8 = 6 }, ""},
		{"bit_depth_luma_minus8 7", func(f *spsFields) { f.bitDepthLumaMinus8 = 7 }, "bit_depth_luma_minus8"},
		{"bit_depth_chroma_minus8 7", func(f *spsFields) { f.bitDepthChromaMinus8 = 7 }, "bit_depth_chroma_minus8"},
		{"log2_max_frame_num_minus4 12", func(f *spsFields) { f.log2MaxFrameNumMinus4 = 12 }, ""},
		{"log2_max_frame_num_minus4 13", func(f *spsFields) { f.log2MaxFrameNumMinus4 = 13 }, "log2_max_frame_num_minus4"},
		{"log2_max_pic_order_cnt_lsb_minus4 13", func(f *spsFields) { f.log2MaxPocLsbMinus4 = 13 },
			"log2_max_pic_order_cnt_lsb_minus4"},
		{"pic_order_cnt_type 2", func(f *spsFields) { f.picOrderCntType = 2 }, ""},
		{"pic_order_cnt_type 3", func(f *spsFields) { f.picOrderCntType = 3 }, "pic_order_cnt_type"},
		{"width at MaxFS", func(f *spsFields) { f.picWidthInMbsMinus1 = maxFrameSizeInMbs - 1 }, ""},
		{"width above MaxFS", func(f *spsFields) { f.picWidthInMbsMinus1 = maxFrameSizeInMbs }, "pic_width_in_mbs_minus1"},
		{"height above MaxFS", func(f *spsFields) { f.picHeightInMapUnitsMinus1 = maxFrameSizeInMbs },
			"pic_height_in_map_units_minus1"},
		// 16x16 pixels in 4:2:0 has CropUnitX = CropUnitY = 2, so at most 7 crop units per direction.
		{"crop leaving 2x2 pixels", func(f *spsFields) { f.crop = []uint{3, 4, 4, 3} }, ""},
		{"crop left+right = width", func(f *spsFields) { f.crop = []uint{4, 4, 0, 0} }, "frame cropping"},
		{"crop top+bottom = height", func(f *spsFields) { f.crop = []uint{0, 0, 0, 8} }, "frame cropping"},
		{"crop right far beyond width", func(f *spsFields) { f.crop = []uint{0, 100, 0, 0} }, "frame cropping"},
		{"crop right 2^32-2", func(f *spsFields) { f.crop = []uint{1, math.MaxUint32 - 1, 0, 0} }, "frame cropping"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := defaultSPSFields()
			c.modify(&f)
			sps, err := ParseSPSNALUnit(writeSPS(t, f), false)
			checkErr(t, err, c.wantErr)
			if c.wantErr != "" && sps != nil {
				t.Errorf("expected nil SPS on error, got %+v", sps)
			}
		})
	}
}

// TestSPSParserHrdCpbCnt verifies that cpb_cnt_minus1 in the VUI HRD parameters
// is limited to 31 (ISO/IEC 14496-10 Section E.2.2). A bogus value made the
// parser append CPB entries until it ran out of memory, or forever for
// 2^64-1. Found by fuzzing.
func TestSPSParserHrdCpbCnt(t *testing.T) {
	cases := []struct {
		cpbCntMinus1 uint
		wantErr      string
	}{
		{0, ""},
		{31, ""},
		{32, "cpb_cnt_minus1"},
		{1 << 20, "cpb_cnt_minus1"},
	}
	for _, c := range cases {
		f := defaultSPSFields()
		f.nalHrdCpbCntMinus1 = &c.cpbCntMinus1
		sps, err := ParseSPSNALUnit(writeSPS(t, f), true)
		checkErr(t, err, c.wantErr)
		if c.wantErr == "" && err == nil {
			hrd := sps.VUI.NalHrdParameters
			if hrd == nil || len(hrd.CpbEntries) != int(c.cpbCntMinus1)+1 || hrd.TimeOffsetLength != 24 {
				t.Errorf("cpb_cnt_minus1 %d: got HRD parameters %+v", c.cpbCntMinus1, hrd)
			}
		}
	}
}

// TestPPSParserRanges verifies that PPS values outside the ranges of
// ISO/IEC 14496-10 Section 7.4.2.2 are rejected.
func TestPPSParserRanges(t *testing.T) {
	intPtr := func(v int) *int { return &v }
	cases := []struct {
		name    string
		modify  func(f *ppsFields)
		wantErr string
	}{
		{"valid", func(f *ppsFields) {}, ""},
		{"valid with second_chroma_qp_index_offset", func(f *ppsFields) { f.secondChromaQpIndexOffset = intPtr(-12) }, ""},
		{"num_ref_idx_l0_default_active_minus1 31", func(f *ppsFields) { f.numRefIdxL0DefaultActiveMinus1 = 31 }, ""},
		{"num_ref_idx_l0_default_active_minus1 32", func(f *ppsFields) { f.numRefIdxL0DefaultActiveMinus1 = 32 },
			"num_ref_idx_l0_default_active_minus1"},
		{"num_ref_idx_l1_default_active_minus1 32", func(f *ppsFields) { f.numRefIdxL1DefaultActiveMinus1 = 32 },
			"num_ref_idx_l1_default_active_minus1"},
		{"pic_init_qp_minus26 25", func(f *ppsFields) { f.picInitQpMinus26 = 25 }, ""},
		{"pic_init_qp_minus26 26", func(f *ppsFields) { f.picInitQpMinus26 = 26 }, "pic_init_qp_minus26"},
		{"pic_init_qp_minus26 -62", func(f *ppsFields) { f.picInitQpMinus26 = -62 }, ""},
		{"pic_init_qp_minus26 -63", func(f *ppsFields) { f.picInitQpMinus26 = -63 }, "pic_init_qp_minus26"},
		{"pic_init_qs_minus26 -27", func(f *ppsFields) { f.picInitQsMinus26 = -27 }, "pic_init_qs_minus26"},
		{"chroma_qp_index_offset 13", func(f *ppsFields) { f.chromaQpIndexOffset = 13 }, "chroma_qp_index_offset"},
		{"chroma_qp_index_offset -13", func(f *ppsFields) { f.chromaQpIndexOffset = -13 }, "chroma_qp_index_offset"},
		{"second_chroma_qp_index_offset 13", func(f *ppsFields) { f.secondChromaQpIndexOffset = intPtr(13) },
			"second_chroma_qp_index_offset"},
		{"weighted_bipred_idc 2", func(f *ppsFields) { f.weightedBipredIDC = 2 }, ""},
		{"weighted_bipred_idc 3", func(f *ppsFields) { f.weightedBipredIDC = 3 }, "weighted_bipred_idc"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var f ppsFields
			c.modify(&f)
			pps, err := ParsePPSNALUnit(writePPS(t, f), nil)
			checkErr(t, err, c.wantErr)
			if c.wantErr != "" && pps != nil {
				t.Errorf("expected nil PPS on error, got %+v", pps)
			}
		})
	}
}

// TestParseSliceHeaderSliceQPY verifies that slice_qp_delta is limited so that
// SliceQPY = 26 + pic_init_qp_minus26 + slice_qp_delta is in the range
// -QpBdOffsetY to 51 (ISO/IEC 14496-10 Section 7.4.3). -3085 was found by
// fuzzing a decoder, which then indexed its dequantization tables with a
// negative QP.
func TestParseSliceHeaderSliceQPY(t *testing.T) {
	cases := []struct {
		name               string
		bitDepthLumaMinus8 uint
		picInitQpMinus26   int
		sliceQPDelta       int
		wantErr            string
	}{
		{"QP 51", 0, 0, 25, ""},
		{"QP 52", 0, 0, 26, "SliceQPY"},
		{"QP 0", 0, 0, -26, ""},
		{"QP -1 at 8 bits", 0, 0, -27, "SliceQPY"},
		{"QP -12 at 10 bits", 2, 0, -38, ""},
		{"QP -13 at 10 bits", 2, 0, -39, "SliceQPY"},
		{"QP 51 from pic_init_qp_minus26", 0, 25, 0, ""},
		{"QP -26 from both", 0, -26, -26, "SliceQPY"},
		{"fuzzed slice_qp_delta", 0, 0, -3085, "SliceQPY"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sf := defaultSPSFields()
			sf.bitDepthLumaMinus8 = c.bitDepthLumaMinus8
			spsMap, ppsMap := paramSetMaps(t, sf, ppsFields{picInitQpMinus26: c.picInitQpMinus26})
			sh, err := ParseSliceHeader(writeIDRSlice(t, c.sliceQPDelta), spsMap, ppsMap)
			checkErr(t, err, c.wantErr)
			if c.wantErr == "" && err == nil && int(sh.SliceQPDelta) != c.sliceQPDelta {
				t.Errorf("got slice_qp_delta %d, want %d", sh.SliceQPDelta, c.sliceQPDelta)
			}
		})
	}
}

// TestParseSliceHeaderDeblocking verifies that disable_deblocking_filter_idc is
// limited to 0..2 and slice_alpha_c0_offset_div2 and slice_beta_offset_div2 to
// -6..+6 (ISO/IEC 14496-10 Section 7.4.3). The offsets shift the indices into
// the deblocking filter tables. The fuzzed values below made a decoder index
// those tables out of range.
func TestParseSliceHeaderDeblocking(t *testing.T) {
	cases := []struct {
		name    string
		d       deblockingFields
		wantErr string
	}{
		{"enabled", deblockingFields{0, 0, 0}, ""},
		{"offsets -6", deblockingFields{0, -6, -6}, ""},
		{"offsets 6", deblockingFields{0, 6, 6}, ""},
		{"idc 2 offsets 6", deblockingFields{2, 6, -6}, ""},
		{"alpha 7", deblockingFields{0, 7, 0}, "slice_alpha_c0_offset_div2"},
		{"alpha -7", deblockingFields{0, -7, 0}, "slice_alpha_c0_offset_div2"},
		{"beta 7", deblockingFields{0, 0, 7}, "slice_beta_offset_div2"},
		{"beta -7", deblockingFields{0, 0, -7}, "slice_beta_offset_div2"},
		{"idc 3", deblockingFields{3, 0, 0}, "disable_deblocking_filter_idc"},
		{"idc 2^32-2", deblockingFields{math.MaxUint32 - 1, 0, 0}, "disable_deblocking_filter_idc"},
		{"fuzzed", deblockingFields{5, 24, 771}, "disable_deblocking_filter_idc"},
		{"fuzzed offsets", deblockingFields{0, 24, 771}, "slice_alpha_c0_offset_div2"},
		{"fuzzed beta offset", deblockingFields{0, 0, 771}, "slice_beta_offset_div2"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			spsMap, ppsMap := paramSetMaps(t, defaultSPSFields(), ppsFields{})
			sh, err := ParseSliceHeader(writeIDRSliceDeblocking(t, 0, c.d), spsMap, ppsMap)
			checkErr(t, err, c.wantErr)
			if c.wantErr != "" || err != nil {
				return
			}
			if got := (deblockingFields{uint(sh.DisableDeblockingFilterIDC),
				int(sh.SliceAlphaC0OffsetDiv2), int(sh.SliceBetaOffsetDiv2)}); got != c.d {
				t.Errorf("got %+v, want %+v", got, c.d)
			}
		})
	}
}

// parseSliceHeaderWithTimeout runs ParseSliceHeader, failing the test instead
// of hanging if it does not return.
func parseSliceHeaderWithTimeout(t *testing.T, nalu []byte, spsMap map[uint32]*SPS,
	ppsMap map[uint32]*PPS) (*SliceHeader, error) {
	t.Helper()
	type result struct {
		sh  *SliceHeader
		err error
	}
	done := make(chan result, 1)
	go func() {
		sh, err := ParseSliceHeader(nalu, spsMap, ppsMap)
		done <- result{sh, err}
	}()
	select {
	case r := <-done:
		return r.sh, r.err
	case <-time.After(5 * time.Second):
		t.Fatalf("ParseSliceHeader did not return within 5s for a %d-byte NAL unit", len(nalu))
		return nil, nil
	}
}

// TestParseSliceHeaderNumRefIdxActive verifies that num_ref_idx_l0_active_minus1
// is limited to 31 (ISO/IEC 14496-10 Section 7.4.3). It sets the number of
// pred_weight_table entries, and 0xFFFFFFFF made that loop endless, since its
// uint32 counter can never exceed it.
func TestParseSliceHeaderNumRefIdxActive(t *testing.T) {
	spsMap, ppsMap := paramSetMaps(t, defaultSPSFields(), ppsFields{weightedPredFlag: true})

	sh, err := parseSliceHeaderWithTimeout(t, writePSlice(t, 31, true), spsMap, ppsMap)
	checkErr(t, err, "")
	if err == nil && sh.NumRefIdxL0ActiveMinus1 != 31 {
		t.Errorf("got num_ref_idx_l0_active_minus1 %d, want 31", sh.NumRefIdxL0ActiveMinus1)
	}
	for _, n := range []uint{32, math.MaxUint32 - 1} {
		_, err := parseSliceHeaderWithTimeout(t, writePSlice(t, n, false), spsMap, ppsMap)
		checkErr(t, err, "num_ref_idx_l0_active_minus1")
	}
}

// TestParseSliceHeaderTruncated verifies that a NAL unit ending inside the
// slice header is an error, instead of a header with zero values for the
// missing fields.
func TestParseSliceHeaderTruncated(t *testing.T) {
	spsMap, ppsMap := paramSetMaps(t, defaultSPSFields(), ppsFields{})
	nalu := writeIDRSlice(t, 0)
	full, err := ParseSliceHeader(nalu, spsMap, ppsMap)
	if err != nil {
		t.Fatalf("full NAL unit: %v", err)
	}
	// full.Size counts the NAL header byte and the partly read last byte.
	for n := 1; n < int(full.Size); n++ {
		sh, err := ParseSliceHeader(nalu[:n], spsMap, ppsMap)
		if err == nil {
			t.Errorf("%d of %d bytes: expected an error, got header %+v", n, len(nalu), sh)
		}
	}
}

// sliceFields are the slice header syntax elements varied by the range tests.
type sliceFields struct {
	sliceType                 uint
	cabacInitIDC              uint // written for CABAC slices other than I and SI
	lumaLog2WeightDenom       uint // written with a pred_weight_table
	chromaLog2WeightDenom     uint // written with a pred_weight_table
	sliceQSDelta              int  // written for SP and SI slices
	sliceGroupChangeCycle     uint // written in sliceGroupChangeCycleBits bits, if not 0
	sliceGroupChangeCycleBits int
}

// writeSlice writes a non-IDR slice NAL unit with nal_ref_idc 2 for an SPS from writeSPS with
// pic_order_cnt_type 0 and a PPS from writePPS with the fields pf, followed by one byte of slice data.
func writeSlice(t *testing.T, pf ppsFields, f sliceFields) []byte {
	t.Helper()
	buf := bytes.Buffer{}
	w := bits.NewEBSPWriter(&buf)
	sliceType := SliceType(f.sliceType % 5)
	w.Write(0x41, 8)    // NAL header: nal_ref_idc=2, nal_unit_type=1 (non-IDR)
	w.WriteExpGolomb(0) // first_mb_in_slice
	w.WriteExpGolomb(f.sliceType)
	w.WriteExpGolomb(0) // pic_parameter_set_id
	w.Write(1, 4)       // frame_num
	w.Write(2, 4)       // pic_order_cnt_lsb
	if sliceType == SLICE_B {
		w.Write(1, 1) // direct_spatial_mv_pred_flag
	}
	if sliceType == SLICE_P || sliceType == SLICE_SP || sliceType == SLICE_B {
		w.Write(0, 1) // num_ref_idx_active_override_flag
	}
	if sliceType != SLICE_I && sliceType != SLICE_SI {
		w.Write(0, 1) // ref_pic_list_modification_flag_l0
	}
	if sliceType == SLICE_B {
		w.Write(0, 1) // ref_pic_list_modification_flag_l1
	}
	if pf.weightedPredFlag && (sliceType == SLICE_P || sliceType == SLICE_SP) ||
		pf.weightedBipredIDC == 1 && sliceType == SLICE_B {
		w.WriteExpGolomb(f.lumaLog2WeightDenom)
		w.WriteExpGolomb(f.chromaLog2WeightDenom)
		w.Write(0, 2) // luma_weight_l0_flag[0], chroma_weight_l0_flag[0]
		if sliceType == SLICE_B {
			w.Write(0, 2) // luma_weight_l1_flag[0], chroma_weight_l1_flag[0]
		}
	}
	w.Write(0, 1) // adaptive_ref_pic_marking_mode_flag
	if pf.entropyCodingModeFlag && sliceType != SLICE_I && sliceType != SLICE_SI {
		w.WriteExpGolomb(f.cabacInitIDC)
	}
	writeSignedGolomb(w, 0) // slice_qp_delta
	if sliceType == SLICE_SP {
		w.Write(0, 1) // sp_for_switch_flag
	}
	if sliceType == SLICE_SP || sliceType == SLICE_SI {
		writeSignedGolomb(w, f.sliceQSDelta)
	}
	w.WriteExpGolomb(1) // disable_deblocking_filter_idc
	if f.sliceGroupChangeCycleBits > 0 {
		w.Write(f.sliceGroupChangeCycle, f.sliceGroupChangeCycleBits)
	}
	w.Write(0xff, 8) // slice data
	w.WriteRbspTrailingBits()
	if w.AccError() != nil {
		t.Fatal(w.AccError())
	}
	return buf.Bytes()
}

// TestParseSliceHeaderRanges verifies that slice_type (7.4.3), cabac_init_idc (7.4.3) and the
// pred_weight_table denominators (7.4.3.2) are rejected outside their ranges.
func TestParseSliceHeaderRanges(t *testing.T) {
	cases := []struct {
		name    string
		pf      ppsFields
		f       sliceFields
		wantErr string
	}{
		{"slice_type 9", ppsFields{}, sliceFields{sliceType: 9}, ""},
		{"slice_type 10", ppsFields{}, sliceFields{sliceType: 10}, "slice_type"},
		{"cabac_init_idc 2", ppsFields{entropyCodingModeFlag: true}, sliceFields{cabacInitIDC: 2}, ""},
		{"cabac_init_idc 3", ppsFields{entropyCodingModeFlag: true}, sliceFields{cabacInitIDC: 3}, "cabac_init_idc"},
		{"weight denominators 7", ppsFields{weightedPredFlag: true},
			sliceFields{lumaLog2WeightDenom: 7, chromaLog2WeightDenom: 7}, ""},
		{"luma_log2_weight_denom 8", ppsFields{weightedPredFlag: true}, sliceFields{lumaLog2WeightDenom: 8},
			"luma_log2_weight_denom"},
		{"chroma_log2_weight_denom 8", ppsFields{weightedPredFlag: true}, sliceFields{chromaLog2WeightDenom: 8},
			"chroma_log2_weight_denom"},
		{"B slice chroma_log2_weight_denom 8", ppsFields{weightedBipredIDC: 1},
			sliceFields{sliceType: 1, chromaLog2WeightDenom: 8}, "chroma_log2_weight_denom"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			spsMap, ppsMap := paramSetMaps(t, defaultSPSFields(), c.pf)
			sh, err := ParseSliceHeader(writeSlice(t, c.pf, c.f), spsMap, ppsMap)
			checkErr(t, err, c.wantErr)
			if c.wantErr != "" || err != nil {
				return
			}
			got := sliceFields{sliceType: uint(sh.SliceType), cabacInitIDC: uint(sh.CabacInitIDC),
				lumaLog2WeightDenom: uint(sh.LumaLog2WeightDenom), chromaLog2WeightDenom: uint(sh.ChromaLog2WeightDenom)}
			if got != c.f {
				t.Errorf("got %+v, want %+v", got, c.f)
			}
		})
	}
}

// TestParseSliceHeaderSliceQSY verifies that slice_qs_delta is limited so that
// QSY = 26 + pic_init_qs_minus26 + slice_qs_delta is in the range 0 to 51 (7.4.3).
func TestParseSliceHeaderSliceQSY(t *testing.T) {
	cases := []struct {
		name             string
		sliceType        uint
		picInitQsMinus26 int
		sliceQSDelta     int
		wantErr          string
	}{
		{"SP QS 0", 3, 0, -26, ""},
		{"SP QS -1", 3, 0, -27, "QSY"},
		{"SP QS 51", 8, 0, 25, ""},
		{"SP QS 52", 8, 0, 26, "QSY"},
		{"SI QS 51 from pic_init_qs_minus26", 4, 25, 0, ""},
		{"SI QS 52", 9, 25, 1, "QSY"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			pf := ppsFields{picInitQsMinus26: c.picInitQsMinus26}
			spsMap, ppsMap := paramSetMaps(t, defaultSPSFields(), pf)
			nalu := writeSlice(t, pf, sliceFields{sliceType: c.sliceType, sliceQSDelta: c.sliceQSDelta})
			sh, err := ParseSliceHeader(nalu, spsMap, ppsMap)
			checkErr(t, err, c.wantErr)
			if c.wantErr == "" && err == nil && int(sh.SliceQSDelta) != c.sliceQSDelta {
				t.Errorf("got slice_qs_delta %d, want %d", sh.SliceQSDelta, c.sliceQSDelta)
			}
		})
	}
}

// TestParseSliceHeaderSliceGroupChangeCycle verifies the length and range of
// slice_group_change_cycle for slice group map types 3 to 5 (7.4.3) in a 176x144 picture,
// which has 99 map units, and that slice_group_change_rate_minus1 must be less than 99 (7.4.2.2).
func TestParseSliceHeaderSliceGroupChangeCycle(t *testing.T) {
	cases := []struct {
		name             string
		mapType          uint
		changeRateMinus1 uint
		cycle            uint
		nrBits           int
		wantErr          string
	}{
		{"rate 1 cycle 0", 3, 0, 0, 7, ""},
		{"rate 1 cycle 99", 3, 0, 99, 7, ""}, // Ceil(Log2(99 + 1)) = 7 bits
		{"rate 1 cycle 100", 3, 0, 100, 7, "slice_group_change_cycle"},
		{"rate 13 cycle 8", 4, 12, 8, 4, ""}, // Ceil(Log2(99 ÷ 13 + 1)) = Ceil(Log2(8.6)) = 4 bits
		{"rate 13 cycle 9", 4, 12, 9, 4, "slice_group_change_cycle"},
		{"rate 99 cycle 1", 5, 98, 1, 1, ""}, // Ceil(Log2(99 ÷ 99 + 1)) = 1 bit
		{"rate 100", 5, 99, 0, 1, "slice_group_change_rate_minus1"},
		{"rate 2^32-1", 3, math.MaxUint32 - 1, 0, 1, "slice_group_change_rate_minus1"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sf := defaultSPSFields()
			sf.picWidthInMbsMinus1 = 10
			sf.picHeightInMapUnitsMinus1 = 8
			pf := ppsFields{numSliceGroupsMinus1: 1, sliceGroups: func(w *bits.EBSPWriter) {
				w.WriteExpGolomb(c.mapType)
				w.Write(1, 1) // slice_group_change_direction_flag
				w.WriteExpGolomb(c.changeRateMinus1)
			}}
			spsMap, ppsMap := paramSetMaps(t, sf, pf)
			nalu := writeSlice(t, pf, sliceFields{sliceType: 7, sliceGroupChangeCycle: c.cycle,
				sliceGroupChangeCycleBits: c.nrBits})
			sh, err := ParseSliceHeader(nalu, spsMap, ppsMap)
			checkErr(t, err, c.wantErr)
			if c.wantErr == "" && err == nil && uint(sh.SliceGroupChangeCycle) != c.cycle {
				t.Errorf("got slice_group_change_cycle %d, want %d", sh.SliceGroupChangeCycle, c.cycle)
			}
		})
	}
}

// TestPPSParserSliceGroups verifies the slice group syntax of the PPS (7.3.2.2). Map type 2 has
// a rectangle for every slice group but the last, which is the background, and map type 6 has
// pic_size_in_map_units_minus1 followed by one slice_group_id per map unit. The
// chroma_qp_index_offset of -3 after the slice groups shows that they were read with the right
// length.
func TestPPSParserSliceGroups(t *testing.T) {
	type sliceGroups struct {
		RunLengthMinus1            []uint
		TopLeft                    []uint
		BottomRight                []uint
		SliceGroupChangeRateMinus1 uint
		PicSizeInMapUnitsMinus1    uint
		SliceGroupID               []uint
	}
	ids := make([]uint, 99)
	for i := range ids {
		ids[i] = uint(i % 3)
	}
	cases := []struct {
		name                 string
		numSliceGroupsMinus1 uint
		write                func(w *bits.EBSPWriter)
		want                 sliceGroups
		wantErr              string
	}{
		{"type 0", 2, func(w *bits.EBSPWriter) {
			for _, v := range []uint{0, 10, 20, 30} { // slice_group_map_type, run_length_minus1
				w.WriteExpGolomb(v)
			}
		}, sliceGroups{RunLengthMinus1: []uint{10, 20, 30}}, ""},
		{"type 2", 2, func(w *bits.EBSPWriter) {
			for _, v := range []uint{2, 0, 12, 24, 36} { // slice_group_map_type, top_left, bottom_right
				w.WriteExpGolomb(v)
			}
		}, sliceGroups{TopLeft: []uint{0, 24}, BottomRight: []uint{12, 36}}, ""},
		{"type 4", 1, func(w *bits.EBSPWriter) {
			w.WriteExpGolomb(4)  // slice_group_map_type
			w.Write(0, 1)        // slice_group_change_direction_flag
			w.WriteExpGolomb(12) // slice_group_change_rate_minus1
		}, sliceGroups{SliceGroupChangeRateMinus1: 12}, ""},
		{"type 6", 2, func(w *bits.EBSPWriter) {
			w.WriteExpGolomb(6)  // slice_group_map_type
			w.WriteExpGolomb(98) // pic_size_in_map_units_minus1
			for _, id := range ids {
				w.Write(id, 2) // slice_group_id
			}
		}, sliceGroups{PicSizeInMapUnitsMinus1: 98, SliceGroupID: ids}, ""},
		{"type 6 with more map units than bits", 1, func(w *bits.EBSPWriter) {
			w.WriteExpGolomb(6)                  // slice_group_map_type
			w.WriteExpGolomb(math.MaxUint32 - 1) // pic_size_in_map_units_minus1
			w.Write(0, 16)                       // slice_group_id
		}, sliceGroups{}, "pic_size_in_map_units_minus1"},
		{"type 7", 1, func(w *bits.EBSPWriter) { w.WriteExpGolomb(7) }, sliceGroups{}, "slice_group_map_type"},
		{"change rate 2^64-1", 1, func(w *bits.EBSPWriter) {
			w.WriteExpGolomb(3) // slice_group_map_type
			w.Write(0, 1)       // slice_group_change_direction_flag
			// slice_group_change_rate_minus1 as a ue(v) code with 64 leading zero bits
			w.Write(0, 32)
			w.Write(0, 32)
			w.Write(1, 1)
			w.Write(0, 32)
			w.Write(0, 32)
		}, sliceGroups{}, "exp-Golomb"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			pf := ppsFields{numSliceGroupsMinus1: c.numSliceGroupsMinus1, sliceGroups: c.write, chromaQpIndexOffset: -3}
			nalu := writePPS(t, pf)
			var pps *PPS
			var err error
			if alloc := allocatedBytes(func() { pps, err = ParsePPSNALUnit(nalu, nil) }); alloc > 1<<16 {
				t.Errorf("allocated %d bytes for a %d-byte NAL unit", alloc, len(nalu))
			}
			checkErr(t, err, c.wantErr)
			if c.wantErr != "" || err != nil {
				return
			}
			got := sliceGroups{pps.RunLengthMinus1, pps.TopLeft, pps.BottomRight, pps.SliceGroupChangeRateMinus1,
				pps.PicSizeInMapUnitsMinus1, pps.SliceGroupID}
			if diff := deep.Equal(got, c.want); diff != nil {
				t.Error(diff)
			}
			if pps.ChromaQpIndexOffset != -3 {
				t.Errorf("got chroma_qp_index_offset %d, want -3", pps.ChromaQpIndexOffset)
			}
		})
	}
}

// allocatedBytes returns the number of bytes allocated while running f.
func allocatedBytes(f func()) uint64 {
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	f()
	runtime.ReadMemStats(&after)
	return after.TotalAlloc - before.TotalAlloc
}

// TestParserDeltaScale verifies that delta_scale in the scaling lists of the SPS and the PPS is
// limited to -128..+127 (7.4.2.1.1.1), which keeps all list entries in the range 1 to 255.
func TestParserDeltaScale(t *testing.T) {
	atLimits := ScalingList{1, 128, 255, 127}
	for len(atLimits) < 16 {
		atLimits = append(atLimits, 127)
	}
	cases := []struct {
		name    string
		deltas  []int
		want    ScalingList
		wantErr string
	}{
		{"deltas at the limits", []int{-7, 127, 127, -128}, atLimits, ""},
		{"delta_scale 128", []int{128}, nil, "delta_scale"},
		{"delta_scale -129", []int{-7, -129}, nil, "delta_scale"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sf := defaultSPSFields()
			sf.scalingList0Deltas = c.deltas
			sps, err := ParseSPSNALUnit(writeSPS(t, sf), false)
			checkErr(t, err, c.wantErr)
			if c.wantErr == "" && err == nil {
				if diff := deep.Equal(sps.SeqScalingLists[0], c.want); diff != nil {
					t.Errorf("SPS: %v", diff)
				}
			}
			pps, err := ParsePPSNALUnit(writePPS(t, ppsFields{scalingList0Deltas: c.deltas}), nil)
			checkErr(t, err, c.wantErr)
			if c.wantErr == "" && err == nil {
				if diff := deep.Equal(pps.PicScalingLists[0], c.want); diff != nil {
					t.Errorf("PPS: %v", diff)
				}
			}
		})
	}
}
