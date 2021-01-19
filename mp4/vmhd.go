package mp4

import (
	"io"
	"io/ioutil"
)

// VmhdBox - Video Media Header Box (vhmd - mandatory for video tracks)
//
// Contained in : Media Information Box (minf)
type VmhdBox struct {
	Version      byte
	Flags        uint32
	GraphicsMode uint16
	OpColor      [3]uint16
}

// CreateVmhd - Create Video Media Header Box
func CreateVmhd() *VmhdBox {
	// Flags should be 0x000001 according to ISO/IEC 14496-12 Sec.12.1.2.1
	return &VmhdBox{Flags: 0x000001}
}

// DecodeVmhd - box-specific decode
func DecodeVmhd(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	s := NewSliceReader(data)
	versionAndFlags := s.ReadUint32()
	b := &VmhdBox{
		Version:      byte(versionAndFlags >> 24),
		Flags:        versionAndFlags & flagsMask,
		GraphicsMode: s.ReadUint16(),
	}
	for i := 0; i < 3; i++ {
		b.OpColor[i] = s.ReadUint16()
	}
	return b, nil
}

// Type - box-specific type
func (b *VmhdBox) Type() string {
	return "vmhd"
}

// Size - calculated size of box
func (b *VmhdBox) Size() uint64 {
	return boxHeaderSize + 12
}

// Encode - write box to w
func (b *VmhdBox) Encode(w io.Writer) error {
	err := EncodeHeader(b, w)
	if err != nil {
		return err
	}
	buf := makebuf(b)
	sw := NewSliceWriter(buf)
	versionAndFlags := (uint32(b.Version) << 24) + b.Flags
	sw.WriteUint32(versionAndFlags)
	sw.WriteUint16(b.GraphicsMode)
	for i := 0; i < 3; i++ {
		sw.WriteUint16(b.OpColor[i])
	}
	_, err = w.Write(buf)
	return err
}

func (b *VmhdBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, int(b.Version), b.Flags)
	return bd.err
}
