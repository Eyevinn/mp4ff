package mp4

import (
	"fmt"
	"io"

	"github.com/Eyevinn/mp4ff/bits"
)

// LablIsGroupLabelFlag - flag mask telling that the label is a summary label for a group of labels.
const LablIsGroupLabelFlag uint32 = 0x000001

// LablBox - Label Box (labl)
// Defined in ISO/IEC 14496-12:2026 Section 8.10.5.
// Contained in UserDataBox of a TrackBox, AudioElementBox,
// AudioElementSelectionBox, or PreselectionGroupBox.
//
// DASH-IF Ingest signals track labels with labl boxes in the udta box of a track,
// using one group label as the name to present and one label per language.
type LablBox struct {
	Version byte
	Flags   uint32
	// LabelID identifies the label group the label belongs to. Zero means no group.
	LabelID uint16
	// Language is an IETF BCP 47 compliant language tag such as "en-US".
	Language string
	// Label is the textual description.
	Label string
}

// IsGroupLabel - true if the label is a summary label for the group given by LabelID.
func (b *LablBox) IsGroupLabel() bool {
	return b.Flags&LablIsGroupLabelFlag != 0
}

// DecodeLabl - box-specific decode
func DecodeLabl(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := readBoxBody(r, hdr)
	if err != nil {
		return nil, err
	}
	sr := bits.NewFixedSliceReader(data)
	return DecodeLablSR(hdr, startPos, sr)
}

// DecodeLablSR - box-specific decode
func DecodeLablSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	payloadLen := hdr.payloadLen()
	if payloadLen < 8 { // version + flags + label_id + two zero-terminated strings
		return nil, fmt.Errorf("decode labl: payload too short: %d", payloadLen)
	}
	versionAndFlags := sr.ReadUint32()
	b := LablBox{
		Version: byte(versionAndFlags >> 24),
		Flags:   versionAndFlags & flagsMask,
		LabelID: sr.ReadUint16(),
	}
	maxLen := payloadLen - 6 - 1
	b.Language = sr.ReadZeroTerminatedString(maxLen)
	maxLen = payloadLen - 6 - len(b.Language) - 1
	b.Label = sr.ReadZeroTerminatedString(maxLen)
	if err := sr.AccError(); err != nil {
		return nil, fmt.Errorf("decode labl: %w", err)
	}
	return &b, nil
}

// Type - box type
func (b *LablBox) Type() string {
	return "labl"
}

// Size - calculated size of box
func (b *LablBox) Size() uint64 {
	return uint64(boxHeaderSize + 4 + 2 + len(b.Language) + 1 + len(b.Label) + 1)
}

// Encode - write box to w
func (b *LablBox) Encode(w io.Writer) error {
	sw := bits.NewFixedSliceWriter(int(b.Size()))
	err := b.EncodeSW(sw)
	if err != nil {
		return err
	}
	_, err = w.Write(sw.Bytes())
	return err
}

// EncodeSW - box-specific encode to slicewriter
func (b *LablBox) EncodeSW(sw bits.SliceWriter) error {
	err := EncodeHeaderSW(b, sw)
	if err != nil {
		return err
	}
	versionAndFlags := (uint32(b.Version) << 24) + b.Flags
	sw.WriteUint32(versionAndFlags)
	sw.WriteUint16(b.LabelID)
	sw.WriteString(b.Language, true)
	sw.WriteString(b.Label, true)
	return sw.AccError()
}

// Info - write box-specific information
func (b *LablBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, int(b.Version), b.Flags)
	bd.write(" - labelID: %d", b.LabelID)
	bd.write(" - isGroupLabel: %t", b.IsGroupLabel())
	bd.write(" - language: %s", b.Language)
	bd.write(" - label: %s", b.Label)
	return bd.err
}
