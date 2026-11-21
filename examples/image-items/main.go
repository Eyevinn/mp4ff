package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/Eyevinn/mp4ff/avc"
	"github.com/Eyevinn/mp4ff/internal"
	"github.com/Eyevinn/mp4ff/mp4"
)

const (
	appName = "image-items"

	contentIdentifierKey = "com.apple.quicktime.content.identifier"
	stillImageTimeKey    = "com.apple.quicktime.still-image-time"
)

var usg = `%s lists the items of a HEIF, HEIC or AVIF image file: the images with their size and role,
and the metadata. With -o, it writes the items to files: coded images as HEVC (.hevc) or AV1 (.obu)
elementary streams, where a grid is written as a stream of its tiles in row order, Exif as TIFF (.tif)
and XMP as .xmp.

Given the movie of an iPhone Live Photo as well, it tells if the image and the movie belong together,
and where in the movie the photo was taken.

Usage of %s:
`

type options struct {
	outDir  string
	version bool
}

func parseOptions(fs *flag.FlagSet, args []string) (*options, error) {
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, usg, appName, appName)
		fmt.Fprintf(os.Stderr, "\n%s [options] image [movie]\n\noptions:\n", appName)
		fs.PrintDefaults()
	}

	opts := options{}
	fs.StringVar(&opts.outDir, "o", "", "Directory to write the items to")
	fs.BoolVar(&opts.version, "version", false, "Get mp4ff version")

	err := fs.Parse(args[1:])
	return &opts, err
}

