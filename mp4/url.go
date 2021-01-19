package mp4

import (
	"io"
	"io/ioutil"
)

// URLBox - DataEntryUrlBox ('url ')
//
// Contained in : DrefBox (dref
type URLBox struct {
	Version  byte
	Flags    uint32
	Location string // Zero-terminated string
}

const dataIsSelfContainedFlag = 0x000001

// DecodeURLBox - box-specific decode
func DecodeURLBox(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	s := NewSliceReader(data)
	versionAndFlags := s.ReadUint32()
	version := byte(versionAndFlags >> 24)
	flags := versionAndFlags & flagsMask
	location := ""
	if flags != dataIsSelfContainedFlag {
		location, err = s.ReadZeroTerminatedString()
	}

	u := &URLBox{
		Version:  version,
		Flags:    flags,
		Location: location,
	}
	return u, err
}

// CreateURLBox - Create a self-referencing URL box
func CreateURLBox() *URLBox {
	return &URLBox{
		Version:  0,
		Flags:    dataIsSelfContainedFlag,
		Location: "",
	}
}

// Type - return box type
func (u *URLBox) Type() string {
	return "url "
}

// Size - return calculated size
func (u *URLBox) Size() uint64 {
	size := uint64(boxHeaderSize + 4)
	if u.Flags != uint32(dataIsSelfContainedFlag) {
		size += uint64(len(u.Location) + 1)
	}
	return size
}

// Encode - write box to w
func (u *URLBox) Encode(w io.Writer) error {
	err := EncodeHeader(u, w)
	if err != nil {
		return err
	}
	buf := makebuf(u)
	sw := NewSliceWriter(buf)
	versionAndFlags := (uint32(u.Version) << 24) + u.Flags
	sw.WriteUint32(versionAndFlags)
	if u.Flags != dataIsSelfContainedFlag {
		sw.WriteString(u.Location, true)
	}
	_, err = w.Write(buf)
	return err
}
func (u *URLBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, u, -1, 0)
	bd.write(" - location: %q", u.Location)
	return bd.err
}
