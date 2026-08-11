package mp4

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"

	"github.com/Eyevinn/mp4ff/bits"
)

// AudioSampleEntryBox according to ISO/IEC 14496-12.
// QuickTime overlays its sound sample description version, revision level,
// and vendor on the first ISO reserved bytes; all three are preserved on
// encode. Versions 1 and 2 insert extra fields before the child boxes,
// carried in QuickTimeV1 and QuickTimeV2 — those pointers (not the number)
// decide the encoded layout, and encoding fails when they disagree with
// QuickTimeVersion.
type AudioSampleEntryBox struct {
	name                   string
	DataReferenceIndex     uint16
	QuickTimeVersion       uint16
	QuickTimeRevisionLevel uint16
	QuickTimeVendor        uint32
	ChannelCount           uint16
	SampleSize             uint16
	// CompressionID is pre_defined (0) in ISO; QuickTime writes -2 in
	// version 2 entries and in version 1 entries with redefined (VBR)
	// sample tables.
	CompressionID int16
	SampleRate    uint16 // Integer part
	QuickTimeV1   *QuickTimeV1SoundDescription
	QuickTimeV2   *QuickTimeV2SoundDescription
	Esds          *EsdsBox
	Dac3          *Dac3Box
	Dac4          *Dac4Box
	Dec3          *Dec3Box
	DfLa          *DfLaBox
	Dops          *DopsBox
	Iacb          *IacbBox
	MhaC          *MhaCBox
	Btrt          *BtrtBox
	Sinf          *SinfBox
	Wave          *WaveBox
	Children      []Box
}

// QuickTimeV1SoundDescription - the 16 bytes that a QuickTime sound sample
// description version 1 inserts before the child boxes
// (QuickTime File Format Specification, Sound Sample Description v1).
type QuickTimeV1SoundDescription struct {
	SamplesPerPacket uint32
	BytesPerPacket   uint32
	BytesPerFrame    uint32
	BytesPerSample   uint32
}

// QuickTimeV2Marker - the defined value of the always7F000000 field in a
// QuickTime version 2 sound sample description.
const QuickTimeV2Marker = 0x7f000000

// QuickTimeV2SoundDescription - the 36 bytes that a QuickTime sound sample
// description version 2 inserts before the child boxes
// (QuickTime File Format Specification, Sound Sample Description v2).
// In a version 2 entry the fixed fields hold the constants 3 (channel count),
// 16 (sample size), and 1.0 (the raw 16.16 sample rate value 0x00010000);
// the actual values live here. Child boxes are expected directly after the
// 36 bytes; SizeOfStructOnly (72 for this layout) is preserved but not used
// to place them.
type QuickTimeV2SoundDescription struct {
	SizeOfStructOnly              uint32
	AudioSampleRate               float64
	NumAudioChannels              uint32
	Always7F000000                uint32
	ConstBitsPerChannel           uint32
	FormatSpecificFlags           uint32
	ConstBytesPerAudioPacket      uint32
	ConstLPCMFramesPerAudioPacket uint32
}

// NewAudioSampleEntryBox - Create new empty mp4a box
func NewAudioSampleEntryBox(name string) *AudioSampleEntryBox {
	return &AudioSampleEntryBox{name: name, DataReferenceIndex: 1}
}

func makeFixed32Uint(nr uint16) uint32 {
	return uint32(nr) << 16
}

func makeUint16FromFixed32(nr uint32) uint16 {
	return uint16(nr >> 16)
}

// CreateAudioSampleEntryBox - Create new AudioSampleEntry such as mp4
func CreateAudioSampleEntryBox(name string, nrChannels, sampleSize, sampleRate uint16, child Box) *AudioSampleEntryBox {
	a := &AudioSampleEntryBox{
		name:               name,
		DataReferenceIndex: 1,
		ChannelCount:       nrChannels,
		SampleSize:         sampleSize,
		SampleRate:         sampleRate,
		Children:           nil,
	}
	if child != nil {
		a.AddChild(child)
	}
	return a
}

