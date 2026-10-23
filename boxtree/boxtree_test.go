package boxtree

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/Eyevinn/mp4ff/mp4"
)

// testFiles returns the MP4 files in the test data of the mp4 package.
func testFiles(t *testing.T) []string {
	t.Helper()
	var files []string
	for _, pattern := range []string{"*.mp4", "*.m4s", "*.cmfv", "*.m4i", "*.isma", "*.m4v", "*.mov"} {
		matches, err := filepath.Glob(filepath.Join("..", "mp4", "testdata", pattern))
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, matches...)
	}
	if len(files) == 0 {
		t.Fatal("no test files")
	}
	return files
}

var posRegexp = regexp.MustCompile(` pos=\d+\n`)

// TestDecodeTestData checks that the tree of each test file has its boxes at the right positions,
// and, for files that mp4.DecodeFile decodes, that there are no errors and that the tree output
// is the Info output with the box positions added.
func TestDecodeTestData(t *testing.T) {
	for _, file := range testFiles(t) {
		t.Run(filepath.Base(file), func(t *testing.T) {
			data, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			tree, err := Decode(bytes.NewReader(data), int64(len(data)), "all:1")
			if err != nil {
				t.Fatal(err)
			}
			checkPositions(t, data, tree.Boxes)
			f, err := mp4.DecodeFile(bytes.NewReader(data), mp4.WithDecodeMode(mp4.DecModeLazyMdat))
			if err != nil {
				if len(tree.Errors()) == 0 {
					t.Logf("DecodeFile error without box errors: %v", err)
				}
				return
			}
			for _, e := range tree.Errors() {
				t.Errorf("unexpected box error: %v", e)
			}
			var want, got bytes.Buffer
			if err := f.Info(&want, "all:1", "", "  "); err != nil {
				t.Fatal(err)
			}
			if err := tree.Fprint(&got); err != nil {
				t.Fatal(err)
			}
			if gotInfo := posRegexp.ReplaceAllString(got.String(), "\n"); gotInfo != want.String() {
				t.Errorf("tree output without positions differs from Info output:\n%s", firstDiff(want.String(), gotInfo))
			}
		})
	}
}

// checkPositions checks that each box header is at the position of its node.
func checkPositions(t *testing.T, data []byte, nodes []*Node) {
	t.Helper()
	for _, n := range nodes {
		if n.HdrLen > 0 && string(data[n.Pos+4:n.Pos+8]) != n.Type {
			t.Errorf("%s box at position %d, but the data has %q", n.Type, n.Pos, data[n.Pos+4:n.Pos+8])
		}
		checkPositions(t, data, n.Children)
	}
}

// firstDiff returns the first line that differs.
func firstDiff(want, got string) string {
	wl, gl := strings.Split(want, "\n"), strings.Split(got, "\n")
	for i := 0; i < len(wl) && i < len(gl); i++ {
		if wl[i] != gl[i] {
			return fmt.Sprintf("line %d\nwant: %s\ngot:  %s", i+1, wl[i], gl[i])
		}
	}
	return fmt.Sprintf("want %d lines, got %d", len(wl), len(gl))
}

// TestChildOffset checks that childOffset gives the start of the children that the decoders of the mp4
// package use, for the container boxes in the test files.
func TestChildOffset(t *testing.T) {
	checked := map[string]bool{}
	var check func(data []byte, nodes []*Node)
	check = func(data []byte, nodes []*Node) {
		for _, n := range nodes {
			if n.Box != nil && n.Err == nil && len(n.Children) > 0 {
				payload := data[n.Pos+uint64(n.HdrLen) : n.Pos+n.Size]
				if off, ok := childOffset(n.Type, payload); ok {
					checked[n.Type] = true
					if want := n.Pos + uint64(n.HdrLen+off); n.Children[0].Pos != want {
						t.Errorf("%s at %d: first child at %d, childOffset gives %d", n.Type, n.Pos, n.Children[0].Pos, want)
					}
				}
			}
			check(data, n.Children)
		}
	}
	for _, file := range testFiles(t) {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		tree, err := Decode(bytes.NewReader(data), int64(len(data)), "")
		if err != nil {
			t.Fatal(err)
		}
		check(data, tree.Boxes)
	}
	for _, boxType := range []string{"moov", "trak", "stsd", "dref", "avc1", "mp4a", "meta", "udta"} {
		if !checked[boxType] {
			t.Errorf("no %s box with children in the test files", boxType)
		}
	}
}

