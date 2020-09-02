package mp4

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"testing"
)

const uuidTfxdRaw = "0000002c757569646d1d9b0542d544e680e2141daff757b201000000000105c649bda4000000000000054600"
const uuidTfrfRaw = "0000002d75756964d4807ef2ca3946958e5426cb9e46a79f0100000001000105c649c2ea000000000000054600"

func TestTfxd(t *testing.T) {

	inRawBox, _ := hex.DecodeString(uuidTfxdRaw)
	inbuf := bytes.NewBuffer(inRawBox)
	hdr, err := decodeHeader(inbuf)
	if err != nil {
		t.Error(err)
	}
	uuidRead, err := DecodeUUID(hdr, 0, inbuf)
	if err != nil {
		t.Error(err)
	}

	outbuf := &bytes.Buffer{}

	err = uuidRead.Encode(outbuf)
	if err != nil {
		t.Error(err)
	}

	outRawBox := outbuf.Bytes()

	if !bytes.Equal(inRawBox, outRawBox) {
		for i := 0; i < len(inRawBox); i++ {
			fmt.Printf("%3d %02x %02x\n", i, inRawBox[i], outRawBox[i])
		}
		t.Error("Non-matching in and out binaries")
	}
}

func TestTfrf(t *testing.T) {

	inRawBox, _ := hex.DecodeString(uuidTfrfRaw)
	inbuf := bytes.NewBuffer(inRawBox)
	hdr, err := decodeHeader(inbuf)
	if err != nil {
		t.Error(err)
	}
	uuidRead, err := DecodeUUID(hdr, 0, inbuf)
	if err != nil {
		t.Error(err)
	}

	outbuf := &bytes.Buffer{}

	err = uuidRead.Encode(outbuf)
	if err != nil {
		t.Error(err)
	}

	outRawBox := outbuf.Bytes()

	if !bytes.Equal(inRawBox, outRawBox) {
		for i := 0; i < len(inRawBox); i++ {
			fmt.Printf("%3d %02x %02x\n", i, inRawBox[i], outRawBox[i])
		}
		t.Error("Non-matching in and out binaries")
	}
}
