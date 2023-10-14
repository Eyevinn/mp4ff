package mp4

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/Eyevinn/mp4ff/bits"
)

func TestDecodeDescriptor(t *testing.T) {
	esDesc, err := hex.DecodeString(esdsProgIn[24:])
	if err != nil {
		t.Error(err)
	}
	sr := bits.NewFixedSliceReader(esDesc)
	desc, err := DecodeESDescriptor(sr, uint32(len(esDesc)))
	if err != nil {
		t.Error(err)
	}
	if desc.Tag() != ES_DescrTag {
		t.Error("tag is not 3")
	}
	out := make([]byte, len(esDesc))
	sw := bits.NewFixedSliceWriterFromSlice(out)
	err = desc.EncodeSW(sw)
	if err != nil {
		t.Error(err)
	}
	if !bytes.Equal(sw.Bytes(), esDesc) {
		t.Errorf("written es descriptor differs from read\n%s\n%s",
			hex.EncodeToString(sw.Bytes()), hex.EncodeToString(esDesc))
	}
}
