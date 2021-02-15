package mp4

import (
	"io"
	"io/ioutil"
)

// KindBox - Track Kind Box
type KindBox struct {
	SchemeURI string
	Value     string
}

// DecodeKind - box-specific decode
func DecodeKind(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	s := NewSliceReader(data)
	schemeURI, err := s.ReadZeroTerminatedString()
	if err != nil {
		return nil, err
	}
	value, err := s.ReadZeroTerminatedString()
	if err != nil {
		return nil, err
	}
	b := &KindBox{
		SchemeURI: schemeURI,
		Value:     value,
	}
	return b, nil
}

// Type - box type
func (b *KindBox) Type() string {
	return "kind"
}

// Size - calculated size of box
func (b *KindBox) Size() uint64 {
	return uint64(boxHeaderSize + len(b.SchemeURI) + 1 + len(b.Value) + 1)
}

// Encode - write box to w
func (b *KindBox) Encode(w io.Writer) error {
	err := EncodeHeader(b, w)
	if err != nil {
		return err
	}
	buf := makebuf(b)
	sw := NewSliceWriter(buf)
	sw.WriteString(b.SchemeURI, true)
	sw.WriteString(b.Value, true)
	_, err = w.Write(buf)
	return err
}

func (b *KindBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, -1, 0)
	bd.write(" - schemeURI: %s", b.SchemeURI)
	bd.write(" - value: %s", b.Value)
	return bd.err
}
