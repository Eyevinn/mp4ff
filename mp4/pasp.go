package mp4

import (
	"io"
	"io/ioutil"
)

// PaspBox - Pixel Aspect Ratio Box, ISO/IEC 14496-12 2020 Sec. 12.1.4
type PaspBox struct {
	HSpacing uint32
	VSpacing uint32
}

// DecodePasp - box-specific decode
func DecodePasp(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	pasp := &PaspBox{}
	sr := NewSliceReader(data)
	pasp.HSpacing = sr.ReadUint32()
	pasp.VSpacing = sr.ReadUint32()
	return pasp, nil
}

// Type - box type
func (b *PaspBox) Type() string {
	return "pasp"
}

// Size - calculated size of box
func (b *PaspBox) Size() uint64 {
	return uint64(boxHeaderSize + 8)
}

// Encode - write box to w
func (b *PaspBox) Encode(w io.Writer) error {
	err := EncodeHeader(b, w)
	if err != nil {
		return err
	}
	buf := makebuf(b)
	sw := NewSliceWriter(buf)
	sw.WriteUint32(b.HSpacing)
	sw.WriteUint32(b.VSpacing)
	_, err = w.Write(buf)
	return err
}

func (b *PaspBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, -1, 0)
	bd.write(" - hSpacing:vSpacing: %d:%d", b.HSpacing, b.VSpacing)
	return bd.err
}
