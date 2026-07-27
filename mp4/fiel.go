package mp4

import (
	"io"

	"github.com/Eyevinn/mp4ff/bits"
)

// FielBox - Field/frame information box (fiel)
// Defined in QuickTime File Format as video sample description extension,
// with interpretation details in Technical Note TN2162 [tn2162].
// Used in mjpg sample entries produced by Apple mediafilesegmenter.
//
// [tn2162]: https://developer.apple.com/library/archive/technotes/tn2162/_index.html
type FielBox struct {
	// FieldCount is 1 for progressive scan and 2 for 2:1 interlaced video
	FieldCount byte
	// FieldOrdering describes which field is temporally first and how the two
	// fields of an interlaced frame are laid out in the buffer:
	//	0 = unknown ordering (required when FieldCount is 1)
	//	1 = top field first, fields stored separately in temporal order
	//	6 = bottom field first, fields stored separately in temporal order
	//	9 = top field first, field lines woven together in spatial order
	//	14 = bottom field first, field lines woven together in spatial order
	FieldOrdering byte
}

// DecodeFiel - box-specific decode
func DecodeFiel(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := readBoxBody(r, hdr)
	if err != nil {
		return nil, err
	}
	sr := bits.NewFixedSliceReader(data)
	return DecodeFielSR(hdr, startPos, sr)
}

// DecodeFielSR - box-specific decode
func DecodeFielSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	b := FielBox{}
	b.FieldCount = sr.ReadUint8()
	b.FieldOrdering = sr.ReadUint8()
	return &b, sr.AccError()
}

// Type - box type
func (b *FielBox) Type() string {
	return "fiel"
}

// Size - calculated size of box
func (b *FielBox) Size() uint64 {
	return uint64(boxHeaderSize + 2)
}

// Encode - write box to w
func (b *FielBox) Encode(w io.Writer) error {
	sw := bits.NewFixedSliceWriter(int(b.Size()))
	err := b.EncodeSW(sw)
	if err != nil {
		return err
	}
	_, err = w.Write(sw.Bytes())
	return err
}

// EncodeSW - box-specific encode to slicewriter
func (b *FielBox) EncodeSW(sw bits.SliceWriter) error {
	err := EncodeHeaderSW(b, sw)
	if err != nil {
		return err
	}
	sw.WriteUint8(b.FieldCount)
	sw.WriteUint8(b.FieldOrdering)
	return sw.AccError()
}

// Info - write box-specific information
func (b *FielBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, -1, 0)
	bd.write(" - fieldCount: %d (%s), fieldOrdering: %d (%s)",
		b.FieldCount, fieldCountDescription(b.FieldCount),
		b.FieldOrdering, fieldOrderingDescription(b.FieldOrdering))
	return bd.err
}

// fieldCountDescription - verbal interpretation of fieldCount
func fieldCountDescription(fieldCount byte) string {
	switch fieldCount {
	case 1:
		return "progressive"
	case 2:
		return "interlaced"
	default:
		return "invalid"
	}
}

// fieldOrderingDescription - verbal interpretation of fieldOrdering
func fieldOrderingDescription(fieldOrdering byte) string {
	switch fieldOrdering {
	case 0:
		return "unknown"
	case 1:
		return "top field first, fields stored separately"
	case 6:
		return "bottom field first, fields stored separately"
	case 9:
		return "top field first, fields woven together"
	case 14:
		return "bottom field first, fields woven together"
	default:
		return "invalid"
	}
}
