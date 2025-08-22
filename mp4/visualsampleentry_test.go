package mp4_test

import (
	"encoding/hex"
	"testing"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/Eyevinn/mp4ff/mp4"
)

func TestVisualSampleEntryBoxVP9(t *testing.T) {
	vppC := &mp4.VppCBox{
		Version:                 1,
		Flags:                   0,
		Profile:                 1,
		Level:                   2,
		BitDepth:                8,
		ChromaSubsampling:       0,
		VideoFullRangeFlag:      0,
		ColourPrimaries:         0,
		TransferCharacteristics: 0,
		MatrixCoefficients:      0,
		CodecInitData:           nil,
	}
	vp9 := mp4.CreateVisualSampleEntryBox("vp09", 1280, 720, vppC)
	smdm := mp4.CreateSmDmBox(0, 1, 2, 3, 4, 5, 6, 7, 255, 255)
	vp9.AddChild(smdm)
	coll := mp4.CreateCoLLBox(1000, 500)
	vp9.AddChild(coll)
	boxDiffAfterEncodeAndDecode(t, vp9)

	minFilled := mp4.NewVisualSampleEntryBox("avc3")
	boxDiffAfterEncodeAndDecode(t, minFilled)

	minAvc1 := mp4.NewVisualSampleEntryBox("avc1")
	err := minAvc1.ConvertAvc3ToAvc1(nil, nil)
	if err == nil {
		t.Error("ConvertAvc3ToAvc1 should return error")
	}

	sps1, err := hex.DecodeString(sps1nalu)
	if err != nil {
		t.Error(err)
	}
	pps1, err := hex.DecodeString(pps1nalu)
	if err != nil {
		t.Error(err)
	}
	spss := [][]byte{sps1}
	ppss := [][]byte{pps1}
	avcC, err := mp4.CreateAvcC(spss, ppss, false /* includePS */)
	if err != nil {
		t.Errorf("error creating avcC: %s", err.Error())
		t.Fail()
	}
	avcx := mp4.CreateVisualSampleEntryBox("avc3", 1280, 720, avcC)
	err = avcx.ConvertAvc3ToAvc1(spss, ppss)
	if err != nil {
		t.Errorf("")
	}
}

func TestAvc1WithTrailingBytes(t *testing.T) {
	avc1Hex := "0000008b6176633100000000000000010000000000000000000000000000000002800168004800000048000000000000000100" +
		"000000000000000000000000000000000000000000000000000000000000000018ffff00000031617663430164001effe100196764001ea" +
		"cd940a02ff9610000030001000003003c8f162d9601000568ebecb22c00000000"
	avc1Raw, err := hex.DecodeString(avc1Hex)
	if err != nil {
		t.Error(err)
	}
	sr := bits.NewFixedSliceReader(avc1Raw)
	// Decode the box
	box, err := mp4.DecodeBoxSR(0, sr)
	if err != nil {
		t.Error(err)
	}
	// Check the box type
	if box.Type() != "avc1" {
		t.Errorf("expected box type avc1, got %s", box.Type())
	}
	// Check the box size
	if box.Size() != uint64(len(avc1Raw)) {
		t.Errorf("expected box size %d, got %d", len(avc1Raw), box.Size())
	}
	avc1 := box.(*mp4.VisualSampleEntryBox)
	if len(avc1.TrailingBytes) != 4 {
		t.Errorf("expected 4 trailing bytes, got %d", len(avc1.TrailingBytes))
	}
	boxDiffAfterEncodeAndDecode(t, avc1)
}
