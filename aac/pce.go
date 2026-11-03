package aac

import (
	"fmt"

	"github.com/Eyevinn/mp4ff/bits"
)

// ProgramConfigElement is a program_config_element according to
// ISO/IEC 14496-3:2019 Table 4.2. In an AudioSpecificConfig, it gives the
// channel layout when the channel configuration is 0.
type ProgramConfigElement struct {
	ElementInstanceTag      byte
	ObjectType              byte // 2-bit index from Table 4.88: audio object type minus 1
	SamplingFrequencyIndex  byte
	FrontElements           []PCEChannelElement
	SideElements            []PCEChannelElement
	BackElements            []PCEChannelElement
	LFEElementTags          []byte
	AssocDataElementTags    []byte
	CCElements              []PCECouplingElement
	MonoMixdownPresent      bool
	MonoMixdownElement      byte // Present if MonoMixdownPresent
	StereoMixdownPresent    bool
	StereoMixdownElement    byte // Present if StereoMixdownPresent
	MatrixMixdownIdxPresent bool
	MatrixMixdownIdx        byte   // Present if MatrixMixdownIdxPresent
	PseudoSurroundEnable    bool   // Present if MatrixMixdownIdxPresent
	Comment                 []byte // comment_field_data, which may hold a height_extension_element
}

// PCEChannelElement is a front, side or back element of a program config element.
type PCEChannelElement struct {
	IsCPE     bool // Channel pair element (two channels) instead of a single channel element
	TagSelect byte
}

// PCECouplingElement is a coupling channel element of a program config element.
type PCECouplingElement struct {
	IsIndSw   bool // Independently switched coupling channel
	TagSelect byte
}

// NrChannels returns the number of channels of the program: two for each
// channel pair element and one for each single channel element and LFE
// element. Coupling channels are not counted.
func (p *ProgramConfigElement) NrChannels() int {
	n := len(p.LFEElementTags)
	for _, elems := range [][]PCEChannelElement{p.FrontElements, p.SideElements, p.BackElements} {
		for _, e := range elems {
			n++
			if e.IsCPE {
				n++
			}
		}
	}
	return n
}

// decodeProgramConfigElement reads a program config element. br must start at
// the start of the AudioSpecificConfig, since the byte alignment before the
// comment field is relative to that start.
func decodeProgramConfigElement(br *bits.Reader) (*ProgramConfigElement, error) {
	p := &ProgramConfigElement{}
	p.ElementInstanceTag = byte(br.Read(4))
	p.ObjectType = byte(br.Read(2))
	p.SamplingFrequencyIndex = byte(br.Read(4))
	nrFront := int(br.Read(4))
	nrSide := int(br.Read(4))
	nrBack := int(br.Read(4))
	nrLFE := int(br.Read(2))
	nrAssocData := int(br.Read(3))
	nrCC := int(br.Read(4))
	p.MonoMixdownPresent = br.ReadFlag()
	if p.MonoMixdownPresent {
		p.MonoMixdownElement = byte(br.Read(4))
	}
	p.StereoMixdownPresent = br.ReadFlag()
	if p.StereoMixdownPresent {
		p.StereoMixdownElement = byte(br.Read(4))
	}
	p.MatrixMixdownIdxPresent = br.ReadFlag()
	if p.MatrixMixdownIdxPresent {
		p.MatrixMixdownIdx = byte(br.Read(2))
		p.PseudoSurroundEnable = br.ReadFlag()
	}
	p.FrontElements = readPCEChannelElements(br, nrFront)
	p.SideElements = readPCEChannelElements(br, nrSide)
	p.BackElements = readPCEChannelElements(br, nrBack)
	p.LFEElementTags = readPCETags(br, nrLFE)
	p.AssocDataElementTags = readPCETags(br, nrAssocData)
	if nrCC > 0 {
		p.CCElements = make([]PCECouplingElement, nrCC)
		for i := range p.CCElements {
			p.CCElements[i].IsIndSw = br.ReadFlag()
			p.CCElements[i].TagSelect = byte(br.Read(4))
		}
	}
	br.ByteAlign()
	nrCommentBytes := int(br.Read(8))
	if br.AccError() == nil && nrCommentBytes > 0 {
		p.Comment = make([]byte, nrCommentBytes)
		for i := range p.Comment {
			p.Comment[i] = byte(br.Read(8))
		}
	}
	if br.AccError() != nil {
		return p, fmt.Errorf("program config element too short: %w", br.AccError())
	}
	return p, nil
}

// readPCEChannelElements returns nil for n == 0, so that decoded and
// constructed elements compare equal.
func readPCEChannelElements(br *bits.Reader, n int) []PCEChannelElement {
	if n == 0 {
		return nil
	}
	elems := make([]PCEChannelElement, n)
	for i := range elems {
		elems[i].IsCPE = br.ReadFlag()
		elems[i].TagSelect = byte(br.Read(4))
	}
	return elems
}

