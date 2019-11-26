package mp4

import (
	"fmt"
	"io"
)

// MoovBox - Movie Box (moov - mandatory)
//
// Status: partially decoded (anything other than mvhd, iods, trak or udta is ignored)
//
// Contains all meta-data. To be able to stream a file, the moov box should be placed before the mdat box.
type MoovBox struct {
	Mvhd  *MvhdBox
	Trak  []*TrakBox
	Mvex  *MvexBox
	boxes []Box
}

// DecodeMoov - box-specific decode
func DecodeMoov(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	l, err := DecodeContainerChildren(hdr, startPos, r)
	if err != nil {
		return nil, err
	}
	m := &MoovBox{}
	m.boxes = l
	for _, b := range l {
		switch b.Type() {
		case "mvhd":
			m.Mvhd = b.(*MvhdBox)
		case "trak":
			m.Trak = append(m.Trak, b.(*TrakBox))
		case "mvex":
			m.Mvex = b.(*MvexBox)
		}
	}
	return m, err
}

// Type - box type
func (b *MoovBox) Type() string {
	return "moov"
}

// Size - calculated size of box
func (b *MoovBox) Size() uint64 {
	return containerSize(b.boxes)
}

// Dump - print box info
func (b *MoovBox) Dump() {
	b.Mvhd.Dump()
	for i, t := range b.Trak {
		fmt.Println("Track", i)
		t.Dump()
	}
}

// Encode - write box to w
func (b *MoovBox) Encode(w io.Writer) error {
	err := EncodeHeader(b, w)
	if err != nil {
		return err
	}
	for _, b := range b.boxes {
		err = b.Encode(w)
		if err != nil {
			return err
		}
	}
	return nil
}
