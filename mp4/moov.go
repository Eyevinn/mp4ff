package mp4

import (
	"io"

	"github.com/edgeware/mp4ff/bits"
)

// MoovBox - Movie Box (moov - mandatory)
//
// Contains all meta-data. To be able to stream a file, the moov box should be placed before the mdat box.
type MoovBox struct {
	Mvhd     *MvhdBox
	Trak     *TrakBox // The first trak box
	Traks    []*TrakBox
	Mvex     *MvexBox
	Pssh     *PsshBox
	Psshs    []*PsshBox
	Children []Box
	StartPos uint64
}

// NewMoovBox - Generate a new empty moov box
func NewMoovBox() *MoovBox {
	return &MoovBox{}
}

// AddChild - Add a child box
func (m *MoovBox) AddChild(box Box) {
	switch box.Type() {
	case "mvhd":
		m.Mvhd = box.(*MvhdBox)
	case "trak":
		trak := box.(*TrakBox)
		if m.Trak == nil {
			m.Trak = trak
		}
		m.Traks = append(m.Traks, trak)
		// Possibley re-order to keep traks together on same
		// side of mvex or similar. Put this trak after last previous trak
		lastTrakIdx := 0
		for i, child := range m.Children {
			if child.Type() == "trak" {
				lastTrakIdx = i
			}
		}
		if lastTrakIdx != 0 && lastTrakIdx != len(m.Children)-1 { // last one in middle
			m.Children = append(m.Children[:lastTrakIdx+2], m.Children[lastTrakIdx+1:]...)
			m.Children[lastTrakIdx+1] = trak
			return
		}
	case "mvex":
		m.Mvex = box.(*MvexBox)
	case "pssh":
		pssh := box.(*PsshBox)
		if m.Pssh == nil {
			m.Pssh = pssh
		}
		m.Psshs = append(m.Psshs, pssh)
	}
	m.Children = append(m.Children, box)
}

// DecodeMoov - box-specific decode
func DecodeMoov(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	data := make([]byte, hdr.payloadLen())
	_, err := io.ReadFull(r, data)
	if err != nil {
		return nil, err
	}
	sr := bits.NewFixedSliceReader(data)
	children, err := DecodeContainerChildrenSR(hdr, startPos+8, startPos+hdr.Size, sr)
	if err != nil {
		return nil, err
	}
	m := MoovBox{Children: make([]Box, 0, len(children))}
	m.StartPos = startPos
	for _, c := range children {
		m.AddChild(c)
	}
	return &m, err
}

// DecodeMoovSR - box-specific decode
func DecodeMoovSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	children, err := DecodeContainerChildrenSR(hdr, startPos+8, startPos+hdr.Size, sr)
	if err != nil {
		return nil, err
	}
	m := MoovBox{Children: make([]Box, 0, len(children))}
	m.StartPos = startPos
	for _, c := range children {
		m.AddChild(c)
	}
	return &m, err
}

// Type - box type
func (m *MoovBox) Type() string {
	return "moov"
}

// Size - calculated size of box
func (m *MoovBox) Size() uint64 {
	return containerSize(m.Children)
}

// GetChildren - list of child boxes
func (m *MoovBox) GetChildren() []Box {
	return m.Children
}

// Encode - write moov container to w
func (m *MoovBox) Encode(w io.Writer) error {
	return EncodeContainer(m, w)
}

// Encode - write moov container to sw
func (m *MoovBox) EncodeSW(sw bits.SliceWriter) error {
	return EncodeContainerSW(m, sw)
}

// Info - write box-specific information
func (m *MoovBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	return ContainerInfo(m, w, specificBoxLevels, indent, indentStep)
}

// RemovePsshs - remove and return all psshs children boxes
func (m *MoovBox) RemovePsshs() []*PsshBox {
	if m.Pssh == nil {
		return nil
	}
	psshs := m.Psshs
	newChildren := make([]Box, 0, len(m.Children)-len(m.Psshs))
	for i := range m.Children {
		if m.Children[i].Type() != "pssh" {
			newChildren = append(newChildren, m.Children[i])
		}
	}
	m.Children = newChildren
	m.Pssh = nil
	m.Psshs = nil

	return psshs
}

func (m *MoovBox) GetSinf(trackID uint32) *SinfBox {
	for _, trak := range m.Traks {
		if trak.Tkhd.TrackID == trackID {
			stsd := trak.Mdia.Minf.Stbl.Stsd
			sd := stsd.Children[0] // Get first (and only)
			if visual, ok := sd.(*VisualSampleEntryBox); ok {
				return visual.Sinf
			}
			if audio, ok := sd.(*AudioSampleEntryBox); ok {
				return audio.Sinf
			}
		}
	}
	return nil
}
