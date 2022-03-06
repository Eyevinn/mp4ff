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

func TestWriteBits(t *testing.T) {
	sw := NewFixedSliceWriter(4)
	sw.WriteBits(0xf, 4)
	sw.WriteBits(0x2, 4)
	sw.WriteBits(0x5, 4)
	sw.FlushBits()
	if sw.AccError() != nil {
		t.Errorf("unexpected error writing bits")
	}
	result := sw.Bytes()
	if !(result[0] == 0xf2 && result[1] == 0x50) {
		t.Errorf("got %02x%02x instead of 0xf250", result[0], result[1])
	}
	sw.WriteUint16(0xffff)
}
