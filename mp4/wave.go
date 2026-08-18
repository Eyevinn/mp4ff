package mp4

import (
	"encoding/binary"
	"io"

	"github.com/Eyevinn/mp4ff/bits"
)

// WaveBox - siDecompressionParam atom (QuickTime File Format), a container
// inside audio sample entries that wraps the decoder configuration,
// typically frma + esds followed by a terminator atom. Children that do not
// decode cleanly (such as a 12-byte mp4a stub) are kept as UnknownBox, and a
// tail that is not a well-formed box (such as a size-zero terminator) is
// preserved verbatim in RawTail, so the round trip stays byte-exact.
type WaveBox struct {
	Frma     *FrmaBox
	Esds     *EsdsBox
	Children []Box
	// RawTail holds the bytes from the first position where a well-formed box
	// header could not be read to the end of the wave payload. Since the
	// terminator atom comes last, this is normally either empty or just that
	// atom, but a malformed atom anywhere in the payload puts everything from
	// it onwards here, so any boxes after it are not decoded into Children.
	// Encode writes RawTail verbatim after the children.
	RawTail []byte
}

// GetChildren - list of child boxes. Note that the bytes in RawTail are not
// part of the children, so a generic container encode would drop them.
func (b *WaveBox) GetChildren() []Box {
	return b.Children
}

// AddChild - add a child box
func (b *WaveBox) AddChild(child Box) {
	switch box := child.(type) {
	case *FrmaBox:
		b.Frma = box
	case *EsdsBox:
		b.Esds = box
	}
	b.Children = append(b.Children, child)
}

// DecodeWave - box-specific decode
func DecodeWave(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := readBoxBody(r, hdr)
	if err != nil {
		return nil, err
	}
	return decodeWaveFromData(hdr, startPos, data)
}

// DecodeWaveSR - box-specific decode
func DecodeWaveSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	data := sr.ReadBytes(hdr.payloadLen())
	if sr.AccError() != nil {
		return nil, sr.AccError()
	}
	return decodeWaveFromData(hdr, startPos, data)
}

func decodeWaveFromData(hdr BoxHeader, startPos uint64, data []byte) (Box, error) {
	b := &WaveBox{}
	pos := 0
	for pos+8 <= len(data) {
		size := int(binary.BigEndian.Uint32(data[pos : pos+4]))
		name := string(data[pos+4 : pos+8])
		if size < 8 || size > len(data)-pos {
			break // malformed child (e.g. a size-zero terminator): keep the tail verbatim
		}
		childData := data[pos : pos+size]
		childStartPos := startPos + uint64(hdr.Hdrlen) + uint64(pos)
		child, err := DecodeBoxSR(childStartPos, bits.NewFixedSliceReader(childData))
		if err != nil || child.Size() != uint64(size) { // malformed child: keep the bytes
			payload := make([]byte, size-8)
			copy(payload, childData[8:])
			child = CreateUnknownBox(name, uint64(size), payload)
		}
		b.AddChild(child)
		pos += size
	}
	if pos < len(data) {
		b.RawTail = make([]byte, len(data)-pos)
		copy(b.RawTail, data[pos:])
	}
	return b, nil
}

// Type - box type
func (b *WaveBox) Type() string {
	return "wave"
}

// Size - calculated size of box
func (b *WaveBox) Size() uint64 {
	size := uint64(boxHeaderSize) + uint64(len(b.RawTail))
	for _, child := range b.Children {
		size += child.Size()
	}
	return size
}

// Encode - write box to w
func (b *WaveBox) Encode(w io.Writer) error {
	sw := bits.NewFixedSliceWriter(int(b.Size()))
	err := b.EncodeSW(sw)
	if err != nil {
		return err
	}
	_, err = w.Write(sw.Bytes())
	return err
}

// EncodeSW - box-specific encode to slicewriter
func (b *WaveBox) EncodeSW(sw bits.SliceWriter) error {
	err := EncodeHeaderSW(b, sw)
	if err != nil {
		return err
	}
	for _, child := range b.Children {
		err = child.EncodeSW(sw)
		if err != nil {
			return err
		}
	}
	sw.WriteBytes(b.RawTail)
	return sw.AccError()
}

// Info - write box-specific information
func (b *WaveBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, -1, 0)
	if bd.err != nil {
		return bd.err
	}
	if len(b.RawTail) > 0 {
		bd.write(" - raw_tail: %d bytes", len(b.RawTail))
	}
	var err error
	for _, child := range b.Children {
		err = child.Info(w, specificBoxLevels, indent+indentStep, indentStep)
		if err != nil {
			return err
		}
	}
	return nil
}
