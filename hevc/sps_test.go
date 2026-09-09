package hevc

import (
	"encoding/hex"
	"testing"

	"github.com/go-test/deep"
)

const (
	spsNalu = ("420101022000000300b0000003000003007ba0078200887db6718b92448053888892" +
		"cf24a69272c9124922dc91aa48fca223ff000100016a02020201")
	spsNaluHdr10 = "420101022000000300b0000003000003009ca001e020021c4d8815ee4595602d4244024020"
	spsNaluHrd   = ("42010101400000030000030000030000030096a001e02002207c4e5ad290964b8c04040000" +
		"03000400000300658017794400014fb1000004c4b3c40")
)

func TestSPSParser1(t *testing.T) {
	byteData, _ := hex.DecodeString(spsNalu)

	wantedVUI := VUIParameters{
		SampleAspectRatioWidth:     1,
		SampleAspectRatioHeight:    1,
		VideoSignalTypePresentFlag: true,
		VideoFormat:                5,
		ColourDescriptionFlag:      true,
		ColourPrimaries:            1,
		TransferCharacteristics:    1,
		MatrixCoefficients:         1,
	}
	wanted := SPS{
		VpsID:                 0,
		MaxSubLayersMinus1:    0,
		TemporalIDNestingFlag: true,
		ProfileTierLevel: ProfileTierLevel{
			GeneralProfileSpace:              0,
			GeneralTierFlag:                  false,
			GeneralProfileIDC:                2,
			GeneralProfileCompatibilityFlags: 536870912,
			GeneralConstraintIndicatorFlags:  193514046488576,
			GeneralProgressiveSourceFlag:     true,
			GeneralInterlacedSourceFlag:      false,
			GeneralNonPackedConstraintFlag:   true,
			GeneralFrameOnlyConstraintFlag:   true,
			GeneralLevelIDC:                  123,
		},
		SpsID:                   0,
		ChromaFormatIDC:         1,
		SeparateColourPlaneFlag: false,
		ConformanceWindowFlag:   true,
		PicWidthInLumaSamples:   960,
		PicHeightInLumaSamples:  544,
		ConformanceWindow: ConformanceWindow{
			LeftOffset:   0,
			RightOffset:  0,
			TopOffset:    0,
			BottomOffset: 2,
		},
		BitDepthLumaMinus8:              2,
		BitDepthChromaMinus8:            2,
		Log2MaxPicOrderCntLsbMinus4:     6,
		SubLayerOrderingInfoPresentFlag: false,
		SubLayeringOrderingInfos: []SubLayerOrderingInfo{
			{
				MaxDecPicBufferingMinus1: 5,
				MaxNumReorderPics:        4,
				MaxLatencyIncreasePlus1:  0,
			},
		},
		Log2MinLumaCodingBlockSizeMinus3:     0,
		Log2DiffMaxMinLumaCodingBlockSize:    3,
		Log2MinLumaTransformBlockSizeMinus2:  0,
		Log2DiffMaxMinLumaTransformBlockSize: 3,
		MaxTransformHierarchyDepthInter:      1,
		MaxTransformHierarchyDepthIntra:      1,
		ScalingListEnabledFlag:               false,
		ScalingListDataPresentFlag:           false,
		AmpEnabledFlag:                       false,
		SampleAdaptiveOffsetEnabledFlag:      false,
		PCMEnabledFlag:                       false,
		NumShortTermRefPicSets:               9,
		ShortTermRefPicSets: []ShortTermRPS{
			{
				DeltaPocS0:      []uint32{8, 8},
				DeltaPocS1:      []uint32{},
				UsedByCurrPicS0: []bool{true, true},
				UsedByCurrPicS1: []bool{},
				NumNegativePics: 2,
				NumPositivePics: 0,
				NumDeltaPocs:    2,
			},
			{
				DeltaPocS0:      []uint32{1},
				DeltaPocS1:      []uint32{7},
				UsedByCurrPicS0: []bool{true},
				UsedByCurrPicS1: []bool{true},
				NumNegativePics: 1,
				NumPositivePics: 1,
				NumDeltaPocs:    2,
			},
			{
				DeltaPocS0:      []uint32{2},
				DeltaPocS1:      []uint32{6},
				UsedByCurrPicS0: []bool{true},
				UsedByCurrPicS1: []bool{true},
				NumNegativePics: 1,
				NumPositivePics: 1,
				NumDeltaPocs:    2,
			},
			{
				DeltaPocS0:      []uint32{3},
				DeltaPocS1:      []uint32{5},
				UsedByCurrPicS0: []bool{true},
				UsedByCurrPicS1: []bool{true},
				NumNegativePics: 1,
				NumPositivePics: 1,
				NumDeltaPocs:    2,
			},
			{
				DeltaPocS0:      []uint32{4},
				DeltaPocS1:      []uint32{4},
				UsedByCurrPicS0: []bool{true},
				UsedByCurrPicS1: []bool{true},
				NumNegativePics: 1,
				NumPositivePics: 1,
				NumDeltaPocs:    2,
			},
			{
				DeltaPocS0:      []uint32{5},
				DeltaPocS1:      []uint32{3},
				UsedByCurrPicS0: []bool{true},
				UsedByCurrPicS1: []bool{true},
				NumNegativePics: 1,
				NumPositivePics: 1,
				NumDeltaPocs:    2,
			},
			{
				DeltaPocS0:      []uint32{6},
				DeltaPocS1:      []uint32{2},
				UsedByCurrPicS0: []bool{true},
				UsedByCurrPicS1: []bool{true},
				NumNegativePics: 1,
				NumPositivePics: 1,
				NumDeltaPocs:    2,
			},
			{
				DeltaPocS0:      []uint32{7},
				DeltaPocS1:      []uint32{1},
				UsedByCurrPicS0: []bool{true},
				UsedByCurrPicS1: []bool{true},
				NumNegativePics: 1,
				NumPositivePics: 1,
				NumDeltaPocs:    2,
			},
			{
				DeltaPocS0:      []uint32{8},
				DeltaPocS1:      []uint32{},
				UsedByCurrPicS0: []bool{true},
				UsedByCurrPicS1: []bool{},
				NumNegativePics: 1,
				NumPositivePics: 0,
				NumDeltaPocs:    1,
			},
		},
		LongTermRefPicsPresentFlag:      false,
		SpsTemporalMvpEnabledFlag:       false,
		StrongIntraSmoothingEnabledFlag: false,
		VUIParametersPresentFlag:        true,
		VUI:                             &wantedVUI,
	}
	got, err := ParseSPSNALUnit(byteData)
	if err != nil {
		t.Fatal("Error parsing SPS")
	}

	if diff := deep.Equal(*got, wanted); diff != nil {
		t.Error(diff)
	}
	gotWidth, gotHeight := got.ImageSize()
	var expWidth, expHeight uint32 = 960, 540
	if gotWidth != expWidth || gotHeight != expHeight {
		t.Errorf("Got %dx%d instead of %dx%d", gotWidth, gotHeight, expWidth, expHeight)
	}
}

