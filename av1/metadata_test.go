package av1

import (
	"bytes"
	"encoding/hex"
	"errors"
	"testing"

	"github.com/go-test/deep"
)

func TestMetadataOBUEncode(t *testing.T) {
	cases := []struct {
		name string
		obu  MetadataOBU
		want string // full OBU: header, obu_size, metadata_type, payload, trailing_bits
	}{
		{
			name: "HDR content light level",
			obu:  MetadataOBU{Type: MetadataTypeHDRCLL, Payload: []byte{0x03, 0xe8, 0x01, 0x90}},
			want: "2a06" + "01" + "03e80190" + "80",
		},
		{
			name: "empty payload",
			obu:  MetadataOBU{Type: MetadataTypeTimecode},
			want: "2a02" + "05" + "80",
		},
		{
			name: "metadata type needing two LEB128 bytes",
			obu:  MetadataOBU{Type: 200, Payload: []byte{0xaa}},
			want: "2a04" + "c801" + "aa" + "80",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := c.obu.Encode()
			if hex.EncodeToString(got) != c.want {
				t.Errorf("got %s, want %s", hex.EncodeToString(got), c.want)
			}
			if c.obu.Size() != len(got) {
				t.Errorf("Size() %d, but encoded %d bytes", c.obu.Size(), len(got))
			}
			// The encoded OBU must split and parse back into the same metadata OBU.
			obus, err := SplitOBUs(got)
			if err != nil {
				t.Fatalf("SplitOBUs: %v", err)
			}
			if len(obus) != 1 {
				t.Fatalf("got %d OBUs, want 1", len(obus))
			}
			if obus[0].Header.Type != OBUMetadata {
				t.Fatalf("got OBU of type %s, want %s", obus[0].Header.Type, OBUMetadata)
			}
			back, err := ParseMetadataOBUFromOBU(obus[0])
			if err != nil {
				t.Fatalf("ParseMetadataOBUFromOBU: %v", err)
			}
			if back.Type != c.obu.Type {
				t.Errorf("type: got %s, want %s", back.Type, c.obu.Type)
			}
			// bytes.Equal treats a nil and an empty payload as equal, which they are here.
			if !bytes.Equal(back.Payload, c.obu.Payload) {
				t.Errorf("payload: got %x, want %x", back.Payload, c.obu.Payload)
			}
			// Encoding the parsed OBU again must not add a second trailing_bits byte.
			if again := hex.EncodeToString(back.Encode()); again != c.want {
				t.Errorf("re-encode: got %s, want %s", again, c.want)
			}
		})
	}
}

func TestParseMetadataOBUTrailingBits(t *testing.T) {
	cases := []struct {
		name         string
		payload      string // OBU payload: metadata_type + body
		wantType     MetadataType
		wantPayload  string
		wantHasTBits bool
		wantReencode string // OBU payload after Payload is encoded again
	}{
		{
			name:         "trailing_bits byte is dropped",
			payload:      "01" + "03e8" + "80",
			wantType:     MetadataTypeHDRCLL,
			wantPayload:  "03e8",
			wantReencode: "01" + "03e8" + "80",
		},
		{
			name:         "padding zeros after trailing_bits are dropped",
			payload:      "01" + "03e8" + "80" + "000000",
			wantType:     MetadataTypeHDRCLL,
			wantPayload:  "03e8",
			wantReencode: "01" + "03e8" + "80",
		},
		{
			name:         "zero bytes before trailing_bits are kept",
			payload:      "04" + "b50000" + "80" + "00",
			wantType:     MetadataTypeITUTT35,
			wantPayload:  "b50000",
			wantReencode: "04" + "b50000" + "80",
		},
		{
			name: "missing trailing_bits is tolerated",
			// Indistinguishable from an unaligned payload, so it stays without
			// trailing_bits rather than gaining a byte that may not belong there.
			payload:      "01" + "03e8",
			wantType:     MetadataTypeHDRCLL,
			wantPayload:  "03e8",
			wantHasTBits: true,
			wantReencode: "01" + "03e8",
		},
		{
			name: "byte shared with trailing_bits is kept",
			// A payload that is not byte-aligned ends in a byte that is not 0x80. Adding
			// a trailing_bits byte on re-encode would give it two trailing_one_bits.
			payload:      "05" + "1234" + "90",
			wantType:     MetadataTypeTimecode,
			wantPayload:  "123490",
			wantHasTBits: true,
			wantReencode: "05" + "123490",
		},
		{
			name:         "trailing_bits only",
			payload:      "05" + "80",
			wantType:     MetadataTypeTimecode,
			wantPayload:  "",
			wantReencode: "05" + "80",
		},
		{
			name:         "no payload at all",
			payload:      "05",
			wantType:     MetadataTypeTimecode,
			wantPayload:  "",
			wantReencode: "05" + "80",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			data, err := hex.DecodeString(c.payload)
			if err != nil {
				t.Fatal(err)
			}
			got, err := ParseMetadataOBU(data)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Type != c.wantType {
				t.Errorf("type: got %s, want %s", got.Type, c.wantType)
			}
			if hex.EncodeToString(got.Payload) != c.wantPayload {
				t.Errorf("payload: got %s, want %s", hex.EncodeToString(got.Payload), c.wantPayload)
			}
			if got.PayloadHasTrailingBits != c.wantHasTBits {
				t.Errorf("PayloadHasTrailingBits: got %t, want %t", got.PayloadHasTrailingBits, c.wantHasTBits)
			}
			if enc := hex.EncodeToString(got.OBU().Payload); enc != c.wantReencode {
				t.Errorf("re-encoded OBU payload: got %s, want %s", enc, c.wantReencode)
			}
			if got.Size() != len(got.Encode()) {
				t.Errorf("Size() %d, but encoded %d bytes", got.Size(), len(got.Encode()))
			}
			// Re-encoding is stable: parsing the result gives the same metadata OBU back.
			obus, err := SplitOBUs(got.Encode())
			if err != nil {
				t.Fatalf("SplitOBUs: %v", err)
			}
			if len(obus) != 1 {
				t.Fatalf("got %d OBUs, want 1", len(obus))
			}
			again, err := ParseMetadataOBUFromOBU(obus[0])
			if err != nil {
				t.Fatalf("ParseMetadataOBUFromOBU: %v", err)
			}
			if diff := deep.Equal(again, got); diff != nil {
				t.Error(diff)
			}
		})
	}
}