// AddChild - add a child box (avcC normally, but clap and pasp could be part of visual entry)
func (a *AudioSampleEntryBox) AddChild(child Box) {
	switch child.Type() {
	case "esds":
		a.Esds = child.(*EsdsBox)
	case "dac3":
		a.Dac3 = child.(*Dac3Box)
	case "dac4":
		a.Dac4 = child.(*Dac4Box)
	case "dec3":
		a.Dec3 = child.(*Dec3Box)
	case "dfLa":
		a.DfLa = child.(*DfLaBox)
	case "dOps":
		a.Dops = child.(*DopsBox)
	case "iacb":
		a.Iacb = child.(*IacbBox)
	case "mhaC":
		a.MhaC = child.(*MhaCBox)
	case "btrt":
		a.Btrt = child.(*BtrtBox)
	case "sinf":
		a.Sinf = child.(*SinfBox)
	case "wave":
		a.Wave = child.(*WaveBox)
	}

	a.Children = append(a.Children, child)
}

const nrAudioSampleBytesBeforeChildren = 36

// preChildrenSize - number of bytes before the child boxes, including
// the box header and any QuickTime version 1 or 2 extension.
func (a *AudioSampleEntryBox) preChildrenSize() uint64 {
	switch {
	case a.QuickTimeV1 != nil:
		return nrAudioSampleBytesBeforeChildren + 16
	case a.QuickTimeV2 != nil:
		return nrAudioSampleBytesBeforeChildren + 36
	default:
		return nrAudioSampleBytesBeforeChildren
	}
}

// decodeAudioSampleEntryFromData decodes the entry body. The child boxes are
// first tried at the offset the QuickTime version dictates; when that fails,
// the entry is re-read with the plain ISO layout (children directly after the
// fixed fields), so files whose reserved bytes merely look like a QuickTime
// version keep decoding as before. When both layouts fail, the error of the
// claimed (QuickTime) layout is reported.
func decodeAudioSampleEntryFromData(hdr BoxHeader, startPos uint64, data []byte) (*AudioSampleEntryBox, error) {
	a, qtErr := decodeAudioSampleEntryLayout(hdr, startPos, data, true)
	if qtErr == nil {
		return a, nil
	}
	if len(data) >= 10 && binary.BigEndian.Uint16(data[8:10]) != 0 {
		if a, err := decodeAudioSampleEntryLayout(hdr, startPos, data, false); err == nil {
			return a, nil
		}
	}
	return nil, qtErr
}

func decodeAudioSampleEntryLayout(hdr BoxHeader, startPos uint64, data []byte,
	useQuickTimeLayout bool) (*AudioSampleEntryBox, error) {
	sr := bits.NewFixedSliceReader(data)
	a := NewAudioSampleEntryBox(hdr.Name)

	// 14496-12 8.5.2.2 Sample entry (8 bytes)
	sr.SkipBytes(6) // Skip 6 reserved bytes
	a.DataReferenceIndex = sr.ReadUint16()

	// 14496-12 12.2.3.2 Audio Sample entry (20 bytes).
	// QuickTime overlays version + revision + vendor on the first 8 reserved bytes.
	a.QuickTimeVersion = sr.ReadUint16()
	a.QuickTimeRevisionLevel = sr.ReadUint16()
	a.QuickTimeVendor = sr.ReadUint32()
	a.ChannelCount = sr.ReadUint16()
	a.SampleSize = sr.ReadUint16()
	a.CompressionID = int16(sr.ReadUint16())
	sr.SkipBytes(2) // packet size / reserved
	a.SampleRate = makeUint16FromFixed32(sr.ReadUint32())

	if useQuickTimeLayout {
		switch a.QuickTimeVersion {
		case 1:
			a.QuickTimeV1 = &QuickTimeV1SoundDescription{
				SamplesPerPacket: sr.ReadUint32(),
				BytesPerPacket:   sr.ReadUint32(),
				BytesPerFrame:    sr.ReadUint32(),
				BytesPerSample:   sr.ReadUint32(),
			}
		case 2:
			q := &QuickTimeV2SoundDescription{
				SizeOfStructOnly:              sr.ReadUint32(),
				AudioSampleRate:               math.Float64frombits(sr.ReadUint64()),
				NumAudioChannels:              sr.ReadUint32(),
				Always7F000000:                sr.ReadUint32(),
				ConstBitsPerChannel:           sr.ReadUint32(),
				FormatSpecificFlags:           sr.ReadUint32(),
				ConstBytesPerAudioPacket:      sr.ReadUint32(),
				ConstLPCMFramesPerAudioPacket: sr.ReadUint32(),
			}
			if sr.AccError() == nil && q.Always7F000000 != QuickTimeV2Marker {
				return nil, fmt.Errorf("version 2 sound description has %#x instead of the always7F000000 marker",
					q.Always7F000000)
			}
			a.QuickTimeV2 = q
		}
	}
	if sr.AccError() != nil {
		return nil, sr.AccError()
	}

	pos := startPos + a.preChildrenSize() // Size of all previous data
	lastPos := startPos + hdr.Size
	for pos < lastPos {
		box, err := DecodeBoxSR(pos, sr)
		if err != nil {
			return nil, err
		}
		if box != nil {
			a.AddChild(box)
			pos += box.Size()
		}
		if pos > lastPos {
			return nil, fmt.Errorf("bad size when decoding %s", hdr.Name)
		}
	}
	return a, sr.AccError()
}

