package mp4

import (
	"fmt"
	"io"

	"github.com/Eyevinn/mp4ff/bits"
)

// StppBox - XMLSubtitleSampleEntry Box (stpp or stpc)
// Defined in ISO/IEC 14496-12 Sec. 12.6.3.2 and ISO/IEC 14496-30.
//
// stpc is the experimental paint-model variant. It has the same syntax as stpp,
// and its document samples are byte-identical to stpp samples, but a sample may
// instead be a TtmnBox or a TtmbBox. The 4CC is not registered with MP4RA.
//
// Contained in : Media Information Box (minf)
type StppBox struct {
	Namespace                 string   // Mandatory
	SchemaLocation            string   // Optional
	AuxiliaryMimeTypes        string   // Optional, but required if auxiliary types present
	Btrt                      *BtrtBox // Optional
	Children                  []Box
	DataReferenceIndex        uint16
	name                      string // stpp unless set, e.g. to stpc
	nrMissingOptionalEndBytes byte   // 0, 1, or 2 depending on whether SchemaLocation and AuxiliaryMimeTypes have a zero end byte
}

// NewStppBox - Create new stpp box
// namespace, schemaLocation and auxiliaryMimeType are space-separated utf8-lists with zero-termination
// schemaLocation and auxiliaryMimeTypes are optional but must at least have a zero byte.
func NewStppBox(namespace, schemaLocation, auxiliaryMimeTypes string) *StppBox {
	return NewXMLSubtitleSampleEntryBox("stpp", namespace, schemaLocation, auxiliaryMimeTypes)
}

// NewStpcBox - Create new stpc box, the experimental paint-model variant of stpp.
// The arguments are the same as for NewStppBox.
func NewStpcBox(namespace, schemaLocation, auxiliaryMimeTypes string) *StppBox {
	return NewXMLSubtitleSampleEntryBox("stpc", namespace, schemaLocation, auxiliaryMimeTypes)
}

