package mp4

import (
	"fmt"
	"io"
	"time"

	"github.com/Eyevinn/mp4ff/bits"
)

// ChplBox is the Nero chapter list chpl in the udta of moov. It is not in a published specification. Nero, ffmpeg
// and GPAC write it with version 1, and 4 reserved bytes before the number of chapters.
type ChplBox struct {
	Version  byte
	Flags    uint32
	Reserved uint32
	Chapters []NeroChapter
}

// NeroChapter is a chapter in a ChplBox.
type NeroChapter struct {
	// Start is the start time of the chapter in units of 100 nanoseconds.
	Start uint64
	// Title is the title of the chapter in UTF-8.
	Title string
}

// StartTime returns the start time of the chapter.
func (c NeroChapter) StartTime() time.Duration {
	return time.Duration(c.Start) * 100
}

// DecodeChpl - box-specific decode
func DecodeChpl(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := readBoxBody(r, hdr)
	if err != nil {
		return nil, err
	}
	return decodeOrUnknown(hdr, startPos, data, decodeChplSR)
}

// DecodeChplSR - box-specific decode
func DecodeChplSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	return decodeOrUnknownSR(hdr, startPos, sr, decodeChplSR)
}

func decodeChplSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	if hdr.payloadLen() < 9 {
		return nil, fmt.Errorf("chpl: payload of %d bytes is too short", hdr.payloadLen())
	}
	end := sr.GetPos() + hdr.payloadLen()
	versionAndFlags := sr.ReadUint32()
	b := ChplBox{Version: byte(versionAndFlags >> 24), Flags: versionAndFlags & flagsMask}
	b.Reserved = sr.ReadUint32()
	count := int(sr.ReadUint8())
	for i := 0; i < count; i++ {
		if end-sr.GetPos() < 9 {
			return nil, fmt.Errorf("chpl: chapter %d of %d does not fit", i+1, count)
		}
		start := sr.ReadUint64()
		n := int(sr.ReadUint8())
		if n > end-sr.GetPos() {
			return nil, fmt.Errorf("chpl: title of chapter %d does not fit", i+1)
		}
		b.Chapters = append(b.Chapters, NeroChapter{Start: start, Title: sr.ReadFixedLengthString(n)})
	}
	return &b, sr.AccError()
}

// Type - box type
func (b *ChplBox) Type() string {
	return "chpl"
}

// Size - calculated size of box
func (b *ChplBox) Size() uint64 {
	size := uint64(boxHeaderSize + 9)
	for _, c := range b.Chapters {
		size += 9 + uint64(len(c.Title))
	}
	return size
}

// Encode - write box to w
func (b *ChplBox) Encode(w io.Writer) error {
	sw := bits.NewFixedSliceWriter(int(b.Size()))
	err := b.EncodeSW(sw)
	if err != nil {
		return err
	}
	_, err = w.Write(sw.Bytes())
	return err
}

// EncodeSW - box-specific encode to slicewriter
func (b *ChplBox) EncodeSW(sw bits.SliceWriter) error {
	if len(b.Chapters) > 255 {
		return fmt.Errorf("chpl: %d chapters are more than 255", len(b.Chapters))
	}
	for _, c := range b.Chapters {
		if len(c.Title) > 255 {
			return fmt.Errorf("chpl: title of %d bytes is longer than 255", len(c.Title))
		}
	}
	err := EncodeHeaderSW(b, sw)
	if err != nil {
		return err
	}
	sw.WriteUint32(uint32(b.Version)<<24 | b.Flags)
	sw.WriteUint32(b.Reserved)
	sw.WriteUint8(byte(len(b.Chapters)))
	for _, c := range b.Chapters {
		sw.WriteUint64(c.Start)
		sw.WriteUint8(byte(len(c.Title)))
		sw.WriteString(c.Title, false)
	}
	return sw.AccError()
}

// Info - write box-specific information
func (b *ChplBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, int(b.Version), b.Flags)
	for i, c := range b.Chapters {
		bd.write(" - chapter[%d]: start=%s title=%q", i+1, c.StartTime(), c.Title)
	}
	return bd.err
}
