package bits

import (
	"bytes"
	"fmt"
	"testing"
)

func TestExpGolomb(t *testing.T) {
	cases := []struct {
		bits string
		n    uint
	}{
		{"1", 0},
		{"010", 1},
		{"011", 2},
		{"00100", 3},
		{"0001111", 14},
		{"000010000", 15},
		{"000011111", 30},
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
