package mp4

import (
	"encoding/binary"
	"fmt"
	"io"

	"github.com/Eyevinn/mp4ff/bits"
)

// NameBox is a name atom that holds a name. It has one of two layouts:
//
//   - In a freeform metadata item ---- in ilst, the name of the item, such as iTunSMPB, comes after version and
//     flags, which are 0, and FullBox is true. The mean atom of the item, a MeanBox, has the domain of the name.
//   - In a QuickTime udta atom, the name of the movie or track is the whole payload.
//
// A name atom whose payload is a list of QuickTime user data texts, as the name of the source tape in a timecode
// sample entry, decodes as a UserDataTextBox instead.
type NameBox struct {
	// FullBox tells if version and flags, both 0, come before the name.
	FullBox bool
	Name    string
}

// DecodeName decodes a name atom: as a UserDataTextBox if its payload is a list of QuickTime user data texts,
// and otherwise as a NameBox.
func DecodeName(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := readBoxBody(r, hdr)
	if err != nil {
		return nil, err
	}
	return DecodeNameSR(hdr, startPos, bits.NewFixedSliceReader(data))
}

// DecodeNameSR decodes a name atom, as DecodeName.
func DecodeNameSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	n := hdr.payloadLen()
	if sr.NrRemainingBytes() < n {
		return nil, fmt.Errorf("name: payload of %d bytes, but %d bytes left", n, sr.NrRemainingBytes())
	}
	data := make([]byte, n)
	if err := sr.LookAhead(0, data); err != nil {
		return nil, err
	}
	if _, ok := parseUserDataTexts(data); ok {
		return DecodeUserDataTextSR(hdr, startPos, sr)
	}
	b := NameBox{FullBox: n >= 4 && binary.BigEndian.Uint32(data) == 0}
	if b.FullBox {
		sr.SkipBytes(4) // version and flags
		n -= 4
	}
	b.Name = sr.ReadFixedLengthString(n)
	return &b, sr.AccError()
}

// Type - box type
func (b *NameBox) Type() string {
	return "name"
}

// Size - calculated size of box
func (b *NameBox) Size() uint64 {
	size := uint64(boxHeaderSize + len(b.Name))
	if b.FullBox {
		size += 4
	}
	return size
}

// Encode - write box to w
func (b *NameBox) Encode(w io.Writer) error {
	sw := bits.NewFixedSliceWriter(int(b.Size()))
	err := b.EncodeSW(sw)
	if err != nil {
		return err
	}
	_, err = w.Write(sw.Bytes())
	return err
}

// EncodeSW - box-specific encode to slicewriter
func (b *NameBox) EncodeSW(sw bits.SliceWriter) error {
	err := EncodeHeaderSW(b, sw)
	if err != nil {
		return err
	}
	if b.FullBox {
		sw.WriteUint32(0)
	}
	sw.WriteString(b.Name, false)
	return sw.AccError()
}

// Info - write box-specific information
func (b *NameBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	version := -1
	if b.FullBox {
		version = 0
	}
	bd := newInfoDumper(w, indent, b, version, 0)
	bd.write(" - name: %q", b.Name)
	return bd.err
}
