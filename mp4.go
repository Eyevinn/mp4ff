package mp4

import "io"

// A MPEG-4 media
//
// A MPEG-4 media contains three main boxes :
//
//   ftyp : the file type box
//   moov : the movie box (meta-data)
//   mdat : the media data (chunks and samples)
//
// Other boxes can also be present (pdin, moof, mfra, free, ...), but are not decoded.
type MP4 struct {
	Ftyp  *FtypBox
	Moov  *MoovBox
	Mdat  *MdatBox
	boxes []Box
}

// Decode decodes a media from a Reader
func Decode(r io.Reader) (*MP4, error) {
	v := &MP4{
		boxes: []Box{},
	}
LoopBoxes:
	for {
		h, err := DecodeHeader(r)
		if err != nil {
			return nil, err
		}
		box, err := DecodeBox(h, r)
		if err != nil {
			return nil, err
		}
		v.boxes = append(v.boxes, box)
		switch h.Type {
		case "ftyp":
			v.Ftyp = box.(*FtypBox)
		case "moov":
			v.Moov = box.(*MoovBox)
		case "mdat":
			v.Mdat = box.(*MdatBox)
			v.Mdat.ContentSize = h.Size - BoxHeaderSize
			break LoopBoxes
		}
	}
	return v, nil
}

// Dump displays some information about a media
func (m *MP4) Dump() {
	m.Ftyp.Dump()
	m.Moov.Dump()
}

// Boxes lists the top-level boxes from a media
func (m *MP4) Boxes() []Box {
	return m.boxes
}

// Encode encodes a media to a Writer
func (m *MP4) Encode(w io.Writer) error {
	err := m.Ftyp.Encode(w)
	if err != nil {
		return err
	}
	err = m.Moov.Encode(w)
	if err != nil {
		return err
	}
	for _, b := range m.boxes {
		if b.Type() != "ftyp" && b.Type() != "moov" {
			err = b.Encode(w)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
