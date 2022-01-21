package bits

import (
	"bytes"
	"testing"
)

func TestFixedSliceWriter(t *testing.T) {
	sw := NewFixedSliceWriter(20)
	sw.WriteUint8(0xff)
	sw.WriteUint16(0xffff)
	sw.WriteUint24(0xffffff)
	sw.WriteUint32(0xffffffff)
	sw.WriteUint64(0xffffffffffffffff)
	sw.WriteUint16(0)
	expected := make([]byte, 20)
	for i := range expected {
		if i == 18 {
			break
		}
		expected[i] = 0xff

	}
	if !bytes.Equal(expected, sw.Bytes()) {
		t.Errorf("bytes differ: %v %v", expected, sw.Bytes())
	}
	sw.WriteUint24(0xffffff)
	if sw.AccError() == nil {
		t.Errorf("no overflow error")
	}
}
