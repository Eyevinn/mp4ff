package mp4

import (
	"io"
	"log"
	"os"
)

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
	Ftyp         *FtypBox
	Moov         *MoovBox
	Mdat         *MdatBox
	boxes        []Box
	isFragmented bool
}

// Decode decodes a media from a Reader
func Decode(r io.Reader) (*MP4, error) {
	v := &MP4{
		boxes: []Box{},
	}
LoopBoxes:
	for {
		//f := r.(*os.File)
		//p, err := f.Seek(0, os.SEEK_CUR)
		//log.Printf("Byte position is %v", p)

		h, err := DecodeHeader(r)
		if err == io.EOF || h.Size == 0 {
			break LoopBoxes
		}
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
			if len(v.Moov.Trak[0].Mdia.Minf.Stbl.Stts.SampleCount) == 0 {
				v.isFragmented = true
			}
		case "mdat":
			if !v.isFragmented {
				v.Mdat = box.(*MdatBox)
				v.Mdat.ContentSize = h.Size - BoxHeaderSize
			}
			if rs, seekable := r.(io.Seeker); seekable {
				nextPos := int64(h.Size) - BoxHeaderSize
				newPos, err := rs.Seek(nextPos, os.SEEK_CUR)
				if err != nil {
					log.Fatal("Could not seek")
				}
				log.Printf("New pos after mdat is %v", newPos)
			}
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
	for _, b := range m.Boxes() {
		err := b.Encode(w)
		if err != nil {
			return err
		}
	}
	return nil
}