func TestParseMetadataOBUErrors(t *testing.T) {
	if _, err := ParseMetadataOBU(nil); !errors.Is(err, ErrTruncatedLEB128) {
		t.Errorf("empty payload: got %v, want ErrTruncatedLEB128", err)
	}
	// A metadata_type LEB128 that never terminates.
	if _, err := ParseMetadataOBU([]byte{0x80}); !errors.Is(err, ErrTruncatedLEB128) {
		t.Errorf("unterminated metadata_type: got %v, want ErrTruncatedLEB128", err)
	}
	td := OBU{Header: OBUHeader{Type: OBUTemporalDelimiter, HasSizeField: true, HeaderSize: 1}}
	if _, err := ParseMetadataOBUFromOBU(td); !errors.Is(err, ErrNotMetadataOBU) {
		t.Errorf("temporal delimiter: got %v, want ErrNotMetadataOBU", err)
	}
}

func TestITUTT35(t *testing.T) {
	cases := []struct {
		name string
		hex  string // metadata_itu_t_t35() body
		want ITUTT35
	}{
		{
			name: "USA country code",
			hex:  "b5" + "0031",
			want: ITUTT35{CountryCode: 0xb5, Payload: []byte{0x00, 0x31}},
		},
		{
			name: "country code with extension byte",
			hex:  "ff" + "2a" + "0102",
			want: ITUTT35{CountryCode: 0xff, CountryCodeExtension: 0x2a, Payload: []byte{0x01, 0x02}},
		},
		{
			name: "empty payload",
			hex:  "b5",
			want: ITUTT35{CountryCode: 0xb5, Payload: []byte{}},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			data, err := hex.DecodeString(c.hex)
			if err != nil {
				t.Fatal(err)
			}
			got, err := ParseITUTT35(data)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if diff := deep.Equal(got, c.want); diff != nil {
				t.Error(diff)
			}
			if enc := got.Encode(); hex.EncodeToString(enc) != c.hex {
				t.Errorf("Encode: got %s, want %s", hex.EncodeToString(enc), c.hex)
			}
			if got.Size() != len(data) {
				t.Errorf("Size() %d, want %d", got.Size(), len(data))
			}
			// The wrapped metadata OBU must round-trip back to the same body.
			m := got.MetadataOBU()
			if m.Type != MetadataTypeITUTT35 {
				t.Errorf("metadata type: got %s, want ITUTT35", m.Type)
			}
			obus, err := SplitOBUs(m.Encode())
			if err != nil {
				t.Fatalf("SplitOBUs: %v", err)
			}
			back, err := ParseMetadataOBUFromOBU(obus[0])
			if err != nil {
				t.Fatalf("ParseMetadataOBUFromOBU: %v", err)
			}
			if hex.EncodeToString(back.Payload) != c.hex {
				t.Errorf("round trip: got %s, want %s", hex.EncodeToString(back.Payload), c.hex)
			}
		})
	}
}

func TestParseITUTT35Errors(t *testing.T) {
	if _, err := ParseITUTT35(nil); !errors.Is(err, ErrTruncatedITUTT35) {
		t.Errorf("empty payload: got %v, want ErrTruncatedITUTT35", err)
	}
	// Country code 0xff promises an extension byte that is not there.
	if _, err := ParseITUTT35([]byte{0xff}); !errors.Is(err, ErrTruncatedITUTT35) {
		t.Errorf("missing extension byte: got %v, want ErrTruncatedITUTT35", err)
	}
}

func TestMetadataTypeString(t *testing.T) {
	cases := []struct {
		mt   MetadataType
		want string
	}{
		{MetadataTypeHDRCLL, "HDRCLL"},
		{MetadataTypeHDRMDCV, "HDRMDCV"},
		{MetadataTypeScalability, "Scalability"},
		{MetadataTypeITUTT35, "ITUTT35"},
		{MetadataTypeTimecode, "Timecode"},
		{6, "Reserved(6)"},
	}
	for _, c := range cases {
		if got := c.mt.String(); got != c.want {
			t.Errorf("got %s, want %s", got, c.want)
		}
	}
}
