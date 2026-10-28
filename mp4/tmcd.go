package mp4

import (
	"fmt"
	"io"

	"github.com/Eyevinn/mp4ff/bits"
)

// QuickTime uses the type tmcd for three atoms: the timecode sample entry in stsd, the timecode media
// information in gmhd, which holds a tcmi atom, and a track reference type in tref, which TrefBox decodes.

// Flags of a TimecodeSampleEntryBox.
const (
	TimecodeFlagDropFrame       uint32 = 0x0001
	TimecodeFlag24HourMax       uint32 = 0x0002
	TimecodeFlagNegativeTimesOK uint32 = 0x0004
	TimecodeFlagCounter         uint32 = 0x0008
)

// TimecodeSampleEntryBox is the QuickTime timecode sample entry tmcd, which tells how to read the samples of a
// timecode track.
type TimecodeSampleEntryBox struct {
	DataReferenceIndex uint16
	// Reserved is a 32-bit field after the data reference index, 0.
	Reserved uint32
	// Flags are TimecodeFlagDropFrame, TimecodeFlag24HourMax, TimecodeFlagNegativeTimesOK and TimecodeFlagCounter.
	Flags uint32
	// TimeScale is the time scale of FrameDuration.
	TimeScale     uint32
	FrameDuration uint32
	// NumberOfFrames is the number of frames per second, or per counter tick if TimecodeFlagCounter is set.
	NumberOfFrames byte
	// Reserved2 are the reserved bytes after NumberOfFrames: one, as ffmpeg writes it, or three, as the
	// QuickTime File Format specification gives it. nil is written as one zero byte.
	Reserved2 []byte
	// Children are the atoms after the fields, such as a name atom with the name of the source tape.
	Children []Box
}

// timecodeEntryFieldsSize is the size of the fields of a timecode sample entry after the box header, up to
// NumberOfFrames.
const timecodeEntryFieldsSize = 25

// DecodeTmcd decodes a tmcd atom: as the timecode media information, a GenericContainerBox, if it holds a tcmi
// atom, and otherwise as a TimecodeSampleEntryBox. A payload in neither layout decodes as an UnknownBox.
func DecodeTmcd(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := readBoxBody(r, hdr)
	if err != nil {
		return nil, err
	}
	return decodeOrUnknown(hdr, startPos, data, decodeTmcdSR)
}

// DecodeTmcdSR decodes a tmcd atom, as DecodeTmcd.
func DecodeTmcdSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	return decodeOrUnknownSR(hdr, startPos, sr, decodeTmcdSR)
}

func decodeTmcdSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	head := make([]byte, 8)
	if sr.LookAhead(0, head) == nil && string(head[4:8]) == "tcmi" {
		return DecodeGenericContainerBoxSR(hdr, startPos, sr)
	}
	return decodeTimecodeSampleEntrySR(hdr, startPos, sr)
}

func decodeTimecodeSampleEntrySR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	if hdr.payloadLen() < timecodeEntryFieldsSize+1 {
		return nil, fmt.Errorf("tmcd: payload of %d bytes is too short", hdr.payloadLen())
	}
	sr.SkipBytes(6) // reserved
	b := TimecodeSampleEntryBox{DataReferenceIndex: sr.ReadUint16()}
	b.Reserved = sr.ReadUint32()
	b.Flags = sr.ReadUint32()
	b.TimeScale = sr.ReadUint32()
	b.FrameDuration = sr.ReadUint32()
	b.NumberOfFrames = sr.ReadUint8()
	// The child atoms come after one or three reserved bytes.
	rest := make([]byte, hdr.payloadLen()-timecodeEntryFieldsSize)
	if err := sr.LookAhead(0, rest); err != nil {
		return nil, err
	}
	end := startPos + hdr.Size
	for _, n := range []int{1, 3} {
		start := startPos + uint64(hdr.Hdrlen) + timecodeEntryFieldsSize + uint64(n)
		if n > len(rest) || !childrenFill(hdr, start, end, rest[n:]) {
			continue
		}
		b.Reserved2 = sr.ReadBytes(n)
		children, err := DecodeContainerChildrenSR(hdr, start, end, sr)
		if err != nil {
			return nil, err
		}
		if len(children) > 0 {
			b.Children = children
		}
		return &b, sr.AccError()
	}
	return nil, fmt.Errorf("tmcd: no child atoms after the reserved bytes")
}

// childrenFill tells if data, from position start up to end, is a list of boxes.
func childrenFill(hdr BoxHeader, start, end uint64, data []byte) bool {
	sr := bits.NewFixedSliceReader(data)
	_, err := DecodeContainerChildrenSR(hdr, start, end, sr)
	return err == nil && sr.NrRemainingBytes() == 0
}

// Type - box type
func (b *TimecodeSampleEntryBox) Type() string {
	return "tmcd"
}

// Size - calculated size of box
func (b *TimecodeSampleEntryBox) Size() uint64 {
	size := uint64(boxHeaderSize+timecodeEntryFieldsSize) + uint64(max(1, len(b.Reserved2)))
	for _, c := range b.Children {
		size += c.Size()
	}
	return size
}

// GetChildren - list of child boxes
func (b *TimecodeSampleEntryBox) GetChildren() []Box {
	return b.Children
}

// Encode - write box to w
func (b *TimecodeSampleEntryBox) Encode(w io.Writer) error {
	sw := bits.NewFixedSliceWriter(int(b.Size()))
	err := b.EncodeSW(sw)
	if err != nil {
		return err
	}
	_, err = w.Write(sw.Bytes())
	return err
}

// EncodeSW - box-specific encode to slicewriter
func (b *TimecodeSampleEntryBox) EncodeSW(sw bits.SliceWriter) error {
	err := EncodeHeaderSW(b, sw)
	if err != nil {
		return err
	}
	sw.WriteZeroBytes(6)
	sw.WriteUint16(b.DataReferenceIndex)
	sw.WriteUint32(b.Reserved)
	sw.WriteUint32(b.Flags)
	sw.WriteUint32(b.TimeScale)
	sw.WriteUint32(b.FrameDuration)
	sw.WriteUint8(b.NumberOfFrames)
	if len(b.Reserved2) == 0 {
		sw.WriteUint8(0)
	}
	sw.WriteBytes(b.Reserved2)
	for _, c := range b.Children {
		if err := c.EncodeSW(sw); err != nil {
			return err
		}
	}
	return sw.AccError()
}

// Info - write box-specific information
func (b *TimecodeSampleEntryBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, -1, 0)
	bd.write(" - dataReferenceIndex: %d", b.DataReferenceIndex)
	bd.write(" - flags: %#x (dropFrame=%t 24HourMax=%t negativeTimesOK=%t counter=%t)", b.Flags,
		b.Flags&TimecodeFlagDropFrame != 0, b.Flags&TimecodeFlag24HourMax != 0,
		b.Flags&TimecodeFlagNegativeTimesOK != 0, b.Flags&TimecodeFlagCounter != 0)
	bd.write(" - timeScale: %d", b.TimeScale)
	bd.write(" - frameDuration: %d", b.FrameDuration)
	bd.write(" - numberOfFrames: %d", b.NumberOfFrames)
	if bd.err != nil {
		return bd.err
	}
	for _, c := range b.Children {
		if err := c.Info(w, specificBoxLevels, indent+indentStep, indentStep); err != nil {
			return err
		}
	}
	return nil
}