func main() {
	if err := run(os.Args, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet(appName, flag.ContinueOnError)
	o, err := parseOptions(fs, args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if o.version {
		fmt.Fprintf(stdout, "%s %s\n", appName, internal.Version())
		return nil
	}
	if len(fs.Args()) < 1 || len(fs.Args()) > 2 {
		fs.Usage()
		return fmt.Errorf("need an image file, and optionally a movie file")
	}

	image, err := mp4.ReadMP4File(fs.Arg(0))
	if err != nil {
		return fmt.Errorf("image: %w", err)
	}
	exif, err := listItems(stdout, image, o.outDir)
	if err != nil {
		return err
	}
	if len(fs.Args()) == 1 {
		return nil
	}
	movie, err := mp4.ReadMP4File(fs.Arg(1))
	if err != nil {
		return fmt.Errorf("movie: %w", err)
	}
	return showLivePhoto(stdout, movie, exif)
}

// listItems writes a line for each item of the image file f, and writes the items to outDir if it is set.
// It returns the data of the Exif item, if there is one.
func listItems(w io.Writer, f *mp4.File, outDir string) (exif []byte, err error) {
	meta := f.Meta
	if meta == nil || meta.Pitm == nil || meta.Iinf == nil || meta.Iloc == nil || meta.Iprp == nil {
		return nil, fmt.Errorf("no meta box with pitm, iinf, iloc and iprp: not an image file")
	}
	if meta.Iref == nil {
		meta.Iref = &mp4.IrefBox{} // a file with one item has no references
	}
	if outDir != "" {
		if err := os.MkdirAll(outDir, 0o755); err != nil {
			return nil, err
		}
	}
	// The tiles of a grid are part of the line of the grid.
	isTile := map[uint32]bool{}
	for _, ref := range meta.Iref.Refs {
		if ref.Name == "dimg" {
			for _, id := range ref.ToItemIDs {
				isTile[id] = true
			}
		}
	}
	for _, infe := range meta.Iinf.Infes {
		id := infe.ItemID
		if isTile[id] {
			continue
		}
		desc, err := describeItem(meta, f.Mdat, infe)
		if err != nil {
			return nil, err
		}
		fmt.Fprintf(w, "item %d: %s%s\n", id, desc, role(meta, id))
		if infe.ItemType == "Exif" {
			if exif, err = meta.GetItemData(id, f.Mdat, nil); err != nil {
				return nil, err
			}
		}
		if outDir == "" {
			continue
		}
		name, data, err := itemFile(meta, f.Mdat, infe)
		if err != nil {
			return nil, err
		}
		path := filepath.Join(outDir, name)
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return nil, err
		}
		fmt.Fprintf(w, "  written to %s\n", path)
	}
	return exif, nil
}

// describeItem tells what kind of item infe is: its type, and for an image its size and transformations.
func describeItem(meta *mp4.MetaBox, mdat *mp4.MdatBox, infe *mp4.InfeBox) (string, error) {
	desc := infe.ItemType
	if infe.ItemType == "mime" {
		desc += " " + infe.ContentType
	}
	if props := imageProperties(meta, infe.ItemID); props != "" {
		desc += " " + props
	} else if item := meta.Iloc.GetItem(infe.ItemID); item != nil {
		var size uint64
		for _, e := range item.Extents {
			size += e.Length
		}
		desc += fmt.Sprintf(" %d bytes", size)
	}
	if infe.ItemType != "grid" {
		return desc, nil
	}
	data, err := meta.GetItemData(infe.ItemID, mdat, nil)
	if err != nil {
		return "", err
	}
	grid, err := mp4.DecodeImageGrid(data)
	if err != nil {
		return "", err
	}
	desc += fmt.Sprintf(" in %d rows and %d columns of tiles", grid.Rows, grid.Columns)
	if tiles := meta.Iref.GetToItemIDs("dimg", infe.ItemID); len(tiles) > 0 {
		desc += " of " + imageProperties(meta, tiles[0])
	}
	return desc, nil
}

// imageProperties returns what the properties of an item tell about its image, or "" if they tell nothing.
func imageProperties(meta *mp4.MetaBox, itemID uint32) string {
	desc := ""
	for _, p := range meta.Iprp.GetItemProperties(itemID) {
		switch prop := p.Property.(type) {
		case *mp4.IspeBox:
			desc += fmt.Sprintf(" %dx%d", prop.ImageWidth, prop.ImageHeight)
		case *mp4.IrotBox:
			if prop.Angle != 0 {
				desc += fmt.Sprintf(" rotated %d degrees anti-clockwise", int(prop.Angle)*90)
			}
		case *mp4.ImirBox:
			desc += " mirrored"
		case *mp4.AuxCBox:
			desc += " of type " + prop.AuxType
		}
	}
	if desc == "" {
		return ""
	}
	return desc[1:]
}

// role tells how an item belongs to the other items.
func role(meta *mp4.MetaBox, itemID uint32) string {
	if itemID == meta.Pitm.ItemID {
		return ", primary image"
	}
	for _, r := range []struct{ refType, role string }{
		{"thmb", "thumbnail"}, {"auxl", "auxiliary image"}, {"cdsc", "metadata"},
	} {
		if ids := meta.Iref.GetToItemIDs(r.refType, itemID); len(ids) > 0 {
			return fmt.Sprintf(", %s of item %d", r.role, ids[0])
		}
	}
	return ""
}

// itemFile returns the name and the content of the file to write for an item.
func itemFile(meta *mp4.MetaBox, mdat *mp4.MdatBox, infe *mp4.InfeBox) (name string, data []byte, err error) {
	id := infe.ItemID
	coded := []uint32{id} // the coded images to write
	itemType := infe.ItemType
	if itemType == "grid" {
		coded = meta.Iref.GetToItemIDs("dimg", id)
		if len(coded) == 0 {
			return "", nil, fmt.Errorf("grid item %d has no tiles", id)
		}
		if tile := meta.Iinf.GetInfe(coded[0]); tile != nil {
			itemType = tile.ItemType
		}
	}
	switch itemType {
	case "hvc1":
		for _, c := range coded {
			if data, err = appendHEVCImage(data, meta, mdat, c); err != nil {
				return "", nil, err
			}
		}
		return fmt.Sprintf("item-%d.hevc", id), data, nil
	case "av01":
		// The data of an AV1 image item is a stream of OBUs that starts with a sequence header.
		for _, c := range coded {
			image, err := meta.GetItemData(c, mdat, nil)
			if err != nil {
				return "", nil, err
			}
			data = append(data, image...)
		}
		return fmt.Sprintf("item-%d.obu", id), data, nil
	}
	data, err = meta.GetItemData(id, mdat, nil)
	if err != nil {
		return "", nil, err
	}
	switch {
	case itemType == "Exif":
		// The data starts with the offset of the TIFF header, counted from the byte after the offset.
		if len(data) < 4 || uint64(len(data)-4) < uint64(be32(data)) {
			return "", nil, fmt.Errorf("item %d: Exif data of %d bytes is too short", id, len(data))
		}
		return fmt.Sprintf("item-%d.tif", id), data[4+be32(data):], nil
	case itemType == "mime" && infe.ContentType == "application/rdf+xml":
		return fmt.Sprintf("item-%d.xmp", id), data, nil
	}
	return fmt.Sprintf("item-%d.bin", id), data, nil
}

func be32(data []byte) uint32 {
	return uint32(data[0])<<24 | uint32(data[1])<<16 | uint32(data[2])<<8 | uint32(data[3])
}

// appendHEVCImage appends the HEVC image item itemID to stream as NAL units with start codes: the parameter sets
// of its hvcC property and then the NAL units of its data.
func appendHEVCImage(stream []byte, meta *mp4.MetaBox, mdat *mp4.MdatBox, itemID uint32) ([]byte, error) {
	var hvcC *mp4.HvcCBox
	for _, p := range meta.Iprp.GetItemProperties(itemID) {
		if box, ok := p.Property.(*mp4.HvcCBox); ok {
			hvcC = box
		}
	}
	if hvcC == nil || hvcC.LengthSizeMinusOne != 3 {
		return nil, fmt.Errorf("item %d has no hvcC property with 4-byte NAL unit lengths", itemID)
	}
	data, err := meta.GetItemData(itemID, mdat, nil)
	if err != nil {
		return nil, err
	}
	nalus, err := avc.GetNalusFromSample(data)
	if err != nil {
		return nil, fmt.Errorf("item %d: %w", itemID, err)
	}
	startCode := []byte{0, 0, 0, 1}
	for _, array := range hvcC.NaluArrays {
		for _, nalu := range array.Nalus {
			stream = append(append(stream, startCode...), nalu...)
		}
	}
	for _, nalu := range nalus {
		stream = append(append(stream, startCode...), nalu...)
	}
	return stream, nil
}

// showLivePhoto writes what the movie of a Live Photo tells about its photo. exif is the Exif data of the
// image, which has the content identifier of the pair in the maker note of the camera.
func showLivePhoto(w io.Writer, movie *mp4.File, exif []byte) error {
	if movie.Moov == nil || movie.Moov.Mvhd == nil || movie.Moov.Mvhd.Timescale == 0 {
		return fmt.Errorf("movie: no moov box with a time scale")
	}
	id := contentIdentifier(movie.Moov)
	switch {
	case id == "":
		fmt.Fprintf(w, "movie: no content identifier, so it is not the movie of a Live Photo\n")
	case bytes.Contains(exif, []byte(id)):
		fmt.Fprintf(w, "movie: content identifier %s is in the Exif data of the image, so they are a Live Photo\n", id)
	default:
		fmt.Fprintf(w, "movie: content identifier %s is not in the Exif data of the image\n", id)
	}
	timescale := float64(movie.Moov.Mvhd.Timescale)
	if ticks, ok := stillImageTime(movie.Moov); ok {
		fmt.Fprintf(w, "movie: the photo was taken %.3f s into the %.3f s of the movie\n", float64(ticks)/timescale,
			float64(movie.Moov.Mvhd.Duration)/timescale)
	} else {
		fmt.Fprintf(w, "movie: no track tells when the photo was taken\n")
	}
	return nil
}

// contentIdentifier returns the content identifier in the QuickTime metadata of the movie, or "".
func contentIdentifier(moov *mp4.MoovBox) string {
	for _, c := range moov.Children {
		meta, ok := c.(*mp4.MetaBox)
		if !ok || meta.Keys == nil {
			continue
		}
		for _, mc := range meta.Children {
			ilst, ok := mc.(*mp4.IlstBox)
			if !ok {
				continue
			}
			// The type of an item is the index of its key.
			for _, item := range ilst.Children {
				key, _ := meta.Keys.KeyOfItem(item.Type())
				container, ok := item.(*mp4.GenericContainerBox)
				if key.Value != contentIdentifierKey || !ok {
					continue
				}
				for _, v := range container.Children {
					if data, ok := v.(*mp4.DataBox); ok {
						return string(data.Data)
					}
				}
			}
		}
	}
	return ""
}

// stillImageTime returns the time of the photo in the movie, in the time scale of the movie. It is not the
// value of a sample: it is the time where the edit list of the track with the still image time key puts
// the start of its media, after its empty edits.
func stillImageTime(moov *mp4.MoovBox) (ticks uint64, ok bool) {
	for _, trak := range moov.Traks {
		if trak.Mdia == nil || trak.Mdia.Minf == nil || trak.Mdia.Minf.Stbl == nil || trak.Mdia.Minf.Stbl.Stsd == nil {
			continue
		}
		mebx := trak.Mdia.Minf.Stbl.Stsd.Mebx
		if mebx == nil || mebx.Keys == nil || mebx.Keys.FindKey("mdta", stillImageTimeKey) == nil {
			continue
		}
		if trak.Edts != nil {
			for _, elst := range trak.Edts.Elst {
				for _, e := range elst.Entries {
					if e.MediaTime != -1 { // the first edit with media
						return ticks, true
					}
					ticks += e.SegmentDuration
				}
			}
		}
		return ticks, true
	}
	return 0, false
}