// NewXMLSubtitleSampleEntryBox - Create new XMLSubtitleSampleEntry box with name stpp or stpc.
// namespace, schemaLocation and auxiliaryMimeType are space-separated utf8-lists with zero-termination
// schemaLocation and auxiliaryMimeTypes are optional but must at least have a zero byte.
func NewXMLSubtitleSampleEntryBox(name, namespace, schemaLocation, auxiliaryMimeTypes string) *StppBox {
	return &StppBox{
		Namespace:                 namespace,
		DataReferenceIndex:        1,
		SchemaLocation:            schemaLocation,
		AuxiliaryMimeTypes:        auxiliaryMimeTypes,
		name:                      name,
		nrMissingOptionalEndBytes: 0,
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

// DecodeStpp - Decode XMLSubtitleSampleEntry (stpp or stpc)
func DecodeStpp(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := readBoxBody(r, hdr)
	if err != nil {
		return nil, err
	}
	sr := bits.NewFixedSliceReader(data)
	return DecodeStppSR(hdr, startPos, sr)

}

// DecodeStppSR - Decode XMLSubtitleSampleEntry (stpp or stpc)
func DecodeStppSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	payloadLen := hdr.payloadLen()

	remainingBytes := func(sr bits.SliceReader, initPos, payloadLen int) int {
		return payloadLen - (sr.GetPos() - initPos)
	}

	b := StppBox{name: hdr.Name}
	// 14496-12 8.5.2.2 Sample entry (8 bytes)
	initPos := sr.GetPos()
	sr.SkipBytes(6) // Skip 6 reserved bytes
	b.DataReferenceIndex = sr.ReadUint16()
	b.Namespace = sr.ReadZeroTerminatedString(hdr.payloadLen() - 8)
	if maxLen := remainingBytes(sr, initPos, payloadLen); maxLen > 0 {
		b.SchemaLocation = sr.ReadZeroTerminatedString(maxLen)
	} else {
		b.nrMissingOptionalEndBytes++
	}

	if maxLen := remainingBytes(sr, initPos, payloadLen); maxLen > 0 {
		b.AuxiliaryMimeTypes = sr.ReadZeroTerminatedString(maxLen)
	} else {
		b.nrMissingOptionalEndBytes++
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

// Type - return box type, stpp unless set to something else such as stpc
func (b *StppBox) Type() string {
	if b.name == "" {
		return "stpp"
	}
	return b.name
}

// Size - return calculated size
func (b *StppBox) Size() uint64 {
	nrSampleEntryBytes := 8
	totalSize := uint64(boxHeaderSize + nrSampleEntryBytes + len(b.Namespace) + 1)
	totalSize += uint64(len(b.SchemaLocation)) + 1
	totalSize += uint64(len(b.AuxiliaryMimeTypes)) + 1
	totalSize -= uint64(b.nrMissingOptionalEndBytes)
	for _, child := range b.Children {
		totalSize += child.Size()
	}
	return totalSize
}

// Encode - write box to w via a SliceWriter
func (b *StppBox) Encode(w io.Writer) error {
	sw := bits.NewFixedSliceWriter(int(b.Size()))
	err := b.EncodeSW(sw)
	if err != nil {
		return err
	}
	_, err = w.Write(sw.Bytes())
	return err
}

// EncodeSW - write box to sw
func (b *StppBox) EncodeSW(sw bits.SliceWriter) error {
	err := EncodeHeaderSW(b, sw)
	if err != nil {
		return err
	}
	sw.WriteZeroBytes(6)
	sw.WriteUint16(b.DataReferenceIndex)
	sw.WriteString(b.Namespace, true)
	sw.WriteString(b.SchemaLocation, b.nrMissingOptionalEndBytes < 2)
	sw.WriteString(b.AuxiliaryMimeTypes, b.nrMissingOptionalEndBytes < 1)

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

// stpc sample boxes
// A sample is either a complete TTML document, as for stpp, or one of the boxes below.
// A sample whose first eight bytes are a box header with type ttmn or ttmb and a size
// equal to the sample size is that box. Any other sample is a TTML document, which can
// never start with the zero byte that opens a box size.

////////////////////////////// ttmn //////////////////////////////

// TtmnBox - TTMLNoChangeBox (ttmn), an empty box that is a full sample.
// It says that the currently active TTML document continues unchanged.
// Experimental: the 4CC is not registered with MP4RA.
type TtmnBox struct {
}

// DecodeTtmn - box-specific decode
func DecodeTtmn(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	return &TtmnBox{}, nil
}

// DecodeTtmnSR - box-specific decode
func DecodeTtmnSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	return &TtmnBox{}, nil
}

// Type - box-specific type
func (b *TtmnBox) Type() string {
	return "ttmn"
}

// Size - calculated size of box
func (b *TtmnBox) Size() uint64 {
	return uint64(boxHeaderSize)
}

// Encode - write box to w
func (b *TtmnBox) Encode(w io.Writer) error {
	return EncodeHeader(b, w)
}

// EncodeSW - box-specific encode to slicewriter
func (b *TtmnBox) EncodeSW(sw bits.SliceWriter) error {
	return EncodeHeaderSW(b, sw)
}

// Info - write box-specific information
func (b *TtmnBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, -1, 0)
	return bd.err
}

////////////////////////////// ttmb //////////////////////////////

// TtmbBox - TTMLBodyBox (ttmb), a full sample carrying only the body element of a
// TTML document. The head comes from the first sample of the same fragment, so a
// ttmb sample is not a sync sample. Body is a boxstring: UTF-8 filling the box with
// neither a length prefix nor a trailing zero byte.
// Experimental: the 4CC is not registered with MP4RA.
type TtmbBox struct {
	Body string
}

// DecodeTtmb - box-specific decode
func DecodeTtmb(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := readBoxBody(r, hdr)
	if err != nil {
		return nil, err
	}
	sr := bits.NewFixedSliceReader(data)
	return DecodeTtmbSR(hdr, startPos, sr)
}

// DecodeTtmbSR - box-specific decode
func DecodeTtmbSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	return &TtmbBox{Body: sr.ReadFixedLengthString(hdr.payloadLen())}, sr.AccError()
}

// Type - box-specific type
func (b *TtmbBox) Type() string {
	return "ttmb"
}

// Size - calculated size of box
func (b *TtmbBox) Size() uint64 {
	return uint64(boxHeaderSize + len(b.Body))
}

// Encode - write box to w
func (b *TtmbBox) Encode(w io.Writer) error {
	sw := bits.NewFixedSliceWriter(int(b.Size()))
	err := b.EncodeSW(sw)
	if err != nil {
		return err
	}
	_, err = w.Write(sw.Bytes())
	return err
}

// EncodeSW - box-specific encode to slicewriter
func (b *TtmbBox) EncodeSW(sw bits.SliceWriter) error {
	err := EncodeHeaderSW(b, sw)
	if err != nil {
		return err
	}
	sw.WriteString(b.Body, false)
	return sw.AccError()
}

// Info - write box-specific information
func (b *TtmbBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, -1, 0)
	bd.write(" - body: %q", b.Body)
	return bd.err
}
