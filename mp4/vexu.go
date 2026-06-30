package mp4

import (
	"fmt"
	"io"

	"github.com/Eyevinn/mp4ff/bits"
)

// Stereo-view information flags (StriBox.StereoFlags), Apple "QuickTime and ISO
// Base Media File Formats and Spatial and Immersive Media" Sec. 1.4.
const (
	StriHasLeftEyeView     = 0x01 // has_left_eye_view
	StriHasRightEyeView    = 0x02 // has_right_eye_view
	StriHasAdditionalViews = 0x04 // has_additional_views
	StriEyeViewsReversed   = 0x08 // eye_views_reversed
)

// VexuBox - VideoExtendedUsageBox ('vexu'), Apple spatial/immersive video
// metadata container held in a VisualSampleEntry. It typically contains a
// StereoViewBox ('eyes') and/or a ProjectionBox ('proj'). Unknown child boxes
// are preserved as generic boxes so they round-trip.
type VexuBox struct {
	Eyes     *EyesBox
	Proj     *ProjBox
	Children []Box
}

// DecodeVexu - box-specific decode
func DecodeVexu(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	children, err := DecodeContainerChildren(hdr, startPos+8, startPos+hdr.Size, r)
	if err != nil {
		return nil, err
	}
	b := VexuBox{Children: make([]Box, 0, len(children))}
	for _, c := range children {
		b.AddChild(c)
	}
	return &b, nil
}

// DecodeVexuSR - box-specific decode
func DecodeVexuSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	children, err := DecodeContainerChildrenSR(hdr, startPos+8, startPos+hdr.Size, sr)
	if err != nil {
		return nil, err
	}
	b := VexuBox{Children: make([]Box, 0, len(children))}
	for _, c := range children {
		b.AddChild(c)
	}
	return &b, nil
}

// AddChild - add child box
func (b *VexuBox) AddChild(child Box) {
	switch box := child.(type) {
	case *EyesBox:
		b.Eyes = box
	case *ProjBox:
		b.Proj = box
	}
	b.Children = append(b.Children, child)
}

// Type - box type
func (b *VexuBox) Type() string { return "vexu" }

// Size - calculated size
func (b *VexuBox) Size() uint64 { return containerSize(b.Children) }

// GetChildren - list of child boxes
func (b *VexuBox) GetChildren() []Box { return b.Children }

// Encode - write box to w
func (b *VexuBox) Encode(w io.Writer) error { return EncodeContainer(b, w) }

// EncodeSW - write box to sw
func (b *VexuBox) EncodeSW(sw bits.SliceWriter) error { return EncodeContainerSW(b, sw) }

// Info - write box info
func (b *VexuBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	return ContainerInfo(b, w, specificBoxLevels, indent, indentStep)
}

// EyesBox - StereoViewBox ('eyes'), a container inside vexu for stereo-view
// information, hero eye and camera-system parameters.
type EyesBox struct {
	Stri     *StriBox
	Hero     *HeroBox
	Cams     *CamsBox
	Children []Box
}

// DecodeEyes - box-specific decode
func DecodeEyes(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	children, err := DecodeContainerChildren(hdr, startPos+8, startPos+hdr.Size, r)
	if err != nil {
		return nil, err
	}
	b := EyesBox{Children: make([]Box, 0, len(children))}
	for _, c := range children {
		b.AddChild(c)
	}
	return &b, nil
}

// DecodeEyesSR - box-specific decode
func DecodeEyesSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	children, err := DecodeContainerChildrenSR(hdr, startPos+8, startPos+hdr.Size, sr)
	if err != nil {
		return nil, err
	}
	b := EyesBox{Children: make([]Box, 0, len(children))}
	for _, c := range children {
		b.AddChild(c)
	}
	return &b, nil
}

// AddChild - add child box
func (b *EyesBox) AddChild(child Box) {
	switch box := child.(type) {
	case *StriBox:
		b.Stri = box
	case *HeroBox:
		b.Hero = box
	case *CamsBox:
		b.Cams = box
	}
	b.Children = append(b.Children, child)
}

// Type - box type
func (b *EyesBox) Type() string { return "eyes" }