func TestSPSParser2(t *testing.T) {
	byteData, _ := hex.DecodeString(spsNaluHdr10)

	wantedVUI := VUIParameters{
		SampleAspectRatioWidth:     1,
		SampleAspectRatioHeight:    1,
		VideoSignalTypePresentFlag: true,
		VideoFormat:                5,
		ColourDescriptionFlag:      true,
		ColourPrimaries:            9,
		TransferCharacteristics:    16,
		MatrixCoefficients:         9,
	}
	wanted := SPS{
		VpsID:                 0,
		MaxSubLayersMinus1:    0,
		TemporalIDNestingFlag: true,
		ProfileTierLevel: ProfileTierLevel{
			GeneralProfileSpace:              0,
			GeneralTierFlag:                  false,
			GeneralProfileIDC:                2,
			GeneralProfileCompatibilityFlags: 536870912,
			GeneralConstraintIndicatorFlags:  193514046488576,
			GeneralProgressiveSourceFlag:     true,
			GeneralInterlacedSourceFlag:      false,
			GeneralNonPackedConstraintFlag:   true,
			GeneralFrameOnlyConstraintFlag:   true,
			GeneralLevelIDC:                  156,
		},
		SpsID:                   0,
		ChromaFormatIDC:         1,
		SeparateColourPlaneFlag: false,
		ConformanceWindowFlag:   false,
		PicWidthInLumaSamples:   3840,
		PicHeightInLumaSamples:  2160,
		ConformanceWindow: ConformanceWindow{
			LeftOffset:   0,
			RightOffset:  0,
			TopOffset:    0,
			BottomOffset: 0,
		},
		BitDepthLumaMinus8:              2,
		BitDepthChromaMinus8:            2,
		Log2MaxPicOrderCntLsbMinus4:     7,
		SubLayerOrderingInfoPresentFlag: false,
		SubLayeringOrderingInfos: []SubLayerOrderingInfo{
			{
				MaxDecPicBufferingMinus1: 4,
				MaxNumReorderPics:        2,
				MaxLatencyIncreasePlus1:  0,
			},
		},
		Log2MinLumaCodingBlockSizeMinus3:     0,
		Log2DiffMaxMinLumaCodingBlockSize:    2,
		Log2MinLumaTransformBlockSizeMinus2:  0,
		Log2DiffMaxMinLumaTransformBlockSize: 3,
		MaxTransformHierarchyDepthInter:      1,
		MaxTransformHierarchyDepthIntra:      0,
		ScalingListEnabledFlag:               true,
		ScalingListDataPresentFlag:           false,
		AmpEnabledFlag:                       false,
		SampleAdaptiveOffsetEnabledFlag:      true,
		PCMEnabledFlag:                       false,
		NumShortTermRefPicSets:               0,
		LongTermRefPicsPresentFlag:           false,
		SpsTemporalMvpEnabledFlag:            true,
		StrongIntraSmoothingEnabledFlag:      false,
		VUIParametersPresentFlag:             true,
		VUI:                                  &wantedVUI,
	}
	got, err := ParseSPSNALUnit(byteData)
	if err != nil {
		t.Error("Error parsing SPS")
	}

	if diff := deep.Equal(*got, wanted); diff != nil {
		t.Error(diff)
	}
	gotWidth, gotHeight := got.ImageSize()
	var expWidth, expHeight uint32 = 3840, 2160
	if gotWidth != expWidth || gotHeight != expHeight {
		t.Errorf("Got %dx%d instead of %dx%d", gotWidth, gotHeight, expWidth, expHeight)
	}
}

