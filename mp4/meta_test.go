package mp4_test

import (
	"bytes"
	"encoding/hex"
	"os"
	"strings"
	"testing"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/Eyevinn/mp4ff/mp4"
	"github.com/go-test/deep"
)

func TestMeta(t *testing.T) {
	hdlr, err := mp4.CreateHdlr("zzzz")
	if err != nil {
		t.Error(err)
	}
	meta := mp4.CreateMetaBox(0, hdlr)
	boxDiffAfterEncodeAndDecode(t, meta)
}

func TestQuickTimeMeta(t *testing.T) {
	quickTimeMetaAtom := (`000000416d6574610000002168646c7200000000000000006d64746100000000` +
		`000000000000000000000000106b657973000000000000000000000008696c7374`)
	data, err := hex.DecodeString(quickTimeMetaAtom)
	if err != nil {
		t.Error(err)
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
		meta, ok := box.(*mp4.MetaBox)
		if !ok {
			t.Fatalf("%s: box is not meta", d.name)
		}
		if !meta.IsQuickTime() {
			t.Errorf("%s: meta box not detected as QuickTime", d.name)
		}
		info := bytes.Buffer{}
		err = meta.Info(&info, "", "", "")
		if err != nil {
			t.Error(err)
		}
		if !strings.Contains(info.String(), "is QuickTime meta atom") {
			t.Errorf("%s: lacks QuickTime in info string", d.name)
		}
		outBuf := bytes.Buffer{}
		err = meta.Encode(&outBuf)
		if err != nil {
			t.Error(err)
		}
		if !bytes.Equal(data, outBuf.Bytes()) {
			t.Errorf("%s: Encode output for QuickTime meta differs from input", d.name)
		}
		sw := bits.NewFixedSliceWriter(int(meta.Size()))
		err = meta.EncodeSW(sw)
		if err != nil {
			t.Errorf("%s: EncodeSW: %v", d.name, err)
		}
		if !bytes.Equal(data, sw.Bytes()) {
			t.Errorf("%s: EncodeSW output for QuickTime meta differs from input", d.name)
		}
	}
}

// TestMetaImageItems checks the item boxes of an AVIF file with one image: that they give the image,
// its properties and its data, that the file is the same after decoding and encoding, and the Info output.
func TestMetaImageItems(t *testing.T) {
	const testFile = "testdata/ex60.avif"
	raw, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range fileDecoders(raw) {
		t.Run(d.name, func(t *testing.T) {
			f, err := d.decode()
			if err != nil {
				t.Fatal(err)
			}
			meta := imageMeta(t, f)
			itemID := meta.Pitm.ItemID
			infe := meta.Iinf.GetInfe(itemID)
			if infe == nil || infe.ItemType != "av01" {
				t.Fatalf("primary item %d is not an av01 item: %+v", itemID, infe)
			}

			// The properties of the image, by their index in ipco.
			ipma := meta.Iprp.Ipmas[0]
			wantAssociations := []mp4.IpmaAssociation{{Essential: true, PropertyIndex: 1}, {PropertyIndex: 2}, {PropertyIndex: 3}}
			if len(ipma.Entries) != 1 || ipma.Entries[0].ItemID != itemID {
				t.Fatalf("ipma is not for item %d: %+v", itemID, ipma.Entries)
			}
			if diff := deep.Equal(ipma.Entries[0].Associations, wantAssociations); diff != nil {
				t.Error(diff)
			}
			if _, ok := meta.Iprp.Ipco.GetProperty(1).(*mp4.Av1CBox); !ok {
				t.Error("property 1 is not av1C")
			}
			if ispe, ok := meta.Iprp.Ipco.GetProperty(2).(*mp4.IspeBox); !ok || ispe.ImageWidth != 600 || ispe.ImageHeight != 338 {
				t.Errorf("property 2 is not ispe with 600x338: %+v", meta.Iprp.Ipco.GetProperty(2))
			}
			if pixi, ok := meta.Iprp.Ipco.GetProperty(3).(*mp4.PixiBox); !ok || !bytes.Equal(pixi.BitsPerChannel, []byte{8, 8, 8}) {
				t.Errorf("property 3 is not pixi with 3 channels of 8 bits: %+v", meta.Iprp.Ipco.GetProperty(3))
			}

			// The image data is the payload of mdat.
			if len(meta.Iloc.Items) != 1 || meta.Iloc.Items[0].ItemID != itemID || len(meta.Iloc.Items[0].Extents) != 1 {
				t.Fatalf("iloc does not have one extent for item %d: %+v", itemID, meta.Iloc.Items)
			}
			item := meta.Iloc.Items[0]
			if item.ConstructionMethod != mp4.IlocConstructionFile || item.DataReferenceIndex != 0 {
				t.Errorf("item data is not in the file: %+v", item)
			}
			start := item.BaseOffset + item.Extents[0].Offset
			if start != f.Mdat.PayloadAbsoluteOffset() || item.Extents[0].Length != uint64(len(f.Mdat.Data)) {
				t.Errorf("item data at %d with length %d is not the mdat payload at %d with length %d",
					start, item.Extents[0].Length, f.Mdat.PayloadAbsoluteOffset(), len(f.Mdat.Data))
			}

			checkFileEncode(t, f, raw)
			if err = compareOrUpdateInfo(t, f, "testdata/golden_ex60_avif_dump.txt"); err != nil {
				t.Error(err)
			}
		})
	}
}

