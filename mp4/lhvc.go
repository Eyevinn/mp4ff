package mp4

import (
	"encoding/hex"
	"io"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/Eyevinn/mp4ff/hevc"
)

// LhvCBox - LHEVCConfigurationBox (ISO/IEC 14496-15 Ed. 7 Sec. 9.5.3.1)
// Carries an L-HEVC decoder configuration record for the enhancement layers of
// a layered HEVC (MV-HEVC/SHVC) stream. It appears alongside an HvcCBox in the
// visual sample entry. The binary format differs from hvcC: the profile/tier/
// level and chroma/bit-depth/frame-rate fields are omitted (see hevc Sec. 9.4.3).
type LhvCBox struct {
	hevc.DecConfRec
}

// CreateLhvCFromNalus creates an LhvCBox from enhancement-layer parameter sets.
func CreateLhvCFromNalus(spsNalus, ppsNalus [][]byte) *LhvCBox {
	naluArrays := []hevc.NaluArray{
		hevc.NewNaluArray(true, hevc.NALU_SPS, spsNalus),
		hevc.NewNaluArray(true, hevc.NALU_PPS, ppsNalus),
	}
	dcr := hevc.DecConfRec{
		ConfigurationVersion: 1,
		LengthSizeMinusOne:   3,
		NaluArrays:           naluArrays,
	}
	return &LhvCBox{dcr}
}

// DecodeLhvC - box-specific decode
func DecodeLhvC(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := readBoxBody(r, hdr)
	if err != nil {
		return nil, err
	}
	hdcr, err := hevc.DecodeLHEVCDecConfRec(data)
	if err != nil {
		return nil, err
	}
	return &LhvCBox{hdcr}, nil
}

// DecodeLhvCSR - box-specific decode
func DecodeLhvCSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	hdcr, err := hevc.DecodeLHEVCDecConfRec(sr.ReadBytes(hdr.payloadLen()))
	if err != nil {
		return nil, err
	}
	return &LhvCBox{hdcr}, nil
}

// Type - return box type
func (b *LhvCBox) Type() string {
	return "lhvC"
}

// Size - return calculated size
func (b *LhvCBox) Size() uint64 {
	return uint64(boxHeaderSize) + b.LHEVCSize()
}

// Encode - write box to w
func (b *LhvCBox) Encode(w io.Writer) error {
	err := EncodeHeader(b, w)
	if err != nil {
		return err
	}
	return b.EncodeLHEVC(w)
}

// EncodeSW - write box to sw
func (b *LhvCBox) EncodeSW(sw bits.SliceWriter) error {
	err := EncodeHeaderSW(b, sw)
	if err != nil {
		return err
	}
	return b.EncodeLHEVCSW(sw)
}

// Info - box-specific Info
func (b *LhvCBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, -1, 0)
	hdcr := b.DecConfRec
	bd.write(" - MinSpatialSegmentationIDC: %d", hdcr.MinSpatialSegmentationIDC)
	bd.write(" - ParallelismType: %d", hdcr.ParallellismType)
	bd.write(" - NumTemporalLayers: %d", hdcr.NumTemporalLayers)
	bd.write(" - TemporalIdNested: %d", hdcr.TemporalIDNested)
	bd.write(" - LengthSizeMinusOne: %d", hdcr.LengthSizeMinusOne)
	for _, array := range hdcr.NaluArrays {
		bd.write("   - %s complete: %d", array.NaluType(), array.Complete())
		for _, nalu := range array.Nalus {
			bd.write("    %s", hex.EncodeToString(nalu))
		}
	}
	return bd.err
}
