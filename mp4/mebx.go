package mp4

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"github.com/Eyevinn/mp4ff/bits"
)

// MebxBox - Boxed Metadata Sample Entry (mebx), ISO/IEC 14496-12:2026 Sec. 12.8.4
//
// Contained in: Sample Description Box (stsd)
//
// It is the sample entry of a timed metadata track whose samples can hold values for several keys, such as
// the orientation, detected face and Live Photo tracks of an iPhone recording. Keys declares the keys.
// A sample is a list of values, which DecodeMebxSample returns.
//
// A mebx box with another layout decodes as an UnknownBox.
type MebxBox struct {
	DataReferenceIndex uint16
	Keys               *MetadataKeyTableBox
	Btrt               *BtrtBox
	Children           []Box
}

// DecodeMebx - box-specific decode
func DecodeMebx(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := readBoxBody(r, hdr)
	if err != nil {
		return nil, err
	}
	return decodeOrUnknown(hdr, startPos, data, decodeMebxSR)
}

// DecodeMebxSR - box-specific decode
func DecodeMebxSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	return decodeOrUnknownSR(hdr, startPos, sr, decodeMebxSR)
}

func decodeMebxSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	if hdr.payloadLen() < 8 {
		return nil, fmt.Errorf("mebx: payload of %d bytes is too short", hdr.payloadLen())
	}
	sr.SkipBytes(6) // reserved
	b := MebxBox{DataReferenceIndex: sr.ReadUint16()}
	// The keys box here has another layout than the keys box of QuickTime metadata that the decoder table has.
	decode := func(chdr BoxHeader, pos uint64) (Box, error) {
		if chdr.Name == "keys" {
			return oneBoxDeeperSR(chdr, pos, sr, func() (Box, error) {
				return decodeMetadataKeyTableSR(chdr, pos, sr)
			})
		}
		return DecodeBoxBodySR(pos, chdr, sr)
	}
	err := decodeBoxesSR("mebx", startPos+uint64(hdr.Hdrlen)+8, startPos+hdr.Size, sr, decode, b.AddChild)
	if err != nil {
		return nil, err
	}
	return &b, sr.AccError()
}

// decodeBoxesSR decodes the boxes from pos to end and passes them to add. decode gets the header and the
// position of each box, and decodes its payload.
func decodeBoxesSR(parent string, pos, end uint64, sr bits.SliceReader, decode func(hdr BoxHeader, pos uint64) (Box, error),
	add func(Box)) error {
	for pos < end {
		if end-pos < boxHeaderSize {
			return fmt.Errorf("%s: %d bytes after the last child", parent, end-pos)
		}
		hdr, err := DecodeHeaderSR(sr)
		if err != nil {
			return fmt.Errorf("%s: %w", parent, err)
		}
		if hdr.Size > end-pos {
			return fmt.Errorf("%s: child %s of size %d goes beyond the end", parent, displayBoxType(hdr.Name), hdr.Size)
		}
		child, err := decode(hdr, pos)
		if errors.Is(err, ErrBoxTooDeep) {
			return err // already names the box that is too deep
		}
		if err != nil {
			return fmt.Errorf("%s: %w", parent, err)
		}
		add(child)
		pos += hdr.Size
	}
	return nil
}

// AddChild - add a child box
func (b *MebxBox) AddChild(child Box) {
	switch box := child.(type) {
	case *MetadataKeyTableBox:
		b.Keys = box
	case *BtrtBox:
		b.Btrt = box
	}
	b.Children = append(b.Children, child)
}

// Type - box type
func (b *MebxBox) Type() string {
	return "mebx"
}

// Size - calculated size of box
func (b *MebxBox) Size() uint64 {
	return containerSize(b.Children) + 8
}

// GetChildren - list of child boxes
func (b *MebxBox) GetChildren() []Box {
	return b.Children
}

// Encode - write box to w
func (b *MebxBox) Encode(w io.Writer) error {
	sw := bits.NewFixedSliceWriter(int(b.Size()))
	err := b.EncodeSW(sw)
	if err != nil {
		return err
	}
	_, err = w.Write(sw.Bytes())
	return err
}

