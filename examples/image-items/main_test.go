package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Eyevinn/mp4ff/mp4"
)

const (
	heicPath = "../../mp4/testdata/gradient_grid_alpha.heic"
	avifPath = "../../mp4/testdata/ex60.avif"
)

func TestCommandLine(t *testing.T) {
	cases := []struct {
		desc    string
		args    []string
		wantErr bool
	}{
		{desc: "help", args: []string{appName, "-h"}},
		{desc: "version", args: []string{appName, "-version"}},
		{desc: "no args", args: []string{appName}, wantErr: true},
		{desc: "unknown option", args: []string{appName, "-x"}, wantErr: true},
		{desc: "three files", args: []string{appName, heicPath, heicPath, heicPath}, wantErr: true},
		{desc: "missing image", args: []string{appName, "missing.heic"}, wantErr: true},
		{desc: "missing movie", args: []string{appName, heicPath, "missing.mov"}, wantErr: true},
		{desc: "movie as image", args: []string{appName, "../../mp4/testdata/prog_8s.mp4"}, wantErr: true},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			err := run(c.args, &bytes.Buffer{})
			if (err != nil) != c.wantErr {
				t.Errorf("got error %v, want error %t", err, c.wantErr)
			}
		})
	}
}

func TestListAndWriteItems(t *testing.T) {
	dir := t.TempDir()
	var out bytes.Buffer
	if err := run([]string{appName, "-o", dir, heicPath}, &out); err != nil {
		t.Fatal(err)
	}
	want := "item 3: grid 1024x64 in 1 rows and 2 columns of tiles of 512x512, primary image\n" +
		"  written to " + filepath.Join(dir, "item-3.hevc") + "\n" +
		"item 6: grid 1024x64 of type urn:mpeg:hevc:2015:auxid:1 in 1 rows and 2 columns of tiles of 512x512, " +
		"auxiliary image of item 3\n" +
		"  written to " + filepath.Join(dir, "item-6.hevc") + "\n" +
		"item 7: Exif 72 bytes, metadata of item 3\n" +
		"  written to " + filepath.Join(dir, "item-7.tif") + "\n" +
		"item 8: mime application/rdf+xml 572 bytes, metadata of item 3\n" +
		"  written to " + filepath.Join(dir, "item-8.xmp") + "\n"
	if out.String() != want {
		t.Errorf("got output:\n%swant:\n%s", out.String(), want)
	}

	// Each of the two tiles of the grid has three parameter sets and one slice.
	hevc := readFile(t, filepath.Join(dir, "item-3.hevc"))
	startCode := []byte{0, 0, 0, 1}
	if n := bytes.Count(hevc, startCode); n != 8 || !bytes.HasPrefix(hevc, append(startCode, 0x40, 0x01)) {
		t.Errorf("item-3.hevc has %d start codes and starts with %x, want 8 and a VPS", n, hevc[:6])
	}
	if tif := readFile(t, filepath.Join(dir, "item-7.tif")); !bytes.HasPrefix(tif, []byte("MM\x00*")) || len(tif) != 62 {
		t.Errorf("item-7.tif of %d bytes does not start with a TIFF header: %x", len(tif), tif[:4])
	}
	if xmp := readFile(t, filepath.Join(dir, "item-8.xmp")); !bytes.HasPrefix(xmp, []byte("<x:xmpmeta")) {
		t.Errorf("item-8.xmp starts with %q", xmp[:10])
	}
}

