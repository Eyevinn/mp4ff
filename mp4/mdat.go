package mp4

import (
	"io"
	"io/ioutil"
)

// Media Data Box (mdat - optional)
//
// Status: not decoded
//
// The mdat box contains media chunks/samples.
//
// It is not read, only the io.Reader is stored, and will be used to Encode (io.Copy) the box to a io.Writer.
type MdatBox struct {
	Data []byte
}

func DecodeMdat(r io.Reader) (Box, error) {

	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return &MdatBox{data}, nil
}

func (m *MdatBox) Type() string {
	return "mdat"
}

func (m *MdatBox) Size() int {
	return BoxHeaderSize + len(m.Data)
}

func (m *MdatBox) Encode(w io.Writer) error {
	err := EncodeHeader(m, w)
	if err != nil {
		return err
	}
	w.Write(m.Data)
	return err
}