// EncodeSW - box-specific encode to slicewriter
func (b *MebxBox) EncodeSW(sw bits.SliceWriter) error {
	err := EncodeHeaderSW(b, sw)
	if err != nil {
		return err
	}
	sw.WriteZeroBytes(6)
	sw.WriteUint16(b.DataReferenceIndex)
	for _, c := range b.Children {
		err = c.EncodeSW(sw)
		if err != nil {
			return err
		}
	}
	return sw.AccError()
}

// Info - write box-specific information
func (b *MebxBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, -1, 0)
	bd.write(" - dataReferenceIndex: %d", b.DataReferenceIndex)
	if bd.err != nil {
		return bd.err
	}
	for _, c := range b.Children {
		err := c.Info(w, specificBoxLevels, indent+indentStep, indentStep)
		if err != nil {
			return err
		}
	}
	return nil
}

// MetadataKeyTableBox - Metadata Key Table Box (keys) of a MebxBox, ISO/IEC 14496-12:2026 Sec. 12.8.4.2
//
// Contained in: Boxed Metadata Sample Entry (mebx)
//
// Keys has one MetadataKeyBox for each key that the samples of the track can have a value for. They are
// the only children of the box.
//
// The keys box of QuickTime metadata has the same type but another layout, and is a KeysBox.
type MetadataKeyTableBox struct {
	Keys []*MetadataKeyBox
}

// decodeMetadataKeyTableSR decodes the keys box of a mebx box. Every child is a MetadataKeyBox, whatever
// its type is, so the children are not decoded by the decoder of their type.
func decodeMetadataKeyTableSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	b := MetadataKeyTableBox{}
	decode := func(chdr BoxHeader, pos uint64) (Box, error) {
		return oneBoxDeeperSR(chdr, pos, sr, func() (Box, error) {
			return decodeMetadataKeySR(chdr, pos, sr)
		})
	}
	add := func(child Box) {
		b.Keys = append(b.Keys, child.(*MetadataKeyBox))
	}
	err := decodeBoxesSR("keys", startPos+uint64(hdr.Hdrlen), startPos+hdr.Size, sr, decode, add)
	if err != nil {
		return nil, err
	}
	return &b, sr.AccError()
}

// Key returns the key with the local key ID, or nil if there is none.
func (b *MetadataKeyTableBox) Key(localKeyID uint32) *MetadataKeyBox {
	for _, k := range b.Keys {
		if k.LocalKeyID == localKeyID {
			return k
		}
	}
	return nil
}

// FindKey returns the first key declared with the namespace and the value, such as mdta and
// com.apple.quicktime.still-image-time, or nil if there is none.
func (b *MetadataKeyTableBox) FindKey(namespace, value string) *MetadataKeyBox {
	for _, k := range b.Keys {
		if k.Keyd != nil && k.Keyd.Namespace == namespace && k.Keyd.Value == value {
			return k
		}
	}
	return nil
}

// Type - box type
func (b *MetadataKeyTableBox) Type() string {
	return "keys"
}

// Size - calculated size of box
func (b *MetadataKeyTableBox) Size() uint64 {
	size := uint64(boxHeaderSize)
	for _, k := range b.Keys {
		size += k.Size()
	}
	return size
}

// GetChildren - list of child boxes
func (b *MetadataKeyTableBox) GetChildren() []Box {
	children := make([]Box, 0, len(b.Keys))
	for _, k := range b.Keys {
		children = append(children, k)
	}
	return children
}

// Encode - write box to w
func (b *MetadataKeyTableBox) Encode(w io.Writer) error {
	sw := bits.NewFixedSliceWriter(int(b.Size()))
	err := b.EncodeSW(sw)
	if err != nil {
		return err
	}
	_, err = w.Write(sw.Bytes())
	return err
}

// EncodeSW - box-specific encode to slicewriter
func (b *MetadataKeyTableBox) EncodeSW(sw bits.SliceWriter) error {
	err := EncodeHeaderSW(b, sw)
	if err != nil {
		return err
	}
	for _, k := range b.Keys {
		err = k.EncodeSW(sw)
		if err != nil {
			return err
		}
	}
	return sw.AccError()
}

