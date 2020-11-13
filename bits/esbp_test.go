package bits

import (
	"bytes"
	"encoding/hex"
	"io"
	"testing"

	"github.com/go-test/deep"
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

func TestEbspParser(t *testing.T) {

	cases := []struct{ name, start, want string }{
		{
			"read last byte",
			"32",
			"32",
		},
		{
			"remove escape but not 007",
			"27640020ac2ec05005bb011000000300100000078e840016e300005b8d8bdef83b438627",
			"27640020ac2ec05005bb0110000000100000078e840016e300005b8d8bdef83b438627",
		},
	}

	for _, c := range cases {
		byteData, _ := hex.DecodeString(c.start)
		buf := bytes.NewBuffer(byteData)
		r := NewEBSPReader(buf)
		got := []byte{}
		for {
			b, err := r.Read(8)
			if err == io.EOF {
				break
			}
			got = append(got, byte(b))
		}
		wantBytes, err := hex.DecodeString(c.want)
		if err != nil {
			t.Error(err)
		}
		if diff := deep.Equal(got, wantBytes); diff != nil {
			t.Errorf("%s: %v", c.name, diff)
		}
	}
}
