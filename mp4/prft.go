package mp4

import (
	"io"

	"github.com/edgeware/mp4ff/bits"
)

// PrftBox - Producer Reference Box (prft)
//
// Contained in File before moof box
type PrftBox struct {
	Version      byte
	Flags        uint32
	NTPTimestamp uint64
	MediaTime    uint64
}

// CreatePrftBox - Create a new PrftBox
func CreatePrftBox(version byte, ntp uint64, mediatime uint64) *PrftBox {
	return &PrftBox{
		Version:      version,
		Flags:        0,
		NTPTimestamp: ntp,
		MediaTime:    mediatime,
	}
}

// DecodePrft - box-specific decode
func DecodePrft(hdr boxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := readBoxBody(r, hdr)
	if err != nil {
		return nil, err
	}
	s := NewSliceReader(data)
	versionAndFlags := s.ReadUint32()
	version := byte(versionAndFlags >> 24)
	flags := versionAndFlags & flagsMask
	ntp := s.ReadUint64()
	var mediatime uint64
	if version == 0 {
		mediatime = uint64(s.ReadUint32())
	} else {
		mediatime = s.ReadUint64()
	}

	p := &PrftBox{
		Version:      version,
		Flags:        flags,
		NTPTimestamp: ntp,
		MediaTime:    mediatime,
	}
	return p, nil
}

// Type - return box type
func (b *PrftBox) Type() string {
	return "prft"
}

// Size - return calculated size
func (b *PrftBox) Size() uint64 {
	return uint64(boxHeaderSize + 16 + 4*int(b.Version))
}

// Encode - write box to w
func (b *PrftBox) Encode(w io.Writer) error {
	sw := bits.NewSliceWriterWithSize(int(b.Size()))
	err := b.EncodeSW(sw)
	if err != nil {
		return err
	}
	_, err = w.Write(sw.Bytes())
	return err
}

// EncodeSW - box-specific encode to slicewriter
func (b *PrftBox) EncodeSW(sw bits.SliceWriter) error {
	err := EncodeHeaderSW(b, sw)
	if err != nil {
		return err
	}
	versionAndFlags := (uint32(b.Version) << 24) + b.Flags
	sw.WriteUint32(versionAndFlags)
	sw.WriteUint64(b.NTPTimestamp)
	if b.Version == 0 {
		sw.WriteUint32(uint32(b.MediaTime))
	} else {
		sw.WriteUint64(b.MediaTime)
	}
	return sw.AccError()
}

// Info - write box-specific information
func (b *PrftBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, int(b.Version), b.Flags)
	bd.write(" - ntpTimestamp: %d", b.NTPTimestamp)
	bd.write(" - mediaTime: %d", b.MediaTime)
	return bd.err
}
