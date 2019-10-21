package mp4

import (
	"bytes"
	"encoding/hex"
	"testing"
)

func TestEsdsEncodeAndDecode(t *testing.T) {

	decCfg := []byte{0x11, 0x90}

	esdsIn := CreateEsdsBox(decCfg)

	// Write to a buffer so that we can read and check
	var buf bytes.Buffer
	esdsIn.Encode(&buf)

	// Read back from buffer
	decodedBox, err := DecodeBox(0, &buf)
	if err != nil {
		t.Error("Did not get a box back")
	}
	esdsOut := decodedBox.(*EsdsBox)
	decCfgOut := esdsOut.DecConfig
	if bytes.Compare(decCfgOut, decCfg) != 0 {
		t.Errorf("Decode cfg out %s differs from decode cfg in %s",
			hex.EncodeToString(decCfgOut), hex.EncodeToString(decCfg))
	}
}
