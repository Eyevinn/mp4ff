package av1

import (
	"encoding/hex"
	"testing"

	"github.com/Eyevinn/mp4ff/sei"
)

// ccData for one frame with the field-1 control pair 0x94 0x2c, an empty field 2 and
// cc_count = 3, i.e. one DTVCC padding construct (ATSC A/53 Part 4 Section 6.2.2).
const ccData942c = "c3ff" + "fc942c" + "f90000" + "fa0000" + "ff"

func TestCreateCTA608MetadataOBU(t *testing.T) {
	ccData, err := hex.DecodeString(ccData942c)
	if err != nil {
		t.Fatal(err)
	}
	// The complete OBU, byte by byte:
	//   2a                       obu_header: type = OBUMetadata(5), obu_has_size_field = 1
	//   16                       obu_size = 22
	//   04                       metadata_type = METADATA_TYPE_ITUT_T35 (4)
	//   b5 0031 47413934 03      country USA, provider ATSC, "GA94", cc_data
	//   c3ff fc942c f90000 fa0000 ff    cc_data()
	//   80                       trailing_bits
	want := "2a" + "16" + "04" + "b5003147413934" + "03" + ccData942c + "80"
	got := CreateCTA608MetadataOBU(ccData)
	if hex.EncodeToString(got) != want {
		t.Errorf("got  %s\nwant %s", hex.EncodeToString(got), want)
	}
	if len(got) != 24 {
		t.Errorf("got %d bytes, want 24", len(got))
	}
}

// TestCTA608PayloadAlignedWithSEI locks in that the AV1 and the AVC/HEVC path carry the
// exact same T.35 payload, so only the envelope differs between the codecs.
func TestCTA608PayloadAlignedWithSEI(t *testing.T) {
	ccData, err := hex.DecodeString(ccData942c)
	if err != nil {
		t.Fatal(err)
	}
	obu := CreateCTA608MetadataOBU(ccData)
	obus, err := SplitOBUs(obu)
	if err != nil {
		t.Fatal(err)
	}
	m, err := ParseMetadataOBUFromOBU(obus[0])
	if err != nil {
		t.Fatal(err)
	}
	seiPayload := sei.CreateCTA608SEIMessage(ccData).Payload()
	if hex.EncodeToString(m.Payload) != hex.EncodeToString(seiPayload) {
		t.Errorf("OBU payload %s differs from SEI payload %s",
			hex.EncodeToString(m.Payload), hex.EncodeToString(seiPayload))
	}
}

func TestExtractCTA608(t *testing.T) {
	// A temporal unit as found in an av01 sample: temporal delimiter, the caption
	// metadata OBU, and a frame OBU.
	td := OBU{Header: OBUHeader{Type: OBUTemporalDelimiter, HasSizeField: true, HeaderSize: 1}}
	frame := OBU{Header: OBUHeader{Type: OBUFrame, HasSizeField: true, HeaderSize: 1}, Payload: []byte{0x10, 0x20}}

	cases := []struct {
		name       string
		ccData     string
		wantField1 string
		wantField2 string
	}{
		{
			name:       "field 1 only",
			ccData:     ccData942c,
			wantField1: "942c",
		},
		{
			name:       "both fields",
			ccData:     "c2ff" + "fc942c" + "fd1529" + "ff",
			wantField1: "942c",
			wantField2: "1529",
		},
		{
			name:   "no valid pairs",
			ccData: "c2ff" + "f80000" + "f90000" + "ff",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ccData, err := hex.DecodeString(c.ccData)
			if err != nil {
				t.Fatal(err)
			}
			tu := append([]byte{}, td.Encode()...)
			tu = append(tu, CreateCTA608MetadataOBU(ccData)...)
			tu = append(tu, frame.Encode()...)

			obus, err := SplitOBUs(tu)
			if err != nil {
				t.Fatalf("SplitOBUs: %v", err)
			}
			if len(obus) != 3 {
				t.Fatalf("got %d OBUs, want 3", len(obus))
			}
			field1, field2, err := ExtractCTA608(obus)
			if err != nil {
				t.Fatalf("ExtractCTA608: %v", err)
			}
			if hex.EncodeToString(field1) != c.wantField1 {
				t.Errorf("field1: got %s, want %s", hex.EncodeToString(field1), c.wantField1)
			}
			if hex.EncodeToString(field2) != c.wantField2 {
				t.Errorf("field2: got %s, want %s", hex.EncodeToString(field2), c.wantField2)
			}
		})
	}
}

