package mp4

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"io/ioutil"
)

// Boxes needed for wvtt according to ISO/IEC 14496-30

////////////////////////////// wvtt //////////////////////////////

// WvttBox - WVTTSampleEntry (wvtt)
// Extends PlainTextSampleEntry which extends SampleEntry
type WvttBox struct {
	VttC               *VttCBox
	Vlab               *VlabBox
	Btrt               *BtrtBox
	Children           []Box
	DataReferenceIndex uint16
}

// NewAudioSampleEntryBox - Create new empty mp4a box
func NewWvttBox() *WvttBox {
	w := &WvttBox{}
	w.DataReferenceIndex = 1
	return w
}

// AddChild - add a child box (avcC normally, but clap and pasp could be part of visual entry)
func (w *WvttBox) AddChild(box Box) {
	switch b := box.(type) {
	case *VttCBox:
		w.VttC = b
	case *VlabBox:
		w.Vlab = b
	case *BtrtBox:
		w.Btrt = b
	default:
		// Other box
	}

	w.Children = append(w.Children, box)
}

const nrWvttBytesBeforeChildren = 16

// DecodeWvtt - Decoder wvtt Sample Entry (wvtt)
func DecodeWvtt(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	w := &WvttBox{}
	s := NewSliceReader(data)

	// 14496-12 8.5.2.2 Sample entry (8 bytes)
	s.SkipBytes(6) // Skip 6 reserved bytes
	w.DataReferenceIndex = s.ReadUint16()

	remaining := s.RemainingBytes()
	restReader := bytes.NewReader(remaining)

	pos := startPos + nrWvttBytesBeforeChildren
	for {
		box, err := DecodeBox(pos, restReader)
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}
		if box != nil {
			w.AddChild(box)
			pos += box.Size()
		}
		if pos == startPos+hdr.size {
			break
		} else if pos > startPos+hdr.size {
			return nil, errors.New("Bad size in wvtt")
		}
	}
	return w, nil
}

// Type - return box type
func (a *WvttBox) Type() string {
	return "wvtt"
}

// Size - return calculated size
func (a *WvttBox) Size() uint64 {
	totalSize := uint64(nrWvttBytesBeforeChildren)
	for _, child := range a.Children {
		totalSize += child.Size()
	}
	return totalSize
}

// Encode - write box to w
func (a *WvttBox) Encode(w io.Writer) error {
	err := EncodeHeader(a, w)
	if err != nil {
		return err
	}
	buf := makebuf(a)
	sw := NewSliceWriter(buf)
	sw.WriteZeroBytes(6)
	sw.WriteUint16(a.DataReferenceIndex)

	_, err = w.Write(buf[:sw.pos]) // Only write written bytes
	if err != nil {
		return err
	}

	// Next output child boxes in order
	for _, child := range a.Children {
		err = child.Encode(w)
		if err != nil {
			return err
		}
	}
	return err
}

func (a *WvttBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, a, -1, 0)
	if bd.err != nil {
		return bd.err
	}
	var err error
	for _, child := range a.Children {
		err = child.Info(w, specificBoxLevels, indent+indentStep, indent)
		if err != nil {
			return err
		}
	}
	return nil
}

////////////////////////////// vttC //////////////////////////////

// VttCBox - WebVTTConfigurationBox (vttC)
type VttCBox struct {
	Config string
}

// DecodeVttC - box-specific decode
func DecodeVttC(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	v := &VttCBox{
		Config: string(data),
	}
	return v, nil
}

// Type - box-specific type
func (v *VttCBox) Type() string {
	return "vttC"
}

// Size - calculated size of box
func (v *VttCBox) Size() uint64 {
	return uint64(boxHeaderSize + len(v.Config))
}

// Encode - write box to w
func (v *VttCBox) Encode(w io.Writer) error {
	err := EncodeHeader(v, w)
	if err != nil {
		return err
	}
	_, err = w.Write([]byte(v.Config))
	return err
}

func (v *VttCBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, v, -1, 0)
	bd.write(" - config: %q", v.Config)
	return bd.err
}

////////////////////////////// vlab //////////////////////////////

// VlabBox - WebVTTSourceLabelBox (vlab)
type VlabBox struct {
	SourceLabel string
}

// DecodeVlab - box-specific decode
func DecodeVlab(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	v := &VlabBox{
		SourceLabel: string(data),
	}
	return v, nil
}

