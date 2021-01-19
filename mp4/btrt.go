package mp4

import (
	"encoding/binary"
	"io"
	"io/ioutil"
)

// BtrtBox - BitRateBox - ISO/IEC 14496-12 Section 8.5.2.2
type BtrtBox struct {
	BufferSizeDB uint32
	MaxBitrate   uint32
	AvgBitrate   uint32
}

// DecodeBtrt - box-specific decode
func DecodeBtrt(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := ioutil.ReadAll(r)
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
	var err error

	err = EncodeHeader(b, w)
	if err != nil {
		return err
	}

	write := func(b uint32) {
		if err != nil {
			return
		}
		err = binary.Write(w, binary.BigEndian, b)
	}

	write(b.BufferSizeDB)
	write(b.MaxBitrate)
	write(b.AvgBitrate)

	return err
}

func (b *BtrtBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, -1, 0)
	bd.write(" - bufferSizeDB: %d", b.BufferSizeDB)
	bd.write(" - maxBitrate: %d", b.MaxBitrate)
	bd.write(" - AvgBitrate: %d", b.AvgBitrate)
	return bd.err
}