func TestSPSParser3(t *testing.T) {
	byteData, _ := hex.DecodeString(spsNaluHrd)

	wantedHrd := HrdParameters{
		NalHrdParametersPresentFlag:        true,
		InitialCpbRemovalDelayLengthMinus1: 23,
		AuCpbRemovalDelayLengthMinus1:      15,
		DpbOutputDelayLengthMinus1:         5,
		SubLayerHrd: []SubLayerHrd{
			{
				NalHrdParameters: []SubLayerHrdParameters{
					{
						BitRateValueMinus1: 171873,
						CpbSizeValueMinus1: 1249998,
					},
				},
			},
		},
	}

	wantedVUI := VUIParameters{
		SampleAspectRatioWidth:     1,
		SampleAspectRatioHeight:    1,
		VideoSignalTypePresentFlag: false,
		VideoFormat:                0,
		ColourDescriptionFlag:      false,
		ColourPrimaries:            0,
		TransferCharacteristics:    0,
		MatrixCoefficients:         0,
		TimingInfoPresentFlag:      true,
		NumUnitsInTick:             1,
		TimeScale:                  25,
		HrdParametersPresentFlag:   true,
		HrdParameters:              &wantedHrd,
	}
	wanted := SPS{
		VpsID:                 0,
		MaxSubLayersMinus1:    0,
		TemporalIDNestingFlag: true,
		ProfileTierLevel: ProfileTierLevel{
			GeneralProfileSpace:              0,
			GeneralProfileIDC:                1,
			GeneralProfileCompatibilityFlags: 1073741824,
			GeneralLevelIDC:                  150,
		},
		SpsID:                   0,
		ChromaFormatIDC:         1,
		SeparateColourPlaneFlag: false,
		ConformanceWindowFlag:   true,
		PicWidthInLumaSamples:   3840,
		PicHeightInLumaSamples:  2176,
		ConformanceWindow: ConformanceWindow{
			LeftOffset:   0,
			RightOffset:  0,
			TopOffset:    0,
			BottomOffset: 8,
		},
		Log2MaxPicOrderCntLsbMinus4:     4,
		SubLayerOrderingInfoPresentFlag: true,
		SubLayeringOrderingInfos: []SubLayerOrderingInfo{
			{
				MaxDecPicBufferingMinus1: 1,
				MaxNumReorderPics:        0,
				MaxLatencyIncreasePlus1:  0,
			},
		},
		Log2MinLumaCodingBlockSizeMinus3:     1,
		Log2DiffMaxMinLumaCodingBlockSize:    1,
		Log2MinLumaTransformBlockSizeMinus2:  0,
		Log2DiffMaxMinLumaTransformBlockSize: 3,
		MaxTransformHierarchyDepthInter:      3,
		MaxTransformHierarchyDepthIntra:      0,
		ScalingListEnabledFlag:               false,
		ScalingListDataPresentFlag:           false,
		AmpEnabledFlag:                       true,
		SampleAdaptiveOffsetEnabledFlag:      true,
		PCMEnabledFlag:                       false,
		NumShortTermRefPicSets:               1,
		ShortTermRefPicSets: []ShortTermRPS{
			{
				DeltaPocS0:      []uint32{1},
				DeltaPocS1:      []uint32{},
				UsedByCurrPicS0: []bool{true},
				UsedByCurrPicS1: []bool{},
				NumNegativePics: 1,
				NumPositivePics: 0,
				NumDeltaPocs:    1,
			},
		},
		LongTermRefPicsPresentFlag:      false,
		SpsTemporalMvpEnabledFlag:       false,
		StrongIntraSmoothingEnabledFlag: false,
		VUIParametersPresentFlag:        true,
		VUI:                             &wantedVUI,
	}
	got, err := ParseSPSNALUnit(byteData)
	if err != nil {
		t.Error("Error parsing SPS")
	}

	if diff := deep.Equal(*got, wanted); diff != nil {
		t.Error(diff)
	}
	gotWidth, gotHeight := got.ImageSize()
	var expWidth, expHeight uint32 = 3840, 2160
	if gotWidth != expWidth || gotHeight != expHeight {
		t.Errorf("Got %dx%d instead of %dx%d", gotWidth, gotHeight, expWidth, expHeight)
	}
}

