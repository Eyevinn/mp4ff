package sei_test

import (
	"bytes"
	"errors"
	"io"
	"runtime"
	"testing"

	"github.com/Eyevinn/mp4ff/sei"
)

// seiValue returns the bytes of an SEI payload type or payload size value.
func seiValue(v int) []byte {
	out := bytes.Repeat([]byte{0xff}, v/255)
	return append(out, byte(v%255))
}

func TestExtractSEIDataPayloadSizeBeyondData(t *testing.T) {
	data := append([]byte{0x05}, seiValue(4095*255+254)...) // payload type 5 and a 1 MiB payload size
	data = append(data, 0x11, 0x22)                         // two bytes of payload
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	_, err := sei.ExtractSEIData(bytes.NewReader(data))
	runtime.ReadMemStats(&after)
	if !errors.Is(err, io.EOF) {
		t.Errorf("got error %v, want io.EOF", err)
	}
	if alloc := after.TotalAlloc - before.TotalAlloc; alloc > 64<<10 {
		t.Errorf("allocated %d bytes for %d bytes of input", alloc, len(data))
	}
}

func TestExtractSEIDataLargePayload(t *testing.T) {
	payload := bytes.Repeat([]byte{0x11, 0x22, 0x33}, 3000)
	data := append([]byte{0x05}, seiValue(len(payload))...)
	data = append(data, payload...)
	data = append(data, 0x80) // rbsp_trailing_bits
	seiData, err := sei.ExtractSEIData(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if len(seiData) != 1 || seiData[0].Type() != 5 || !bytes.Equal(seiData[0].Payload(), payload) {
		t.Errorf("got %d SEI messages, want one of type 5 with a %d-byte payload", len(seiData), len(payload))
	}
}
