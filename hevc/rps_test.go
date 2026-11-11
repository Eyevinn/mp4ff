package hevc

import (
	"encoding/hex"
	"testing"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/go-test/deep"
)

const ltrSPS = "4201010160000003000003000003000003003fa00d080f1fe58c26491b61a588aa93130cbecfaf37e59f5e1446272ee27008180c000080c08072"
const rplmSPS = "4201010160000003000003000003000003003ca00d080f1fe59499246d861622aa4c4c32fb3ebcdf967d7851189cbb20"
const ppsHex = "4401c1a5581320"

func TestParsePredictedRPSConformance(t *testing.T) {
	// SPS, PPS and slice-header bytes from the JCT-VC conformance streams at
	// https://github.com/kupix/bitstreams/tree/b19d683c6cfd5377240a85242a15270414f43155/hevc/jctvc/draft_conformance/HEVC_v1
	// Slice payloads are omitted. Header sizes, QP deltas and list entries agree
	// with FFmpeg trace_headers; RPS distances follow H.265 7.4.8.

	type sliceCase struct {
		hex   string
		rps   ShortTermRPS
		qp    int
		lists *RefPicListsModification
	}
	cases := []struct {
		name   string
		sps    string
		slices []sliceCase
	}{
		{name: "LTRPSPS_A_Qualcomm_1", sps: ltrSPS, slices: []sliceCase{
			{hex: "0201e118ff91d8", rps: usedRPS([]uint32{8}, nil), qp: 7},                                            // slice 2
			{hex: "0201e099f01843", rps: usedRPS([]uint32{4}, []uint32{4}), qp: 8, lists: &RefPicListsModification{}}, // slice 3
			{hex: "0201e48904fc6d8430", rps: usedRPS([]uint32{4}, []uint32{4}), qp: 8,
				lists: &RefPicListsModification{RefPicListModificationFlagL1: true, ListEntryL1: []uint8{1, 0}}}, // slice 35, local RPS
		}},
		{name: "RPLM_A_qualcomm_4", sps: rplmSPS, slices: []sliceCase{
			{hex: "0201e05adab45873", rps: usedRPS([]uint32{2}, []uint32{2, 4}), qp: 14,
				lists: &RefPicListsModification{
					RefPicListModificationFlagL0: true, ListEntryL0: []uint8{1},
					RefPicListModificationFlagL1: true, ListEntryL1: []uint8{1, 0},
				}}, // slice 4
			{hex: "0201e548a2c27ab0e6", rps: usedRPS([]uint32{2, 2}, nil), qp: 14,
				lists: &RefPicListsModification{
					RefPicListModificationFlagL0: true, ListEntryL0: []uint8{0},
					RefPicListModificationFlagL1: true, ListEntryL1: []uint8{0},
				}}, // slice 298, local RPS
		}},
		{name: "RPLM_B_qualcomm_4", sps: rplmSPS, slices: []sliceCase{
			{hex: "02015504ced630d6", rps: usedRPS([]uint32{4}, []uint32{4}), qp: 13,
				lists: &RefPicListsModification{RefPicListModificationFlagL0: true, ListEntryL0: []uint8{1}}}, // slice 8
			{hex: "0201e548a2c3568a8730", rps: usedRPS([]uint32{2, 2}, nil), qp: 14,
				lists: &RefPicListsModification{RefPicListModificationFlagL0: true, ListEntryL0: []uint8{0, 1}}}, // slice 892, local RPS
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			decode := func(s string) []byte {
				data, err := hex.DecodeString(s)
				if err != nil {
					t.Fatal(err)
				}
				return data
			}
			sps, err := ParseSPSNALUnit(decode(tc.sps))
			if err != nil {
				t.Fatalf("ParseSPSNALUnit: %v", err)
			}
			// Sets 1..7 form a prediction chain with shifts of both signs.
			wantSets := []ShortTermRPS{
				usedRPS([]uint32{8, 2, 2, 4}, nil),
				usedRPS([]uint32{4, 2}, []uint32{4}),
				usedRPS([]uint32{2, 2}, []uint32{2, 4}),
				usedRPS([]uint32{1}, []uint32{1, 2, 4}),
				usedRPS([]uint32{1, 2}, []uint32{1, 4}),
				usedRPS([]uint32{2, 2, 2}, []uint32{2}),
				usedRPS([]uint32{1, 4}, []uint32{1, 2}),
				usedRPS([]uint32{1, 2, 4}, []uint32{1}),
				usedRPS([]uint32{8}, nil),
				usedRPS([]uint32{4}, []uint32{4}),
				usedRPS([]uint32{2}, []uint32{2, 4}),
			}
			if tc.sps == ltrSPS {
				wantSets = append(wantSets, usedRPS(nil, nil))
			}
			if diff := deep.Equal(sps.ShortTermRefPicSets, wantSets); diff != nil {
				t.Errorf("got RPS sets: %v", diff)
			}
			spsMap := map[uint32]*SPS{uint32(sps.SpsID): sps}
			pps, err := ParsePPSNALUnit(decode(ppsHex), spsMap)
			if err != nil {
				t.Fatalf("ParsePPSNALUnit: %v", err)
			}
			ppsMap := map[uint32]*PPS{pps.PicParameterSetID: pps}
			for _, slice := range tc.slices {
				data := decode(slice.hex)
				sh, err := ParseSliceHeader(data, spsMap, ppsMap)
				if err != nil {
					t.Errorf("ParseSliceHeader(%s): %v", slice.hex, err)
					continue
				}
				if diff := deep.Equal(sh.ShortTermRefPicSet, slice.rps); diff != nil {
					t.Errorf("got RPS: %v", diff)
				}
				if sh.ShortTermRefPicSet.countInUsePics() != slice.rps.NumDeltaPocs {
					t.Errorf("got RPS current-picture count %d, want %d", sh.ShortTermRefPicSet.countInUsePics(), slice.rps.NumDeltaPocs)
				}
				if diff := deep.Equal(sh.RefPicListsModification, slice.lists); diff != nil {
					t.Errorf("got reference lists: %v", diff)
				}
				if sh.QpDelta != slice.qp || sh.Size != uint32(len(data)) {
					t.Errorf("got QP delta %d, header size %d; want %d, %d", sh.QpDelta, sh.Size, slice.qp, len(data))
				}
			}
		})
	}
}

