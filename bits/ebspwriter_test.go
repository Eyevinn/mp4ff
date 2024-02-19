package bits_test

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/go-test/deep"
)

func TestEBSPWriter(t *testing.T) {
	t.Run("insert startcode emulation prevention bytes", func(t *testing.T) {
		testCases := []struct {
			in  []byte
			out []byte
		}{
			{
				in:  []byte{0, 0, 0, 1},
				out: []byte{0, 0, 3, 0, 1},
			},
			{
				in:  []byte{1, 0, 0, 2},
				out: []byte{1, 0, 0, 3, 2},
			},
			{
				in:  []byte{0, 0, 0, 0, 0},
				out: []byte{0, 0, 3, 0, 0, 3, 0},
			},
			{
				in:  []byte{0, 0, 0, 0, 5, 0},
				out: []byte{0, 0, 3, 0, 0, 5, 0},
			},
		}
		for _, tc := range testCases {
			buf := bytes.Buffer{}
			w := bits.NewEBSPWriter(&buf)
			for _, b := range tc.in {
				w.Write(uint(b), 8)
			}
			diff := deep.Equal(buf.Bytes(), tc.out)
			if diff != nil {
				t.Errorf("Got %v but wanted %d", buf.Bytes(), tc.out)
			}
		}
	})
	t.Run("write exp golomb", func(t *testing.T) {
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
			w := bits.NewEBSPWriter(&b)
			w.WriteExpGolomb(tc.n)
			gotBits := getBitsWritten(w, &b)
			if gotBits != tc.bits {
				t.Errorf("wanted %s but got %s for %d", tc.bits, gotBits, tc.n)
			}
		}
	})
}

func getBitsWritten(w *bits.EBSPWriter, b *bytes.Buffer) string {
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
