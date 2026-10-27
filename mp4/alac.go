package mp4

import (
	"encoding/hex"
	"fmt"
	"io"

	"github.com/Eyevinn/mp4ff/bits"
)

// alacSpecificConfigLen is the size of ALACSpecificConfig, the mandatory part of the ALAC magic cookie.
const alacSpecificConfigLen = 24

// AlacBox - ALAC specific box (alac) carrying the Apple Lossless "magic cookie".
// Defined in https://github.com/macosforge/alac/blob/master/ALACMagicCookieDescription.txt
//
// It is a full box holding ALACSpecificConfig, optionally followed by a 24-byte
// ALACChannelLayoutInfo. The box shares its type with the alac audio sample entry
// that contains it, directly in MP4 files or inside a wave box in QuickTime files.
// Only as a child of an audio sample entry or wave box is an alac box decoded as
// an AlacBox; anywhere else, alac is the sample entry.
type AlacBox struct {
	Version           byte
	Flags             uint32
	FrameLength       uint32 // samples per frame (packet), normally 4096
	CompatibleVersion byte
	BitDepth          byte
	Pb                byte
	Mb                byte
	Kb                byte
	NumChannels       byte
	MaxRun            uint16
	MaxFrameBytes     uint32 // 0 means unknown
	AvgBitRate        uint32 // bits per second, 0 means unknown
	SampleRate        uint32
	// ChannelLayoutInfo holds the bytes after ALACSpecificConfig verbatim:
	// empty, or the 24-byte ALACChannelLayoutInfo.
	ChannelLayoutInfo []byte
}

// DecodeAlac - box-specific decode
func DecodeAlac(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := readBoxBody(r, hdr)
	if err != nil {
		return nil, err
	}
	sr := bits.NewFixedSliceReader(data)
	return DecodeAlacSR(hdr, startPos, sr)
}

// DecodeAlacSR - box-specific decode
func DecodeAlacSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	payloadLen := hdr.payloadLen()
	if payloadLen < 4+alacSpecificConfigLen {
		return nil, fmt.Errorf("alac payload of %d bytes is shorter than the %d of version, flags and ALACSpecificConfig",
			payloadLen, 4+alacSpecificConfigLen)
	}
	versionAndFlags := sr.ReadUint32()
	b := &AlacBox{
		Version:           byte(versionAndFlags >> 24),
		Flags:             versionAndFlags & flagsMask,
		FrameLength:       sr.ReadUint32(),
		CompatibleVersion: sr.ReadUint8(),
		BitDepth:          sr.ReadUint8(),
		Pb:                sr.ReadUint8(),
		Mb:                sr.ReadUint8(),
		Kb:                sr.ReadUint8(),
		NumChannels:       sr.ReadUint8(),
		MaxRun:            sr.ReadUint16(),
		MaxFrameBytes:     sr.ReadUint32(),
		AvgBitRate:        sr.ReadUint32(),
		SampleRate:        sr.ReadUint32(),
	}
	if n := payloadLen - 4 - alacSpecificConfigLen; n > 0 {
		b.ChannelLayoutInfo = sr.ReadBytes(n)
	}
	return b, sr.AccError()
}

// decodeAudioChildBoxSR decodes a child box of an audio sample entry or wave box,
// where a box of type alac is an AlacBox instead of an audio sample entry.
func decodeAudioChildBoxSR(startPos uint64, sr bits.SliceReader) (Box, error) {
	hdr, d, err := decodeHeaderSR(sr)
	if err != nil {
		return nil, err
	}
	if hdr.Name == "alac" {
		d = DecodeAlacSR
	}
	return decodeBoxBodySR(startPos, hdr, d, sr)
}

// Type - return box type
func (b *AlacBox) Type() string {
	return "alac"
}

// Size - return calculated size
func (b *AlacBox) Size() uint64 {
	return uint64(boxHeaderSize + 4 + alacSpecificConfigLen + len(b.ChannelLayoutInfo))
}

// Encode - write box to w
func (b *AlacBox) Encode(w io.Writer) error {
	sw := bits.NewFixedSliceWriter(int(b.Size()))
	err := b.EncodeSW(sw)
	if err != nil {
		return err
	}
	_, err = w.Write(sw.Bytes())
	return err
}

// EncodeSW - box-specific encode to slicewriter
func (b *AlacBox) EncodeSW(sw bits.SliceWriter) error {
	err := EncodeHeaderSW(b, sw)
	if err != nil {
		return err
	}
	versionAndFlags := (uint32(b.Version) << 24) + b.Flags
	sw.WriteUint32(versionAndFlags)
	sw.WriteUint32(b.FrameLength)
	sw.WriteUint8(b.CompatibleVersion)
	sw.WriteUint8(b.BitDepth)
	sw.WriteUint8(b.Pb)
	sw.WriteUint8(b.Mb)
	sw.WriteUint8(b.Kb)
	sw.WriteUint8(b.NumChannels)
	sw.WriteUint16(b.MaxRun)
	sw.WriteUint32(b.MaxFrameBytes)
	sw.WriteUint32(b.AvgBitRate)
	sw.WriteUint32(b.SampleRate)
	sw.WriteBytes(b.ChannelLayoutInfo)
	return sw.AccError()
}

// Info - write box-specific information
func (b *AlacBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, int(b.Version), b.Flags)
	bd.write(" - frameLength: %d", b.FrameLength)
	bd.write(" - compatibleVersion: %d", b.CompatibleVersion)
	bd.write(" - bitDepth: %d", b.BitDepth)
	bd.write(" - pb: %d, mb: %d, kb: %d", b.Pb, b.Mb, b.Kb)
	bd.write(" - numChannels: %d", b.NumChannels)
	bd.write(" - maxRun: %d", b.MaxRun)
	bd.write(" - maxFrameBytes: %d", b.MaxFrameBytes)
	bd.write(" - avgBitRate: %d", b.AvgBitRate)
	bd.write(" - sampleRate: %d", b.SampleRate)
	if len(b.ChannelLayoutInfo) > 0 {
		bd.write(" - channelLayoutInfo: %s", hex.EncodeToString(b.ChannelLayoutInfo))
	}
	return bd.err
}