// usedRPS makes a reference set whose pictures are all used by the current picture.
func usedRPS(negative, positive []uint32) ShortTermRPS {
	if negative == nil {
		negative = []uint32{}
	}
	if positive == nil {
		positive = []uint32{}
	}
	rps := ShortTermRPS{
		DeltaPocS0: negative, DeltaPocS1: positive,
		NumNegativePics: byte(len(negative)), NumPositivePics: byte(len(positive)),
		NumDeltaPocs:    byte(len(negative) + len(positive)),
		UsedByCurrPicS0: make([]bool, len(negative)), UsedByCurrPicS1: make([]bool, len(positive)),
	}
	for i := range negative {
		rps.UsedByCurrPicS0[i] = true
	}
	for i := range positive {
		rps.UsedByCurrPicS1[i] = true
	}
	return rps
}

func TestShortTermRPSPrediction(t *testing.T) {
	cases := []struct {
		name                       string
		delta                      int
		used, keep                 []bool
		negative, positive         []uint32
		usedNegative, usedPositive []bool
	}{
		{
			name: "positive shift", delta: 2,
			used: []bool{true, false, false, true, false}, keep: []bool{true, true, false, true, true},
			negative: []uint32{1}, positive: []uint32{1, 1, 5},
			usedNegative: []bool{false}, usedPositive: []bool{true, false, true},
		},
		{
			name: "negative shift", delta: -3,
			used: []bool{true, false, true, true, true}, keep: []bool{true, true, true, true, true},
			negative: []uint32{1, 2, 1, 2}, positive: []uint32{2},
			usedNegative: []bool{true, true, true, false}, usedPositive: []bool{true},
		},
		{
			name: "zero POC excluded", delta: 1,
			used: []bool{false, true, false, false, false}, keep: []bool{true, true, false, false, false},
			negative: []uint32{2}, usedNegative: []bool{true},
		},
		{name: "empty prediction", delta: 1, used: make([]bool, 5), keep: make([]bool, 5)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := newNalBuilder(NALU_SPS)
			b.flag(true) // inter_ref_pic_set_prediction_flag
			b.flag(tc.delta < 0)
			magnitude := tc.delta
			if magnitude < 0 {
				magnitude = -magnitude
			}
			b.ue(uint(magnitude - 1))
			for i, used := range tc.used {
				b.flag(used)
				if !used {
					b.flag(tc.keep[i])
				}
			}
			b.u(0x55, 8) // next syntax, not part of the RPS
			r := bits.NewEBSPReaderFromSlice(b.truncated())
			r.Read(16)
			sps := &SPS{ShortTermRefPicSets: []ShortTermRPS{usedRPS([]uint32{1, 2}, []uint32{2, 3})}}
			got := parseShortTermRPS(r, 1, 2, sps)
			if r.AccError() != nil {
				t.Fatalf("parseShortTermRPS: %v", r.AccError())
			}
			want := usedRPS(tc.negative, tc.positive)
			copy(want.UsedByCurrPicS0, tc.usedNegative)
			copy(want.UsedByCurrPicS1, tc.usedPositive)
			if diff := deep.Equal(got, want); diff != nil {
				t.Errorf("got RPS: %v", diff)
			}
			if next := r.Read(8); next != 0x55 {
				t.Errorf("got next syntax %#x, want 0x55", next)
			}
		})
	}
}