// Size - calculated size
func (b *EyesBox) Size() uint64 { return containerSize(b.Children) }

// GetChildren - list of child boxes
func (b *EyesBox) GetChildren() []Box { return b.Children }

// Encode - write box to w
func (b *EyesBox) Encode(w io.Writer) error { return EncodeContainer(b, w) }

// EncodeSW - write box to sw
func (b *EyesBox) EncodeSW(sw bits.SliceWriter) error { return EncodeContainerSW(b, sw) }

// Info - write box info
func (b *EyesBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	return ContainerInfo(b, w, specificBoxLevels, indent, indentStep)
}

// CamsBox - StereoCameraSystemBox ('cams'), a container inside eyes for
// camera-system parameters such as the baseline distance.
type CamsBox struct {
	Blin     *BlinBox
	Children []Box
}

// DecodeCams - box-specific decode
func DecodeCams(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	children, err := DecodeContainerChildren(hdr, startPos+8, startPos+hdr.Size, r)
	if err != nil {
		return nil, err
	}
	b := CamsBox{Children: make([]Box, 0, len(children))}
	for _, c := range children {
		b.AddChild(c)
	}
	return &b, nil
}

// DecodeCamsSR - box-specific decode
func DecodeCamsSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	children, err := DecodeContainerChildrenSR(hdr, startPos+8, startPos+hdr.Size, sr)
	if err != nil {
		return nil, err
	}
	b := CamsBox{Children: make([]Box, 0, len(children))}
	for _, c := range children {
		b.AddChild(c)
	}
	return &b, nil
}

// AddChild - add child box
func (b *CamsBox) AddChild(child Box) {
	if box, ok := child.(*BlinBox); ok {
		b.Blin = box
	}
	b.Children = append(b.Children, child)
}

// Type - box type
func (b *CamsBox) Type() string { return "cams" }

// Size - calculated size
func (b *CamsBox) Size() uint64 { return containerSize(b.Children) }

// GetChildren - list of child boxes
func (b *CamsBox) GetChildren() []Box { return b.Children }

// Encode - write box to w
func (b *CamsBox) Encode(w io.Writer) error { return EncodeContainer(b, w) }

// EncodeSW - write box to sw
func (b *CamsBox) EncodeSW(sw bits.SliceWriter) error { return EncodeContainerSW(b, sw) }

// Info - write box info
func (b *CamsBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	return ContainerInfo(b, w, specificBoxLevels, indent, indentStep)
}

// ProjBox - ProjectionBox ('proj'), a container inside vexu holding a
// ProjectionInformationBox ('prji') and optionally a projection-kind-specific box.
type ProjBox struct {
	Prji     *PrjiBox
	Children []Box
}

// DecodeProj - box-specific decode
func DecodeProj(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	children, err := DecodeContainerChildren(hdr, startPos+8, startPos+hdr.Size, r)
	if err != nil {
		return nil, err
	}
	b := ProjBox{Children: make([]Box, 0, len(children))}
	for _, c := range children {
		b.AddChild(c)
	}
	return &b, nil
}

// DecodeProjSR - box-specific decode
func DecodeProjSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	children, err := DecodeContainerChildrenSR(hdr, startPos+8, startPos+hdr.Size, sr)
	if err != nil {
		return nil, err
	}
	b := ProjBox{Children: make([]Box, 0, len(children))}
	for _, c := range children {
		b.AddChild(c)
	}
	return &b, nil
}

// AddChild - add child box
func (b *ProjBox) AddChild(child Box) {
	if box, ok := child.(*PrjiBox); ok {
		b.Prji = box
	}
	b.Children = append(b.Children, child)
}

// Type - box type
func (b *ProjBox) Type() string { return "proj" }

// Size - calculated size
func (b *ProjBox) Size() uint64 { return containerSize(b.Children) }

// GetChildren - list of child boxes
func (b *ProjBox) GetChildren() []Box { return b.Children }

// Encode - write box to w
func (b *ProjBox) Encode(w io.Writer) error { return EncodeContainer(b, w) }

// EncodeSW - write box to sw
func (b *ProjBox) EncodeSW(sw bits.SliceWriter) error { return EncodeContainerSW(b, sw) }

