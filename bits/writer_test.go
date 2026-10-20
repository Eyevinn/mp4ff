package bits_test

import (
	"bytes"
	"fmt"
	"math"
	"testing"

	"github.com/Eyevinn/mp4ff/bits"
)

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
		writer := bits.NewWriter(&buf)

		for _, input := range tc.inputs {
			writer.Write(input, tc.size)
		}
		err := writer.AccError()
		if err != nil {
			t.Fatalf("Write should not fail: %s", err)
		}

		writer.Flush()
		err = writer.AccError()
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
		m := bits.Mask(tc.input)
		if got := fmt.Sprintf("%08b", m); got != tc.want {
			t.Errorf("mask(%d)=%s,want=%s", tc.input, got, tc.want)
		}
	}
}

// TestWrite32BitsNotByteAligned checks that 32 bits written after 7 bits keep their first bit, also where uint is
// 32 bits wide.
func TestWrite32BitsNotByteAligned(t *testing.T) {
	want := []byte{0x01, 0x23, 0x45, 0x67, 0x89} // 0000000 1 00100011 01000101 01100111 1000100 1
	var buf, ebspBuf bytes.Buffer
	w := bits.NewWriter(&buf)
	ew := bits.NewEBSPWriter(&ebspBuf)
	sw := bits.NewFixedSliceWriter(len(want))
	writers := []struct {
		name  string
		write func(bits uint, n int)
		bytes func() []byte
	}{
		{"Writer", w.Write, buf.Bytes},
		{"EBSPWriter", ew.Write, ebspBuf.Bytes},
		{"FixedSliceWriter", sw.WriteBits, sw.Bytes},
	}
	for _, wr := range writers {
		wr.write(0, 7)
		wr.write(0x91a2b3c4, 32)
		wr.write(1, 1)
		if got := wr.bytes(); !bytes.Equal(got, want) {
			t.Errorf("%s wrote %x, want %x", wr.name, got, want)
		}
	}
}

// TestWriteExpGolombLargeValues checks ue(v) codes of up to 65 bits, written after 0 to 7 bits.
func TestWriteExpGolombLargeValues(t *testing.T) {
	type bitReader interface {
		Read(n int) uint
		ReadExpGolomb() uint
		AccError() error
	}
	values := []uint{1<<28 - 2, 1<<29 - 1, 1 << 31, math.MaxUint32}
	for nrLeadingBits := 0; nrLeadingBits < 8; nrLeadingBits++ {
		for _, v := range values {
			var buf bytes.Buffer
			ew := bits.NewEBSPWriter(&buf)
			ew.Write(bits.Mask(nrLeadingBits), nrLeadingBits)
			ew.WriteExpGolomb(v)
			ew.WriteRbspTrailingBits()
			sw := bits.NewFixedSliceWriter(10)
			sw.WriteBits(bits.Mask(nrLeadingBits), nrLeadingBits)
			sw.WriteExpGolomb(v)
			sw.WriteFlag(true)
			sw.FlushBits()
			readers := []struct {
				name string
				r    bitReader
			}{
				{"EBSPWriter", bits.NewEBSPReaderFromSlice(buf.Bytes())},
				{"FixedSliceWriter", bits.NewReaderFromSlice(sw.Bytes())},
			}
			for _, rd := range readers {
				r := rd.r
				if got := r.Read(nrLeadingBits); got != bits.Mask(nrLeadingBits) {
					t.Errorf("%s, %d bits before ue(%d): read %b before", rd.name, nrLeadingBits, v, got)
				}
				if got := r.ReadExpGolomb(); got != v {
					t.Errorf("%s, %d bits before ue(%d): read ue(%d)", rd.name, nrLeadingBits, v, got)
				}
				if got := r.Read(1); got != 1 || r.AccError() != nil {
					t.Errorf("%s, %d bits before ue(%d): read %d after, err %v", rd.name, nrLeadingBits, v, got,
						r.AccError())
				}
			}
		}
	}

	// 101, ue(2^32-1) as 32 zeros, a one and 32 zeros, then a one.
	sw := bits.NewFixedSliceWriter(9)
	sw.WriteBits(0b101, 3)
	sw.WriteExpGolomb(math.MaxUint32)
	sw.WriteFlag(true)
	sw.FlushBits()
	want := []byte{0xa0, 0x00, 0x00, 0x00, 0x10, 0x00, 0x00, 0x00, 0x08}
	if got := sw.Bytes(); !bytes.Equal(got, want) {
		t.Errorf("FixedSliceWriter wrote %x, want %x", got, want)
	}
}
