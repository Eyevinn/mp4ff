package bits

import (
	"bytes"
	"fmt"
	"testing"
)

func TestExpGolomb(t *testing.T) {
	cases := []struct {
		n    uint
		bits string
	}{
		{0, "1"},
		{1, "010"},
		{2, "011"},
		{3, "00100"},
		{14, "0001111"},
		{15, "000010000"},
		{30, "000011111"},
	}

	for _, tc := range cases {
		b := bytes.Buffer{}
		w := NewEBSPWriter(&b)
		w.WriteExpGolomb(tc.n)
		gotBits := getBitsWritten(w, &b)
		if gotBits != tc.bits {
			t.Errorf("wanted %s but got %s for %d", tc.bits, gotBits, tc.n)
		}
	}
}

func getBitsWritten(w *EBSPWriter, b *bytes.Buffer) string {
	bits := ""
	for _, c := range b.Bytes() {
		bits += fmt.Sprintf("%08b", c)
	}
	valueInWriter, nrBitsInWriter := w.BitsInBuffer()
	if nrBitsInWriter > 0 {
		fullByte := fmt.Sprintf("%08b", valueInWriter)
		bits += fullByte[8-nrBitsInWriter:]
	}
	return bits
}
