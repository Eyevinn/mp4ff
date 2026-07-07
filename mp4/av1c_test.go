package mp4_test

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/Eyevinn/mp4ff/av1"
	"github.com/Eyevinn/mp4ff/mp4"
)

func TestEncodeDecodeAvc1(t *testing.T) {
	adc := mp4.Av1CBox{
		CodecConfRec: av1.CodecConfRec{
			Version: 1,
		},
	}

	boxDiffAfterEncodeAndDecode(t, &adc)

}

func TestAv1CInfo(t *testing.T) {
	// ConfigOBUs holds a real sequence header OBU (352x288, 8-bit) from the AOM fate-suite.
	configOBUs, err := hex.DecodeString("0a0b00000004457e3e7dfcc060")
	if err != nil {
		t.Fatal(err)
	}
	av1c := &mp4.Av1CBox{
		CodecConfRec: av1.CodecConfRec{
			Version:            1,
			SeqProfile:         0,
			SeqLevelIdx0:       0,
			ChromaSubsamplingX: 1,
			ChromaSubsamplingY: 1,
			ConfigOBUs:         configOBUs,
		},
	}
	var buf bytes.Buffer
	if err := av1c.Info(&buf, "", "", "  "); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"codecString: av01.0.00M.08",
		"width: 352",
		"height: 288",
		"bitDepth: 8",
		"fullCodecString: av01.0.00M.08.0.110.02.02.02.0",
	} {
		if !bytes.Contains(buf.Bytes(), []byte(want)) {
			t.Errorf("Info output missing %q\ngot:\n%s", want, buf.String())
		}
	}
}