// DecodeAudioSampleEntry - decode mp4a... box
func DecodeAudioSampleEntry(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := readBoxBody(r, hdr)
	if err != nil {
		return nil, err
	}
	return decodeAudioSampleEntryFromData(hdr, startPos, data)
}

// DecodeAudioSampleEntrySR - decode mp4a... box
func DecodeAudioSampleEntrySR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	data := sr.ReadBytes(hdr.payloadLen())
	if sr.AccError() != nil {
		return nil, sr.AccError()
	}
	return decodeAudioSampleEntryFromData(hdr, startPos, data)
}

// Type - return box type
func (a *AudioSampleEntryBox) Type() string {
	return a.name
}

// SetType sets the type (name) of the box
func (a *AudioSampleEntryBox) SetType(name string) {
	a.name = name
}

// Size - return calculated size
func (a *AudioSampleEntryBox) Size() uint64 {
	totalSize := a.preChildrenSize()
	for _, child := range a.Children {
		totalSize += child.Size()
	}
	return totalSize
}

// validateQuickTime checks that the QuickTime version halfword agrees with
// the version-specific extension pointers before encoding. A mismatch would
// produce an entry that this package itself cannot decode. A nonzero
// QuickTimeVersion without a pointer stays valid: that is how entries whose
// reserved bytes merely look like a version are preserved.
func (a *AudioSampleEntryBox) validateQuickTime() error {
	switch {
	case a.QuickTimeV1 != nil && a.QuickTimeV2 != nil:
		return fmt.Errorf("both QuickTimeV1 and QuickTimeV2 are set")
	case a.QuickTimeV1 != nil && a.QuickTimeVersion != 1:
		return fmt.Errorf("QuickTimeV1 is set, but QuickTimeVersion is %d, not 1", a.QuickTimeVersion)
	case a.QuickTimeV2 != nil && a.QuickTimeVersion != 2:
		return fmt.Errorf("QuickTimeV2 is set, but QuickTimeVersion is %d, not 2", a.QuickTimeVersion)
	case a.QuickTimeV2 != nil && a.QuickTimeV2.Always7F000000 != QuickTimeV2Marker:
		return fmt.Errorf("QuickTimeV2.Always7F000000 is %#x, not the defined %#x",
			a.QuickTimeV2.Always7F000000, uint32(QuickTimeV2Marker))
	}
	return nil
}

// encodeAudioSampleEntryStart - encode the fields before the child boxes.
func (a *AudioSampleEntryBox) encodeAudioSampleEntryStart(sw bits.SliceWriter) {
	sw.WriteZeroBytes(6)
	sw.WriteUint16(a.DataReferenceIndex)
	sw.WriteUint16(a.QuickTimeVersion)
	sw.WriteUint16(a.QuickTimeRevisionLevel)
	sw.WriteUint32(a.QuickTimeVendor)
	sw.WriteUint16(a.ChannelCount)
	sw.WriteUint16(a.SampleSize)
	sw.WriteUint16(uint16(a.CompressionID))
	sw.WriteZeroBytes(2)                          // packet size / reserved
	sw.WriteUint32(makeFixed32Uint(a.SampleRate)) // nrAudioSampleBytesBeforeChildren bytes this far

	switch {
	case a.QuickTimeV1 != nil:
		q := a.QuickTimeV1
		sw.WriteUint32(q.SamplesPerPacket)
		sw.WriteUint32(q.BytesPerPacket)
		sw.WriteUint32(q.BytesPerFrame)
		sw.WriteUint32(q.BytesPerSample)
	case a.QuickTimeV2 != nil:
		q := a.QuickTimeV2
		sw.WriteUint32(q.SizeOfStructOnly)
		sw.WriteUint64(math.Float64bits(q.AudioSampleRate))
		sw.WriteUint32(q.NumAudioChannels)
		sw.WriteUint32(q.Always7F000000)
		sw.WriteUint32(q.ConstBitsPerChannel)
		sw.WriteUint32(q.FormatSpecificFlags)
		sw.WriteUint32(q.ConstBytesPerAudioPacket)
		sw.WriteUint32(q.ConstLPCMFramesPerAudioPacket)
	}
}

