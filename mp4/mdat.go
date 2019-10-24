package mp4

import (
	"io"
	"io/ioutil"
)

// MdatBox - Media Data Box (mdat)
// The mdat box contains media chunks/samples.
//
type MdatBox struct {
	Data []byte
}

// DecodeMdat - box-specific decode
func DecodeMdat(r io.Reader) (Box, error) {

	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return &MdatBox{data}, nil
}

// Type - return box type
func (m *MdatBox) Type() string {
	return "mdat"
}

// Size - return calculated size
func (m *MdatBox) Size() int {
	return BoxHeaderSize + len(m.Data)
}

// Encode - write box to w
func (m *MdatBox) Encode(w io.Writer) error {
	err := EncodeHeader(m, w)
	if err != nil {
		return err
	}
	w.Write(m.Data)
	return err
}
