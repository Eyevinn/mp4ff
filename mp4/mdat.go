package mp4

import (
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
)

// MdatBox - Media Data Box (mdat)
// The mdat box contains media chunks/samples.
type MdatBox struct {
	StartPos uint64
	Data     []byte
}

// DecodeMdat - box-specific decode
func DecodeMdat(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return &MdatBox{startPos, data}, nil
}

// Type - return box type
func (m *MdatBox) Type() string {
	return "mdat"
}

// Size - return calculated size. If bigger 32-bit max, it should be escaped.
func (m *MdatBox) Size() uint64 {
	contentSize := uint64(len(m.Data))
	return headerLength(contentSize) + contentSize
}

// AddSampleData -  a sample data to an mdat box
func (m *MdatBox) AddSampleData(s []byte) {
	m.Data = append(m.Data, s...)
}

// Encode - write box to w
func (m *MdatBox) Encode(w io.Writer) error {
	err := EncodeHeader(m, w)
	if err != nil {
		return err
	}
	if m.Size() >= (1 << 32) { // Need to write
		err = binary.Write(w, binary.BigEndian, m.Data)
		if err != nil {
			return err
		}
	}
	_, err = w.Write(m.Data)
	return err
}

func (m *MdatBox) Dump(w io.Writer, indent, indentStep string) error {
	_, err := fmt.Fprintf(w, "%s%s size=%d\n", indent, m.Type(), m.Size())
	return err
}
