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

// TestEbspParser including startCodeEmulationPrevention removal
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
		{
			"Long zero sequence",
			"00000300000300",
			"0000000000",
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

func TestGetPrecisePosition(t *testing.T) {

	testCases := []struct {
		name             string
		inBytes          []byte
		nrBitsToRead     int
		nrBytesRead      int
		nrBitsReadInByte int
	}{
		{
			name:             "Read 13 bits",
			inBytes:          []byte{0, 1, 2},
			nrBitsToRead:     13,
			nrBytesRead:      2,
			nrBitsReadInByte: 5,
		},
	}

	for _, c := range testCases {

		buf := bytes.NewBuffer(c.inBytes)
		r := NewEBSPReader(buf)
		_, err := r.Read(c.nrBitsToRead)
		if err != nil {
			t.Error(err)
		}
		if r.NrBytesRead() != c.nrBytesRead {
			t.Errorf("%s: got %d bytes want %d bytes", c.name, r.NrBytesRead(), c.nrBytesRead)
		}
		if r.NrBitsReadInCurrentByte() != c.nrBitsReadInByte {
			t.Errorf("%s: got %d bits want %d bits", c.name, r.NrBitsReadInCurrentByte(), c.nrBitsReadInByte)
		}

	}
}

func TestMoreRbspData(t *testing.T) {

	testCases := []struct {
		name         string
		inBytes      []byte
		nrBitsBefore int
		moreRbsp     bool
		nrBitsAfter  int
		valueRead    uint
	}{
		{
			name:         "start of byte",
			inBytes:      []byte{0b10000000},
			nrBitsBefore: 0,
			moreRbsp:     false,
			nrBitsAfter:  0,
			valueRead:    0,
		},
		{
			name:         "one bit left",
			inBytes:      []byte{0b11000000},
			nrBitsBefore: 0,
			moreRbsp:     true,
			nrBitsAfter:  2,
			valueRead:    3,
		},
		{
			name:         "after one bit",
			inBytes:      []byte{0b11000000},
			nrBitsBefore: 1,
			moreRbsp:     false,
			nrBitsAfter:  0,
			valueRead:    0,
		},
		{
			name:         "after one byte",
			inBytes:      []byte{0b11110111, 0b11001000},
			nrBitsBefore: 9,
			moreRbsp:     true,
			nrBitsAfter:  4,
			valueRead:    0b00001001,
		},
	}

	for _, c := range testCases {

		brd := bytes.NewReader(c.inBytes)
		r := NewEBSPReader(brd)
		_, err := r.Read(c.nrBitsBefore)
		if err != nil {
			t.Error(err)
		}
		moreRbsp, err := r.MoreRbspData()
		if err != nil {
			t.Error(err)
		}
		if moreRbsp != c.moreRbsp {
			t.Errorf("%s: got %t want %t", c.name, moreRbsp, c.moreRbsp)
		}
		got, err := r.Read(c.nrBitsAfter)
		if err != nil {
			t.Error(err)
		}
		if got != c.valueRead {
			t.Errorf("%s: got %d want %d after check", c.name, got, c.valueRead)
		}
	}
}

func TestReadTrailingRbspBits(t *testing.T) {
	input := []byte{0b10000000}
	brd := bytes.NewReader(input)
	reader := NewEBSPReader(brd)
	err := reader.ReadRbspTrailingBits()
	if err != nil {
		t.Error(err)
	}
	_, err = reader.Read(1)
	if err != io.EOF {
		t.Errorf("Not at end after reading rbsp_trailing_bits")
	}
}

func TestEBSPWriter(t *testing.T) {
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
	}
	for _, tc := range testCases {
		buf := bytes.Buffer{}
		w := NewEBSPWriter(&buf)
		for _, b := range tc.in {
			w.Write(uint(b), 8)
		}
		diff := deep.Equal(buf.Bytes(), tc.out)
		if diff != nil {
			t.Errorf("Got %v but wanted %d", buf.Bytes(), tc.out)
		}
	}
}
