package mp4

import (
	"fmt"
	"io"

	"github.com/Eyevinn/mp4ff/bits"
)

// KeydBox - Metadata Key Declaration Box (keyd), ISO/IEC 14496-12:2026 Sec. 12.8.4.4
//
// Contained in: Metadata Key Box (MetadataKeyBox)
//
// Namespace tells how to read Value: mdta for a reverse DNS name such as com.apple.quicktime.video-orientation,
// uiso for the four bytes of a user data type, or me4c for the four bytes of a metadata sample entry type.
type KeydBox struct {
	Namespace string
	Value     string
}

// DecodeKeyd - box-specific decode
func DecodeKeyd(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := readBoxBody(r, hdr)
	if err != nil {
		return nil, err
	}
	return decodeOrUnknown(hdr, startPos, data, decodeKeydSR)
}

// DecodeKeydSR - box-specific decode
func DecodeKeydSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	return decodeOrUnknownSR(hdr, startPos, sr, decodeKeydSR)
}

func decodeKeydSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	if hdr.payloadLen() < 4 {
		return nil, fmt.Errorf("keyd: payload of %d bytes is too short", hdr.payloadLen())
	}
	b := KeydBox{Namespace: sr.ReadFixedLengthString(4)}
	b.Value = sr.ReadFixedLengthString(hdr.payloadLen() - 4)
	return &b, sr.AccError()
}

// Type - box type
func (b *KeydBox) Type() string {
	return "keyd"
}

// Size - calculated size of box
func (b *KeydBox) Size() uint64 {
	return uint64(boxHeaderSize + 4 + len(b.Value))
}

// Key returns the namespace and the value as a MetadataKey.
func (b *KeydBox) Key() MetadataKey {
	return MetadataKey{Namespace: b.Namespace, Value: b.Value}
}

// Encode - write box to w
func (b *KeydBox) Encode(w io.Writer) error {
	sw := bits.NewFixedSliceWriter(int(b.Size()))
	err := b.EncodeSW(sw)
	if err != nil {
		return err
	}
	_, err = w.Write(sw.Bytes())
	return err
}

// EncodeSW - box-specific encode to slicewriter
//
// It returns an error for a namespace that is not 4 bytes.
func (b *KeydBox) EncodeSW(sw bits.SliceWriter) error {
	if len(b.Namespace) != 4 {
		return fmt.Errorf("keyd: namespace %q is not 4 bytes", b.Namespace)
	}
	err := EncodeHeaderSW(b, sw)
	if err != nil {
		return err
	}
	sw.WriteString(b.Namespace, false)
	sw.WriteString(b.Value, false)
	return sw.AccError()
}

// Info - write box-specific information
func (b *KeydBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, -1, 0)
	bd.write(" - key: %s", b.Key())
	return bd.err
}
