package mp4

import (
	"io"

	"github.com/Eyevinn/mp4ff/bits"
)

// DopsBox - Opus Specific Box (dOps)
// Following https://opus-codec.org/docs/opus_in_isobmff.html
type DopsBox struct {
	Version              byte
	OutputChannelCount   byte
	PreSkip              uint16
	InputSampleRate      uint32
	OutputGain           int16
	ChannelMappingFamily byte
	StreamCount          byte
	CoupledCount         byte
	ChannelMapping       []byte
}

// DecodeDops - box-specific decode
func DecodeDops(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := readBoxBody(r, hdr)
	if err != nil {
		return nil, err
	}
	sr := bits.NewFixedSliceReader(data)
	return DecodeDopsSR(hdr, startPos, sr)
}

// DecodeDopsSR - box-specific decode
func DecodeDopsSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	d := &DopsBox{}
	d.Version = sr.ReadUint8()
	d.OutputChannelCount = sr.ReadUint8()
	d.PreSkip = sr.ReadUint16()
	d.InputSampleRate = sr.ReadUint32()
	d.OutputGain = sr.ReadInt16()
	d.ChannelMappingFamily = sr.ReadUint8()

	if d.ChannelMappingFamily != 0 {
		d.StreamCount = sr.ReadUint8()
		d.CoupledCount = sr.ReadUint8()
		d.ChannelMapping = sr.ReadBytes(int(d.OutputChannelCount))
	}

	return d, sr.AccError()
}

// Type - return box type
func (d *DopsBox) Type() string {
	return "dOps"
}

// Size - return calculated size
func (d *DopsBox) Size() uint64 {
	size := uint64(boxHeaderSize + 11) // Version + OutputChannelCount + PreSkip + InputSampleRate + OutputGain + ChannelMappingFamily
	if d.ChannelMappingFamily != 0 {
		size += 2                             // StreamCount + CoupledCount
		size += uint64(len(d.ChannelMapping)) // ChannelMapping
	}
	return size
}

// Encode - write box to w
func (d *DopsBox) Encode(w io.Writer) error {
	sw := bits.NewFixedSliceWriter(int(d.Size()))
	err := d.EncodeSW(sw)
	if err != nil {
		return err
	}
	_, err = w.Write(sw.Bytes())
	return err
}

// EncodeSW - write box to sw
func (d *DopsBox) EncodeSW(sw bits.SliceWriter) error {
	err := EncodeHeaderSW(d, sw)
	if err != nil {
		return err
	}
	sw.WriteUint8(d.Version)
	sw.WriteUint8(d.OutputChannelCount)
	sw.WriteUint16(d.PreSkip)
	sw.WriteUint32(d.InputSampleRate)
	sw.WriteInt16(d.OutputGain)
	sw.WriteUint8(d.ChannelMappingFamily)

	if d.ChannelMappingFamily != 0 {
		sw.WriteUint8(d.StreamCount)
		sw.WriteUint8(d.CoupledCount)
		sw.WriteBytes(d.ChannelMapping)
	}

	return sw.AccError()
}

// Info - write box info to w
func (d *DopsBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, d, -1, 0)
	bd.write(" - Version: %d", d.Version)
	bd.write(" - OutputChannelCount: %d", d.OutputChannelCount)
	bd.write(" - PreSkip: %d", d.PreSkip)
	bd.write(" - InputSampleRate: %d", d.InputSampleRate)
	bd.write(" - OutputGain: %d", d.OutputGain)
	bd.write(" - ChannelMappingFamily: %d", d.ChannelMappingFamily)

	if d.ChannelMappingFamily != 0 {
		bd.write(" - StreamCount: %d", d.StreamCount)
		bd.write(" - CoupledCount: %d", d.CoupledCount)
		bd.write(" - ChannelMapping: %v", d.ChannelMapping)
	}

	return bd.err
}
