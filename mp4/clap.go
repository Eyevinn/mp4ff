package mp4

import (
	"io"
	"io/ioutil"
)

// ClapBox - Clean Aperture Box, ISO/IEC 14496-12 2020 Sec. 12.1.4
type ClapBox struct {
	CleanApertureWidthN  uint32
	CleanApertureWidthD  uint32
	CleanApertureHeightN uint32
	CleanApertureHeightD uint32
	HorizOffN            uint32
	HorizOffD            uint32
	VertOffN             uint32
	VertOffD             uint32
}

// DecideClap - box-specific decode
func DecodeClap(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	clap := &ClapBox{}
	sr := NewSliceReader(data)
	clap.CleanApertureWidthN = sr.ReadUint32()
	clap.CleanApertureWidthD = sr.ReadUint32()
	clap.CleanApertureHeightN = sr.ReadUint32()
	clap.CleanApertureHeightD = sr.ReadUint32()
	clap.HorizOffN = sr.ReadUint32()
	clap.HorizOffD = sr.ReadUint32()
	clap.VertOffN = sr.ReadUint32()
	clap.VertOffD = sr.ReadUint32()
	return clap, nil
}

// Type - box type
func (b *ClapBox) Type() string {
	return "clap"
}

// Size - calculated size of box
func (b *ClapBox) Size() uint64 {
	return uint64(boxHeaderSize + 32)
}

// Encode - write box to w
func (b *ClapBox) Encode(w io.Writer) error {
	err := EncodeHeader(b, w)
	if err != nil {
		return err
	}
	buf := makebuf(b)
	sw := NewSliceWriter(buf)
	sw.WriteUint32(b.CleanApertureWidthN)
	sw.WriteUint32(b.CleanApertureWidthD)
	sw.WriteUint32(b.CleanApertureHeightN)
	sw.WriteUint32(b.CleanApertureHeightD)
	sw.WriteUint32(b.HorizOffN)
	sw.WriteUint32(b.HorizOffD)
	sw.WriteUint32(b.VertOffN)
	sw.WriteUint32(b.VertOffD)
	_, err = w.Write(buf)
	return err
}

func (b *ClapBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, -1, 0)
	bd.write(" - cleanAperturWidth: %d/%d", b.CleanApertureWidthN, b.CleanApertureWidthD)
	bd.write(" - cleanApertureHeight: %d/%d", b.CleanApertureHeightN, b.CleanApertureHeightD)
	bd.write(" - horizOff: %d/%d", b.HorizOffN, b.HorizOffD)
	bd.write(" - vertOff: %d/%d", b.VertOffN, b.VertOffD)
	return bd.err
}