// Info - write box info
func (b *ProjBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	return ContainerInfo(b, w, specificBoxLevels, indent, indentStep)
}

// StriBox - StereoViewInformationBox ('stri'), Apple Sec. 1.4. FullBox with a
// single flags byte: reserved(4) | eye_views_reversed(1) | has_additional_views(1)
// | has_right_eye_view(1) | has_left_eye_view(1).
type StriBox struct {
	Version     byte
	Flags       uint32
	StereoFlags byte
}

// DecodeStri - box-specific decode
func DecodeStri(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := readBoxBody(r, hdr)
	if err != nil {
		return nil, err
	}
	sr := bits.NewFixedSliceReader(data)
	return DecodeStriSR(hdr, startPos, sr)
}

// DecodeStriSR - box-specific decode
func DecodeStriSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	versionAndFlags := sr.ReadUint32()
	b := StriBox{
		Version:     byte(versionAndFlags >> 24),
		Flags:       versionAndFlags & flagsMask,
		StereoFlags: sr.ReadUint8(),
	}
	return &b, sr.AccError()
}

// Type - box type
func (b *StriBox) Type() string { return "stri" }

// Size - calculated size (header + version/flags + 1 byte)
func (b *StriBox) Size() uint64 { return uint64(boxHeaderSize + 4 + 1) }

// HasLeftEye returns true if a left-eye view is present.
func (b *StriBox) HasLeftEye() bool { return b.StereoFlags&StriHasLeftEyeView != 0 }

// HasRightEye returns true if a right-eye view is present.
func (b *StriBox) HasRightEye() bool { return b.StereoFlags&StriHasRightEyeView != 0 }

// HasAdditionalViews returns true if views beyond left/right eye are present.
func (b *StriBox) HasAdditionalViews() bool { return b.StereoFlags&StriHasAdditionalViews != 0 }

// EyeViewsReversed returns true if the eye views are in reversed order.
func (b *StriBox) EyeViewsReversed() bool { return b.StereoFlags&StriEyeViewsReversed != 0 }

// Encode - write box to w
func (b *StriBox) Encode(w io.Writer) error {
	sw := bits.NewFixedSliceWriter(int(b.Size()))
	err := b.EncodeSW(sw)
	if err != nil {
		return err
	}
	_, err = w.Write(sw.Bytes())
	return err
}

// EncodeSW - write box to sw
func (b *StriBox) EncodeSW(sw bits.SliceWriter) error {
	err := EncodeHeaderSW(b, sw)
	if err != nil {
		return err
	}
	versionAndFlags := (uint32(b.Version) << 24) | b.Flags
	sw.WriteUint32(versionAndFlags)
	sw.WriteUint8(b.StereoFlags)
	return sw.AccError()
}

// Info - write box info
func (b *StriBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, int(b.Version), b.Flags)
	bd.write(" - hasLeftEye: %t", b.HasLeftEye())
	bd.write(" - hasRightEye: %t", b.HasRightEye())
	bd.write(" - hasAdditionalViews: %t", b.HasAdditionalViews())
	bd.write(" - eyeViewsReversed: %t", b.EyeViewsReversed())
	return bd.err
}

// HeroBox - HeroStereoEyeDescriptionBox ('hero'), Apple Sec. 1.5. FullBox with a
// single hero_eye_indicator byte: 0 = none, 1 = left, 2 = right, >= 3 reserved.
type HeroBox struct {
	Version byte
	Flags   uint32
	HeroEye byte
}

// DecodeHero - box-specific decode
func DecodeHero(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := readBoxBody(r, hdr)
	if err != nil {
		return nil, err
	}
	sr := bits.NewFixedSliceReader(data)
	return DecodeHeroSR(hdr, startPos, sr)
}

// DecodeHeroSR - box-specific decode
func DecodeHeroSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	versionAndFlags := sr.ReadUint32()
	b := HeroBox{
		Version: byte(versionAndFlags >> 24),
		Flags:   versionAndFlags & flagsMask,
		HeroEye: sr.ReadUint8(),
	}
	return &b, sr.AccError()
}

// Type - box type
func (b *HeroBox) Type() string { return "hero" }

