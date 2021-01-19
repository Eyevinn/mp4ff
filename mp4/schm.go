package mp4

import (
	"io"
	"io/ioutil"
)

// SchmBox - Scheme Type Box
type SchmBox struct {
	Version       byte
	Flags         uint32
	SchemeType    string // 4CC represented as uint32
	SchemeVersion uint32
	SchemeUri     string // Absolute null-terminated URL
}

// DecodeSchm - box-specific decode
func DecodeSchm(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	s := NewSliceReader(data)
	versionAndFlags := s.ReadUint32()
	version := byte(versionAndFlags >> 24)

	b := &SchmBox{
		Version: version,
		Flags:   versionAndFlags & flagsMask,
	}
	b.SchemeType = s.ReadFixedLengthString(4)
	b.SchemeVersion = s.ReadUint32()
	if b.Flags&0x01 != 0 {
		b.SchemeUri, err = s.ReadZeroTerminatedString()
		if err != nil {
			return nil, err
		}
	}
	return b, nil
}

// Type - return box type
func (b *SchmBox) Type() string {
	return "schm"
}

// Size - return calculated size
func (b *SchmBox) Size() uint64 {
	size := uint64(20)
	if b.Flags&0x01 != 0 {
		size += uint64(len(b.SchemeUri) + 1)
	}
	return size
}

// Encode - write box to w
func (b *SchmBox) Encode(w io.Writer) error {
	err := EncodeHeader(b, w)
	if err != nil {
		return err
	}
	buf := makebuf(b)
	sw := NewSliceWriter(buf)
	versionAndFlags := (uint32(b.Version) << 24) + b.Flags
	sw.WriteUint32(versionAndFlags)
	sw.WriteString(b.SchemeType, false)
	sw.WriteUint32(b.SchemeVersion)
	if b.Flags&0x01 != 0 {
		sw.WriteString(b.SchemeUri, true)
	}
	_, err = w.Write(buf)
	return err
}

// Info - write box info to w
func (b *SchmBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) (err error) {
	bd := newInfoDumper(w, indent, b, int(b.Version), b.Flags)
	bd.write(" - schemeType: %s", b.SchemeType)
	bd.write(" - schemeVersion: %d", b.SchemeVersion)
	if b.Flags&0x01 != 0 {
		bd.write(" - schemeURI: %q", b.SchemeUri)
	}
	return bd.err
}
