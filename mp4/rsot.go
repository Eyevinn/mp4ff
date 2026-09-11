package mp4

import (
	"fmt"
	"io"

	"github.com/Eyevinn/mp4ff/bits"
)

// Flags of the RsotBox. The names follow ISO/IEC 14496-12 Section 8.8.18.3,
// where they are called rsot_original_duration and rsot_elapsed_duration.
const (
	RsotOriginalDurationPresentFlag uint32 = 0x000001
	RsotElapsedDurationPresentFlag  uint32 = 0x000002
)

// RsotBox - Redundant Sample Original Timing Box (rsot)
//
// Contained in : Track Fragment Box (traf)
//
// Defined in ISO/IEC 14496-12:2026 Section 8.8.18. It documents that the first sample of
// the track fragment is a copy of the previous sample, and how long that sample has
// already been presented (ElapsedDuration), and that the last sample of the fragment was
// truncated to fit the fragment, and how long it was meant to last (OriginalDuration).
//
// When ElapsedDuration is signalled, the first sample of the fragment shall have
// sample_depends_on = 2 and sample_has_redundancy = 1 in its sample flags. A receiver that
// already has the previous sample extends that sample by the duration of this one and
// ignores ElapsedDuration; one that tunes in here instead presents the sample at its
// decode time as if it had already run for ElapsedDuration.
type RsotBox struct {
	Version          byte
	Flags            uint32
	OriginalDuration uint32
	ElapsedDuration  uint32
}

// CreateRsotBox creates a new RsotBox with the durations that are non-zero, and sets the
// corresponding flags. The standard requires a signalled duration to be non-zero, so a
// zero value means that the duration is not signalled. Both zero gives an empty box.
func CreateRsotBox(originalDuration, elapsedDuration uint32) *RsotBox {
	b := RsotBox{}
	if originalDuration != 0 {
		b.Flags |= RsotOriginalDurationPresentFlag
		b.OriginalDuration = originalDuration
	}
	if elapsedDuration != 0 {
		b.Flags |= RsotElapsedDurationPresentFlag
		b.ElapsedDuration = elapsedDuration
	}
	return &b
}

// DecodeRsot - box-specific decode
func DecodeRsot(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := readBoxBody(r, hdr)
	if err != nil {
		return nil, err
	}
	sr := bits.NewFixedSliceReader(data)
	return DecodeRsotSR(hdr, startPos, sr)
}

// DecodeRsotSR - box-specific decode
func DecodeRsotSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	versionAndFlags := sr.ReadUint32()
	b := RsotBox{
		Version: byte(versionAndFlags >> 24),
		Flags:   versionAndFlags & flagsMask,
	}
	// The size follows from the flags, so a box of any other size is malformed
	// rather than something to be read as far as it goes.
	if hdr.Hdrlen != boxHeaderSize || hdr.Size != b.Size() {
		return nil, fmt.Errorf("invalid box size %d", hdr.Size)
	}
	if b.HasOriginalDuration() {
		b.OriginalDuration = sr.ReadUint32()
	}
	if b.HasElapsedDuration() {
		b.ElapsedDuration = sr.ReadUint32()
	}
	return &b, sr.AccError()
}

// HasOriginalDuration - interpreted flags value
func (b *RsotBox) HasOriginalDuration() bool {
	return b.Flags&RsotOriginalDurationPresentFlag != 0
}

// HasElapsedDuration - interpreted flags value
func (b *RsotBox) HasElapsedDuration() bool {
	return b.Flags&RsotElapsedDurationPresentFlag != 0
}

// Type - return box type
func (b *RsotBox) Type() string {
	return "rsot"
}

// Size - return calculated size
func (b *RsotBox) Size() uint64 {
	size := uint64(boxHeaderSize + 4)
	if b.HasOriginalDuration() {
		size += 4
	}
	if b.HasElapsedDuration() {
		size += 4
	}
	return size
}

// Encode - write box to w
func (b *RsotBox) Encode(w io.Writer) error {
	sw := bits.NewFixedSliceWriter(int(b.Size()))
	err := b.EncodeSW(sw)
	if err != nil {
		return err
	}
	_, err = w.Write(sw.Bytes())
	return err
}

// EncodeSW - box-specific encode to slicewriter
func (b *RsotBox) EncodeSW(sw bits.SliceWriter) error {
	err := EncodeHeaderSW(b, sw)
	if err != nil {
		return err
	}
	versionAndFlags := (uint32(b.Version) << 24) + b.Flags
	sw.WriteUint32(versionAndFlags)
	if b.HasOriginalDuration() {
		sw.WriteUint32(b.OriginalDuration)
	}
	if b.HasElapsedDuration() {
		sw.WriteUint32(b.ElapsedDuration)
	}
	return sw.AccError()
}

// Info - write box-specific information
func (b *RsotBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, int(b.Version), b.Flags)
	if b.HasOriginalDuration() {
		bd.write(" - originalDuration: %d", b.OriginalDuration)
	}
	if b.HasElapsedDuration() {
		bd.write(" - elapsedDuration: %d", b.ElapsedDuration)
	}
	return bd.err
}
