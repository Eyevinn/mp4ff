package mp4_test

import (
	"testing"

	"github.com/Eyevinn/mp4ff/mp4"
)

func TestIprp(t *testing.T) {
	ispe := &mp4.IspeBox{ImageWidth: 600, ImageHeight: 338}
	pixi := &mp4.PixiBox{BitsPerChannel: []byte{8, 8, 8}}
	ipco := &mp4.IpcoBox{}
	ipco.AddChild(ispe)
	ipco.AddChild(mp4.NewFreeBox([]byte{0, 0}))
	ipco.AddChild(pixi)
	ipco.AddChild(mp4.CreateUnknownBox("zzzz", 11, []byte{1, 2, 3}))
	ipma := &mp4.IpmaBox{Entries: []mp4.IpmaEntry{
		{ItemID: 1, Associations: []mp4.IpmaAssociation{{PropertyIndex: 1}, {PropertyIndex: 3}}},
	}}
	largeIpma := &mp4.IpmaBox{Version: 1, Flags: mp4.IpmaFlagLargeIndex, Entries: []mp4.IpmaEntry{
		{ItemID: 0x10000, Associations: []mp4.IpmaAssociation{{Essential: true, PropertyIndex: 4}}},
	}}
	iprp := &mp4.IprpBox{}
	iprp.AddChild(ipco)
	iprp.AddChild(ipma)
	iprp.AddChild(largeIpma)
	boxDiffAfterEncodeAndDecode(t, iprp)

	if iprp.Ipco != ipco || len(iprp.Ipmas) != 2 || iprp.Ipmas[0] != ipma || iprp.Ipmas[1] != largeIpma {
		t.Error("ipco and ipma children not set")
	}
	// The property index is one-based and counts the free box.
	cases := []struct {
		index uint16
		want  mp4.Box
	}{{0, nil}, {1, ispe}, {3, pixi}, {4, ipco.Children[3]}, {5, nil}}
	for _, c := range cases {
		if got := ipco.GetProperty(c.index); got != c.want {
			t.Errorf("property %d: got %v, want %v", c.index, got, c.want)
		}
	}
}