// Type - box-specific type
func (v *VlabBox) Type() string {
	return "vlab"
}

// Size - calculated size of box
func (v *VlabBox) Size() uint64 {
	return uint64(boxHeaderSize + len(v.SourceLabel))
}

// Encode - write box to w
func (v *VlabBox) Encode(w io.Writer) error {
	err := EncodeHeader(v, w)
	if err != nil {
		return err
	}
	_, err = w.Write([]byte(v.SourceLabel))
	return err
}

func (v *VlabBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, v, -1, 0)
	bd.write(" - sourceLabel: %s", v.SourceLabel)
	return bd.err
}

// wvtt Sample boxes
// A sample is either one vtte box or one or more vttc or vta boxes

////////////////////////////// vtte //////////////////////////////

// VtteBox - VTTEmptyBox (vtte)
type VtteBox struct {
}

// Type - box-specific type
func (v *VtteBox) Type() string {
	return "vtte"
}

// DecodeVtte - box-specific decode
func DecodeVtte(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	return &VtteBox{}, nil
}

// Size - calculated size of box
func (v *VtteBox) Size() uint64 {
	return uint64(boxHeaderSize)
}

// Encode - write box to w
func (v *VtteBox) Encode(w io.Writer) error {
	return EncodeHeader(v, w)
}

func (v *VtteBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, v, -1, 0)
	return bd.err
}

////////////////////////////// vttc //////////////////////////////

// VttcBox - VTTCueBox (vttc)
type VttcBox struct {
	Vsid     *VsidBox
	Iden     *IdenBox
	Ctim     *CtimBox
	Sttg     *SttgBox
	Payl     *PaylBox
	Children []Box
}

// AddChild - Add a child box
func (v *VttcBox) AddChild(box Box) {

	switch b := box.(type) {
	case *VsidBox:
		v.Vsid = b
	case *IdenBox:
		v.Iden = b
	case *CtimBox:
		v.Ctim = b
	case *SttgBox:
		v.Sttg = b
	case *PaylBox:
		v.Payl = b
	default:
		// Type outside ISO/IEC 14496-30 spec
	}
	v.Children = append(v.Children, box)
}

// DecodeVttc - box-specific decode
func DecodeVttc(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	l, err := DecodeContainerChildren(hdr, startPos+8, startPos+hdr.size, r)
	if err != nil {
		return nil, err
	}
	m := &VttcBox{}
	for _, b := range l {
		m.AddChild(b)
	}
	return m, nil
}

// Type - return box type
func (v *VttcBox) Type() string {
	return "vttc"
}

// Size - return calculated size
func (v *VttcBox) Size() uint64 {
	return containerSize(v.Children)
}

// GetChildren - list of child boxes
func (v *VttcBox) GetChildren() []Box {
	return v.Children
}

// Encode - write mvex container to w
func (v *VttcBox) Encode(w io.Writer) error {
	return EncodeContainer(v, w)
}

func (v *VttcBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	return ContainerInfo(v, w, specificBoxLevels, indent, indentStep)
}

////////////////////////////// vsid //////////////////////////////

// VsidBox - CueSourceIDBox (vsid)
type VsidBox struct {
	SourceID uint32
}

// DecodeVsid - box-specific decode
func DecodeVsid(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	v := &VsidBox{
		SourceID: binary.BigEndian.Uint32(data[0:4]),
	}
	return v, nil
}

// Type - box-specific type
func (v *VsidBox) Type() string {
	return "vsid"
}

// Size - calculated size of box
func (v *VsidBox) Size() uint64 {
	return uint64(boxHeaderSize + 4) // len of uint32
}

// Encode - write box to w
func (v *VsidBox) Encode(w io.Writer) error {
	err := EncodeHeader(v, w)
	if err != nil {
		return err
	}
	buf := make([]byte, 4)
	binary.BigEndian.PutUint32(buf, v.SourceID)
	_, err = w.Write(buf)
	return err
}

func (v *VsidBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, v, -1, 0)
	bd.write(" - sourceID: %d", v.SourceID)
	return bd.err
}

////////////////////////////// ctim //////////////////////////////

// CtimBox - CueTimeBox (ctim)
// CueCurrentTime is current time indication (for split cues)
type CtimBox struct {
	CueCurrentTime string
}

