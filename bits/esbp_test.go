package bits

import (
	"bytes"
	"testing"
)

func TestGolomb(t *testing.T) {
	t.Run("unsignedGolomb", func(t *testing.T) {
		cases := []struct {
			input []byte
			want  uint
		}{
			{[]byte{0b10000000}, 0},
			{[]byte{0b01000000}, 1},
			{[]byte{0b01100000}, 2},
			{[]byte{0b00010000}, 7},
			{[]byte{0b00010010}, 8},
			{[]byte{0b00011110}, 14},
		}

		for _, c := range cases {
			buf := bytes.NewBuffer(c.input)
			r := NewEBSPReader(buf)
			got := r.MustReadExpGolomb()
			if got != c.want {
				t.Errorf("got %d want %d", got, c.want)
			}
		}
	})
	t.Run("signedGolomb", func(t *testing.T) {
		cases := []struct {
			input []byte
			want  int
		}{
			{[]byte{0b10000000}, 0},
			{[]byte{0b01000000}, 1},
			{[]byte{0b01100000}, -1},
			{[]byte{0b00010000}, 4},
			{[]byte{0b00010010}, -4},
			{[]byte{0b00011110}, -7},
		}

		for _, c := range cases {
			buf := bytes.NewBuffer(c.input)
			r := NewEBSPReader(buf)
			got := r.MustReadSignedGolomb()
			if got != c.want {
				t.Errorf("got %d want %d", got, c.want)
			}
		}
	})
}
