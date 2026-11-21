package mp4_test

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"os"
	"strings"
	"testing"

	"github.com/go-test/deep"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/Eyevinn/mp4ff/mp4"
)

// propertyTypes returns the box types of props with a star after the essential ones, such as "colr* ispe".
func propertyTypes(props []mp4.ItemProperty) string {
	var types []string
	for _, p := range props {
		name := "nil"
		if p.Property != nil {
			name = p.Property.Type()
		}
		if p.Essential {
			name += "*"
		}
		types = append(types, name)
	}
	return strings.Join(types, " ")
}

// TestItemsOfGridImage gets the items of an HEIC file with a grid of two tiles, an alpha plane as a second
// grid, Exif and XMP, with the mdat box in memory and decoded lazily.
func TestItemsOfGridImage(t *testing.T) {
	raw, err := os.ReadFile("testdata/gradient_grid_alpha.heic")
	if err != nil {
		t.Fatal(err)
	}
	const gridID, alphaGridID, exifID, xmpID = 3, 6, 7, 8
	decoders := []struct {
		name   string
		rs     io.ReadSeeker // the file, for a lazily decoded mdat
		decode func() (*mp4.File, error)
	}{
		{"DecodeFile", nil, func() (*mp4.File, error) { return mp4.DecodeFile(bytes.NewReader(raw)) }},
		{"DecodeFileSR", nil, func() (*mp4.File, error) { return mp4.DecodeFileSR(bits.NewFixedSliceReader(raw)) }},
		{"lazy mdat", bytes.NewReader(raw), func() (*mp4.File, error) {
			return mp4.DecodeFile(bytes.NewReader(raw), mp4.WithDecodeMode(mp4.DecModeLazyMdat))
		}},
	}
	for _, d := range decoders {
		t.Run(d.name, func(t *testing.T) {
			f, err := d.decode()
			if err != nil {
				t.Fatal(err)
			}
			meta := f.Meta
			if meta == nil || meta.Pitm == nil || meta.Pitm.ItemID != gridID {
				t.Fatalf("no file-level meta box with primary item %d", gridID)
			}

			// The primary item is a grid of one row with two tiles, whose data is in the idat box.
			data, err := meta.GetItemData(gridID, f.Mdat, d.rs)
			if err != nil {
				t.Fatal(err)
			}
			grid, err := mp4.DecodeImageGrid(data)
			if err != nil {
				t.Fatal(err)
			}
			if want := (mp4.ImageGrid{Rows: 1, Columns: 2, OutputWidth: 1024, OutputHeight: 64}); *grid != want {
				t.Errorf("grid %+v, want %+v", *grid, want)
			}
			if got := propertyTypes(meta.Iprp.GetItemProperties(gridID)); got != "colr* clli ispe pixi irot*" {
				t.Errorf("properties of the grid: %s", got)
			}

			refs := []struct {
				desc string
				got  []uint32
				want []uint32
			}{
				{"tiles of the grid", meta.Iref.GetToItemIDs("dimg", gridID), []uint32{1, 2}},
				{"tiles of a tile", meta.Iref.GetToItemIDs("dimg", 1), nil},
				{"auxiliary images of the grid", meta.Iref.GetFromItemIDs("auxl", gridID), []uint32{alphaGridID}},
				{"metadata of the grid", meta.Iref.GetFromItemIDs("cdsc", gridID), []uint32{exifID, xmpID}},
				{"thumbnails of the grid", meta.Iref.GetFromItemIDs("thmb", gridID), nil},
			}
			for _, r := range refs {
				if diff := deep.Equal(r.got, r.want); diff != nil {
					t.Errorf("%s: %v", r.desc, diff)
				}
			}

			// The tiles and the metadata are in the mdat box, at the offsets that iloc gives.
			for _, c := range []struct {
				itemID      uint32
				offset, len int
			}{{1, 1710, 738}, {2, 2448, 348}, {exifID, 1066, 72}, {xmpID, 1138, 572}} {
				data, err := meta.GetItemData(c.itemID, f.Mdat, d.rs)
				if err != nil {
					t.Errorf("item %d: %v", c.itemID, err)
				}
				if !bytes.Equal(data, raw[c.offset:c.offset+c.len]) {
					t.Errorf("item %d: got %d bytes that differ from the %d bytes at %d", c.itemID, len(data), c.len, c.offset)
				}
			}
			if got := propertyTypes(meta.Iprp.GetItemProperties(1)); got != "ispe* colr* clli hvcC*" {
				t.Errorf("properties of tile 1: %s", got)
			}

			// The alpha plane is an auxiliary image with the alpha type of HEVC.
			props := meta.Iprp.GetItemProperties(alphaGridID)
			if got := propertyTypes(props); got != "ispe pixi auxC* irot*" {
				t.Fatalf("properties of the alpha plane: %s", got)
			}
			if auxC := props[2].Property.(*mp4.AuxCBox); auxC.AuxType != "urn:mpeg:hevc:2015:auxid:1" || props[2].Index != 8 {
				t.Errorf("auxC %+v with index %d", auxC, props[2].Index)
			}

			if meta.Iloc.GetItem(99) != nil || meta.Iprp.GetItemProperties(99) != nil {
				t.Error("item 99 has a location or properties")
			}
			if _, err := meta.GetItemData(99, f.Mdat, d.rs); err == nil || err.Error() != "item 99 has no location in iloc" {
				t.Errorf("data of item 99: %v", err)
			}
		})
	}

	// A lazily decoded mdat needs the file.
	f, err := decoders[2].decode()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Meta.GetItemData(1, f.Mdat, nil); err == nil {
		t.Error("no error for a lazy mdat without the file")
	}
}

