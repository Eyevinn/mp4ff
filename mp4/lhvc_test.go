package mp4_test

import (
	"bytes"
	"encoding/hex"
	"errors"
	"strings"
	"testing"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/Eyevinn/mp4ff/hevc"
	"github.com/Eyevinn/mp4ff/mp4"
)

func TestLhvC(t *testing.T) {
	// Enhancement-layer SPS and PPS from a reference MV-HEVC mp4 (GPAC output).
	spsNalu, err := hex.DecodeString("42090e85924cae6a020202028180")
	if err != nil {
		t.Fatal(err)
	}
	ppsNalu, err := hex.DecodeString("440948572b062a0140")
	if err != nil {
		t.Fatal(err)
	}

	lhvC := mp4.CreateLhvCFromNalus([][]byte{spsNalu}, [][]byte{ppsNalu})
	boxDiffAfterEncodeAndDecode(t, lhvC)
}

func TestLhvCDecodeFromHex(t *testing.T) {
	// Full lhvC box bytes from a reference MV-HEVC mp4 (47 bytes including header).
	boxHex := "0000002f6c68764301f000ffcf02a10001000e42090e85924cae6a020202028180a200010009440948572b062a0140"
	data, err := hex.DecodeString(boxHex)
	if err != nil {
		t.Fatal(err)
	}

	box, err := mp4.DecodeBox(0, bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	lhvC, ok := box.(*mp4.LhvCBox)
	if !ok {
		t.Fatalf("expected *LhvCBox, got %T", box)
	}
	if lhvC.Type() != "lhvC" {
		t.Errorf("Type() = %q, want lhvC", lhvC.Type())
	}
	if lhvC.LengthSizeMinusOne != 3 {
		t.Errorf("LengthSizeMinusOne = %d, want 3", lhvC.LengthSizeMinusOne)
	}
	if lhvC.NumTemporalLayers != 1 {
		t.Errorf("NumTemporalLayers = %d, want 1", lhvC.NumTemporalLayers)
	}
	if len(lhvC.NaluArrays) != 2 {
		t.Fatalf("NaluArrays len = %d, want 2", len(lhvC.NaluArrays))
	}

	spsNalus := lhvC.GetNalusForType(hevc.NALU_SPS)
	if len(spsNalus) != 1 {
		t.Fatalf("SPS nalus = %d, want 1", len(spsNalus))
	}
	if hex.EncodeToString(spsNalus[0]) != "42090e85924cae6a020202028180" {
		t.Errorf("SPS nalu mismatch: got %s", hex.EncodeToString(spsNalus[0]))
	}

	boxDiffAfterEncodeAndDecode(t, lhvC)
}

// TestVisualSampleEntryWithLhvC verifies that an lhvC box decodes as a child of
// a visual sample entry (alongside hvcC) and is reachable via the LhvC pointer.
func TestVisualSampleEntryWithLhvC(t *testing.T) {
	hvc1 := mp4.CreateVisualSampleEntryBox("hvc1", 1920, 1080, nil)
	spsNalu, _ := hex.DecodeString("42090e85924cae6a020202028180")
	ppsNalu, _ := hex.DecodeString("440948572b062a0140")
	hvc1.AddChild(mp4.CreateLhvCFromNalus([][]byte{spsNalu}, [][]byte{ppsNalu}))

	boxDiffAfterEncodeAndDecode(t, hvc1)

	decoded := boxAfterEncodeAndDecode(t, hvc1).(*mp4.VisualSampleEntryBox)
	if decoded.LhvC == nil {
		t.Fatal("expected decoded lhvC child reachable via LhvC pointer")
	}
	if len(decoded.LhvC.GetNalusForType(hevc.NALU_SPS)) != 1 {
		t.Error("expected one SPS nalu in decoded lhvC")
	}
}

// TestLhvCLengthSize checks that an lhvC box with lengthSizeMinusOne 2 (3-byte
// NALU lengths, which ISO/IEC 14496-15 does not allow) fails to decode on both
// decode paths, while the allowed 1-byte value still decodes.
func TestLhvCLengthSize(t *testing.T) {
	// Same box as in TestLhvCDecodeFromHex. Byte 12 is reserved(2) + numTemporalLayers(3) +
	// temporalIdNested(1) + lengthSizeMinusOne(2), here 0xcf for lengthSizeMinusOne 3.
	boxHex := "0000002f6c68764301f000ffcf02a10001000e42090e85924cae6a020202028180a200010009440948572b062a0140"
	for _, c := range []struct {
		flags   byte
		wantErr bool
	}{
		{flags: 0xcc, wantErr: false},
		{flags: 0xce, wantErr: true},
	} {
		data, err := hex.DecodeString(boxHex)
		if err != nil {
			t.Fatal(err)
		}
		data[12] = c.flags
		_, err = mp4.DecodeBox(0, bytes.NewReader(data))
		_, errSR := mp4.DecodeBoxSR(0, bits.NewFixedSliceReader(data))
		for _, e := range []error{err, errSR} {
			if c.wantErr && !errors.Is(e, hevc.ErrInvalidLengthSize) || !c.wantErr && e != nil {
				t.Errorf("lengthSizeMinusOne %d: got error %v, want error %t", c.flags&0x03, e, c.wantErr)
			}
		}
	}
}

// TestLhvCHvcCLengthSizeMismatch checks that a sample entry whose lhvC and hvcC
// declare different NALU length sizes fails to decode and to encode on both
// paths, since ISO/IEC 14496-15 9.5.3 requires them to be the same.
func TestLhvCHvcCLengthSizeMismatch(t *testing.T) {
	spsNalu, _ := hex.DecodeString("42090e85924cae6a020202028180")
	ppsNalu, _ := hex.DecodeString("440948572b062a0140")
	newHvc1 := func(lhvCLengthSizeMinusOne byte) *mp4.VisualSampleEntryBox {
		hvcC := &mp4.HvcCBox{DecConfRec: hevc.DecConfRec{ConfigurationVersion: 1, LengthSizeMinusOne: 3}}
		hvc1 := mp4.CreateVisualSampleEntryBox("hvc1", 1920, 1080, hvcC)
		lhvC := mp4.CreateLhvCFromNalus([][]byte{spsNalu}, [][]byte{ppsNalu})
		lhvC.LengthSizeMinusOne = lhvCLengthSizeMinusOne
		hvc1.AddChild(lhvC)
		return hvc1
	}
	isMismatch := func(err error) bool {
		return err != nil && strings.Contains(err.Error(), "differs from hvcC")
	}

	var buf bytes.Buffer
	if err := newHvc1(3).Encode(&buf); err != nil {
		t.Fatal(err)
	}
	data := buf.Bytes()
	if _, err := mp4.DecodeBox(0, bytes.NewReader(data)); err != nil {
		t.Errorf("matching length sizes: unexpected error %v", err)
	}
	// The lhvC flags byte, reserved(2) + numTemporalLayers(3) + temporalIdNested(1) + lengthSizeMinusOne(2),
	// follows the box type, configurationVersion, min_spatial_segmentation_idc and parallelismType.
	p := bytes.Index(data, []byte("lhvC")) + 8
	data[p] = data[p]&^0x03 | 0x01 // 2-byte NALU lengths
	_, err := mp4.DecodeBox(0, bytes.NewReader(data))
	_, errSR := mp4.DecodeBoxSR(0, bits.NewFixedSliceReader(data))
	for _, e := range []error{err, errSR} {
		if !isMismatch(e) {
			t.Errorf("decode lhvC 2-byte, hvcC 4-byte: got error %v, want mismatch error", e)
		}
	}

	hvc1 := newHvc1(1)
	buf.Reset()
	if err := hvc1.Encode(&buf); !isMismatch(err) || buf.Len() != 0 {
		t.Errorf("Encode lhvC 2-byte, hvcC 4-byte: got error %v and %d bytes, want mismatch error and none", err, buf.Len())
	}
	sw := bits.NewFixedSliceWriter(int(hvc1.Size()))
	if err := hvc1.EncodeSW(sw); !isMismatch(err) || sw.Offset() != 0 {
		t.Errorf("EncodeSW lhvC 2-byte, hvcC 4-byte: got error %v and %d bytes, want mismatch error and none", err, sw.Offset())
	}
}