// Size - calculated size
func (b *HeroBox) Size() uint64 { return uint64(boxHeaderSize + 4 + 1) }

// HeroEyeName returns the hero eye as a string.
func (b *HeroBox) HeroEyeName() string {
	switch b.HeroEye {
	case 0:
		return "none"
	case 1:
		return "left"
	case 2:
		return "right"
	default:
		return fmt.Sprintf("reserved(%d)", b.HeroEye)
	}
}

// Encode - write box to w
func (b *HeroBox) Encode(w io.Writer) error {
	sw := bits.NewFixedSliceWriter(int(b.Size()))
	err := b.EncodeSW(sw)
	if err != nil {
		return err
	}
	_, err = w.Write(sw.Bytes())
	return err
}

// EncodeSW - write box to sw
func (b *HeroBox) EncodeSW(sw bits.SliceWriter) error {
	err := EncodeHeaderSW(b, sw)
	if err != nil {
		return err
	}
	versionAndFlags := (uint32(b.Version) << 24) | b.Flags
	sw.WriteUint32(versionAndFlags)
	sw.WriteUint8(b.HeroEye)
	return sw.AccError()
}

// Info - write box info
func (b *HeroBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, int(b.Version), b.Flags)
	bd.write(" - heroEye: %s (%d)", b.HeroEyeName(), b.HeroEye)
	return bd.err
}

// BlinBox - StereoCameraSystemBaselineBox ('blin'), Apple Sec. 1 (Stereo
// Camera-System). FullBox with the baseline distance between the centers of the
// stereo lenses, as an unsigned 32-bit value in micrometers.
type BlinBox struct {
	Version  byte
	Flags    uint32
	Baseline uint32 // in micrometers
}

// DecodeBlin - box-specific decode
func DecodeBlin(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := readBoxBody(r, hdr)
	if err != nil {
		return nil, err
	}
	sr := bits.NewFixedSliceReader(data)
	return DecodeBlinSR(hdr, startPos, sr)
}

// DecodeBlinSR - box-specific decode
func DecodeBlinSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	versionAndFlags := sr.ReadUint32()
	b := BlinBox{
		Version:  byte(versionAndFlags >> 24),
		Flags:    versionAndFlags & flagsMask,
		Baseline: sr.ReadUint32(),
	}
	return &b, sr.AccError()
}

// Type - box type
func (b *BlinBox) Type() string { return "blin" }

// Size - calculated size
func (b *BlinBox) Size() uint64 { return uint64(boxHeaderSize + 4 + 4) }

// Encode - write box to w
func (b *BlinBox) Encode(w io.Writer) error {
	sw := bits.NewFixedSliceWriter(int(b.Size()))
	err := b.EncodeSW(sw)
	if err != nil {
		return err
	}
	_, err = w.Write(sw.Bytes())
	return err
}

// EncodeSW - write box to sw
func (b *BlinBox) EncodeSW(sw bits.SliceWriter) error {
	err := EncodeHeaderSW(b, sw)
	if err != nil {
		return err
	}
	versionAndFlags := (uint32(b.Version) << 24) | b.Flags
	sw.WriteUint32(versionAndFlags)
	sw.WriteUint32(b.Baseline)
	return sw.AccError()
}

// Info - write box info
func (b *BlinBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, int(b.Version), b.Flags)
	bd.write(" - baseline: %d µm (%.3f mm)", b.Baseline, float64(b.Baseline)/1000.0)
	return bd.err
}

// PrjiBox - ProjectionInformationBox ('prji'), Apple Sec. 2. FullBox holding the
// projection_kind FourCC (e.g. "rect" for rectilinear, "equi" for equirectangular).
type PrjiBox struct {
	Version        byte
	Flags          uint32
	ProjectionType string // 4-char FourCC
}

// DecodePrji - box-specific decode
func DecodePrji(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := readBoxBody(r, hdr)
	if err != nil {
		return nil, err
	}
	sr := bits.NewFixedSliceReader(data)
	return DecodePrjiSR(hdr, startPos, sr)
}

// DecodePrjiSR - box-specific decode
func DecodePrjiSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	versionAndFlags := sr.ReadUint32()
	b := PrjiBox{
		Version:        byte(versionAndFlags >> 24),
		Flags:          versionAndFlags & flagsMask,
		ProjectionType: sr.ReadFixedLengthString(4),
	}
	return &b, sr.AccError()
}

