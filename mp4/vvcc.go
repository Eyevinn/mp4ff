package mp4

import (
	"encoding/hex"
	"io"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/Eyevinn/mp4ff/vvc"
)

// VvcCBox - VVC Configuration Box (ISO/IEC 14496-15)
// Contains one VVCDecoderConfigurationRecord
type VvcCBox struct {
	Version byte
	Flags   uint32
	vvc.DecConfRec
}

// CreateVvcC creates a VvcC box
func CreateVvcC(naluArrays []vvc.NaluArray) (*VvcCBox, error) {
	vvcDecConfRec := vvc.DecConfRec{
		LengthSizeMinusOne: 3, // 4 bytes
		PtlPresentFlag:     false,
		NaluArrays:         naluArrays,
	}

	return &VvcCBox{
		Version:    0,
		Flags:      0,
		DecConfRec: vvcDecConfRec,
	}, nil
}

// DecodeVvcC - box-specific decode
func DecodeVvcC(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := readBoxBody(r, hdr)
	if err != nil {
		return nil, err
	}
	sr := bits.NewFixedSliceReader(data)
	versionAndFlags := sr.ReadUint32()

	vvcDecConfRec, err := vvc.DecodeVVCDecConfRec(sr.ReadBytes(sr.NrRemainingBytes()))
	if err != nil {
		return nil, err
	}
	return &VvcCBox{
		Version:    byte(versionAndFlags >> 24),
		Flags:      versionAndFlags & flagsMask,
		DecConfRec: vvcDecConfRec,
	}, nil
}

// DecodeVvcCSR - box-specific decode
func DecodeVvcCSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	versionAndFlags := sr.ReadUint32()
	vvcDecConfRec, err := vvc.DecodeVVCDecConfRec(sr.ReadBytes(hdr.payloadLen() - 4))
	return &VvcCBox{
		Version:    byte(versionAndFlags >> 24),
		Flags:      versionAndFlags & flagsMask,
		DecConfRec: vvcDecConfRec,
	}, err
}

// Type - return box type
func (b *VvcCBox) Type() string {
	return "vvcC"
}

// Size - return calculated size
func (b *VvcCBox) Size() uint64 {
	return uint64(boxHeaderSize + 4 + b.DecConfRec.Size()) // 4 bytes for version and flags
}

// Encode - write box to w
func (b *VvcCBox) Encode(w io.Writer) error {
	err := EncodeHeader(b, w)
	if err != nil {
		return err
	}
	sw := bits.NewFixedSliceWriter(4)
	sw.WriteUint32(uint32(b.Version)<<24 | b.Flags)
	_, err = w.Write(sw.Bytes())
	if err != nil {
		return err
	}
	return b.DecConfRec.Encode(w)
}

// EncodeSW - write box to sw
func (b *VvcCBox) EncodeSW(sw bits.SliceWriter) error {
	err := EncodeHeaderSW(b, sw)
	if err != nil {
		return err
	}
	sw.WriteUint32(uint32(b.Version)<<24 | b.Flags)
	return b.DecConfRec.EncodeSW(sw)
}

// Info - box-specific Info
func (b *VvcCBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, -1, 0)
	vdcr := b.DecConfRec
	bd.write(" - LengthSizeMinusOne: %d", vdcr.LengthSizeMinusOne)
	bd.write(" - PtlPresentFlag: %t", vdcr.PtlPresentFlag)
	if vdcr.PtlPresentFlag {
		bd.write(" - OlsIdx: %d", vdcr.OlsIdx)
		bd.write(" - NumSublayers: %d", vdcr.NumSublayers)
		bd.write(" - ConstantFrameRate: %d", vdcr.ConstantFrameRate)
		bd.write(" - ChromaFormatIDC: %d", vdcr.ChromaFormatIDC)
		bd.write(" - BitDepthLuma: %d", vdcr.BitDepthMinus8+8)
		bd.write(" - NumBytesConstraintInfo: %d", vdcr.NativePTL.NumBytesConstraintInfo)
		bd.write(" - GeneralProfileIDC: %d", vdcr.NativePTL.GeneralProfileIDC)
		bd.write(" - GeneralTierFlag: %t", vdcr.NativePTL.GeneralTierFlag)
		bd.write(" - GeneralLevelIDC: %d", vdcr.NativePTL.GeneralLevelIDC)
		bd.write(" - PtlFrameOnlyConstraintFlag: %t", vdcr.NativePTL.PtlFrameOnlyConstraintFlag)
		bd.write(" - PtlMultiLayerEnabledFlag: %t", vdcr.NativePTL.PtlMultiLayerEnabledFlag)
		bd.write(" - PtlNumSubProfiles: %d", vdcr.NativePTL.PtlNumSubProfiles)
		bd.write(" - MaxPictureWidth: %d", vdcr.MaxPictureWidth)
		bd.write(" - MaxPictureHeight: %d", vdcr.MaxPictureHeight)
		bd.write(" - AvgFrameRate/256: %d", vdcr.AvgFrameRate)
	}
	for _, array := range vdcr.NaluArrays {
		bd.write("   - %s complete: %t", array.NaluTypeName(), array.Complete)
		for _, nalu := range array.Nalus {
			bd.write("    %s", hex.EncodeToString(nalu))
		}
	}
	return bd.err
}
