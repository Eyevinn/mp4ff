package mp4

import (
	"encoding/binary"
	"fmt"
	"io"

	"github.com/Eyevinn/mp4ff/bits"
)

// KeysBox is the QuickTime metadata item keys atom keys, in a meta atom whose handler is mdta. The items in the
// ilst atom of the same meta atom have the index of their key, from 1, as their type, such as \x00\x00\x00\x01
// for the first key.
//
// A keys atom with another layout, such as the one in a timed metadata sample entry mebx, decodes as an
// UnknownBox.
type KeysBox struct {
	Version byte
	Flags   uint32
	Keys    []MetadataKey
}

// MetadataKey is a key of a KeysBox.
type MetadataKey struct {
	// Namespace is the key namespace, such as mdta for reverse DNS keys or udta for user data types.
	Namespace string
	// Value is the key, such as com.apple.quicktime.make.
	Value string
}

// DecodeKeys - box-specific decode
func DecodeKeys(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := readBoxBody(r, hdr)
	if err != nil {
		return nil, err
	}
	return decodeOrUnknown(hdr, startPos, data, decodeKeysSR)
}

// DecodeKeysSR - box-specific decode
func DecodeKeysSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	return decodeOrUnknownSR(hdr, startPos, sr, decodeKeysSR)
}

func decodeKeysSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	if hdr.payloadLen() < 8 {
		return nil, fmt.Errorf("keys: payload of %d bytes is too short", hdr.payloadLen())
	}
	end := sr.GetPos() + hdr.payloadLen()
	versionAndFlags := sr.ReadUint32()
	b := KeysBox{Version: byte(versionAndFlags >> 24), Flags: versionAndFlags & flagsMask}
	count := sr.ReadUint32()
	for i := uint32(0); i < count; i++ {
		size := sr.ReadUint32()
		if size < 8 || int(size)-4 > end-sr.GetPos() {
			return nil, fmt.Errorf("keys: key %d of size %d does not fit", i+1, size)
		}
		ns := sr.ReadFixedLengthString(4)
		b.Keys = append(b.Keys, MetadataKey{Namespace: ns, Value: sr.ReadFixedLengthString(int(size) - 8)})
	}
	return &b, sr.AccError()
}

// Type - box type
func (b *KeysBox) Type() string {
	return "keys"
}

// Size - calculated size of box
func (b *KeysBox) Size() uint64 {
	size := uint64(boxHeaderSize + 8)
	for _, k := range b.Keys {
		size += 8 + uint64(len(k.Value))
	}
	return size
}

// Key returns the key with index, from 1, as the type of an item in ilst gives it, or false if there is none.
func (b *KeysBox) Key(index uint32) (MetadataKey, bool) {
	if index == 0 || int(index) > len(b.Keys) {
		return MetadataKey{}, false
	}
	return b.Keys[index-1], true
}

// KeyOfItem returns the key that the type of an item in ilst refers to, such as \x00\x00\x00\x01 for the first
// key, or false if the type is not the index of a key.
func (b *KeysBox) KeyOfItem(itemType string) (MetadataKey, bool) {
	if len(itemType) != 4 {
		return MetadataKey{}, false
	}
	return b.Key(binary.BigEndian.Uint32([]byte(itemType)))
}

// Encode - write box to w
func (b *KeysBox) Encode(w io.Writer) error {
	sw := bits.NewFixedSliceWriter(int(b.Size()))
	err := b.EncodeSW(sw)
	if err != nil {
		return err
	}
	_, err = w.Write(sw.Bytes())
	return err
}

// EncodeSW - box-specific encode to slicewriter
func (b *KeysBox) EncodeSW(sw bits.SliceWriter) error {
	for _, k := range b.Keys {
		if len(k.Namespace) != 4 {
			return fmt.Errorf("keys: namespace %q is not 4 bytes", k.Namespace)
		}
	}
	err := EncodeHeaderSW(b, sw)
	if err != nil {
		return err
	}
	sw.WriteUint32(uint32(b.Version)<<24 | b.Flags)
	sw.WriteUint32(uint32(len(b.Keys)))
	for _, k := range b.Keys {
		sw.WriteUint32(uint32(8 + len(k.Value)))
		sw.WriteString(k.Namespace, false)
		sw.WriteString(k.Value, false)
	}
	return sw.AccError()
}

// Info - write box-specific information
func (b *KeysBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, int(b.Version), b.Flags)
	for i, k := range b.Keys {
		bd.write(" - key[%d]: %s %q", i+1, k.Namespace, k.Value)
	}
	return bd.err
}
