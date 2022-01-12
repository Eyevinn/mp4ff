package mp4

import (
	"encoding/binary"
	"io"

	"github.com/edgeware/mp4ff/bits"
)

// BtrtBox - BitRateBox - ISO/IEC 14496-12 Section 8.5.2.2
type BtrtBox struct {
	BufferSizeDB uint32
	MaxBitrate   uint32
	AvgBitrate   uint32
}

// DecodeBtrt - box-specific decode
func DecodeBtrt(hdr boxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := readBoxBody(r, hdr)
	if err != nil {
		return nil, err
	}

	b := &BtrtBox{
		BufferSizeDB: binary.BigEndian.Uint32(data[0:4]),
		MaxBitrate:   binary.BigEndian.Uint32(data[4:8]),
		AvgBitrate:   binary.BigEndian.Uint32(data[8:12]),
	}
	return b, nil
}

// Type - return box type
func (b *BtrtBox) Type() string {
	return "btrt"
}

// Size - return calculated size
func (b *BtrtBox) Size() uint64 {
	return 20
}

// Encode - write box to w
func (b *BtrtBox) Encode(w io.Writer) error {
	sw := bits.NewFixedSliceWriter(int(b.Size()))
	err := b.EncodeSW(sw)
	if err != nil {
		return err
	}
	_, err = w.Write(sw.Bytes())
	return err
}

// EncodeSW - box-specific encode to slicewriter
func (b *BtrtBox) EncodeSW(sw bits.SliceWriter) error {
	err := EncodeHeaderSW(b, sw)
	if err != nil {
		return err
	}
	sw.WriteUint32(b.BufferSizeDB)
	sw.WriteUint32(b.MaxBitrate)
	sw.WriteUint32(b.AvgBitrate)
	return sw.AccError()
}

// Info - write box-specific information
func (b *BtrtBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, -1, 0)
	bd.write(" - bufferSizeDB: %d", b.BufferSizeDB)
	bd.write(" - maxBitrate: %d", b.MaxBitrate)
	bd.write(" - AvgBitrate: %d", b.AvgBitrate)
	return bd.err
}
