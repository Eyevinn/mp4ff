package mp4

import (
	"encoding/binary"
	"fmt"
	"io"

	"github.com/Eyevinn/mp4ff/bits"
)

// generalTypeBox implements the GeneralTypeBox syntax of ISO/IEC 14496-12:2026
// Section 5.2: a major brand, a minor version and a list of compatible brands.
// It is embedded in FtypBox and StypBox, whose methods it provides. The payload
// is kept as raw bytes.
type generalTypeBox struct {
	data []byte
}

// newGeneralTypeBox builds the payload from its fields.
func newGeneralTypeBox(majorBrand string, minorVersion uint32, compatibleBrands []string) generalTypeBox {
	data := make([]byte, 8+4*len(compatibleBrands))
	copy(data, []byte(majorBrand))
	binary.BigEndian.PutUint32(data[4:8], minorVersion)
	for i, cb := range compatibleBrands {
		pos := 8 + 4*i
		copy(data[pos:pos+4], []byte(cb))
	}
	return generalTypeBox{data: data}
}

// decodeGeneralTypeBoxSR reads the payload of a box with GeneralTypeBox syntax.
func decodeGeneralTypeBoxSR(hdr BoxHeader, sr bits.SliceReader) (generalTypeBox, error) {
	if hdr.payloadLen() < 8 {
		return generalTypeBox{}, fmt.Errorf("%s: payload too short: %d < 8", hdr.Name, hdr.payloadLen())
	}
	g := generalTypeBox{data: sr.ReadBytes(hdr.payloadLen())}
	return g, sr.AccError()
}

// MajorBrand - major brand (4 chars)
func (b *generalTypeBox) MajorBrand() string {
	return string(b.data[:4])
}

// MinorVersion - minor version
func (b *generalTypeBox) MinorVersion() uint32 {
	return binary.BigEndian.Uint32(b.data[4:8])
}

// AddCompatibleBrands adds new compatible brands to the box.
func (b *generalTypeBox) AddCompatibleBrands(compatibleBrands []string) {
	for _, cb := range compatibleBrands {
		b.data = append(b.data, []byte(cb)...)
	}
}

// CompatibleBrands - slice of compatible brands (4 chars each)
func (b *generalTypeBox) CompatibleBrands() []string {
	nrCompatibleBrands := (len(b.data) - 8) / 4
	if nrCompatibleBrands == 0 {
		return nil
	}
	compatibleBrands := make([]string, nrCompatibleBrands)
	for i := range nrCompatibleBrands {
		pos := 8 + 4*i
		compatibleBrands[i] = string(b.data[pos : pos+4])
	}
	return compatibleBrands
}

// HasCompatibleBrand reports whether brand is one of the compatible brands.
// The major brand is not included unless it is also listed as compatible.
func (b *generalTypeBox) HasCompatibleBrand(brand string) bool {
	if len(brand) != 4 {
		return false
	}
	for pos := 8; pos+4 <= len(b.data); pos += 4 {
		if string(b.data[pos:pos+4]) == brand {
			return true
		}
	}
	return false
}

// clone returns a deep copy.
func (b *generalTypeBox) clone() generalTypeBox {
	data := make([]byte, len(b.data))
	copy(data, b.data)
	return generalTypeBox{data: data}
}

// size returns the size of the box including its header.
func (b *generalTypeBox) size() uint64 {
	return uint64(boxHeaderSize + len(b.data))
}

// encode writes box, which embeds b, to w.
func (b *generalTypeBox) encode(box Box, w io.Writer) error {
	sw := bits.NewFixedSliceWriter(int(box.Size()))
	err := box.EncodeSW(sw)
	if err != nil {
		return err
	}
	_, err = w.Write(sw.Bytes())
	return err
}

// encodeSW writes box, which embeds b, to sw.
func (b *generalTypeBox) encodeSW(box Box, sw bits.SliceWriter) error {
	err := EncodeHeaderSW(box, sw)
	if err != nil {
		return err
	}
	sw.WriteBytes(b.data)
	return sw.AccError()
}

// info writes the fields of box, which embeds b, to w.
func (b *generalTypeBox) info(box Box, w io.Writer, indent string) error {
	bd := newInfoDumper(w, indent, box, -1, 0)
	bd.write(" - majorBrand: %s", b.MajorBrand())
	bd.write(" - minorVersion: %d", b.MinorVersion())
	for _, cb := range b.CompatibleBrands() {
		bd.write(" - compatibleBrand: %s", cb)
	}
	return bd.err
}