// Encode - write box to w
func (a *AudioSampleEntryBox) Encode(w io.Writer) error {
	if err := a.validateQuickTime(); err != nil {
		return err
	}
	err := EncodeHeader(a, w)
	if err != nil {
		return err
	}
	buf := makebuf(a)
	sw := bits.NewFixedSliceWriterFromSlice(buf)
	a.encodeAudioSampleEntryStart(sw)

	_, err = w.Write(buf[:sw.Offset()]) // Only write written bytes
	if err != nil {
		return err
	}

	// Next output child boxes in order
	for _, child := range a.Children {
		err = child.Encode(w)
		if err != nil {
			return err
		}
	}
	return err
}

// Encode - write box to sw
func (a *AudioSampleEntryBox) EncodeSW(sw bits.SliceWriter) error {
	if err := a.validateQuickTime(); err != nil {
		return err
	}
	err := EncodeHeaderSW(a, sw)
	if err != nil {
		return err
	}
	a.encodeAudioSampleEntryStart(sw)

	// Next output child boxes in order
	for _, child := range a.Children {
		err = child.EncodeSW(sw)
		if err != nil {
			return err
		}
	}
	return err
}

// Info - write box info to w
func (a *AudioSampleEntryBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, a, -1, 0)
	if bd.err != nil {
		return bd.err
	}
	bd.write(" - data_reference_index: %d", a.DataReferenceIndex)
	if a.QuickTimeVersion != 0 {
		bd.write(" - quicktime_version: %d", a.QuickTimeVersion)
	}
	if a.QuickTimeRevisionLevel != 0 {
		bd.write(" - quicktime_revision_level: %d", a.QuickTimeRevisionLevel)
	}
	if a.QuickTimeVendor != 0 {
		bd.write(" - quicktime_vendor: %#08x", a.QuickTimeVendor)
	}
	if q := a.QuickTimeV2; q != nil {
		// The fixed fields of a version 2 entry hold mandated constants
		// (3, 16, -2, 1.0); the true values live in the version 2 struct.
		bd.write(" - channel_count: %d", q.NumAudioChannels)
		if q.ConstBitsPerChannel != 0 {
			bd.write(" - sample_size: %d", q.ConstBitsPerChannel)
		} else {
			bd.write(" - sample_size: %d", a.SampleSize)
		}
		bd.write(" - sample_rate: %g", q.AudioSampleRate)
		bd.write(" - size_of_struct_only: %d, always_7f000000: %#x, format_specific_flags: %#x",
			q.SizeOfStructOnly, q.Always7F000000, q.FormatSpecificFlags)
		bd.write(" - const_bytes_per_audio_packet: %d, const_lpcm_frames_per_audio_packet: %d",
			q.ConstBytesPerAudioPacket, q.ConstLPCMFramesPerAudioPacket)
	} else {
		bd.write(" - channel_count: %d", a.ChannelCount)
		bd.write(" - sample_size: %d", a.SampleSize)
		if a.CompressionID != 0 {
			bd.write(" - compression_id: %d", a.CompressionID)
		}
		bd.write(" - sample_rate: %d", a.SampleRate)
		if q := a.QuickTimeV1; q != nil {
			bd.write(" - samples_per_packet: %d, bytes_per_packet: %d, bytes_per_frame: %d, bytes_per_sample: %d",
				q.SamplesPerPacket, q.BytesPerPacket, q.BytesPerFrame, q.BytesPerSample)
		}
	}
	var err error
	for _, child := range a.Children {
		err = child.Info(w, specificBoxLevels, indent+indentStep, indentStep)
		if err != nil {
			return err
		}
	}
	return nil
}

// RemoveEncryption - remove sinf box and set type to unencrypted type
func (a *AudioSampleEntryBox) RemoveEncryption() (*SinfBox, error) {
	if a.name != "enca" {
		return nil, fmt.Errorf("is not encrypted: %s", a.name)
	}
	sinf := a.Sinf
	if sinf == nil {
		return nil, fmt.Errorf("does not have sinf box")
	}
	for i := range a.Children {
		if a.Children[i].Type() == "sinf" {
			a.Children = append(a.Children[:i], a.Children[i+1:]...)
			a.Sinf = nil
			break
		}
	}
	a.name = sinf.Frma.DataFormat
	return sinf, nil
}