func TestItemsOfAVIF(t *testing.T) {
	raw, err := os.ReadFile("testdata/ex60.avif")
	if err != nil {
		t.Fatal(err)
	}
	f, err := mp4.DecodeFileSR(bits.NewFixedSliceReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	itemID := f.Meta.Pitm.ItemID
	props := f.Meta.Iprp.GetItemProperties(itemID)
	if got := propertyTypes(props); got != "av1C* ispe pixi" {
		t.Fatalf("properties: %s", got)
	}
	if ispe := props[1].Property.(*mp4.IspeBox); ispe.ImageWidth != 600 || ispe.ImageHeight != 338 {
		t.Errorf("ispe: %+v", ispe)
	}
	data, err := f.Meta.GetItemData(itemID, f.Mdat, nil)
	if err != nil {
		t.Fatal(err)
	}
	// The image is all that the mdat box holds.
	if !bytes.Equal(data, f.Mdat.Data) || len(data) == 0 {
		t.Errorf("item data of %d bytes differs from the mdat payload of %d bytes", len(data), len(f.Mdat.Data))
	}
}

func TestGetItemProperties(t *testing.T) {
	ipco := &mp4.IpcoBox{}
	ipco.AddChild(&mp4.IspeBox{ImageWidth: 4, ImageHeight: 2})
	ipco.AddChild(&mp4.IrotBox{Angle: 1})
	ipma0 := &mp4.IpmaBox{Entries: []mp4.IpmaEntry{
		{ItemID: 1, Associations: []mp4.IpmaAssociation{{PropertyIndex: 2, Essential: true}, {PropertyIndex: 0}, {PropertyIndex: 3}}},
	}}
	ipma1 := &mp4.IpmaBox{Version: 1, Entries: []mp4.IpmaEntry{
		{ItemID: 70000, Associations: []mp4.IpmaAssociation{{PropertyIndex: 1}}},
		{ItemID: 1, Associations: []mp4.IpmaAssociation{{PropertyIndex: 1}}},
	}}
	iprp := &mp4.IprpBox{}
	iprp.AddChild(ipco)
	iprp.AddChild(ipma0)
	iprp.AddChild(ipma1)

	// Index 0 is no property, index 3 is beyond the two properties, and the second ipma box adds one.
	want := []mp4.ItemProperty{
		{Essential: true, Index: 2, Property: ipco.Children[1]},
		{Index: 3},
		{Index: 1, Property: ipco.Children[0]},
	}
	if diff := deep.Equal(iprp.GetItemProperties(1), want); diff != nil {
		t.Error(diff)
	}
	if got := propertyTypes(iprp.GetItemProperties(70000)); got != "ispe" {
		t.Errorf("properties of item 70000: %s", got)
	}

	withoutIpco := &mp4.IprpBox{Ipmas: []*mp4.IpmaBox{ipma1}}
	if got := propertyTypes(withoutIpco.GetItemProperties(1)); got != "nil" {
		t.Errorf("properties without ipco: %s", got)
	}
}

func TestGetItemData(t *testing.T) {
	// An mdat box at 100, so that its 10 bytes of payload are at 108 to 117, and an idat box of 6 bytes.
	mdat := &mp4.MdatBox{StartPos: 100}
	mdat.SetData([]byte("0123456789"))
	idat := &mp4.IdatBox{Data: []byte("abcdef")}
	file, idatMethod := byte(mp4.IlocConstructionFile), byte(mp4.IlocConstructionIdat)

	cases := []struct {
		desc    string
		item    mp4.IlocItem
		noMdat  bool
		noIdat  bool
		want    string
		wantErr string
	}{
		{desc: "whole mdat payload", item: mp4.IlocItem{ConstructionMethod: file, Extents: []mp4.IlocExtent{{Offset: 108, Length: 10}}},
			want: "0123456789"},
		{desc: "base offset and extent offset", item: mp4.IlocItem{ConstructionMethod: file, BaseOffset: 100,
			Extents: []mp4.IlocExtent{{Offset: 10, Length: 3}}}, want: "234"},
		{desc: "two extents out of order", item: mp4.IlocItem{ConstructionMethod: file,
			Extents: []mp4.IlocExtent{{Offset: 116, Length: 2}, {Offset: 108, Length: 1}}}, want: "890"},
		{desc: "no extents", item: mp4.IlocItem{ConstructionMethod: file}, want: ""},
		{desc: "extent starts before the payload", item: mp4.IlocItem{ConstructionMethod: file,
			Extents: []mp4.IlocExtent{{Offset: 107, Length: 2}}},
			wantErr: "item 1: extent 1 at 107 is outside the item data source"},
		{desc: "extent ends after the payload", item: mp4.IlocItem{ConstructionMethod: file,
			Extents: []mp4.IlocExtent{{Offset: 108, Length: 11}}},
			wantErr: "item 1: extent 1 at 108 of 11 bytes does not fit in the item data source"},
		{desc: "extents larger than the payload together", item: mp4.IlocItem{ConstructionMethod: file,
			Extents: []mp4.IlocExtent{{Offset: 108, Length: 10}, {Offset: 108, Length: 1}}},
			wantErr: "item 1: extent 2 at 108 of 1 bytes does not fit in the item data source"},
		{desc: "offset overflow", item: mp4.IlocItem{ConstructionMethod: file, BaseOffset: math.MaxUint64,
			Extents: []mp4.IlocExtent{{Offset: 110, Length: 1}}},
			wantErr: "item 1: extent 1 at 109 is outside the item data source"},
		{desc: "length overflow", item: mp4.IlocItem{ConstructionMethod: file,
			Extents: []mp4.IlocExtent{{Offset: 108, Length: math.MaxUint64}, {Offset: 108, Length: 2}}},
			wantErr: "item 1: extent 1 at 108 of 18446744073709551615 bytes does not fit in the item data source"},
		{desc: "extent without length in the file", item: mp4.IlocItem{ConstructionMethod: file,
			Extents: []mp4.IlocExtent{{Offset: 108}}},
			wantErr: "item 1: extent 1 has no length, which is only supported in an idat box"},
		{desc: "another file", item: mp4.IlocItem{ConstructionMethod: file, DataReferenceIndex: 1,
			Extents: []mp4.IlocExtent{{Offset: 108, Length: 1}}}, wantErr: "item 1 is in another file (data reference 1)"},
		{desc: "no mdat", item: mp4.IlocItem{ConstructionMethod: file, Extents: []mp4.IlocExtent{{Offset: 108, Length: 1}}},
			noMdat: true, wantErr: "item 1 is in the file, but there is no mdat box"},
		{desc: "end of idat", item: mp4.IlocItem{ConstructionMethod: idatMethod, Extents: []mp4.IlocExtent{{Offset: 4, Length: 2}}},
			want: "ef"},
		{desc: "idat extent without length", item: mp4.IlocItem{ConstructionMethod: idatMethod,
			Extents: []mp4.IlocExtent{{Offset: 2}}}, want: "cdef"},
		{desc: "extent ends after idat", item: mp4.IlocItem{ConstructionMethod: idatMethod,
			Extents: []mp4.IlocExtent{{Offset: 4, Length: 3}}},
			wantErr: "item 1: extent 1 at 4 of 3 bytes does not fit in the item data source"},
		{desc: "extent starts after idat", item: mp4.IlocItem{ConstructionMethod: idatMethod,
			Extents: []mp4.IlocExtent{{Offset: 7, Length: 1}}},
			wantErr: "item 1: extent 1 at 7 is outside the item data source"},
		{desc: "no idat", item: mp4.IlocItem{ConstructionMethod: idatMethod, Extents: []mp4.IlocExtent{{Length: 1}}},
			noIdat: true, wantErr: "item 1 is in the idat box, but there is none"},
		{desc: "made from other items", item: mp4.IlocItem{ConstructionMethod: mp4.IlocConstructionItem,
			Extents: []mp4.IlocExtent{{Length: 1}}}, wantErr: "item 1 has construction method 2, which is not supported"},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			c.item.ItemID = 1
			meta := &mp4.MetaBox{Iloc: &mp4.IlocBox{Items: []mp4.IlocItem{c.item}}}
			if !c.noIdat {
				meta.Idat = idat
			}
			var m *mp4.MdatBox
			if !c.noMdat {
				m = mdat
			}
			data, err := meta.GetItemData(1, m, nil)
			if got := fmt.Sprint(err); c.wantErr != "" && got != c.wantErr {
				t.Fatalf("got error %q, want %q", got, c.wantErr)
			}
			if c.wantErr == "" && (err != nil || string(data) != c.want) {
				t.Errorf("got %q and error %v, want %q", data, err, c.want)
			}
		})
	}

	var noMeta *mp4.MetaBox
	if _, err := noMeta.GetItemData(1, mdat, nil); err == nil || err.Error() != "no iloc box" {
		t.Errorf("nil meta box: %v", err)
	}
	if _, err := (&mp4.MetaBox{}).GetItemData(1, mdat, nil); err == nil || err.Error() != "no iloc box" {
		t.Errorf("meta box without iloc: %v", err)
	}
}