func readPCETags(br *bits.Reader, n int) []byte {
	if n == 0 {
		return nil
	}
	tags := make([]byte, n)
	for i := range tags {
		tags[i] = byte(br.Read(4))
	}
	return tags
}

func (p *ProgramConfigElement) encode(aw *ascWriter) {
	aw.write(uint(p.ElementInstanceTag), 4)
	aw.write(uint(p.ObjectType), 2)
	aw.write(uint(p.SamplingFrequencyIndex), 4)
	aw.write(uint(len(p.FrontElements)), 4)
	aw.write(uint(len(p.SideElements)), 4)
	aw.write(uint(len(p.BackElements)), 4)
	aw.write(uint(len(p.LFEElementTags)), 2)
	aw.write(uint(len(p.AssocDataElementTags)), 3)
	aw.write(uint(len(p.CCElements)), 4)
	aw.flag(p.MonoMixdownPresent)
	if p.MonoMixdownPresent {
		aw.write(uint(p.MonoMixdownElement), 4)
	}
	aw.flag(p.StereoMixdownPresent)
	if p.StereoMixdownPresent {
		aw.write(uint(p.StereoMixdownElement), 4)
	}
	aw.flag(p.MatrixMixdownIdxPresent)
	if p.MatrixMixdownIdxPresent {
		aw.write(uint(p.MatrixMixdownIdx), 2)
		aw.flag(p.PseudoSurroundEnable)
	}
	for _, elems := range [][]PCEChannelElement{p.FrontElements, p.SideElements, p.BackElements} {
		for _, e := range elems {
			aw.flag(e.IsCPE)
			aw.write(uint(e.TagSelect), 4)
		}
	}
	for _, tags := range [][]byte{p.LFEElementTags, p.AssocDataElementTags} {
		for _, tag := range tags {
			aw.write(uint(tag), 4)
		}
	}
	for _, e := range p.CCElements {
		aw.flag(e.IsIndSw)
		aw.write(uint(e.TagSelect), 4)
	}
	aw.byteAlign()
	aw.write(uint(len(p.Comment)), 8)
	for _, b := range p.Comment {
		aw.write(uint(b), 8)
	}
}

// validate checks that every field fits its bit width and that fields of
// absent options are zero, so that Encode writes what the fields say.
func (p *ProgramConfigElement) validate() error {
	limits := []struct {
		name string
		val  int
		max  int
	}{
		{"element instance tag", int(p.ElementInstanceTag), 15},
		{"object type", int(p.ObjectType), 3},
		{"sampling frequency index", int(p.SamplingFrequencyIndex), 15},
		{"number of front elements", len(p.FrontElements), 15},
		{"number of side elements", len(p.SideElements), 15},
		{"number of back elements", len(p.BackElements), 15},
		{"number of LFE elements", len(p.LFEElementTags), 3},
		{"number of assoc data elements", len(p.AssocDataElementTags), 7},
		{"number of coupling channel elements", len(p.CCElements), 15},
		{"mono mixdown element", int(p.MonoMixdownElement), 15},
		{"stereo mixdown element", int(p.StereoMixdownElement), 15},
		{"matrix mixdown index", int(p.MatrixMixdownIdx), 3},
		{"comment length", len(p.Comment), 255},
	}
	for _, l := range limits {
		if l.val > l.max {
			return fmt.Errorf("program config element: %s %d exceeds %d", l.name, l.val, l.max)
		}
	}
	for _, elems := range [][]PCEChannelElement{p.FrontElements, p.SideElements, p.BackElements} {
		for _, e := range elems {
			if e.TagSelect > 15 {
				return fmt.Errorf("program config element: element tag %d exceeds 15", e.TagSelect)
			}
		}
	}
	for _, tags := range [][]byte{p.LFEElementTags, p.AssocDataElementTags} {
		for _, tag := range tags {
			if tag > 15 {
				return fmt.Errorf("program config element: element tag %d exceeds 15", tag)
			}
		}
	}
	for _, e := range p.CCElements {
		if e.TagSelect > 15 {
			return fmt.Errorf("program config element: element tag %d exceeds 15", e.TagSelect)
		}
	}
	if !p.MonoMixdownPresent && p.MonoMixdownElement != 0 {
		return fmt.Errorf("program config element: mono mixdown element without MonoMixdownPresent")
	}
	if !p.StereoMixdownPresent && p.StereoMixdownElement != 0 {
		return fmt.Errorf("program config element: stereo mixdown element without StereoMixdownPresent")
	}
	if !p.MatrixMixdownIdxPresent && (p.MatrixMixdownIdx != 0 || p.PseudoSurroundEnable) {
		return fmt.Errorf("program config element: matrix mixdown fields without MatrixMixdownIdxPresent")
	}
	return nil
}
