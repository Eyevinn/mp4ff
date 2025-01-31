package mp4

import (
	"encoding/hex"
	"testing"
)

func TestVisualSampleEntryBoxVP9(t *testing.T) {
	vppC := &VppCBox{
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
	vp9 := CreateVisualSampleEntryBox("vp09", 1280, 720, vppC)
	smdm := CreateSmDmBox(0, 1, 2, 3, 4, 5, 6, 7, 255, 255)
	vp9.AddChild(smdm)
	coll := CreateCoLLBox(1000, 500)
	vp9.AddChild(coll)
	boxDiffAfterEncodeAndDecode(t, vp9)

	minFilled := NewVisualSampleEntryBox("avc3")
	boxDiffAfterEncodeAndDecode(t, minFilled)

	minAvc1 := NewVisualSampleEntryBox("avc1")
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
	avcC, err := CreateAvcC(spss, ppss, false /* includePS */)
	if err != nil {
		t.Errorf("error creating avcC: %s", err.Error())
		t.Fail()
	}
	avcx := CreateVisualSampleEntryBox("avc3", 1280, 720, avcC)
	err = avcx.ConvertAvc3ToAvc1(spss, ppss)
	if err != nil {
		t.Errorf("")
	}
}