// TestParseSPSWithNonZeroNumDeltaPocs checks that parsing succeeds (Github issue #279)
func TestParseSPSWithNonZeroNumDeltaPocs(t *testing.T) {
	data := []byte{1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 255,
		3, 32, 0, 1, 0, 25, 64, 1, 12, 1, 255, 255, 1, 96, 0, 0, 3, 0, 0, 3, 0, 0, 3,
		0, 0, 3, 0, 153, 53, 2, 64, 33, 0, 1, 0, 40, 66, 1, 1, 1, 96, 0, 0, 3, 0, 0, 3,
		0, 0, 3, 0, 0, 3, 0, 153, 160, 2, 128, 128, 45, 22, 141, 82, 187, 34, 186, 173,
		146, 169, 119, 53, 1, 1, 1, 0, 128, 34, 0, 1, 0, 8, 68, 1, 192, 36, 103, 192, 204, 100}
	hevcd, err := DecodeHEVCDecConfRec(data)
	if err != nil {
		t.Error(err)
	}

	spsBytes := hevcd.GetNalusForType(NALU_SPS)
	if len(spsBytes) != 1 {
		t.Error("expected 1 sps NALU")
	}
	sps, err := ParseSPSNALUnit(spsBytes[0])
	if err != nil {
		t.Error(err)
	}
	t.Log(sps)
}

