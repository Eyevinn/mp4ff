package mp4

import (
	"io"

	"github.com/edgeware/mp4ff/bits"
)

// URLBox - DataEntryUrlBox ('url ')
//
// Contained in : DrefBox (dref
type URLBox struct {
	Version  byte
	Flags    uint32
	Location string // Zero-terminated string
}

const dataIsSelfContainedFlag = 0x000001

// DecodeURLBox - box-specific decode
func DecodeURLBox(hdr boxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := readBoxBody(r, hdr)
	if err != nil {
		return nil, err
	}
	s := NewSliceReader(data)
	versionAndFlags := s.ReadUint32()
	version := byte(versionAndFlags >> 24)
	flags := versionAndFlags & flagsMask
	location := ""
	if flags != dataIsSelfContainedFlag {
		location, err = s.ReadZeroTerminatedString()
	}

	b := &URLBox{
		Version:  version,
		Flags:    flags,
		Location: location,
	}
	return b, err
}

// CreateURLBox - Create a self-referencing URL box
func CreateURLBox() *URLBox {
	return &URLBox{
		Version:  0,
		Flags:    dataIsSelfContainedFlag,
		Location: "",
	}
}

// Type - return box type
func (b *URLBox) Type() string {
	return "url "
}

// Size - return calculated size
func (b *URLBox) Size() uint64 {
	size := uint64(boxHeaderSize + 4)
	if b.Flags != uint32(dataIsSelfContainedFlag) {
		size += uint64(len(b.Location) + 1)
	}
	return size
}

// Encode - write box to w
func (b *URLBox) Encode(w io.Writer) error {
	sw := bits.NewSliceWriterWithSize(int(b.Size()))
	err := b.EncodeSW(sw)
	if err != nil {
		return err
	}
	_, err = w.Write(sw.Bytes())
	return err
}

// EncodeSW - box-specific encode to slicewriter
func (b *URLBox) EncodeSW(sw bits.SliceWriter) error {
	err := EncodeHeaderSW(b, sw)
	if err != nil {
		return err
	}
	versionAndFlags := (uint32(b.Version) << 24) + b.Flags
	sw.WriteUint32(versionAndFlags)
	if b.Flags != dataIsSelfContainedFlag {
		sw.WriteString(b.Location, true)
	}
	return sw.AccError()
}

// Info - write specific box information
func (b *URLBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, -1, 0)
	bd.write(" - location: %q", b.Location)
	return bd.err
}
