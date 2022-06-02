package mp4

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"testing"
)

func TestUUIDVariants(t *testing.T) {

	testInputs := []struct {
		expectedSubType string
		rawData         string
	}{
		{
			"tfxd", "0000002c757569646d1d9b0542d544e680e2141daff757b201000000000105c649bda4000000000000054600",
		},
		{
			"tfrf", "0000002d75756964d4807ef2ca3946958e5426cb9e46a79f0100000001000105c649c2ea000000000000054600",
		},
		{
			"unknown", "0000002c757569646e1d9b0542d544e680e2141daff757b201000000000105c649bda4000000000000054600",
		},
	}

	for _, ti := range testInputs {
		inRawBox, _ := hex.DecodeString(ti.rawData)
		inbuf := bytes.NewBuffer(inRawBox)
		hdr, err := DecodeHeader(inbuf)
		if err != nil {
			t.Error(err)
		}
		uuidRead, err := DecodeUUIDBox(hdr, 0, inbuf)
		if err != nil {
			t.Error(err)
		}
		uBox := uuidRead.(*UUIDBox)
		if uBox.SubType() != ti.expectedSubType {
			t.Errorf("got subtype %s instead of %s", uBox.SubType(), ti.expectedSubType)
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
			t.Errorf("%s: Non-matching in and out binaries", ti.expectedSubType)
		}
	}
}

func TestSetUUID(t *testing.T) {
	testCases := []struct {
		uuidStr    string
		expected   uuid
		shouldFail bool
	}{
		{
			uuidStr:    "6d1d9b05-42d5-44e6-80e2-141daff757b2",
			shouldFail: false,
		},
		{
			uuidStr:    "6d1d9b05-42d5-44e6-80e2-141daff757",
			shouldFail: true,
		},
	}
	for i, tc := range testCases {
		u := UUIDBox{}
		err := u.SetUUID(tc.uuidStr)
		if tc.shouldFail {
			if err == nil {
				t.Errorf("case %d did not fail as expected", i)
			}
			continue
		}
		if u.UUID() != tc.uuidStr {
			t.Errorf("got %s instead of %s", u.UUID(), tc.uuidStr)
		}
	}
}
