package mp4

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"

	"github.com/Eyevinn/mp4ff/bits"
)

// Namespaces of DtypBox.
const (
	DtypNamespaceWellKnown = 0 // the value is a well-known type as 4 bytes, such as DataTypeUTF8
	DtypNamespaceName      = 1 // the value is a reverse DNS name without terminating zero
)

// DtypBox - Metadata Datatype Definition atom (dtyp) of QuickTime, telling the data type of the values of a key.
// Defined in https://developer.apple.com/documentation/quicktime-file-format/metadata_datatype_definition_atom
//
// Contained in: Metadata Key Box (MetadataKeyBox)
//
// Namespace tells how to read Value: DtypNamespaceWellKnown or DtypNamespaceName.
type DtypBox struct {
	Namespace uint32
	Value     []byte
}

// DecodeDtyp - box-specific decode
func DecodeDtyp(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := readBoxBody(r, hdr)
	if err != nil {
		return nil, err
	}
	return decodeOrUnknown(hdr, startPos, data, decodeDtypSR)
}

// DecodeDtypSR - box-specific decode
func DecodeDtypSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	return decodeOrUnknownSR(hdr, startPos, sr, decodeDtypSR)
}

func decodeDtypSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	if hdr.payloadLen() < 4 {
		return nil, fmt.Errorf("dtyp: payload of %d bytes is too short", hdr.payloadLen())
	}
	b := DtypBox{Namespace: sr.ReadUint32()}
	if n := hdr.payloadLen() - 4; n > 0 {
		b.Value = sr.ReadBytes(n)
	}
	return &b, sr.AccError()
}

// Type - box type
func (b *DtypBox) Type() string {
	return "dtyp"
}

// Size - calculated size of box
func (b *DtypBox) Size() uint64 {
	return uint64(boxHeaderSize + 4 + len(b.Value))
}

// WellKnownType returns the well-known type of the values, such as DataTypeUTF8, or false if the type is given
// in another way.
func (b *DtypBox) WellKnownType() (uint32, bool) {
	if b.Namespace != DtypNamespaceWellKnown || len(b.Value) != 4 {
		return 0, false
	}
	return binary.BigEndian.Uint32(b.Value), true
}

// Encode - write box to w
func (b *DtypBox) Encode(w io.Writer) error {
	sw := bits.NewFixedSliceWriter(int(b.Size()))
	err := b.EncodeSW(sw)
	if err != nil {
		return err
	}
	_, err = w.Write(sw.Bytes())
	return err
}

// EncodeSW - box-specific encode to slicewriter
func (b *DtypBox) EncodeSW(sw bits.SliceWriter) error {
	err := EncodeHeaderSW(b, sw)
	if err != nil {
		return err
	}
	sw.WriteUint32(b.Namespace)
	sw.WriteBytes(b.Value)
	return sw.AccError()
}

// Info - write box-specific information
func (b *DtypBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, -1, 0)
	bd.write(" - namespace: %d", b.Namespace)
	wellKnown, ok := b.WellKnownType()
	switch {
	case ok && dataTypeNames[wellKnown] != "":
		bd.write(" - dataType: %d (%s)", wellKnown, dataTypeNames[wellKnown])
	case ok:
		bd.write(" - dataType: %d", wellKnown)
	case b.Namespace == DtypNamespaceName:
		bd.write(" - dataType: %q", b.Value)
	default:
		bd.write(" - dataType: %s", hex.EncodeToString(b.Value))
	}
	return bd.err
}
