package mp4_test

import (
	"strings"
	"testing"

	"github.com/Eyevinn/mp4ff/mp4"
)

func TestAuxC(t *testing.T) {
	const alpha = "urn:mpeg:mpegB:cicp:systems:auxiliary:alpha"
	withSubtype := &mp4.AuxCBox{AuxType: "urn:mpeg:hevc:2015:auxid:1", AuxSubtype: []byte{0, 0, 0, 8, 0x4e, 1, 0xa5, 4}}
	for _, auxC := range []*mp4.AuxCBox{{}, {AuxType: alpha}, withSubtype} {
		boxDiffAfterEncodeAndDecode(t, auxC)
	}

	var sb strings.Builder
	if err := withSubtype.Info(&sb, "", "", "  "); err != nil {
		t.Fatal(err)
	}
	want := `[auxC] size=47 version=0 flags=000000
 - auxType: "urn:mpeg:hevc:2015:auxid:1"
 - auxSubtype: 000000084e01a504
`
	if sb.String() != want {
		t.Errorf("got info:\n%swant:\n%s", sb.String(), want)
	}

	noTerminator := []byte{0, 0, 0, 15, 'a', 'u', 'x', 'C', 0, 0, 0, 0, 'u', 'r', 'n'}
	assertBoxDecodeError(t, noTerminator, 0, "decode auxC pos 0: auxC: did not find terminating zero")
}