// Use SPS and PPS from https://www.itu.int/wftp3/av-arch/jctvc-site/bitstream_exchange/under_test/
// to get bigger variety of input. Don't consider the actual output, just check that parsing works
func TestSPSandPPS(t *testing.T) {
	cases := []struct {
		desc string
		vps  string
		sps  string
		pps  string
	}{
		{
			desc: "Zero_and_One_Palette_Size_A_Canon_2",
			vps:  "40010c01ffff090040000003000c000003000078ac09",
			sps:  "420101090040000003000c00000300007890007810021cff2d7248db3db643cd81000843",
			pps:  "4401c194964c08b21bdd",
		},
		{
			desc: "SPSSLIST_A_Sony_1",
			vps: "40010c11ffff0701000003000f8800000300007bb5057000003e90000ea6077b1000043024e0f00000030001f10000030" +
				"0000f75880f000882807b050002d0a00e36bdff90a020202c1",
			sps: "4201010160000003000003000003000003007ba0078200887de5b59246f8f2c997932c8501e003fe03fa80203c07f2804" +
				"06010163c040018407001840c203040c206103080c10308184070018404042607f03000e1030018006001800c00600180" +
				"06003000e1030008194d10730a3cc3849cc2028709c2464e1309528240a42a09620114201182304828928b1c9362398c7" +
				"1d14c21ff5ebe4fc47f5f9be2bc21c35822828358f840787878f0583c78f0743c78f1e0481e3c78f1e0561e3c78f1e3c0" +
				"c83c78f1e3c78f0320f1e3c78f1e0561e3c78f1e0481e3c78f0743c78f0583c78787842340fc101e1e1e3c160f1e3c1d0" +
				"f1e3c7812078f1e3c7815878f1e3c78f0320f1e3c78f1e3c0c83c78f1e3c7815878f1e3c7812078f1e3c1d0f1e3c160f1" +
				"e1e1e1063c041840787878f0583c78f0743c78f1e0481e3c78f1e0561e3c78f1e3c0c83c78f1e3c78f0320f1e3c78f1e0" +
				"561e3c78f1e0481e3c78f0743c78f0583c7878784203b80202111c1f1c38168e1c381d8e1c387012470e1c387015c70e1" +
				"c3870e0328e1c3870e1c380ca3870e1c387015c70e1c387012470e1c381d8e1c38168e1c1f1c11807b8040844707c70e0" +
				"5a3870e0763870e1c0491c3870e1c0571c3870e1c380ca3870e1c3870e0328e1c3870e1c0571c3870e1c0491c3870e076" +
				"3870e05a38707c70463c07e044707c70e05a3870e0763870e1c0491c3870e1c0571c3870e1c380ca3870e1c3870e0328e" +
				"1c3870e1c0571c3870e1c0491c3870e0763870e05a38707c7045c45525ed9",
			pps: "4401c1f5811d02a0",
		},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			//vpsBytes, _ := hex.DecodeString(c.vps)
			spsBytes, _ := hex.DecodeString(c.sps)
			ppsBytes, _ := hex.DecodeString(c.pps)

			sps, err := ParseSPSNALUnit(spsBytes)
			if err != nil {
				t.Error("Error parsing SPS Nal unit")
			}
			spsMap := make(map[uint32]*SPS)
			spsMap[uint32(sps.SpsID)] = sps
			pps, err := ParsePPSNALUnit(ppsBytes, spsMap)
			if err != nil {
				t.Error("Error parsing PPS Nal unit")
			}
			if byte(pps.SeqParameterSetID) != sps.SpsID {
				t.Error("PPS SpsID does not match SPS SpsID")
			}
		})
	}

}

// The VPS and the layer-0 and layer-1 SPS of an MV-HEVC stereo bitstream, taken
// from the hvcC and lhvC boxes of cmd/mp4ff-mvhevc/testdata/stereo_spatial.mp4.
// The layer-1 SPS uses the multilayer extension form, so it does not signal the
// chroma format, picture size, conformance window or bit depths itself.
const (
	mvhevcVpsNalu = ("40010c11ffff016000000300b0000003000003003c15c15b3c200028245970602000000b" +
		"f800000300000303c8d00a00080a01e5c52bf708501010100080")
	mvhevcSpsNaluLayer0 = "420101016000000300b0000003000003003ca01420207cb8815ee45951"
	mvhevcSpsNaluLayer1 = "42090e822e458a9404"
)