func TestExtractCTA608SkipsOtherOBUs(t *testing.T) {
	hdrCLL := MetadataOBU{Type: MetadataTypeHDRCLL, Payload: []byte{0x03, 0xe8, 0x01, 0x90}}
	// HDR10+ shares country code 0xb5 with CTA-608 but uses provider code 0x003c.
	hdr10Plus := ITUTT35{CountryCode: 0xb5, Payload: []byte{0x00, 0x3c, 0x00, 0x01, 0x04, 0x00, 0x40}}
	seqHdr := OBU{Header: OBUHeader{Type: OBUSequenceHeader, HasSizeField: true, HeaderSize: 1}, Payload: []byte{0x00}}

	tu := append([]byte{}, seqHdr.Encode()...)
	tu = append(tu, hdrCLL.Encode()...)
	tu = append(tu, hdr10Plus.MetadataOBU().Encode()...)

	obus, err := SplitOBUs(tu)
	if err != nil {
		t.Fatal(err)
	}
	field1, field2, err := ExtractCTA608(obus)
	if err != nil {
		t.Fatalf("ExtractCTA608: %v", err)
	}
	if field1 != nil || field2 != nil {
		t.Errorf("got fields %x / %x, want none", field1, field2)
	}
}

func TestExtractCTA608Errors(t *testing.T) {
	// A metadata OBU whose metadata_type LEB128 never terminates.
	broken := OBU{Header: OBUHeader{Type: OBUMetadata, HasSizeField: true, HeaderSize: 1}, Payload: []byte{0x80}}
	if _, _, err := ExtractCTA608([]OBU{broken}); err == nil {
		t.Error("expected error for unterminated metadata_type, got nil")
	}
	// A CTA-608 payload whose cc_data() is truncated: cc_count promises 3 constructs.
	ccData, err := hex.DecodeString("c3ff" + "fc942c")
	if err != nil {
		t.Fatal(err)
	}
	obus, err := SplitOBUs(CreateCTA608MetadataOBU(ccData))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := ExtractCTA608(obus); err == nil {
		t.Error("expected error for truncated cc_data, got nil")
	}
}

func TestITUTT35ITUData(t *testing.T) {
	ccData, err := hex.DecodeString(ccData942c)
	if err != nil {
		t.Fatal(err)
	}
	obus, err := SplitOBUs(CreateCTA608MetadataOBU(ccData))
	if err != nil {
		t.Fatal(err)
	}
	m, err := ParseMetadataOBUFromOBU(obus[0])
	if err != nil {
		t.Fatal(err)
	}
	tt35, err := ParseITUTT35(m.Payload)
	if err != nil {
		t.Fatal(err)
	}
	data, ok := tt35.ITUData()
	if !ok {
		t.Fatal("ITUData not recognized")
	}
	if data != sei.CTA608ITUData() {
		t.Errorf("got %+v, want %+v", data, sei.CTA608ITUData())
	}
	if !data.IsCTA608() {
		t.Error("IsCTA608 is false")
	}
	if hex.EncodeToString(tt35.CTA608CCData()) != ccData942c {
		t.Errorf("cc_data: got %s, want %s", hex.EncodeToString(tt35.CTA608CCData()), ccData942c)
	}
}

func TestITUTT35ITUDataNotRecognized(t *testing.T) {
	cases := []struct {
		name string
		tt35 ITUTT35
	}{
		{
			name: "too short for the identification",
			tt35: ITUTT35{CountryCode: 0xb5, Payload: []byte{0x00, 0x31}},
		},
		{
			name: "country code with extension byte",
			tt35: ITUTT35{CountryCode: 0xff, CountryCodeExtension: 0x2a,
				Payload: []byte{0x00, 0x31, 0x47, 0x41, 0x39, 0x34, 0x03, 0xc3}},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, ok := c.tt35.ITUData(); ok {
				t.Error("ITUData: got ok, want not ok")
			}
			if got := c.tt35.CTA608CCData(); got != nil {
				t.Errorf("CTA608CCData: got %x, want nil", got)
			}
		})
	}
	// A well-formed identification that is not CTA-608 carries no cc_data either.
	notCC := ITUTT35{CountryCode: 0xb5, Payload: []byte{0x00, 0x3c, 0x00, 0x01, 0x04, 0x00, 0x40, 0x00}}
	if got := notCC.CTA608CCData(); got != nil {
		t.Errorf("HDR10+: got cc_data %x, want nil", got)
	}
	// A CTA-608 identification with an empty cc_data() is not usable either.
	empty := ITUTT35{CountryCode: 0xb5, Payload: sei.CTA608ITUData().Encode()[1:]}
	if got := empty.CTA608CCData(); got != nil {
		t.Errorf("empty cc_data: got %x, want nil", got)
	}
}