// child returns the first node of type boxType in nodes.
func child(t *testing.T, nodes []*Node, boxType string) *Node {
	t.Helper()
	for _, n := range nodes {
		if n.Type == boxType {
			return n
		}
	}
	t.Fatalf("no %s box", boxType)
	return nil
}

// faultyInit returns init.mp4 with the box of boxType changed by corrupt, and the position of that box.
func faultyInit(t *testing.T, boxType string, corrupt func(box []byte)) ([]byte, uint64) {
	t.Helper()
	data, err := os.ReadFile("../mp4/testdata/init.mp4")
	if err != nil {
		t.Fatal(err)
	}
	pos := bytes.Index(data, []byte(boxType)) - 4
	corrupt(data[pos:])
	return data, uint64(pos)
}

// TestDecodeFaultyBox checks that the error of a box is reported for that box, with an error flag on the boxes
// above it, and that the boxes around it are decoded.
func TestDecodeFaultyBox(t *testing.T) {
	cases := []struct {
		desc    string
		boxType string
		corrupt func(box []byte)
		path    []string
	}{
		{
			desc:    "stsz with a sample count but no sample sizes",
			boxType: "stsz",
			corrupt: func(box []byte) { binary.BigEndian.PutUint32(box[16:], 1) },
			path:    []string{"moov", "trak", "mdia", "minf", "stbl", "stsz"},
		},
		{
			desc:    "avcC announcing more SPS than it holds",
			boxType: "avcC",
			corrupt: func(box []byte) { box[8+5] = 0xe0 | 31 },
			path:    []string{"moov", "trak", "mdia", "minf", "stbl", "stsd", "avc1", "avcC"},
		},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			data, pos := faultyInit(t, c.boxType, c.corrupt)
			tree, err := Decode(bytes.NewReader(data), int64(len(data)), "")
			if err != nil {
				t.Fatal(err)
			}
			checkPositions(t, data, tree.Boxes)
			errs := tree.Errors()
			if len(errs) != 1 {
				t.Fatalf("got %d errors, want 1: %v", len(errs), errs)
			}
			if wantPath := strings.Join(c.path, "/"); errs[0].Path != wantPath || errs[0].Pos != pos {
				t.Errorf("got error at %s, position %d, want %s, position %d", errs[0].Path, errs[0].Pos, wantPath, pos)
			}
			nodes := tree.Boxes
			for i, boxType := range c.path {
				n := child(t, nodes, boxType)
				last := i == len(c.path)-1
				if last != (n.Err != nil) || last == n.ChildErr {
					t.Errorf("%s: Err=%v, ChildErr=%t", boxType, n.Err, n.ChildErr)
				}
				for _, sibling := range nodes {
					if sibling != n && (sibling.Box == nil || sibling.Err != nil) {
						t.Errorf("sibling %s of %s is not decoded: %v", sibling.Type, boxType, sibling.Err)
					}
				}
				nodes = n.Children
			}
		})
	}
}

