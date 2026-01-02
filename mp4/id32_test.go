package mp4_test

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/Eyevinn/mp4ff/mp4"
)

func TestID32FromExample(t *testing.T) {
	// Test with the provided example:
	// [ID32] size=98
	// 0000000015c74944330400000000004a5052495600000040000068747470733a2f2f6769746875622e636f6d2f7368616b612d70726f6a6563742f
	// 7368616b612d7061636b616765720064373164333734622d72656c65617365

	// Breaking down the hex:
	// 00000062 - size (98 bytes)
	// 49443332 - 'ID32' box type
	// 00000000 - version 0, flags 0
	// 15c7 - language code (pad bit + 3x5 bit chars)
	// 4944330400000000004a505249560000004000006874747... - ID3v2 data

	payload := "0000000015c74944330400000000004a5052495600000040000068747470733a2f2f6769746875622e636f6d2f7368616b612d70726f6a6563742f" +
		"7368616b612d7061636b616765720064373164333734622d72656c65617365"

	payloadBytes, err := hex.DecodeString(payload)
	if err != nil {
		t.Fatal(err)
	}

	// Calculate total size: 8 (header) + len(payload)
	totalSize := 8 + len(payloadBytes)

	// Build complete box with header
	id32Hex := fmt.Sprintf("%08x", totalSize) + "49443332" + payload

	id32Bytes, err := hex.DecodeString(id32Hex)
	if err != nil {
		t.Fatal(err)
	}

	sr := bits.NewFixedSliceReader(id32Bytes)
	box, err := mp4.DecodeBoxSR(0, sr)
	if err != nil {
		t.Fatal(err)
	}

	id32, ok := box.(*mp4.ID32Box)
	if !ok {
		t.Fatalf("expected *mp4.ID32Box, got %T", box)
	}

	if id32.Version != 0 {
		t.Errorf("got version %d instead of 0", id32.Version)
	}

	if id32.Flags != 0 {
		t.Errorf("got flags %d instead of 0", id32.Flags)
	}

	// Verify language code extraction
	// 0x15c7 = 0001 0101 1100 0111
	// After removing pad bit: 101 0101 1100 0111
	// char1 = 01010 = 10 -> 'j' (10 + 0x60)
	t.Logf("Language: %s", id32.Language)

	// Verify ID3v2 data
	expectedDataHex := "4944330400000000004a5052495600000040000068747470733a2f2f6769746875622e636f6d2f7368616b612d70726f6a6563742f" +
		"7368616b612d7061636b616765720064373164333734622d72656c65617365"
	expectedData, _ := hex.DecodeString(expectedDataHex)

	if len(id32.ID3v2Data) != len(expectedData) {
		t.Errorf("ID3v2 data length: got %d, expected %d", len(id32.ID3v2Data), len(expectedData))
	}

	if !bytes.Equal(id32.ID3v2Data, expectedData) {
		t.Error("ID3v2 data mismatch")
	}
}

func TestCreateID32Box(t *testing.T) {
	// Create an ID32 box with sample data
	id3Data := []byte("ID3\x04\x00\x00\x00\x00\x00test data")

	id32 := &mp4.ID32Box{
		Version:   0,
		Flags:     0,
		Language:  "eng",
		ID3v2Data: id3Data,
	}

	if id32.Type() != "ID32" {
		t.Errorf("got type %s instead of ID32", id32.Type())
	}

	// Test encode/decode round-trip
	boxDiffAfterEncodeAndDecode(t, id32)
}
