package mp4_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Eyevinn/mp4ff/mp4"
)

func TestVmhd(t *testing.T) {

	vmhd := mp4.CreateVmhd()

	boxDiffAfterEncodeAndDecode(t, vmhd)
}

func TestVmhdInfo(t *testing.T) {
	cases := []struct {
		desc string
		vmhd mp4.VmhdBox
		want string
	}{
		{"ISO", mp4.VmhdBox{Flags: 1}, ""},
		{"QuickTime dither copy", mp4.VmhdBox{Flags: 1, GraphicsMode: 0x40, OpColor: [3]uint16{0x8000, 0x8000, 0x8000}},
			" - graphicsMode: 0x40 (dither copy)\n - opcolor: 32768 32768 32768\n"},
		{"opcolor only", mp4.VmhdBox{Flags: 1, OpColor: [3]uint16{0, 0, 1}},
			" - graphicsMode: 0x0 (copy)\n - opcolor: 0 0 1\n"},
		{"unknown mode", mp4.VmhdBox{Flags: 1, GraphicsMode: 0x105},
			" - graphicsMode: 0x105\n - opcolor: 0 0 0\n"},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			boxDiffAfterEncodeAndDecode(t, &c.vmhd)
			var info bytes.Buffer
			if err := c.vmhd.Info(&info, "", "", "  "); err != nil {
				t.Fatal(err)
			}
			if _, lines, _ := strings.Cut(info.String(), "\n"); lines != c.want {
				t.Errorf("got\n%s\nwant\n%s", lines, c.want)
			}
		})
	}
}