// TestDecodeTruncatedFile checks a file that ends inside its moov box.
func TestDecodeTruncatedFile(t *testing.T) {
	data, err := os.ReadFile("../mp4/testdata/init_truncated.mp4")
	if err != nil {
		t.Fatal(err)
	}
	tree, err := Decode(bytes.NewReader(data), int64(len(data)), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(tree.Boxes) != 3 || tree.Boxes[0].Box == nil || tree.Boxes[1].Box == nil {
		t.Fatalf("want decoded ftyp and skip before moov, got %d boxes", len(tree.Boxes))
	}
	moov := tree.Boxes[2]
	if moov.Type != "moov" || moov.Pos != 69 || moov.Size != 646 || moov.Err == nil || !moov.ChildErr {
		t.Errorf("moov: pos=%d size=%d Err=%v ChildErr=%t", moov.Pos, moov.Size, moov.Err, moov.ChildErr)
	}
	mvhd := child(t, moov.Children, "mvhd")
	if mvhd.Pos != 77 || mvhd.Err == nil || !strings.Contains(mvhd.Err.Error(), "larger than the 23 bytes left") {
		t.Errorf("mvhd: pos=%d Err=%v", mvhd.Pos, mvhd.Err)
	}
}

func makeBox(boxType string, payload []byte) []byte {
	b := binary.BigEndian.AppendUint32(nil, uint32(8+len(payload)))
	return append(append(b, boxType...), payload...)
}

// TestDecodeHeaderProblems checks boxes whose headers do not give a usable size.
func TestDecodeHeaderProblems(t *testing.T) {
	ftyp := makeBox("ftyp", []byte("isom\x00\x00\x02\x00isom"))
	largeMoov := append(binary.BigEndian.AppendUint32(nil, 1), "moov"...)
	largeMoov = binary.BigEndian.AppendUint64(largeMoov, 16+8)
	largeMoov = append(largeMoov, makeBox("free", nil)...)
	cases := []struct {
		desc    string
		data    []byte
		wantErr string
		check   func(t *testing.T, n *Node)
	}{
		{
			desc:    "too few bytes for a header",
			data:    []byte{0, 0, 0, 0},
			wantErr: "4 bytes are too few for a box header",
			check: func(t *testing.T, n *Node) {
				if n.HdrLen != 0 || n.Size != 4 || n.Name() != "(4 bytes)" {
					t.Errorf("HdrLen=%d Size=%d name=%q", n.HdrLen, n.Size, n.Name())
				}
			},
		},
		{
			desc:    "too few bytes for a 64-bit size",
			data:    append(binary.BigEndian.AppendUint32(nil, 1), "mdat\x00\x00\x00\x00"...),
			wantErr: "12 bytes are too few for a box header with a 64-bit size",
		},
		{
			desc:    "size 0",
			data:    append(binary.BigEndian.AppendUint32(nil, 0), "free\x00\x00\x00\x00"...),
			wantErr: "size 0, meaning to the end of the file, is not supported",
			check: func(t *testing.T, n *Node) {
				if n.Size != 12 {
					t.Errorf("Size=%d, want 12", n.Size)
				}
			},
		},
		{
			desc:    "size smaller than the header",
			data:    append(binary.BigEndian.AppendUint32(nil, 4), "free"...),
			wantErr: "box size 4 is smaller than the 8-byte box header",
		},
		{
			desc:    "64-bit size of a container",
			data:    largeMoov,
			wantErr: "extended size not supported for box type moov",
			check: func(t *testing.T, n *Node) {
				if n.HdrLen != 16 || n.ChildErr || len(n.Children) != 1 || n.Children[0].Box == nil || n.Children[0].Pos != 20+16 {
					t.Errorf("HdrLen=%d ChildErr=%t children=%d", n.HdrLen, n.ChildErr, len(n.Children))
				}
			},
		},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			data := append(append([]byte{}, ftyp...), c.data...)
			tree, err := Decode(bytes.NewReader(data), int64(len(data)), "")
			if err != nil {
				t.Fatal(err)
			}
			if len(tree.Boxes) != 2 || tree.Boxes[0].Box == nil {
				t.Fatalf("want a decoded ftyp and one more box, got %d boxes", len(tree.Boxes))
			}
			n := tree.Boxes[1]
			if n.Pos != uint64(len(ftyp)) || n.Err == nil || !strings.Contains(n.Err.Error(), c.wantErr) {
				t.Errorf("pos=%d Err=%v, want error %q", n.Pos, n.Err, c.wantErr)
			}
			if c.check != nil {
				c.check(t, n)
			}
		})
	}
}

