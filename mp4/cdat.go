package mp4

import (
	"encoding/hex"
	"io"
	"io/ioutil"
)

// CdatBox - Closed Captioning Sample Data according to QuickTime spec:
// https://developer.apple.com/library/archive/documentation/QuickTime/QTFF/QTFFChap3/qtff3.html#//apple_ref/doc/uid/TP40000939-CH205-SW87
type CdatBox struct {
	Data []byte
}

// DecodeCdat - box-specific decode
func DecodeCdat(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	b := &CdatBox{
		Data: data,
	}
	return b, nil
}

// Type - box type
func (b *CdatBox) Type() string {
	return "cdat"
}

// Size - calculated size of box
func (b *CdatBox) Size() uint64 {
	return uint64(boxHeaderSize + len(b.Data))
}

// Encode - write box to w
func (b *CdatBox) Encode(w io.Writer) error {
	err := EncodeHeader(b, w)
	if err != nil {
		return err
	}
	_, err = w.Write(b.Data)
	return err
}

func (b *CdatBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, -1, 0)
	bd.write(" - data: %s", hex.EncodeToString(b.Data))
	return bd.err
}
