package av1

import (
	"io"

	"github.com/Eyevinn/mp4ff/bits"
)

// DecConfRec - AV1CodecConfigurationRecord
// Specified in https://github.com/AOMediaCodec/av1-isobmff/releases/tag/v1.2.0
type DecConfRec struct {
	Marker                           byte
	Version                          byte
	SeqProfile                       byte
	SeqLevelIdx0                     byte
	SeqTier0                         byte
	HighBitdepth                     byte
	TwelveBit                        byte
	MonoChrome                       byte
	ChromaSubsamplingX               byte
	ChromaSubsamplingY               byte
	ChromaSamplePosition             byte
	Reserved                         byte
	InitialPresentationDelayPresent  byte
	InitialPresentationDelayMinusOne byte
	ConfigOBUs                       []byte
}

// DecodeAVCDecConfRec - decode an AV1DecConfRec
func DecodeAV1DecConfRec(data []byte) (DecConfRec, error) {
	av1drc := DecConfRec{}

	av1drc.Marker = data[0] >> 7
	av1drc.Version = data[0] & 0x7F
	av1drc.SeqProfile = data[1] >> 5
	av1drc.SeqLevelIdx0 = data[1] & 0x1F
	av1drc.SeqTier0 = data[2] >> 7
	av1drc.HighBitdepth = (data[2] >> 6) & 0x01
	av1drc.TwelveBit = (data[2] >> 5) & 0x01
	av1drc.MonoChrome = (data[2] >> 4) & 0x01
	av1drc.ChromaSubsamplingX = (data[2] >> 3) & 0x01
	av1drc.ChromaSubsamplingY = (data[2] >> 2) & 0x01
	av1drc.ChromaSamplePosition = data[2] & 0x03
	av1drc.Reserved = data[3] >> 5
	av1drc.InitialPresentationDelayPresent = (data[3] >> 4) & 0x01
	if av1drc.InitialPresentationDelayPresent == 1 {
		av1drc.InitialPresentationDelayMinusOne = data[3] & 0x0F
	} else {
		av1drc.InitialPresentationDelayMinusOne = 0
	}
	if len(data) > 4 {
		av1drc.ConfigOBUs = data[4:]
	}

	return av1drc, nil
}

// Size - total size in bytes
func (a *DecConfRec) Size() uint64 {
	return uint64(4 + len(a.ConfigOBUs))
}

// EncodeSW- write an AV1DecConfRec to w
func (a *DecConfRec) Encode(w io.Writer) error {
	sw := bits.NewFixedSliceWriter(int(a.Size()))
	err := a.EncodeSW(sw)
	if err != nil {
		return err
	}
	_, err = w.Write(sw.Bytes())
	return err
}

// EncodeSW- write an AV1DecConfRec to sw
func (a *DecConfRec) EncodeSW(sw bits.SliceWriter) error {
	sw.WriteBits(uint(a.Marker), 1)
	sw.WriteBits(uint(a.Version), 7)
	sw.WriteBits(uint(a.SeqProfile), 3)
	sw.WriteBits(uint(a.SeqLevelIdx0), 5)
	sw.WriteBits(uint(a.SeqTier0), 1)
	sw.WriteBits(uint(a.HighBitdepth), 1)
	sw.WriteBits(uint(a.TwelveBit), 1)
	sw.WriteBits(uint(a.MonoChrome), 1)
	sw.WriteBits(uint(a.ChromaSubsamplingX), 1)
	sw.WriteBits(uint(a.ChromaSubsamplingY), 1)
	sw.WriteBits(uint(a.ChromaSamplePosition), 2)
	sw.WriteBits(0, 3)
	sw.WriteBits(uint(a.InitialPresentationDelayPresent), 1)
	if a.InitialPresentationDelayPresent == 1 {
		sw.WriteBits(uint(a.InitialPresentationDelayMinusOne), 4)
	} else {
		sw.WriteBits(0, 4)
	}
	if len(a.ConfigOBUs) != 0 {
		sw.WriteBytes(a.ConfigOBUs)
	}

	return sw.AccError()
}
