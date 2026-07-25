package sei_test

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/Eyevinn/mp4ff/sei"
)

func TestCTA608ITUDataEncode(t *testing.T) {
	// The T.35 header that identifies CTA-608: country USA, provider ATSC, "GA94",
	// user_data_type_code 0x03. Identical in an AVC/HEVC SEI message and an AV1 OBU.
	want := "b5003147413934" + "03"
	got := sei.CTA608ITUData().Encode()
	if hex.EncodeToString(got) != want {
		t.Errorf("got %s, want %s", hex.EncodeToString(got), want)
	}
	if len(got) != sei.ITUDataSize {
		t.Errorf("got %d bytes, want %d", len(got), sei.ITUDataSize)
	}
	if !sei.CTA608ITUData().IsCTA608() {
		t.Error("IsCTA608 is false for CTA608ITUData")
	}
}

func TestCreateCTA608SEIMessage(t *testing.T) {
	// Rebuild the CTA-608 SEI message used in TestParseSEI from its own cc_data():
	// the creator must reproduce the payload byte-exactly.
	rbsp, err := hex.DecodeString(seiCTA608Hex)
	if err != nil {
		t.Fatal(err)
	}
	seis, err := sei.ExtractSEIData(bytes.NewReader(rbsp))
	if err != nil {
		t.Fatal(err)
	}
	if len(seis) != 1 {
		t.Fatalf("got %d SEI messages, want 1", len(seis))
	}
	payload := seis[0].Payload()
	ccData := payload[sei.ITUDataSize:]

	msg := sei.CreateCTA608SEIMessage(ccData)
	if msg.Type() != sei.SEIUserDataRegisteredITUtT35Type {
		t.Errorf("got SEI type %d, want %d", msg.Type(), sei.SEIUserDataRegisteredITUtT35Type)
	}
	if !bytes.Equal(msg.Payload(), payload) {
		t.Errorf("got payload %s, want %s", hex.EncodeToString(msg.Payload()), hex.EncodeToString(payload))
	}
	if msg.Size() != uint(len(payload)) {
		t.Errorf("got size %d, want %d", msg.Size(), len(payload))
	}

	// The created message must survive the SEI framing and decode back to CTA-608.
	var buf bytes.Buffer
	if err := sei.WriteSEIMessages(&buf, []sei.SEIMessage{msg}); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(buf.Bytes(), rbsp) {
		t.Errorf("got RBSP %s, want %s", hex.EncodeToString(buf.Bytes()), hex.EncodeToString(rbsp))
	}
	back, err := sei.ExtractSEIData(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := sei.DecodeSEIMessage(&back[0], sei.AVC)
	if err != nil {
		t.Fatal(err)
	}
	cta, ok := decoded.(*sei.CTA608sei)
	if !ok {
		t.Fatalf("got %T, want *sei.CTA608sei", decoded)
	}
	wantField1 := "942094ae9162e56e67ba91b9b0b0bab0b0bab031bab0b080942c942f"
	if hex.EncodeToString(cta.Field1) != wantField1 {
		t.Errorf("field1: got %s, want %s", hex.EncodeToString(cta.Field1), wantField1)
	}
	if len(cta.Field2) != 0 {
		t.Errorf("field2: got %s, want empty", hex.EncodeToString(cta.Field2))
	}
}

func TestDecodeUserDataRegisteredSEIShortPayload(t *testing.T) {
	// A type-4 (user_data_registered_itu_t_t35) header is 8 bytes. A shorter
	// payload must return an error instead of panicking on out-of-range indexing.
	for size := 0; size < 8; size++ {
		sd := sei.NewSEIData(sei.SEIUserDataRegisteredITUtT35Type, make([]byte, size))
		msg, err := sei.DecodeUserDataRegisteredSEI(sd)
		if err == nil {
			t.Errorf("expected error for %d-byte type-4 payload, got nil (msg %v)", size, msg)
		}
	}
}

func TestExtractCTA608seiShortPayload(t *testing.T) {
	// ExtractCTA608sei is exported, so it must guard the ITUData header itself instead
	// of relying on the length check in DecodeUserDataRegisteredSEI.
	for size := 0; size < sei.ITUDataSize; size++ {
		sd := sei.NewSEIData(sei.SEIUserDataRegisteredITUtT35Type, make([]byte, size))
		cta, err := sei.ExtractCTA608sei(sd)
		if err == nil {
			t.Errorf("expected error for %d-byte type-4 payload, got nil (msg %v)", size, cta)
		}
	}
}

func TestParseCTA608EmptyPayload(t *testing.T) {
	// Empty cc_data payload must return an error instead of panicking on payload[0].
	if _, _, err := sei.ParseCTA608([]byte{}); err == nil {
		t.Error("expected error for empty CTA-608 payload, got nil")
	}
}
