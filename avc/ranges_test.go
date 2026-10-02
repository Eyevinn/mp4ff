package avc

import (
	"bytes"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/Eyevinn/mp4ff/bits"
)

// spsFields are the SPS syntax elements varied by the range tests. The SPS is
// High profile with pic_order_cnt_type 0, frame_mbs_only_flag 1, no scaling
// matrix and no VUI.
type spsFields struct {
	chromaFormatIDC           uint
	bitDepthLumaMinus8        uint
	bitDepthChromaMinus8      uint
	log2MaxFrameNumMinus4     uint
	log2MaxPocLsbMinus4       uint
	picWidthInMbsMinus1       uint
	picHeightInMapUnitsMinus1 uint
	crop                      []uint // left, right, top, bottom offsets, if cropping
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
	w.WriteExpGolomb(0)
	w.WriteExpGolomb(f.chromaFormatIDC)
	if f.chromaFormatIDC == 3 {
		w.Write(0, 1) // separate_colour_plane_flag
	}
	w.WriteExpGolomb(f.bitDepthLumaMinus8)
	w.WriteExpGolomb(f.bitDepthChromaMinus8)
	w.Write(0, 1) // qpprime_y_zero_transform_bypass_flag
	w.Write(0, 1) // seq_scaling_matrix_present_flag
	w.WriteExpGolomb(f.log2MaxFrameNumMinus4)
	w.WriteExpGolomb(0) // pic_order_cnt_type
	w.WriteExpGolomb(f.log2MaxPocLsbMinus4)
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
	w.Write(0, 1) // vui_parameters_present_flag
	w.WriteRbspTrailingBits()
	if w.AccError() != nil {
		t.Fatal(w.AccError())
	}
	return buf.Bytes()
}

// ppsFields are the PPS syntax elements varied by the range tests. The PPS uses
// CAVLC and has deblocking_filter_control_present_flag set.
type ppsFields struct {
	numRefIdxL0DefaultActiveMinus1 uint
	numRefIdxL1DefaultActiveMinus1 uint
	weightedPredFlag               bool
	picInitQpMinus26               int
	picInitQsMinus26               int
	chromaQpIndexOffset            int
	secondChromaQpIndexOffset      *int // written with the High profile extension if set
}

func writePPS(t *testing.T, f ppsFields) []byte {
	t.Helper()
	buf := bytes.Buffer{}
	w := bits.NewEBSPWriter(&buf)
	w.Write(0x68, 8)    // NAL header: nal_ref_idc=3, nal_unit_type=8 (PPS)
	w.WriteExpGolomb(0) // pic_parameter_set_id
	w.WriteExpGolomb(0) // seq_parameter_set_id
	w.Write(0, 1)       // entropy_coding_mode_flag
	w.Write(0, 1)       // bottom_field_pic_order_in_frame_present_flag
	w.WriteExpGolomb(0) // num_slice_groups_minus1
	w.WriteExpGolomb(f.numRefIdxL0DefaultActiveMinus1)
	w.WriteExpGolomb(f.numRefIdxL1DefaultActiveMinus1)
	if f.weightedPredFlag {
		w.Write(1, 1)
	} else {
		w.Write(0, 1)
	}
	w.Write(0, 2) // weighted_bipred_idc
	writeSignedGolomb(w, f.picInitQpMinus26)
	writeSignedGolomb(w, f.picInitQsMinus26)
	writeSignedGolomb(w, f.chromaQpIndexOffset)
	w.Write(1, 1) // deblocking_filter_control_present_flag
	w.Write(0, 1) // constrained_intra_pred_flag
	w.Write(0, 1) // redundant_pic_cnt_present_flag
	if f.secondChromaQpIndexOffset != nil {
		w.Write(0, 1) // transform_8x8_mode_flag
		w.Write(0, 1) // pic_scaling_matrix_present_flag
		writeSignedGolomb(w, *f.secondChromaQpIndexOffset)
	}
	w.WriteRbspTrailingBits()
	if w.AccError() != nil {
		t.Fatal(w.AccError())
	}
	return buf.Bytes()
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
	w.WriteExpGolomb(1) // disable_deblocking_filter_idc
	w.Write(0xff, 8)    // slice data
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
		{"width at MaxFS", func(f *spsFields) { f.picWidthInMbsMinus1 = maxFrameSizeInMbs - 1 }, ""},
		{"width above MaxFS", func(f *spsFields) { f.picWidthInMbsMinus1 = maxFrameSizeInMbs }, "pic_width_in_mbs_minus1"},
		{"height above MaxFS", func(f *spsFields) { f.picHeightInMapUnitsMinus1 = maxFrameSizeInMbs },
			"pic_height_in_map_units_minus1"},
		// 16x16 pixels in 4:2:0 has CropUnitX = CropUnitY = 2, so at most 7 crop units per direction.
		{"crop leaving 2x2 pixels", func(f *spsFields) { f.crop = []uint{3, 4, 4, 3} }, ""},
		{"crop left+right = width", func(f *spsFields) { f.crop = []uint{4, 4, 0, 0} }, "frame cropping"},
		{"crop top+bottom = height", func(f *spsFields) { f.crop = []uint{0, 0, 0, 8} }, "frame cropping"},
		{"crop right far beyond width", func(f *spsFields) { f.crop = []uint{0, 100, 0, 0} }, "frame cropping"},
		{"crop right 2^32-1", func(f *spsFields) { f.crop = []uint{1, math.MaxUint32, 0, 0} }, "frame cropping"},
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
	for _, n := range []uint{32, math.MaxUint32} {
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