// Info - write box-specific information
func (b *MetadataKeyTableBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, -1, 0)
	if bd.err != nil {
		return bd.err
	}
	for _, k := range b.Keys {
		err := k.Info(w, specificBoxLevels, indent+indentStep, indentStep)
		if err != nil {
			return err
		}
	}
	return nil
}

// MetadataKeyBox - Metadata Key Box, ISO/IEC 14496-12:2026 Sec. 12.8.4.3
//
// Contained in: Metadata Key Table Box (MetadataKeyTableBox)
//
// The box type is LocalKeyID, which is also the box type of the values of the key in the samples. It is
// often a small number, such as 1, and 0 marks a key that is not used.
//
// Keyd is the key. Dtyp, an addition of QuickTime, tells the data type of the values. Loca and Setu are optional.
type MetadataKeyBox struct {
	LocalKeyID uint32
	Keyd       *KeydBox
	Dtyp       *DtypBox
	Loca       *LocaBox
	Setu       *SetuBox
	Children   []Box
}

func decodeMetadataKeySR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	children, err := DecodeContainerChildrenSR(hdr, startPos+uint64(hdr.Hdrlen), startPos+hdr.Size, sr)
	if err != nil {
		return nil, err
	}
	b := MetadataKeyBox{LocalKeyID: binary.BigEndian.Uint32([]byte(hdr.Name))}
	for _, c := range children {
		b.AddChild(c)
	}
	return &b, nil
}

// AddChild - add a child box
func (b *MetadataKeyBox) AddChild(child Box) {
	switch box := child.(type) {
	case *KeydBox:
		b.Keyd = box
	case *DtypBox:
		b.Dtyp = box
	case *LocaBox:
		b.Loca = box
	case *SetuBox:
		b.Setu = box
	}
	b.Children = append(b.Children, child)
}

// Type - box type, which is the local key ID as four bytes
func (b *MetadataKeyBox) Type() string {
	return string(binary.BigEndian.AppendUint32(nil, b.LocalKeyID))
}

// Size - calculated size of box
func (b *MetadataKeyBox) Size() uint64 {
	return containerSize(b.Children)
}

// GetChildren - list of child boxes
func (b *MetadataKeyBox) GetChildren() []Box {
	return b.Children
}

// Encode - write box to w
func (b *MetadataKeyBox) Encode(w io.Writer) error {
	return EncodeContainer(b, w)
}

// EncodeSW - box-specific encode to slicewriter
func (b *MetadataKeyBox) EncodeSW(sw bits.SliceWriter) error {
	return EncodeContainerSW(b, sw)
}

// Info - write box-specific information
func (b *MetadataKeyBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	return ContainerInfo(b, w, specificBoxLevels, indent, indentStep)
}

// MebxValue is a value in a sample of a track with a MebxBox sample entry.
type MebxValue struct {
	// LocalKeyID tells which MetadataKeyBox of the sample entry the value belongs to. A value with ID 0
	// has no meaning and is padding.
	LocalKeyID uint32
	// Data is the value, in the data type that the MetadataKeyBox declares.
	Data []byte
}

// DecodeMebxSample returns the values of a sample of a track with a MebxBox sample entry, in the order they
// are stored, as defined in ISO/IEC 14496-12:2026 Sec. 12.8.3. The values share memory with the sample.
func DecodeMebxSample(sample []byte) ([]MebxValue, error) {
	var values []MebxValue
	for pos := 0; pos < len(sample); {
		left := len(sample) - pos
		if left < boxHeaderSize {
			return nil, fmt.Errorf("mebx sample: %d bytes after the last value", left)
		}
		size := binary.BigEndian.Uint32(sample[pos:])
		if size < boxHeaderSize || uint64(size) > uint64(left) {
			return nil, fmt.Errorf("mebx sample: value of size %d at %d does not fit", size, pos)
		}
		values = append(values, MebxValue{
			LocalKeyID: binary.BigEndian.Uint32(sample[pos+4:]),
			Data:       sample[pos+boxHeaderSize : pos+int(size)],
		})
		pos += int(size)
	}
	return values, nil
}
