package mp4

import (
	"fmt"
	"io"
	"io/ioutil"
)

// ElngBox - Extended Language Box
type ElngBox struct {
	Language string
}

// CreateElng - Create an Extended Language Box
func CreateElng(language string) *ElngBox {
	return &ElngBox{Language: language}
}

// DecodeElng - box-specific decode
func DecodeElng(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	b := &ElngBox{
		Language: string(data[:len(data)-1]),
	}
	return b, nil
}

// Type - box type
func (b *ElngBox) Type() string {
	return "elng"
}

// Size - calculated size of box
func (b *ElngBox) Size() uint64 {
	return uint64(boxHeaderSize + len(b.Language) + 1)
}

// Encode - write box to w
func (b *ElngBox) Encode(w io.Writer) error {
	err := EncodeHeader(b, w)
	if err != nil {
		return err
	}
	_, err = w.Write([]byte(b.Language))
	if err != nil {
		return err
	}
	_, err = w.Write([]byte{0})
	return err
}

func (b *ElngBox) Dump(w io.Writer, indent, indentStep string) error {
	_, err := fmt.Fprintf(w, "%s%s size=%d\n%s - Language: %s\n", indent,
		b.Type(), b.Size(), indent, b.Language)
	return err
}
