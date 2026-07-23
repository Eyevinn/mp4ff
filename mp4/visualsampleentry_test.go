package mp4_test

import (
	"encoding/hex"
	"os"
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

func TestVisualSampleEntryBoxHDRMetadata(t *testing.T) {
	hvc1 := mp4.CreateVisualSampleEntryBox("hvc1", 3840, 2160, nil)
	mdcv := mp4.CreateMdcvBox(
		[3]uint16{13250, 7500, 34000},
		[3]uint16{34500, 3000, 16000},
		15635, 16450,
		10000000, 50,
	)
	clli := mp4.CreateClliBox(1000, 400)
	hvc1.AddChild(mdcv)
	hvc1.AddChild(clli)

	boxDiffAfterEncodeAndDecode(t, hvc1)

	decoded := boxAfterEncodeAndDecode(t, hvc1).(*mp4.VisualSampleEntryBox)
	if decoded.Mdcv == nil {
		t.Fatal("expected decoded mdcv child")
	}
	if decoded.Clli == nil {
		t.Fatal("expected decoded clli child")
	}
}

func TestVisualSampleEntryBoxMJpeg(t *testing.T) {
	// mjpg sample entry with a jpgC box (ISO/IEC 23008-12 Annex H)
	jpgC := &mp4.JpgCBox{JpegPrefix: []byte{0xff, 0xd8, 0xff, 0xdb}}
	mjpg := mp4.CreateVisualSampleEntryBox("mjpg", 400, 226, jpgC)
	boxDiffAfterEncodeAndDecode(t, mjpg)
	decoded := boxAfterEncodeAndDecode(t, mjpg).(*mp4.VisualSampleEntryBox)
	if decoded.JpgC == nil {
		t.Error("expected decoded jpgC child")
	}

	// mp4v sample entry with an esds box (ISO/IEC 14496-14)
	esds := mp4.CreateEsdsBox([]byte{0x11, 0x90})
	mp4v := mp4.CreateVisualSampleEntryBox("mp4v", 640, 360, esds)
	boxDiffAfterEncodeAndDecode(t, mp4v)
	decoded = boxAfterEncodeAndDecode(t, mp4v).(*mp4.VisualSampleEntryBox)
	if decoded.Esds == nil {
		t.Error("expected decoded esds child")
	}

	// jpeg sample entry (QuickTime) without children
	jpeg := mp4.NewVisualSampleEntryBox("jpeg")
	boxDiffAfterEncodeAndDecode(t, jpeg)
}

func TestAvc1WithTrailingBytes(t *testing.T) {
	minfWithTrailingAvc1Bytes, err := os.ReadFile("testdata/minf_with_trailing_avc1_bytes.bin")
	if err != nil {
		t.Fatal(err)
	}
	sr := bits.NewFixedSliceReader(minfWithTrailingAvc1Bytes)
	// Decode the box
	box, err := mp4.DecodeBoxSR(0, sr)
	if err != nil {
		t.Error(err)
	}
	// Check the box type
	if box.Type() != "minf" {
		t.Errorf("expected box type minf, got %s", box.Type())
	}
	avc1 := box.(*mp4.MinfBox).Stbl.Stsd.Children[0].(*mp4.VisualSampleEntryBox)
	if len(avc1.TrailingBytes) != 4 {
		t.Errorf("expected 4 trailing bytes, got %d", len(avc1.TrailingBytes))
	}
	boxDiffAfterEncodeAndDecode(t, avc1)
}
