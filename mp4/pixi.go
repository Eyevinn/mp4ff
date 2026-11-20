package mp4

import (
	"fmt"
	"io"

	"github.com/Eyevinn/mp4ff/bits"
)

// PixiBox - Pixel Information Property (pixi), ISO/IEC 23008-12:2025 Sec. 6.5.6
//
// Contained in: Item Property Container Box (ipco)
//
// It gives the bit depth of each channel of the reconstructed image of the associated image item.
type PixiBox struct {
	Version        byte
	Flags          uint32
	BitsPerChannel []byte
}

// DecodePixi - box-specific decode
func DecodePixi(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := readBoxBody(r, hdr)
	if err != nil {
		return nil, err
	}
	sr := bits.NewFixedSliceReader(data)
	return DecodePixiSR(hdr, startPos, sr)
}

// DecodePixiSR - box-specific decode
func DecodePixiSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	versionAndFlags := sr.ReadUint32()
	b := PixiBox{
		Version: byte(versionAndFlags >> 24),
		Flags:   versionAndFlags & flagsMask,
	}
	numChannels := int(sr.ReadUint8())
	b.BitsPerChannel = append(b.BitsPerChannel, sr.ReadBytes(numChannels)...)
	return &b, sr.AccError()
}

// Type - box type
func (b *PixiBox) Type() string {
	return "pixi"
}

// Size - calculated size of box
func (b *PixiBox) Size() uint64 {
	return uint64(boxHeaderSize + 4 + 1 + len(b.BitsPerChannel))
}

// Encode - write box to w
func (b *PixiBox) Encode(w io.Writer) error {
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
// It returns an error for more than 255 channels.
func (b *PixiBox) EncodeSW(sw bits.SliceWriter) error {
	if len(b.BitsPerChannel) > 0xff {
		return fmt.Errorf("pixi: %d channels are more than 255", len(b.BitsPerChannel))
	}
	err := EncodeHeaderSW(b, sw)
	if err != nil {
		return err
	}
	versionAndFlags := (uint32(b.Version) << 24) + b.Flags
	sw.WriteUint32(versionAndFlags)
	sw.WriteUint8(byte(len(b.BitsPerChannel)))
	sw.WriteBytes(b.BitsPerChannel)
	return sw.AccError()
}

// Info - write box-specific information
func (b *PixiBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, int(b.Version), b.Flags)
	bd.write(" - bitsPerChannel: %v", b.BitsPerChannel)
	return bd.err
}
