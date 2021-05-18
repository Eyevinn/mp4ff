package mp4

import (
	"io"
	"io/ioutil"
)

// CslgBox - CompositionToDecodeBox -ISO/IEC 14496-12 2015 Sec. 8.6.1.4
//
// Contained in: Sample Table Box (stbl) or Track Extension Properties Box (trep)
type CslgBox struct {
	Version                      byte
	Flags                        uint32
	CompositionToDTSShift        int64
	LeastDecodeToDisplayDelta    int64
	GreatestDecodeToDisplayDelta int64
	CompositionStartTime         int64
	CompositionEndTime           int64
}

// DecodeCslg - box-specific decode
func DecodeCslg(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	s := NewSliceReader(data)
	versionAndFlags := s.ReadUint32()
	b := CslgBox{
		Version: byte(versionAndFlags >> 24),
		Flags:   versionAndFlags & flagsMask,
	}
	if b.Version == 0 {
		b.CompositionToDTSShift = int64(s.ReadInt32())
		b.LeastDecodeToDisplayDelta = int64(s.ReadInt32())
		b.GreatestDecodeToDisplayDelta = int64(s.ReadInt32())
		b.CompositionStartTime = int64(s.ReadInt32())
		b.CompositionEndTime = int64(s.ReadInt32())
	} else {
		b.CompositionToDTSShift = s.ReadInt64()
		b.LeastDecodeToDisplayDelta = s.ReadInt64()
		b.GreatestDecodeToDisplayDelta = s.ReadInt64()
		b.CompositionStartTime = s.ReadInt64()
		b.CompositionEndTime = s.ReadInt64()
	}
	return &b, nil
}

// Type - box type
func (b *CslgBox) Type() string {
	return "cslg"
}

// Size - calculated size of box
func (b *CslgBox) Size() uint64 {
	// full Box + 5 * 4 + version * 5*4
	return uint64(boxHeaderSize + 4 + 20 + 20*b.Version)
}

// Encode - write box to w
func (b *CslgBox) Encode(w io.Writer) error {
	err := EncodeHeader(b, w)
	if err != nil {
		return err
	}
	buf := makebuf(b)
	sw := NewSliceWriter(buf)
	versionAndFlags := (uint32(b.Version) << 24) + b.Flags
	sw.WriteUint32(versionAndFlags)
	if b.Version == 0 {
		sw.WriteInt32(int32(b.CompositionToDTSShift))
		sw.WriteInt32(int32(b.LeastDecodeToDisplayDelta))
		sw.WriteInt32(int32(b.GreatestDecodeToDisplayDelta))
		sw.WriteInt32(int32(b.CompositionStartTime))
		sw.WriteInt32(int32(b.CompositionEndTime))
	} else {
		sw.WriteInt64(b.CompositionToDTSShift)
		sw.WriteInt64(b.LeastDecodeToDisplayDelta)
		sw.WriteInt64(b.GreatestDecodeToDisplayDelta)
		sw.WriteInt64(b.CompositionStartTime)
		sw.WriteInt64(b.CompositionEndTime)
	}
	_, err = w.Write(buf)
	return err
}

// Info - get details with specificBoxLevels cslg:1 or higher
func (b *CslgBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, int(b.Version), b.Flags)
	if getInfoLevel(b, specificBoxLevels) > 0 {
		bd.write(" - compositionToDTSShift: %d", b.CompositionToDTSShift)
		bd.write(" - leastDecodeToDisplayDelta: %d", b.LeastDecodeToDisplayDelta)
		bd.write(" - greatestDecodeToDisplayDelta: %d", b.GreatestDecodeToDisplayDelta)
		bd.write(" - compositionStartTime: %d", b.CompositionStartTime)
		bd.write(" - compositionEndTime: %d", b.CompositionEndTime)
	}
	return bd.err
}