// Type - box type
func (b *PrjiBox) Type() string { return "prji" }

// Size - calculated size
func (b *PrjiBox) Size() uint64 { return uint64(boxHeaderSize + 4 + 4) }

// Encode - write box to w
func (b *PrjiBox) Encode(w io.Writer) error {
	sw := bits.NewFixedSliceWriter(int(b.Size()))
	err := b.EncodeSW(sw)
	if err != nil {
		return err
	}
	_, err = w.Write(sw.Bytes())
	return err
}

// EncodeSW - write box to sw
func (b *PrjiBox) EncodeSW(sw bits.SliceWriter) error {
	err := EncodeHeaderSW(b, sw)
	if err != nil {
		return err
	}
	versionAndFlags := (uint32(b.Version) << 24) | b.Flags
	sw.WriteUint32(versionAndFlags)
	pt := b.ProjectionType
	if len(pt) > 4 {
		pt = pt[:4]
	}
	for len(pt) < 4 {
		pt += "\x00"
	}
	sw.WriteBytes([]byte(pt))
	return sw.AccError()
}

// Info - write box info
func (b *PrjiBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, int(b.Version), b.Flags)
	bd.write(" - projectionType: %s", b.ProjectionType)
	return bd.err
}

// HfovBox - HorizontalFieldOfViewBox ('hfov'), Apple Sec. 3.1.1.1. A plain Box
// extension of the VisualSampleEntry giving the horizontal field of view as an
// unsigned 32-bit value in thousandths of a degree (104° = 104000).
type HfovBox struct {
	FieldOfView uint32 // in 1/1000th of a degree
}

// DecodeHfov - box-specific decode
func DecodeHfov(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := readBoxBody(r, hdr)
	if err != nil {
		return nil, err
	}
	sr := bits.NewFixedSliceReader(data)
	return DecodeHfovSR(hdr, startPos, sr)
}

// DecodeHfovSR - box-specific decode
func DecodeHfovSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	b := HfovBox{
		FieldOfView: sr.ReadUint32(),
	}
	return &b, sr.AccError()
}

// Type - box type
func (b *HfovBox) Type() string { return "hfov" }

// Size - calculated size
func (b *HfovBox) Size() uint64 { return uint64(boxHeaderSize + 4) }

// Encode - write box to w
func (b *HfovBox) Encode(w io.Writer) error {
	sw := bits.NewFixedSliceWriter(int(b.Size()))
	err := b.EncodeSW(sw)
	if err != nil {
		return err
	}
	_, err = w.Write(sw.Bytes())
	return err
}

// EncodeSW - write box to sw
func (b *HfovBox) EncodeSW(sw bits.SliceWriter) error {
	err := EncodeHeaderSW(b, sw)
	if err != nil {
		return err
	}
	sw.WriteUint32(b.FieldOfView)
	return sw.AccError()
}

// Info - write box info
func (b *HfovBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, -1, 0)
	bd.write(" - fieldOfView: %d/1000 degrees (%.3f°)",
		b.FieldOfView, float64(b.FieldOfView)/1000.0)
	return bd.err
}

// CreateVexuBox builds a vexu box hierarchy (eyes{stri,hero,cams{blin}}, proj{prji})
// for Apple spatial video. stereoFlags is a combination of the Stri* constants.
func CreateVexuBox(stereoFlags byte, heroEye byte, baselineUM uint32, projType string) *VexuBox {
	stri := &StriBox{StereoFlags: stereoFlags}
	hero := &HeroBox{HeroEye: heroEye}
	blin := &BlinBox{Baseline: baselineUM}

	cams := &CamsBox{}
	cams.AddChild(blin)

	eyes := &EyesBox{}
	eyes.AddChild(stri)
	eyes.AddChild(hero)
	eyes.AddChild(cams)

	proj := &ProjBox{}
	proj.AddChild(&PrjiBox{ProjectionType: projType})

	vexu := &VexuBox{}
	vexu.AddChild(eyes)
	vexu.AddChild(proj)
	return vexu
}
