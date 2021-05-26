package mp4

import (
	"io"
	"io/ioutil"
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
func DecodePrft(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := ioutil.ReadAll(r)
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
func (p *PrftBox) Type() string {
	return "prft"
}

// Size - return calculated size
func (p *PrftBox) Size() uint64 {
	return uint64(boxHeaderSize + 16 + 4*int(p.Version))
}

// Encode - write box to w
func (p *PrftBox) Encode(w io.Writer) error {
	err := EncodeHeader(p, w)
	if err != nil {
		return err
	}
	buf := makebuf(p)
	sw := NewSliceWriter(buf)
	versionAndFlags := (uint32(p.Version) << 24) + p.Flags
	sw.WriteUint32(versionAndFlags)
	sw.WriteUint64(p.NTPTimestamp)
	if p.Version == 0 {
		sw.WriteUint32(uint32(p.MediaTime))
	} else {
		sw.WriteUint64(p.MediaTime)
	}
	_, err = w.Write(buf)
	return err
}

func (p *PrftBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, p, int(p.Version), p.Flags)
	bd.write(" - ntpTimestamp: %d", p.NTPTimestamp)
	bd.write(" - mediaTime: %d", p.MediaTime)
	return bd.err
}