// TestFprintAndJSON checks the text and JSON output for a box with an error.
func TestFprintAndJSON(t *testing.T) {
	data, pos := faultyInit(t, "stsz", func(box []byte) { binary.BigEndian.PutUint32(box[16:], 1) })
	tree, err := Decode(bytes.NewReader(data), int64(len(data)), "")
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := tree.Fprint(&out); err != nil {
		t.Fatal(err)
	}
	indent := strings.Repeat("  ", 5)
	wantLines := fmt.Sprintf("%s[stsz] size=20 pos=%d\n%s - error: decode stsz pos %d: ", indent, pos, indent, pos)
	if !strings.Contains(out.String(), wantLines) {
		t.Errorf("output does not contain\n%s\n%s", wantLines, out.String())
	}
	if !strings.HasPrefix(out.String(), "[ftyp] size=32 pos=0\n - majorBrand: iso5\n") {
		t.Errorf("output does not start with the ftyp box:\n%s", out.String())
	}

	j, err := json.Marshal(tree)
	if err != nil {
		t.Fatal(err)
	}
	type jsonNode struct {
		Type     string     `json:"type"`
		Pos      uint64     `json:"pos"`
		Size     uint64     `json:"size"`
		HdrLen   int        `json:"hdrLen"`
		Info     []string   `json:"info"`
		Err      string     `json:"error"`
		ChildErr bool       `json:"childError"`
		Children []jsonNode `json:"children"`
	}
	var got struct {
		Size  int64      `json:"size"`
		Boxes []jsonNode `json:"boxes"`
	}
	if err := json.Unmarshal(j, &got); err != nil {
		t.Fatal(err)
	}
	if got.Size != int64(len(data)) || len(got.Boxes) != 3 {
		t.Fatalf("size=%d with %d boxes", got.Size, len(got.Boxes))
	}
	moov := got.Boxes[2]
	if moov.Type != "moov" || moov.Pos != 69 || moov.HdrLen != 8 || !moov.ChildErr || moov.Err != "" || len(moov.Info) != 0 {
		t.Errorf("moov: %+v", moov)
	}
	n := moov
	for _, i := range []int{1, 1, 2, 2, 3} { // trak, mdia, minf, stbl, stsz
		n = n.Children[i]
	}
	if n.Type != "stsz" || n.Pos != pos || !strings.HasPrefix(n.Err, "decode stsz pos") || n.ChildErr {
		t.Errorf("stsz: %+v", n)
	}
}

func TestDisplayType(t *testing.T) {
	cases := map[string]string{
		"moov":             "moov",
		"url ":             "url ",
		"\xa9nam":          "©nam",
		"\x00\x00\x00\x01": `\x00\x00\x00\x01`,
		"\x80abc":          `\x80abc`,
	}
	for boxType, want := range cases {
		if got := displayType(boxType); got != want {
			t.Errorf("displayType(%q) = %q, want %q", boxType, got, want)
		}
	}
}

type failingReader struct{}

func (failingReader) ReadAt(p []byte, off int64) (int, error) {
	return 0, errors.New("disk on fire")
}

func TestDecodeReadError(t *testing.T) {
	if _, err := Decode(failingReader{}, 100, ""); err == nil || !strings.Contains(err.Error(), "disk on fire") {
		t.Errorf("got error %v", err)
	}
}

// TestUdtaTerminator checks the positions of the children of a udta box that ends with the 32-bit zero of
// QuickTime user data.
func TestUdtaTerminator(t *testing.T) {
	udta := &mp4.UdtaBox{TrailingBytes: []byte{0, 0, 0, 0}}
	udta.AddChild(mp4.CreateUnknownBox("WLOC", 12, []byte{0, 1, 0, 2}))
	var buf bytes.Buffer
	if err := udta.Encode(&buf); err != nil {
		t.Fatal(err)
	}
	tree, err := Decode(bytes.NewReader(buf.Bytes()), int64(buf.Len()), "")
	if err != nil {
		t.Fatal(err)
	}
	if errs := tree.Errors(); len(errs) != 0 {
		t.Fatalf("errors: %v", errs)
	}
	if c := tree.Boxes[0].Children; len(c) != 1 || c[0].Pos != 8 || c[0].Type != "WLOC" {
		t.Errorf("children: %+v", c)
	}
}
