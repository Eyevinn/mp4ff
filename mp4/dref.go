package mp4

import (
	"encoding/binary"
	"io"
)

// DrefBox - Data Reference Box (dref - mandatory)
//
// Contained id: Data Information Box (dinf)
//
// Defines the location of the media data. If the data for the track is located in the same file
// it contains nothing useful.
type DrefBox struct {
	Version    byte
	Flags      uint32
	EntryCount int
	Boxes      []Box
}

// CreateDref - Create an DataReferenceBox for selfcontained content
func CreateDref() *DrefBox {
	url := CreateURLBox()
	dref := &DrefBox{}
	dref.AddChild(url)
	return dref
}

// AddChild - Add a child box and update EntryCount
func (d *DrefBox) AddChild(box Box) {
	d.Boxes = append(d.Boxes, box)
	d.EntryCount++
}

// DecodeDref - box-specific decode
func DecodeDref(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	var versionAndFlags, entryCount uint32
	err := binary.Read(r, binary.BigEndian, &versionAndFlags)
	if err != nil {
		return nil, err
	}
	err = binary.Read(r, binary.BigEndian, &entryCount)
	if err != nil {
		return nil, err
	}

	boxes, err := DecodeContainerChildren(hdr, startPos+16, r) //Note higher startPos since not simple container
	if err != nil {
		return nil, err
	}

	dref := &DrefBox{
		Version: byte(versionAndFlags >> 24),
		Flags:   versionAndFlags & flagsMask,
	}

	for _, b := range boxes {
		dref.AddChild(b)
	}
	if int(entryCount) != dref.EntryCount {
		panic("Inconsistent entry count in Dref")
	}
	return dref, nil
}

// Type - box type
func (d *DrefBox) Type() string {
	return "dref"
}

// Size - calculated size of box
func (d *DrefBox) Size() uint64 {
	return containerSize(d.Boxes) + 8
}

// Encode - write box to w
func (d *DrefBox) Encode(w io.Writer) error {
	err := EncodeHeader(d, w)
	if err != nil {
		return err
	}
	versionAndFlags := (uint32(d.Version) << 24) + d.Flags
	err = binary.Write(w, binary.BigEndian, versionAndFlags)
	if err != nil {
		return err
	}
	err = binary.Write(w, binary.BigEndian, uint32(d.EntryCount))
	if err != nil {
		return err
	}
	for _, b := range d.Boxes {
		err = b.Encode(w)
		if err != nil {
			return err
		}
	}
	return err
}
