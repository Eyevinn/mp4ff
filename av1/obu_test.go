package av1

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-test/deep"
)

func TestParseOBUHeader(t *testing.T) {
	cases := []struct {
		name string
		hex  string
		want OBUHeader
	}{
		{
			name: "temporal delimiter with size field",
			hex:  "12",
			want: OBUHeader{Type: OBUTemporalDelimiter, HasSizeField: true, HeaderSize: 1},
		},
		{
			name: "sequence header with size field",
			hex:  "0a",
			want: OBUHeader{Type: OBUSequenceHeader, HasSizeField: true, HeaderSize: 1},
		},
		{
			name: "frame without size field",
			hex:  "30",
			want: OBUHeader{Type: OBUFrame, HasSizeField: false, HeaderSize: 1},
		},
		{
			name: "frame with extension header",
			hex:  "3668",
			want: OBUHeader{Type: OBUFrame, ExtensionFlag: true, HasSizeField: true,
				TemporalID: 3, SpatialID: 1, HeaderSize: 2},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			data, _ := hex.DecodeString(c.hex)
			got, err := ParseOBUHeader(data)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if diff := deep.Equal(got, c.want); diff != nil {
				t.Error(diff)
			}
		})
	}
}

func TestParseOBUHeaderErrors(t *testing.T) {
	if _, err := ParseOBUHeader(nil); !errors.Is(err, ErrTruncatedOBU) {
		t.Errorf("empty data: got %v, want ErrTruncatedOBU", err)
	}
	if _, err := ParseOBUHeader([]byte{0x80}); !errors.Is(err, ErrForbiddenBit) {
		t.Errorf("forbidden bit: got %v, want ErrForbiddenBit", err)
	}
	if _, err := ParseOBUHeader([]byte{0x36}); !errors.Is(err, ErrTruncatedOBU) {
		t.Errorf("missing extension byte: got %v, want ErrTruncatedOBU", err)
	}
}

func TestReadLEB128(t *testing.T) {
	cases := []struct {
		hex       string
		wantValue uint64
		wantBytes int
	}{
		{"00", 0, 1},
		{"7f", 127, 1},
		{"8001", 128, 2},
		{"ff7f", 16383, 2},
		{"80808001", 1 << 21, 4},
	}
	for _, c := range cases {
		data, _ := hex.DecodeString(c.hex)
		v, n, err := ReadLEB128(data)
		if err != nil {
			t.Fatalf("%s: unexpected error %v", c.hex, err)
		}
		if v != c.wantValue || n != c.wantBytes {
			t.Errorf("%s: got (%d, %d), want (%d, %d)", c.hex, v, n, c.wantValue, c.wantBytes)
		}
	}
	if _, _, err := ReadLEB128([]byte{0x80}); !errors.Is(err, ErrTruncatedLEB128) {
		t.Errorf("truncated: got %v, want ErrTruncatedLEB128", err)
	}
	if _, _, err := ReadLEB128([]byte{0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80}); !errors.Is(err, ErrTooLongLEB128) {
		t.Errorf("too long: got %v, want ErrTooLongLEB128", err)
	}
}

func TestSplitOBUs(t *testing.T) {
	// Temporal unit built from real bytes: temporal delimiter, a real sequence header
	// OBU (payload extracted from AOM fate-suite av1-1-b8-23-film_grain-50.ivf), and a
	// small synthetic frame OBU. Exercises size fields and full consumption.
	tuHex := "1200" + // temporal delimiter, size 0
		"0a0b00000004457e3e7dfcc060" + // sequence header, size 11
		"3203aabbcc" // frame, size 3
	data, _ := hex.DecodeString(tuHex)
	obus, err := SplitOBUs(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantTypes := []OBUType{OBUTemporalDelimiter, OBUSequenceHeader, OBUFrame}
	wantPayloadLen := []int{0, 11, 3}
	if len(obus) != len(wantTypes) {
		t.Fatalf("got %d OBUs, want %d", len(obus), len(wantTypes))
	}
	for i, o := range obus {
		if o.Header.Type != wantTypes[i] {
			t.Errorf("OBU %d: type %s, want %s", i, o.Header.Type, wantTypes[i])
		}
		if len(o.Payload) != wantPayloadLen[i] {
			t.Errorf("OBU %d: payload len %d, want %d", i, len(o.Payload), wantPayloadLen[i])
		}
	}
}

func TestSplitOBUsSizeless(t *testing.T) {
	// A single OBU without a size field extends to the end of data.
	data, _ := hex.DecodeString("08aabbcc") // sequence header, no size field
	obus, err := SplitOBUs(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(obus) != 1 {
		t.Fatalf("got %d OBUs, want 1", len(obus))
	}
	if obus[0].Header.HasSizeField {
		t.Error("expected no size field")
	}
	if diff := deep.Equal(obus[0].Payload, []byte{0xaa, 0xbb, 0xcc}); diff != nil {
		t.Error(diff)
	}
}

func TestSplitOBUsErrors(t *testing.T) {
	// size field announced but no bytes follow
	if _, err := SplitOBUs([]byte{0x12}); err == nil {
		t.Error("expected error for missing size field")
	}
	// payload longer than remaining data
	if _, err := SplitOBUs([]byte{0x0a, 0x05, 0xaa}); err == nil {
		t.Error("expected error for payload exceeding data")
	}
	// obu_size of 2^32, which would wrap negative in a 32-bit int
	if _, err := SplitOBUs([]byte{0x0a, 0x80, 0x80, 0x80, 0x80, 0x10}); err == nil {
		t.Error("expected error for huge obu_size")
	}
}

// TestSplitOBUsFateVectors parses every AV1 IVF test vector found in the directory
// given by MP4FF_AV1_TESTVECTORS_DIR (e.g. AOM/fate-suite vectors) and checks that each
// temporal unit splits into OBUs with no leftover bytes. Skipped when the dir is unset.
func TestSplitOBUsFateVectors(t *testing.T) {
	dir := os.Getenv("MP4FF_AV1_TESTVECTORS_DIR")
	if dir == "" {
		t.Skip("set MP4FF_AV1_TESTVECTORS_DIR to an AV1 IVF test-vector directory to run")
	}
	files, err := filepath.Glob(filepath.Join(dir, "*.ivf"))
	if err != nil || len(files) == 0 {
		t.Skipf("no *.ivf files in %s", dir)
	}
	for _, f := range files {
		f := f
		t.Run(filepath.Base(f), func(t *testing.T) {
			data, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			for fi, frame := range ivfFrames(t, data) {
				if _, err := SplitOBUs(frame); err != nil {
					t.Fatalf("frame %d: %v", fi, err)
				}
			}
		})
	}
}

// ivfFrames returns the per-frame payloads (temporal units) of an IVF file.
func ivfFrames(t *testing.T, data []byte) [][]byte {
	t.Helper()
	if len(data) < 32 || string(data[0:4]) != "DKIF" {
		t.Fatal("not an IVF file")
	}
	hdrSize := int(binary.LittleEndian.Uint16(data[6:8]))
	pos := hdrSize
	var frames [][]byte
	for pos+12 <= len(data) {
		size := int(binary.LittleEndian.Uint32(data[pos : pos+4]))
		pos += 12 // 4-byte size + 8-byte timestamp
		if pos+size > len(data) {
			break
		}
		frames = append(frames, data[pos:pos+size])
		pos += size
	}
	return frames
}
