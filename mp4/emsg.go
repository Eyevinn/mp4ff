package mp4

import (
	"fmt"
	"io"
	"io/ioutil"
)

// EmsgBox - DASHEventMessageBox as defined in ISO/IEC 23009-1
type EmsgBox struct {
	Version               byte
	Flags                 uint32
	TimeScale             uint32
	PresentationTimeDelta uint32
	PresentationTime      uint64
	EventDuration         uint32
	Id                    uint32
	SchemeIdURI           string
	Value                 string
}

// DecodeEmsg - box-specific decode
func DecodeEmsg(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	s := NewSliceReader(data)
	versionAndFlags := s.ReadUint32()
	version := byte(versionAndFlags >> 24)
	b := &EmsgBox{
		Version: version,
		Flags:   versionAndFlags & flagsMask,
	}

	if version == 1 {
		b.TimeScale = s.ReadUint32()
		b.PresentationTime = s.ReadUint64()
		b.EventDuration = s.ReadUint32()
		b.Id = s.ReadUint32()
		b.SchemeIdURI, err = s.ReadZeroTerminatedString()
		if err != nil {
			return nil, fmt.Errorf("Read schemedIDUri error in emsg")
		}
		b.Value, err = s.ReadZeroTerminatedString()
		if err != nil {
			return nil, fmt.Errorf("Read schemedIDUri error in emsg")
		}
	} else if version == 0 {
		b.SchemeIdURI, err = s.ReadZeroTerminatedString()
		if err != nil {
			return nil, fmt.Errorf("Read schemedIDUri error in emsg")
		}
		b.Value, err = s.ReadZeroTerminatedString()
		if err != nil {
			return nil, fmt.Errorf("Read schemedIDUri error in emsg")
		}
		b.TimeScale = s.ReadUint32()
		b.PresentationTimeDelta = s.ReadUint32()
		b.EventDuration = s.ReadUint32()
		b.Id = s.ReadUint32()
	} else {
		return nil, fmt.Errorf("Unknown version for emsg")
	}
	return b, nil
}

// Type - box type
func (b *EmsgBox) Type() string {
	return "emsg"
}

// Size - calculated size of box
func (b *EmsgBox) Size() uint64 {
	if b.Version == 1 {
		return uint64(boxHeaderSize + 4 + 4 + 8 + 4 + 4 + len(b.SchemeIdURI) + 1 + len(b.Value) + 1)
	}
	return uint64(boxHeaderSize + 4 + len(b.SchemeIdURI) + 1 + len(b.Value) + 1 + 4 + 4 + 4 + 4) // m.Version == 0
}

// Encode - write box to w
func (b *EmsgBox) Encode(w io.Writer) error {
	err := EncodeHeader(b, w)
	if err != nil {
		return err
	}
	buf := make([]byte, b.Size()-boxHeaderSize)
	sw := NewSliceWriter(buf)
	versionAndFlags := (uint32(b.Version) << 24) + b.Flags
	sw.WriteUint32(versionAndFlags)
	if b.Version == 1 {
		sw.WriteUint32(b.TimeScale)
		sw.WriteUint64(b.PresentationTime)
		sw.WriteUint32(b.EventDuration)
		sw.WriteUint32(b.Id)
		sw.WriteString(b.SchemeIdURI, true)
		sw.WriteString(b.Value, true)
	} else {
		sw.WriteString(b.SchemeIdURI, true)
		sw.WriteString(b.Value, true)
		sw.WriteUint32(b.TimeScale)
		sw.WriteUint32(b.PresentationTimeDelta)
		sw.WriteUint32(b.EventDuration)
		sw.WriteUint32(b.Id)
	}

	_, err = w.Write(buf)
	return err
}

func (b *EmsgBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, int(b.Version), b.Flags)
	bd.write(" - timeScale: %d", b.TimeScale)
	if b.Version > 0 {
		bd.write(" - presentationTime: %d", b.PresentationTime)
	}
	bd.write(" - eventDuration: %d", b.EventDuration)
	bd.write(" - id: %d", b.Id)
	bd.write(" - schedIdURI: %s", b.SchemeIdURI)
	bd.write(" - value: %s", b.Value)
	if b.Version == 0 {
		bd.write(" - presentationTimeDelta: %d", b.PresentationTimeDelta)
	}
	return bd.err
}
