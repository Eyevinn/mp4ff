package mp4

import (
	"io"

	"github.com/Eyevinn/mp4ff/bits"
)

// MhaCBox - MPEG-H MHACConfigurationBox
// According to ISO/IEC 23008-3: 2018, Section 20.5.2
type MhaCBox struct {
	MHADecoderConfigRecord MHADecoderConfigurationRecord
}

// MHADecoderConfigurationRecord - MPEG-H MHADecoderConfigurationRecord
// According to ISO/IEC 23008-3: 2018, Section 20.4.2
type MHADecoderConfigurationRecord struct {
	ConfigVersion                  uint8
	MpegH3DAProfileLevelIndication uint8
	ReferenceChannelLayout         uint8
	MpegH3DAConfigLength           uint16
	MpegH3DAConfig                 []byte
}

// DecodeMhaC - box-specific decode
func DecodeMhaC(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := readBoxBody(r, hdr)
	if err != nil {
		return nil, err
	}
	return decodeMhacFromData(data)
}

// DecodeMhaCSR - box-specific decode
func DecodeMhaCSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	data := sr.ReadBytes(hdr.payloadLen())
	if sr.AccError() != nil {
		return nil, sr.AccError()
	}
	return decodeMhacFromData(data)
}

func decodeMhacFromData(data []byte) (Box, error) {
	b := MhaCBox{}
	sr := bits.NewFixedSliceReader(data)

	b.MHADecoderConfigRecord.ConfigVersion = sr.ReadUint8()
	b.MHADecoderConfigRecord.MpegH3DAProfileLevelIndication = sr.ReadUint8()
	b.MHADecoderConfigRecord.ReferenceChannelLayout = sr.ReadUint8()
	b.MHADecoderConfigRecord.MpegH3DAConfigLength = sr.ReadUint16()
	b.MHADecoderConfigRecord.MpegH3DAConfig = sr.ReadBytes(int(b.MHADecoderConfigRecord.MpegH3DAConfigLength))

	if sr.AccError() != nil {
		return nil, sr.AccError()
	}
	return &b, nil
}

// Type - box type
func (b *MhaCBox) Type() string {
	return "mhaC"
}

// Size - calculated size of box
func (b *MhaCBox) Size() uint64 {
	return uint64(boxHeaderSize + 5 + len(b.MHADecoderConfigRecord.MpegH3DAConfig))
}

// Encode - write box to w
func (b *MhaCBox) Encode(w io.Writer) error {
	sw := bits.NewFixedSliceWriter(int(b.Size()))
	err := b.EncodeSW(sw)
	if err != nil {
		return err
	}
	_, err = w.Write(sw.Bytes())
	return err
}

// EncodeSW - write box to sw
func (b *MhaCBox) EncodeSW(sw bits.SliceWriter) error {
	err := EncodeHeaderSW(b, sw)
	if err != nil {
		return err
	}

	sw.WriteUint8(b.MHADecoderConfigRecord.ConfigVersion)
	sw.WriteUint8(b.MHADecoderConfigRecord.MpegH3DAProfileLevelIndication)
	sw.WriteUint8(b.MHADecoderConfigRecord.ReferenceChannelLayout)
	sw.WriteUint16(b.MHADecoderConfigRecord.MpegH3DAConfigLength)
	sw.WriteBytes(b.MHADecoderConfigRecord.MpegH3DAConfig)

	return sw.AccError()
}

// Info - write box-specific information
func (b *MhaCBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, -1, 0)
	bd.write(" - configVersion=%d", b.MHADecoderConfigRecord.ConfigVersion)
	bd.write(" - mpegH3DAProfileLevelIndication=%d", b.MHADecoderConfigRecord.MpegH3DAProfileLevelIndication)
	bd.write(" - referenceChannelLayout=%d", b.MHADecoderConfigRecord.ReferenceChannelLayout)
	bd.write(" - mpegH3DAConfigLength=%d", b.MHADecoderConfigRecord.MpegH3DAConfigLength)
	if getInfoLevel(b, specificBoxLevels) > 0 {
		bd.write("   - mpegH3DAConfig=%x", b.MHADecoderConfigRecord.MpegH3DAConfig)
	}
	return bd.err
}
