package mp4

import (
	"fmt"
	"io"

	"github.com/Eyevinn/mp4ff/bits"
)

// StppBox - XMLSubtitleSampleEntry Box (stpp)
// Defined in ISO/IEC 14496-12 Sec. 12.6.3.2 and ISO/IEC 14496-30.
//
// Contained in : Media Information Box (minf)
type StppBox struct {
	Namespace          string   // Mandatory
	SchemaLocation     string   // Optional but at least a zero-byte termination
	AuxiliaryMimeTypes string   // Optional, but required if auxiliary types present
	Btrt               *BtrtBox // Optional
	Children           []Box
	DataReferenceIndex uint16
}

// NewStppBox - Create new stpp box
// namespace, schemaLocation and auxiliaryMimeType are space-separated utf8-lists with zero-termination
// schemaLocation and auxiliaryMimeTypes are optional but must at least have a zero byte.
func NewStppBox(namespace, schemaLocation, auxiliaryMimeTypes string) *StppBox {
	return &StppBox{
		Namespace:          namespace,
		DataReferenceIndex: 1,
		SchemaLocation:     schemaLocation,
		AuxiliaryMimeTypes: auxiliaryMimeTypes,
	}
}

// AddChild - add a child box (avcC normally, but clap and pasp could be part of visual entry)
func (b *StppBox) AddChild(child Box) {
	switch box := child.(type) {
	case *BtrtBox:
		b.Btrt = box
	default:
		// Other box
	}

	b.Children = append(b.Children, child)
}

// DecodeStpp - Decode XMLSubtitleSampleEntry (stpp)
func DecodeStpp(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := readBoxBody(r, hdr)
	if err != nil {
		return nil, err
	}
	sr := bits.NewFixedSliceReader(data)
	return DecodeStppSR(hdr, startPos, sr)

}

// DecodeStppSR - Decode XMLSubtitleSampleEntry (stpp)
func DecodeStppSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	payloadLen := hdr.payloadLen()

	remainingBytes := func(sr bits.SliceReader, initPos, payloadLen int) int {
		return payloadLen - (sr.GetPos() - initPos)
	}

	b := StppBox{}
	// 14496-12 8.5.2.2 Sample entry (8 bytes)
	initPos := sr.GetPos()
	sr.SkipBytes(6) // Skip 6 reserved bytes
	b.DataReferenceIndex = sr.ReadUint16()
	b.Namespace = sr.ReadZeroTerminatedString(hdr.payloadLen() - 8)
	if maxLen := remainingBytes(sr, initPos, payloadLen); maxLen > 0 {
		b.SchemaLocation = sr.ReadZeroTerminatedString(maxLen)
	}

	if maxLen := remainingBytes(sr, initPos, payloadLen); maxLen > 0 {
		b.AuxiliaryMimeTypes = sr.ReadZeroTerminatedString(maxLen)
	}
	if err := sr.AccError(); err != nil {
		return nil, fmt.Errorf("DecodeStpp: %w", err)
	}
	pos := startPos + uint64(hdr.Hdrlen+sr.GetPos()-initPos)
	for {
		rest := remainingBytes(sr, initPos, payloadLen)
		if rest <= 0 {
			break
		}
		box, err := DecodeBoxSR(pos, sr)
		if err != nil {
			return nil, err
		}
		if box != nil {
			b.AddChild(box)
			pos += box.Size()
		} else {
			return nil, fmt.Errorf("no stpp child")
		}
	}
	return &b, sr.AccError()
}

// Type - return box type
func (b *StppBox) Type() string {
	return "stpp"
}

// Size - return calculated size
func (b *StppBox) Size() uint64 {
	nrSampleEntryBytes := 8
	totalSize := uint64(boxHeaderSize + nrSampleEntryBytes + len(b.Namespace) + 1)
	totalSize += uint64(len(b.SchemaLocation)) + 1
	totalSize += uint64(len(b.AuxiliaryMimeTypes)) + 1
	for _, child := range b.Children {
		totalSize += child.Size()
	}
	return totalSize
}

// Encode - write box to w
func (b *StppBox) Encode(w io.Writer) error {
	err := EncodeHeader(b, w)
	if err != nil {
		return err
	}
	buf := makebuf(b)
	sw := bits.NewFixedSliceWriterFromSlice(buf)
	sw.WriteZeroBytes(6)
	sw.WriteUint16(b.DataReferenceIndex)
	sw.WriteString(b.Namespace, true)
	sw.WriteString(b.SchemaLocation, true)
	sw.WriteString(b.AuxiliaryMimeTypes, true)
	_, err = w.Write(buf[:sw.Offset()]) // Only write written bytes
	if err != nil {
		return err
	}

	// Next output child boxes in order
	for _, child := range b.Children {
		err = child.Encode(w)
		if err != nil {
			return err
		}
	}
	return err
}

// Encode - write box to w
func (b *StppBox) EncodeSW(sw bits.SliceWriter) error {
	err := EncodeHeaderSW(b, sw)
	if err != nil {
		return err
	}
	sw.WriteZeroBytes(6)
	sw.WriteUint16(b.DataReferenceIndex)
	sw.WriteString(b.Namespace, true)
	sw.WriteString(b.SchemaLocation, true)
	sw.WriteString(b.AuxiliaryMimeTypes, true)

	// Next output child boxes in order
	for _, child := range b.Children {
		err = child.EncodeSW(sw)
		if err != nil {
			return err
		}
	}
	return err
}

// Info - write specific box info to w
func (b *StppBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, -1, 0)
	bd.write(" - dataReferenceIndex: %d", b.DataReferenceIndex)
	bd.write(" - nameSpace: %q", b.Namespace)
	bd.write(" - schemaLocation: %q", b.SchemaLocation)
	bd.write(" - auxiliaryMimeTypes: %q", b.AuxiliaryMimeTypes)
	if bd.err != nil {
		return bd.err
	}
	var err error
	for _, child := range b.Children {
		err = child.Info(w, specificBoxLevels, indent+indentStep, indent)
		if err != nil {
			return err
		}
	}
	return nil
}