func TestWriteAV1Item(t *testing.T) {
	dir := t.TempDir()
	var out bytes.Buffer
	if err := run([]string{appName, "-o", dir, avifPath}, &out); err != nil {
		t.Fatal(err)
	}
	want := "item 1: av01 600x338, primary image\n  written to " + filepath.Join(dir, "item-1.obu") + "\n"
	if out.String() != want {
		t.Errorf("got output:\n%swant:\n%s", out.String(), want)
	}
	f, err := mp4.ReadMP4File(avifPath)
	if err != nil {
		t.Fatal(err)
	}
	// The image is all that the mdat box holds.
	if obu := readFile(t, filepath.Join(dir, "item-1.obu")); !bytes.Equal(obu, f.Mdat.Data) {
		t.Errorf("item-1.obu of %d bytes differs from the mdat payload of %d bytes", len(obu), len(f.Mdat.Data))
	}
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// livePhotoMoov returns a moov box like the one of a Live Photo movie, without its video and audio tracks:
// QuickTime metadata with the content identifier id, and a timed metadata track with the still image time key
// whose edit list starts with an empty edit of emptyEdit ticks. An empty id gives no metadata, and withTrack
// false no track.
func livePhotoMoov(id string, withTrack bool, emptyEdit uint64) *mp4.MoovBox {
	moov := mp4.NewMoovBox()
	mvhd := mp4.CreateMvhd()
	mvhd.Timescale, mvhd.Duration = 600, 1080
	moov.AddChild(mvhd)
	if id != "" {
		meta := mp4.CreateMetaBox(0, &mp4.HdlrBox{HandlerType: "mdta"})
		meta.AddChild(&mp4.KeysBox{Keys: []mp4.MetadataKey{
			{Namespace: "mdta", Value: "com.apple.quicktime.make"},
			{Namespace: "mdta", Value: contentIdentifierKey},
		}})
		ilst := &mp4.IlstBox{}
		for i, value := range []string{"Apple", id} {
			item := mp4.NewGenericContainerBox(string([]byte{0, 0, 0, byte(i + 1)}))
			item.AddChild(&mp4.DataBox{DataType: mp4.DataTypeUTF8, Data: []byte(value)})
			ilst.AddChild(item)
		}
		meta.AddChild(ilst)
		moov.AddChild(meta)
	}
	if withTrack {
		key := &mp4.MetadataKeyBox{LocalKeyID: 1}
		key.AddChild(&mp4.KeydBox{Namespace: "mdta", Value: stillImageTimeKey})
		mebx := &mp4.MebxBox{DataReferenceIndex: 1}
		mebx.AddChild(&mp4.MetadataKeyTableBox{Keys: []*mp4.MetadataKeyBox{key}})
		stsd := mp4.NewStsdBox()
		stsd.AddChild(mebx)
		stbl := mp4.NewStblBox()
		stbl.AddChild(stsd)
		minf := mp4.NewMinfBox()
		minf.AddChild(stbl)
		mdia := mp4.NewMdiaBox()
		mdia.AddChild(minf)
		trak := mp4.NewTrakBox()
		trak.AddChild(mp4.CreateTkhd())
		elst := &mp4.ElstBox{}
		if emptyEdit > 0 {
			elst.Entries = append(elst.Entries, mp4.ElstEntry{SegmentDuration: emptyEdit, MediaTime: -1, MediaRateInteger: 1})
		}
		elst.Entries = append(elst.Entries, mp4.ElstEntry{SegmentDuration: 1, MediaRateInteger: 1})
		edts := &mp4.EdtsBox{}
		edts.AddChild(elst)
		trak.AddChild(edts)
		trak.AddChild(mdia)
		moov.AddChild(trak)
	}
	return moov
}

func TestLivePhoto(t *testing.T) {
	const id = "0A1B2C3D-0000-4000-8000-123456789ABC"
	exif := []byte("MM\x00*...Apple iOS\x00..." + id + "\x00...")
	cases := []struct {
		desc string
		moov *mp4.MoovBox
		exif []byte
		want string
	}{
		{"pair", livePhotoMoov(id, true, 880), exif,
			"movie: content identifier " + id + " is in the Exif data of the image, so they are a Live Photo\n" +
				"movie: the photo was taken 1.467 s into the 1.800 s of the movie\n"},
		{"other image, photo at the start", livePhotoMoov(id, true, 0), []byte("MM\x00*..."),
			"movie: content identifier " + id + " is not in the Exif data of the image\n" +
				"movie: the photo was taken 0.000 s into the 1.800 s of the movie\n"},
		{"image without Exif", livePhotoMoov(id, true, 880), nil,
			"movie: content identifier " + id + " is not in the Exif data of the image\n" +
				"movie: the photo was taken 1.467 s into the 1.800 s of the movie\n"},
		{"ordinary movie", livePhotoMoov("", false, 0), exif,
			"movie: no content identifier, so it is not the movie of a Live Photo\n" +
				"movie: no track tells when the photo was taken\n"},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			// Encode and decode the movie, so that it is read as a file is.
			movie := mp4.NewFile()
			movie.AddChild(mp4.NewFtyp("qt  ", 0, []string{"qt  "}), 0)
			movie.AddChild(c.moov, 0)
			path := filepath.Join(t.TempDir(), "movie.mov")
			ofd, err := os.Create(path)
			if err != nil {
				t.Fatal(err)
			}
			err = movie.Encode(ofd)
			ofd.Close()
			if err != nil {
				t.Fatal(err)
			}
			decoded, err := mp4.ReadMP4File(path)
			if err != nil {
				t.Fatal(err)
			}
			var out strings.Builder
			if err := showLivePhoto(&out, decoded, c.exif); err != nil {
				t.Fatal(err)
			}
			if out.String() != c.want {
				t.Errorf("got output:\n%swant:\n%s", out.String(), c.want)
			}

			// With the test image, whose Exif data has no content identifier, the two are not a pair.
			var both bytes.Buffer
			if err := run([]string{appName, heicPath, path}, &both); err != nil {
				t.Fatal(err)
			}
			if strings.Contains(both.String(), "so they are a Live Photo") || !strings.Contains(both.String(), "movie: ") {
				t.Errorf("output for the test image and the movie:\n%s", both.String())
			}
		})
	}

	if err := showLivePhoto(&bytes.Buffer{}, mp4.NewFile(), nil); err == nil {
		t.Error("no error for a movie without moov box")
	}
}