// TestMetaImageGrid checks the item boxes of a HEIC file with an image that is a grid of two tiles, with an
// alpha plane, Exif and XMP: the references between the items, the grid description in idat, the properties
// of the alpha plane, that the file is the same after decoding and encoding, and the Info output.
func TestMetaImageGrid(t *testing.T) {
	const testFile = "testdata/gradient_grid_alpha.heic"
	raw, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatal(err)
	}
	const (
		gridID, alphaGridID = 3, 6
		exifID, xmpID       = 7, 8
	)
	for _, d := range fileDecoders(raw) {
		t.Run(d.name, func(t *testing.T) {
			f, err := d.decode()
			if err != nil {
				t.Fatal(err)
			}
			meta := imageMeta(t, f)
			if meta.Iref == nil || meta.Idat == nil {
				t.Fatal("meta box lacks iref or idat")
			}
			if meta.Pitm.ItemID != gridID || meta.Iinf.GetInfe(gridID).ItemType != "grid" {
				t.Fatalf("primary item %d is not the grid item %d", meta.Pitm.ItemID, gridID)
			}

			wantRefs := []*mp4.IrefTypeBox{
				{Name: "dimg", FromItemID: gridID, ToItemIDs: []uint32{1, 2}},
				{Name: "dimg", FromItemID: alphaGridID, ToItemIDs: []uint32{4, 5}},
				{Name: "auxl", FromItemID: alphaGridID, ToItemIDs: []uint32{gridID}},
				{Name: "cdsc", FromItemID: exifID, ToItemIDs: []uint32{gridID}},
				{Name: "cdsc", FromItemID: xmpID, ToItemIDs: []uint32{gridID}},
			}
			if diff := deep.Equal(meta.Iref.Refs, wantRefs); diff != nil {
				t.Error(diff)
			}

			// The data of the grid item is in idat: one row of two columns giving an image of 1024x64 pixels.
			var grid *mp4.IlocItem
			for i := range meta.Iloc.Items {
				if meta.Iloc.Items[i].ItemID == gridID {
					grid = &meta.Iloc.Items[i]
				}
			}
			if grid == nil || grid.ConstructionMethod != mp4.IlocConstructionIdat || len(grid.Extents) != 1 {
				t.Fatalf("grid item does not have one extent in idat: %+v", grid)
			}
			start := grid.BaseOffset + grid.Extents[0].Offset
			gridData := meta.Idat.Data[start : start+grid.Extents[0].Length]
			if wantGrid := []byte{0, 0, 0, 1, 0x04, 0x00, 0x00, 0x40}; !bytes.Equal(gridData, wantGrid) {
				t.Errorf("grid data %x instead of %x", gridData, wantGrid)
			}

			// The properties of the alpha plane tell that it is one, and how it is rotated.
			var alphaProps []mp4.IpmaAssociation
			for _, e := range meta.Iprp.Ipmas[0].Entries {
				if e.ItemID == alphaGridID {
					alphaProps = e.Associations
				}
			}
			var auxC *mp4.AuxCBox
			var irot *mp4.IrotBox
			for _, a := range alphaProps {
				switch p := meta.Iprp.Ipco.GetProperty(a.PropertyIndex).(type) {
				case *mp4.AuxCBox:
					auxC = p
				case *mp4.IrotBox:
					irot = p
				}
			}
			if auxC == nil || auxC.AuxType != "urn:mpeg:hevc:2015:auxid:1" {
				t.Errorf("alpha plane lacks auxC with the HEVC alpha type: %+v", auxC)
			}
			if irot == nil || irot.Angle != 0 {
				t.Errorf("alpha plane lacks irot with angle 0: %+v", irot)
			}

			checkFileEncode(t, f, raw)
			if err = compareOrUpdateInfo(t, f, "testdata/golden_gradient_grid_alpha_heic_dump.txt"); err != nil {
				t.Error(err)
			}
		})
	}
}

// fileDecoder is one of the two ways to decode a file.
type fileDecoder struct {
	name   string
	decode func() (*mp4.File, error)
}

// fileDecoders returns the decoders of raw for the io.Reader and the SliceReader path.
func fileDecoders(raw []byte) []fileDecoder {
	return []fileDecoder{
		{"DecodeFile", func() (*mp4.File, error) { return mp4.DecodeFile(bytes.NewReader(raw)) }},
		{"DecodeFileSR", func() (*mp4.File, error) { return mp4.DecodeFileSR(bits.NewFixedSliceReader(raw)) }},
	}
}

// imageMeta returns the top-level meta box of an image file, with pitm, iloc, iinf and iprp.
func imageMeta(t *testing.T, f *mp4.File) *mp4.MetaBox {
	t.Helper()
	var meta *mp4.MetaBox
	for _, c := range f.Children {
		if m, ok := c.(*mp4.MetaBox); ok {
			meta = m
		}
	}
	if meta == nil || meta.Pitm == nil || meta.Iloc == nil || meta.Iinf == nil || meta.Iprp == nil {
		t.Fatal("meta box lacks pitm, iloc, iinf or iprp")
	}
	return meta
}

// checkFileEncode checks that f is encoded as raw by both Encode and EncodeSW.
func checkFileEncode(t *testing.T, f *mp4.File, raw []byte) {
	t.Helper()
	out := bytes.Buffer{}
	if err := f.Encode(&out); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out.Bytes(), raw) {
		t.Error("file differs after Encode")
	}
	sw := bits.NewFixedSliceWriter(len(raw))
	if err := f.EncodeSW(sw); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(sw.Bytes(), raw) {
		t.Error("file differs after EncodeSW")
	}
}