// TestSPSParserMultiLayerExt verifies that a non-base-layer SPS in the
// multilayer extension form is parsed, and that the values it does not signal
// are inherited from the rep_format() of its VPS.
func TestSPSParserMultiLayerExt(t *testing.T) {
	vpsBytes, err := hex.DecodeString(mvhevcVpsNalu)
	if err != nil {
		t.Fatal(err)
	}
	vps, err := ParseVPSNALUnit(vpsBytes)
	if err != nil {
		t.Fatal(err)
	}
	vpsMap := map[byte]*VPS{vps.VpsID: vps}
	spsBytes, err := hex.DecodeString(mvhevcSpsNaluLayer1)
	if err != nil {
		t.Fatal(err)
	}

	sps, err := ParseSPSNALUnitWithVPS(spsBytes, vpsMap)
	if err != nil {
		t.Fatal(err)
	}
	if sps.NuhLayerID != 1 {
		t.Errorf("nuh_layer_id is %d, wanted 1", sps.NuhLayerID)
	}
	if sps.ExtOrMaxSubLayersMinus1 != 7 || !sps.MultiLayerExtSpsFlag {
		t.Errorf("sps_ext_or_max_sub_layers_minus1 is %d and MultiLayerExtSpsFlag %t, wanted 7 and true",
			sps.ExtOrMaxSubLayersMinus1, sps.MultiLayerExtSpsFlag)
	}
	if sps.UpdateRepFormatFlag {
		t.Error("update_rep_format_flag set, wanted the rep format index from the VPS")
	}
	// Inherited from rep_format() 0 of the VPS.
	if sps.ChromaFormatIDC != 1 {
		t.Errorf("chroma_format_idc is %d, wanted 1", sps.ChromaFormatIDC)
	}
	if sps.PicWidthInLumaSamples != 160 || sps.PicHeightInLumaSamples != 128 {
		t.Errorf("picture size is %dx%d, wanted 160x128",
			sps.PicWidthInLumaSamples, sps.PicHeightInLumaSamples)
	}
	if sps.BitDepthLumaMinus8 != 0 || sps.BitDepthChromaMinus8 != 0 {
		t.Errorf("bit depths are %d/%d, wanted 8/8",
			sps.BitDepthLumaMinus8+8, sps.BitDepthChromaMinus8+8)
	}
	if !sps.ConformanceWindowFlag || sps.ConformanceWindow.BottomOffset != 4 {
		t.Errorf("conformance window is %t %+v, wanted a bottom offset of 4",
			sps.ConformanceWindowFlag, sps.ConformanceWindow)
	}
	if w, h := sps.ImageSize(); w != 160 || h != 120 {
		t.Errorf("image size is %dx%d, wanted 160x120", w, h)
	}
	// The two layers of the stereo pair share their coding structure, so the
	// fields the layer-1 SPS does signal must match the layer-0 SPS.
	layer0Bytes, err := hex.DecodeString(mvhevcSpsNaluLayer0)
	if err != nil {
		t.Fatal(err)
	}
	layer0, err := ParseSPSNALUnitWithVPS(layer0Bytes, vpsMap)
	if err != nil {
		t.Fatal(err)
	}
	if layer0.MultiLayerExtSpsFlag {
		t.Error("the layer-0 SPS should not use the multilayer extension form")
	}
	if sps.Log2MaxPicOrderCntLsbMinus4 != layer0.Log2MaxPicOrderCntLsbMinus4 {
		t.Errorf("log2_max_pic_order_cnt_lsb_minus4 is %d, wanted %d as for layer 0",
			sps.Log2MaxPicOrderCntLsbMinus4, layer0.Log2MaxPicOrderCntLsbMinus4)
	}
	if sps.Log2MinLumaCodingBlockSizeMinus3 != layer0.Log2MinLumaCodingBlockSizeMinus3 ||
		sps.Log2DiffMaxMinLumaCodingBlockSize != layer0.Log2DiffMaxMinLumaCodingBlockSize {
		t.Errorf("luma coding block sizes are %d/%d, wanted %d/%d as for layer 0",
			sps.Log2MinLumaCodingBlockSizeMinus3, sps.Log2DiffMaxMinLumaCodingBlockSize,
			layer0.Log2MinLumaCodingBlockSizeMinus3, layer0.Log2DiffMaxMinLumaCodingBlockSize)
	}
	if !sps.MultilayerExtensionFlag {
		t.Error("sps_multilayer_extension_flag not set")
	}
}

// TestSPSParserMultiLayerExtWithoutVPS verifies that a multilayer extension SPS
// still parses without its VPS, since the payload can be read without it, and
// that the values that would have been inherited are then left unset.
func TestSPSParserMultiLayerExtWithoutVPS(t *testing.T) {
	spsBytes, err := hex.DecodeString(mvhevcSpsNaluLayer1)
	if err != nil {
		t.Fatal(err)
	}
	sps, err := ParseSPSNALUnit(spsBytes)
	if err != nil {
		t.Fatal(err)
	}
	if !sps.MultiLayerExtSpsFlag {
		t.Fatal("MultiLayerExtSpsFlag not set")
	}
	if sps.ChromaFormatIDC != 0 || sps.PicWidthInLumaSamples != 0 || sps.PicHeightInLumaSamples != 0 {
		t.Errorf("inherited values are chroma %d and %dx%d, wanted them unset",
			sps.ChromaFormatIDC, sps.PicWidthInLumaSamples, sps.PicHeightInLumaSamples)
	}
	// The signalled values must be the same as when the VPS is available.
	if sps.Log2MaxPicOrderCntLsbMinus4 != 7 {
		t.Errorf("log2_max_pic_order_cnt_lsb_minus4 is %d, wanted 7", sps.Log2MaxPicOrderCntLsbMinus4)
	}
}