// DecodeCtim - box-specific decode
func DecodeCtim(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	c := &CtimBox{
		CueCurrentTime: string(data),
	}
	return c, nil
}

// Type - box-specific type
func (c *CtimBox) Type() string {
	return "ctim"
}

// Size - calculated size of box
func (c *CtimBox) Size() uint64 {
	return uint64(boxHeaderSize + len(c.CueCurrentTime))
}

// Encode - write box to w
func (c *CtimBox) Encode(w io.Writer) error {
	err := EncodeHeader(c, w)
	if err != nil {
		return err
	}
	_, err = w.Write([]byte(c.CueCurrentTime))
	return err
}

func (c *CtimBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, c, -1, 0)
	bd.write(" - cueCurrentTime: %s", c.CueCurrentTime)
	return bd.err
}

////////////////////////////// iden //////////////////////////////

// IdenBox - CueIDBox (iden)
type IdenBox struct {
	CueID string
}

// DecodeIden - box-specific decode
func DecodeIden(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	i := &IdenBox{
		CueID: string(data),
	}
	return i, nil
}

// Type - box-specific type
func (i *IdenBox) Type() string {
	return "iden"
}

// Size - calculated size of box
func (i *IdenBox) Size() uint64 {
	return uint64(boxHeaderSize + len(i.CueID))
}

// Encode - write box to w
func (i *IdenBox) Encode(w io.Writer) error {
	err := EncodeHeader(i, w)
	if err != nil {
		return err
	}
	_, err = w.Write([]byte(i.CueID))
	return err
}

func (i *IdenBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, i, -1, 0)
	bd.write(" - cueID: %s", i.CueID)
	return bd.err
}

////////////////////////////// sttg //////////////////////////////

// SttgBox - CueSettingsBox (sttg)
type SttgBox struct {
	Settings string
}

// DecodeSttg - box-specific decode
func DecodeSttg(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	s := &SttgBox{
		Settings: string(data),
	}
	return s, nil
}

// Type - box-specific type
func (s *SttgBox) Type() string {
	return "sttg"
}

// Size - calculated size of box
func (s *SttgBox) Size() uint64 {
	return uint64(boxHeaderSize + len(s.Settings))
}

// Encode - write box to w
func (s *SttgBox) Encode(w io.Writer) error {
	err := EncodeHeader(s, w)
	if err != nil {
		return err
	}
	_, err = w.Write([]byte(s.Settings))
	return err
}

func (s *SttgBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, s, -1, 0)
	bd.write(" - settings: %s", s.Settings)
	return bd.err
}

////////////////////////////// payl //////////////////////////////

// PaylBox - CuePayloadBox (payl)
type PaylBox struct {
	CueText string
}

// DecodePayl - box-specific decode
func DecodePayl(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	p := &PaylBox{
		CueText: string(data),
	}
	return p, nil
}

// Type - box-specific type
func (p *PaylBox) Type() string {
	return "payl"
}

// Size - calculated size of box
func (p *PaylBox) Size() uint64 {
	return uint64(boxHeaderSize + len(p.CueText))
}

// Encode - write box to w
func (p *PaylBox) Encode(w io.Writer) error {
	err := EncodeHeader(p, w)
	if err != nil {
		return err
	}
	_, err = w.Write([]byte(p.CueText))
	return err
}

func (p *PaylBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, p, -1, 0)
	bd.write(" - cueText: %q", p.CueText)
	return bd.err
}

////////////////////////////// vtta //////////////////////////////

// VttaBox - VTTAdditionalTextBox (vtta) (corresponds to NOTE in WebVTT)
type VttaBox struct {
	CueAdditionalText string
}

// DecodeVtta - box-specific decode
func DecodeVtta(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	p := &VttaBox{
		CueAdditionalText: string(data),
	}
	return p, nil
}

// Type - box-specific type
func (v *VttaBox) Type() string {
	return "vtta"
}

// Size - calculated size of box
func (v *VttaBox) Size() uint64 {
	return uint64(boxHeaderSize + len(v.CueAdditionalText))
}

// Encode - write box to w
func (v *VttaBox) Encode(w io.Writer) error {
	err := EncodeHeader(v, w)
	if err != nil {
		return err
	}
	_, err = w.Write([]byte(v.CueAdditionalText))
	return err
}

func (v *VttaBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, v, -1, 0)
	bd.write(" - cueAdditionalText: %q", v.CueAdditionalText)
	return bd.err
}
