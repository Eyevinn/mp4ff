package mp4_test

import (
	"bytes"
	"encoding/hex"
	"os"
	"strings"
	"testing"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/Eyevinn/mp4ff/mp4"
)

func TestMvhd(t *testing.T) {
	mvhd := mp4.CreateMvhd()
	boxDiffAfterEncodeAndDecode(t, mvhd)

	recentTime := int64(1721459921)
	mvhd.SetCreationTimeS(recentTime)
	mvhd.SetModificationTimeS(recentTime)
	if mvhd.CreationTimeS() != recentTime {
		t.Errorf("CreationTimeS %d not %d", mvhd.CreationTimeS(), recentTime)
	}
	if mvhd.ModificationTimeS() != recentTime {
		t.Errorf("ModificationTimeS %d not %d", mvhd.ModificationTimeS(), recentTime)
	}
}

func TestMvhdTimeDecodeS(t *testing.T) {
	data, err := os.ReadFile("testdata/mvhd_1970.dat")
	if err != nil {
		t.Error(err)
	}
	reader := bytes.NewReader(data)
	box, err := mp4.DecodeBox(0, reader)
	if err != nil {
		t.Error(err)
	}
	mvhd, ok := box.(*mp4.MvhdBox)
	if !ok {
		t.Errorf("Not a MvhdBox %+v", box)
	}
	if mvhd.CreationTimeS() != 0 {
		t.Errorf("CreationTimeS %d not 0", mvhd.CreationTimeS())
	}
	if mvhd.ModificationTimeS() != 0 {
		t.Errorf("ModificationTimeS %d not 0", mvhd.ModificationTimeS())
	}
}

// TestMovieAndTrackHeaderFields decodes mvhd, mdhd and tkhd boxes from files in the FFmpeg FATE suite with a movie
// matrix, or with QuickTime values in ISO pre_defined and reserved fields, and checks that the values are kept
// and that the boxes encode to the same bytes.
func TestMovieAndTrackHeaderFields(t *testing.T) {
	cases := []struct {
		desc     string
		hex      string
		check    func(b mp4.Box) bool
		wantInfo string
	}{
		{
			desc: "mvhd matrix of mov/displaymatrix.mov",
			hex: "0000006c6d76686400000000c44e602ec44e602f00000258000037580001000001000000000000000000000000" +
				"010000000000000000000000000000000200000000000000000000000000004000000000000000000000000000" +
				"000000000000000000000000000000000002",
			check: func(b mp4.Box) bool {
				return b.(*mp4.MvhdBox).Matrix == [9]int32{0x10000, 0, 0, 0, 0x20000, 0, 0, 0, 0x40000000}
			},
		},
		{
			desc: "mvhd selection time of quickdraw/Airplane.mov",
			hex: "0000006c6d76686400000000b8ac0d78b8ac0d78000002580000027b0001000000ff000000000000000000000001" +
				"000000000000000000000000000000010000000000000000000000000000400000000000000000000000000000" +
				"000000027b000000000000000000000005",
			check:    func(b mp4.Box) bool { return b.(*mp4.MvhdBox).QuickTimeSelectionTime == 635 },
			wantInfo: " - quickTimeSelectionTime: 635\n",
		},
		{
			desc:     "mdhd quality of qtrle/criticalpath-credits.mov",
			hex:      "000000206d64686400000000a8db0690a8db069100000258000019f00000003c",
			check:    func(b mp4.Box) bool { return b.(*mp4.MdhdBox).QuickTimeQuality == 60 },
			wantInfo: " - quickTimeQuality: 60\n",
		},
		{
			desc: "tkhd reserved field of iv32/cubes.mov",
			hex: "0000005c746b68640000000fb125e85ab125e85a00000001000002580000096000000000000000000000000000" +
				"00000000010000000000000000000000000000000100000000000000000000000000004000000000a000000078" +
				"0000",
			check:    func(b mp4.Box) bool { return b.(*mp4.TkhdBox).ReservedAfterTrackID == 600 },
			wantInfo: " - reservedAfterTrackID: 600\n",
		},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			data, err := hex.DecodeString(c.hex)
			if err != nil {
				t.Fatal(err)
			}
			decoders := []struct {
				name   string
				decode func() (mp4.Box, error)
			}{
				{"DecodeBox", func() (mp4.Box, error) { return mp4.DecodeBox(0, bytes.NewReader(data)) }},
				{"DecodeBoxSR", func() (mp4.Box, error) { return mp4.DecodeBoxSR(0, bits.NewFixedSliceReader(data)) }},
			}
			for _, d := range decoders {
				box, err := d.decode()
				if err != nil {
					t.Fatalf("%s: %v", d.name, err)
				}
				if !c.check(box) {
					t.Errorf("%s: wrong value in %+v", d.name, box)
				}
				var buf bytes.Buffer
				if err := box.Encode(&buf); err != nil {
					t.Fatalf("%s: %v", d.name, err)
				}
				if !bytes.Equal(buf.Bytes(), data) {
					t.Errorf("%s: Encode gave\n%x\nwant\n%x", d.name, buf.Bytes(), data)
				}
				sw := bits.NewFixedSliceWriter(int(box.Size()))
				if err := box.EncodeSW(sw); err != nil {
					t.Fatalf("%s: %v", d.name, err)
				}
				if !bytes.Equal(sw.Bytes(), data) {
					t.Errorf("%s: EncodeSW gave\n%x\nwant\n%x", d.name, sw.Bytes(), data)
				}
				var info bytes.Buffer
				if err := box.Info(&info, "", "", "  "); err != nil {
					t.Fatalf("%s: %v", d.name, err)
				}
				if !strings.Contains(info.String(), c.wantInfo) {
					t.Errorf("%s: Info gave\n%s\nwant the line %q", d.name, info.String(), c.wantInfo)
				}
			}
		})
	}
}

func TestMvhdZeroMatrixIsUnity(t *testing.T) {
	mvhd := mp4.CreateMvhd()
	mvhd.Matrix = [9]int32{}
	sw := bits.NewFixedSliceWriter(int(mvhd.Size()))
	if err := mvhd.EncodeSW(sw); err != nil {
		t.Fatal(err)
	}
	box, err := mp4.DecodeBoxSR(0, bits.NewFixedSliceReader(sw.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if got := box.(*mp4.MvhdBox).Matrix; got != mp4.UnityMatrix() {
		t.Errorf("zero matrix encoded as %v, want the unity matrix", got)
	}
}
