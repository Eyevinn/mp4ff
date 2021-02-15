package mp4

import (
	"io"
)

// MfraBox - Movie Fragment Random Access Box (mfra)
// Container for TfraBox(es) that can be used to find sync samples
type MfraBox struct {
	Tfra     *TfraBox
	Tfras    []*TfraBox
	Mfro     *MfroBox
	Children []Box
	StartPos uint64
}

// DecodeMfra - box-specific decode
func DecodeMfra(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	children, err := DecodeContainerChildren(hdr, startPos+8, startPos+hdr.size, r)
	if err != nil {
		return nil, err
	}
	m := &MfraBox{}
	m.StartPos = startPos
	for _, box := range children {
		err := m.AddChild(box)
		if err != nil {
			return nil, err
		}
	}
	return m, nil
}

// AddChild - add child box
func (m *MfraBox) AddChild(child Box) error {
	switch child.Type() {
	case "tfra":
		if m.Tfra == nil {
			m.Tfra = child.(*TfraBox)
		}
		m.Tfras = append(m.Tfras, child.(*TfraBox))
	case "mfro":
		m.Mfro = child.(*MfroBox)
	}
	m.Children = append(m.Children, child)
	return nil
}

// Type - returns box type
func (m *MfraBox) Type() string {
	return "mfra"
}

// Size - returns calculated size
func (m *MfraBox) Size() uint64 {
	return containerSize(m.Children)
}

// Encode - byte-specific encode
func (m *MfraBox) Encode(w io.Writer) error {
	err := EncodeHeader(m, w)
	if err != nil {
		return err
	}
	for _, b := range m.Children {
		err = b.Encode(w)
		if err != nil {
			return err
		}
	}
	return nil
}

// GetChildren - list of child boxes
func (m *MfraBox) GetChildren() []Box {
	return m.Children
}

func (m *MfraBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	return ContainerInfo(m, w, specificBoxLevels, indent, indentStep)
}
