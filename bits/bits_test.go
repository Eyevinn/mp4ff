package bits

import (
	"bytes"
	"fmt"
	"io"
	"testing"
)

func TestReader(t *testing.T) {
	input := []byte{0xff, 0x0f} // 1111 1111 0000 1111
	rd := bytes.NewReader(input)
	reader := NewReader(rd)

	cases := []struct {
		n    int
		want uint
	}{
		{2, 3},  // 11
		{3, 7},  // 111
		{5, 28}, // 11100
		{3, 1},  // 001
		{3, 7},  // 111
	}

	for _, tc := range cases {
		got, err := reader.Read(tc.n)
		if err != nil && err != io.EOF {
			t.Fatalf("Read(%d) should not fail: %s", tc.n, err)
		}

		if got != tc.want {
			t.Errorf("Read(%d)=%b, want=%b", tc.n, got, tc.want)
		}

	}
}

func TestWriter(t *testing.T) {
	cases := []struct {
		inputs []uint
		want   []byte
		size   int
	}{
		{[]uint{255}, []byte{0xff}, 8},
		{[]uint{15, 15}, []byte{0xff}, 4},
		{[]uint{3, 3, 3, 3}, []byte{0xff}, 2},
		{[]uint{1, 1, 1, 1, 1, 1, 1, 1}, []byte{0xff}, 1},

		{[]uint{15, 15, 15}, []byte{0xff, 0xf0}, 4},
		{[]uint{3, 3, 3, 3, 3, 3}, []byte{0xff, 0xf0}, 2},
	}

	for _, tc := range cases {
		var buf bytes.Buffer
		writer := NewWriter(&buf)

		for _, input := range tc.inputs {
			writer.Write(input, tc.size)
		}
		err := writer.Error()
		if err != nil {
			t.Fatalf("Write should not fail: %s", err)
		}

		writer.Flush()
		err = writer.Error()
		if err != nil {
			t.Fatalf("Flush should not fail: %s", err)
		}

		if !bytes.Equal(buf.Bytes(), tc.want) {
			t.Errorf("Write writes %x, want %x", buf.Bytes(), tc.want)
		}
	}
}

func TestMask(t *testing.T) {
	cases := []struct {
		want  string
		input int
	}{
		{"11111111", 8},
		{"00001111", 4},
		{"00000011", 2},
	}

	for _, tc := range cases {
		m := mask(tc.input)
		if got := fmt.Sprintf("%08b", m); got != tc.want {
			t.Errorf("mask(%d)=%s,want=%s", tc.input, got, tc.want)
		}
	}
}