func TestDecodeImageGrid(t *testing.T) {
	cases := []struct {
		desc    string
		data    []byte
		want    mp4.ImageGrid
		wantErr string
	}{
		{desc: "16-bit size", data: []byte{0, 0, 5, 7, 0x0f, 0xc0, 0x0b, 0xd0},
			want: mp4.ImageGrid{Rows: 6, Columns: 8, OutputWidth: 4032, OutputHeight: 3024}},
		{desc: "32-bit size and the largest grid", data: []byte{0, 1, 255, 255, 0, 1, 0, 0, 0, 2, 0, 0},
			want: mp4.ImageGrid{Rows: 256, Columns: 256, OutputWidth: 65536, OutputHeight: 131072}},
		{desc: "7 bytes", data: make([]byte, 7), wantErr: "image grid: 7 bytes are too few"},
		{desc: "version 1", data: []byte{1, 0, 0, 0, 0, 1, 0, 1}, wantErr: "image grid: version 1 is not supported"},
		{desc: "16-bit size in 12 bytes", data: make([]byte, 12),
			wantErr: "image grid: 12 bytes do not match the field size of its flags 0x00"},
		{desc: "32-bit size in 8 bytes", data: []byte{0, 1, 0, 0, 0, 1, 0, 1},
			wantErr: "image grid: 8 bytes do not match the field size of its flags 0x01"},
	}
	for _, c := range cases {
		grid, err := mp4.DecodeImageGrid(c.data)
		if c.wantErr != "" {
			if err == nil || err.Error() != c.wantErr {
				t.Errorf("%s: got error %v, want %q", c.desc, err, c.wantErr)
			}
			continue
		}
		if err != nil || *grid != c.want {
			t.Errorf("%s: got %+v and error %v, want %+v", c.desc, grid, err, c.want)
		}
	}
}

// FuzzItemData gets the properties and the data of all items of image files with malformed boxes.
func FuzzItemData(f *testing.F) {
	for _, name := range []string{"ex60.avif", "gradient_grid_alpha.heic"} {
		raw, err := os.ReadFile("testdata/" + name)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(raw)
	}
	f.Fuzz(func(t *testing.T, raw []byte) {
		file, err := mp4.DecodeFileSR(bits.NewFixedSliceReader(raw))
		if err != nil || file.Meta == nil || file.Meta.Iloc == nil {
			return
		}
		meta := file.Meta
		for _, item := range meta.Iloc.Items {
			data, err := meta.GetItemData(item.ItemID, file.Mdat, nil)
			if err == nil {
				_, _ = mp4.DecodeImageGrid(data)
			}
			if meta.Iprp != nil {
				_ = meta.Iprp.GetItemProperties(item.ItemID)
			}
			if meta.Iref != nil {
				_ = meta.Iref.GetToItemIDs("dimg", item.ItemID)
				_ = meta.Iref.GetFromItemIDs("auxl", item.ItemID)
			}
		}
	})
}
