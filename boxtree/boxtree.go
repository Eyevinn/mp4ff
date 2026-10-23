// Package boxtree decodes the box tree of an MP4 (ISOBMFF) file and keeps going after errors.
//
// Each box is decoded by the mp4 package. When a container box cannot be decoded, its children
// are decoded one at a time, so that the error is reported for the smallest box that fails,
// while the boxes around it are still decoded.
//
// The position of each box in the file is kept in its [Node], not in the mp4 box. For a decoded
// container box, the positions of its children follow from the box sizes, since the children
// come last in the box, before any trailing bytes.
package boxtree

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/Eyevinn/mp4ff/mp4"
)

// maxReadSize is the largest top-level box that Decode reads into memory.
const maxReadSize = 1 << 30

// indentStep is the indentation per box level in the Info output.
const indentStep = "  "

// Tree is the box tree of a file.
type Tree struct {
	// Size is the size of the file.
	Size int64 `json:"size"`
	// Boxes are the top-level boxes.
	Boxes []*Node `json:"boxes"`
}

// Node is a box in a file.
type Node struct {
	// Type is the four-character box type. It is empty for bytes too few to hold a box header.
	Type string
	// Pos is the position of the box in the file.
	Pos uint64
	// Size is the box size given by the box header. In a truncated file, it can extend beyond the data.
	Size uint64
	// HdrLen is the length of the box header: 8, 16 with a 64-bit size, or 0 without a header.
	HdrLen int
	// Box is the box decoded by the mp4 package, or nil if it could not be decoded.
	Box mp4.Box
	// Info is the output of the Info method of the box, without the lines of its child boxes.
	Info []string
	// Err tells why the box could not be decoded. It is nil if the error is in a child box.
	Err error
	// ChildErr is true if a child box, or a box further down, could not be decoded.
	ChildErr bool
	// Children are the child boxes.
	Children []*Node
}

// Decode decodes the boxes in the first size bytes of r. infoLevels sets the details of the
// Info lines, as for the Info methods of the mp4 boxes, for example "all:1".
// The top-level boxes are read one at a time, and the payload of mdat boxes is not read.
// Boxes that cannot be decoded have Err set. The returned error is only for failures to read r.
func Decode(r io.ReaderAt, size int64, infoLevels string) (*Tree, error) {
	t := &Tree{Size: size}
	end := uint64(size)
	hdr := make([]byte, 16)
	var moov *mp4.MoovBox
	for pos := uint64(0); pos < end; {
		left := end - pos
		h := hdr[:min(left, uint64(len(hdr)))]
		if err := readAt(r, h, pos); err != nil {
			return t, err
		}
		n, ok := newNode(h, pos, left)
		t.Boxes = append(t.Boxes, n)
		if !ok {
			break
		}
		avail := min(n.Size, left)
		switch {
		case n.Type == "mdat":
			if n.Err == nil {
				box, err := mp4.DecodeMdatLazily(mp4.BoxHeader{Name: n.Type, Size: n.Size, Hdrlen: n.HdrLen}, n.Pos)
				if err != nil {
					n.Err = err
				} else {
					n.Box, n.Info = box, info(box, infoLevels)
				}
			}
		case avail > maxReadSize:
			if n.Err == nil {
				n.Err = fmt.Errorf("box size %d is too large to decode", n.Size)
			}
		default:
			data := make([]byte, avail)
			if err := readAt(r, data, pos); err != nil {
				return t, err
			}
			decodeNode(n, data, infoLevels)
			switch box := n.Box.(type) {
			case *mp4.MoovBox:
				moov = box
			case *mp4.MoofBox:
				parsed, traf, err := parseSenc(box, moov)
				if parsed || err != nil {
					n.Children, n.ChildErr = nil, false
					setBox(n, box, data, infoLevels)
				}
				if err != nil {
					setTrafError(n, traf, err)
				}
			}
		}
		pos += avail
	}
	return t, nil
}

// parseSenc parses the senc boxes in moof with the per-sample IV size of their tracks in moov, as
// mp4.DecodeFile does. moov is nil if no moov box has been decoded. It returns whether a senc box was
// parsed, and on error, the traf box of the senc box that could not be parsed.
func parseSenc(moof *mp4.MoofBox, moov *mp4.MoovBox) (bool, *mp4.TrafBox, error) {
	parsed := false
	for _, traf := range moof.Trafs {
		if ok, done := traf.ContainsSencBox(); !ok || done || traf.Tfhd == nil {
			continue
		}
		defaultIVSize := byte(0)
		if moov != nil {
			trackID := traf.Tfhd.TrackID
			if !moov.IsEncrypted(trackID) {
				continue
			}
			sinf := moov.GetSinf(trackID)
			if sinf != nil && sinf.Schi != nil && sinf.Schi.Tenc != nil {
				defaultIVSize = sinf.Schi.Tenc.DefaultPerSampleIVSize
			}
		}
		err := traf.ParseReadSenc(defaultIVSize, moof.StartPos)
		switch {
		case err == nil:
			parsed = true
		case moov != nil:
			return parsed, traf, fmt.Errorf("parse senc: %w", err)
		}
	}
	return parsed, nil, nil
}

// setTrafError sets err for the node of traf among the children of the moof node n.
func setTrafError(n *Node, traf *mp4.TrafBox, err error) {
	for _, c := range n.Children {
		if c.Box == mp4.Box(traf) {
			c.Err = err
			n.ChildErr = true
			return
		}
	}
	n.Err = err
}

// readAt fills p with the bytes from pos in r.
func readAt(r io.ReaderAt, p []byte, pos uint64) error {
	n, err := r.ReadAt(p, int64(pos))
	if n == len(p) {
		return nil
	}
	if err == nil || err == io.EOF {
		err = io.ErrUnexpectedEOF
	}
	return fmt.Errorf("read %d bytes at position %d: %w", len(p), pos, err)
}

// walk decodes the boxes in data, which starts at position base in the file.
func walk(data []byte, base uint64, infoLevels string) []*Node {
	var nodes []*Node
	for pos := 0; pos < len(data); {
		left := uint64(len(data) - pos)
		n, ok := newNode(data[pos:pos+int(min(left, 16))], base+uint64(pos), left)
		nodes = append(nodes, n)
		if !ok {
			break
		}
		avail := int(min(n.Size, left))
		decodeNode(n, data[pos:pos+avail], infoLevels)
		pos += avail
	}
	return nodes
}

// newNode makes the node of the box whose header starts h, where left bytes remain from pos.
// It returns false if the bytes do not hold a box header that tells where the box ends.
func newNode(h []byte, pos, left uint64) (*Node, bool) {
	n := &Node{Pos: pos, Size: left}
	if len(h) < 8 {
		n.Err = fmt.Errorf("%d bytes are too few for a box header", left)
		return n, false
	}
	n.Type, n.HdrLen = string(h[4:8]), 8
	size := uint64(binary.BigEndian.Uint32(h))
	switch size {
	case 0:
		n.Err = errors.New("size 0, meaning to the end of the file, is not supported")
		return n, true
	case 1:
		if len(h) < 16 {
			n.Err = fmt.Errorf("%d bytes are too few for a box header with a 64-bit size", left)
			return n, false
		}
		size, n.HdrLen = binary.BigEndian.Uint64(h[8:16]), 16
	}
	if size < uint64(n.HdrLen) {
		n.Err = fmt.Errorf("box size %d is smaller than the %d-byte box header", size, n.HdrLen)
		return n, false
	}
	n.Size = size
	if size > left {
		n.Err = fmt.Errorf("box size %d is larger than the %d bytes left", size, left)
	}
	return n, true
}

// decodeNode decodes the box of n from data, which holds the box, or as much of it as there is.
// A container box that cannot be decoded gets its children decoded one at a time.
func decodeNode(n *Node, data []byte, infoLevels string) {
	decodeErr := false
	if n.Err == nil {
		box, err := mp4.DecodeBoxSR(n.Pos, bits.NewFixedSliceReader(data))
		if err == nil {
			setBox(n, box, data, infoLevels)
			return
		}
		n.Err, decodeErr = err, true
	}
	off, ok := childOffset(n.Type, data[n.HdrLen:])
	start := n.HdrLen + off
	if !ok || start > len(data) {
		return
	}
	n.Children = walk(data[start:], n.Pos+uint64(start), infoLevels)
	for _, c := range n.Children {
		if c.Err != nil || c.ChildErr {
			n.ChildErr = true
			break
		}
	}
	if decodeErr && n.ChildErr {
		n.Err = nil // The error is reported for the child.
	}
}

// setBox sets the decoded box of n and makes the nodes of its children, if any.
// data holds the bytes of the box.
func setBox(n *Node, box mp4.Box, data []byte, infoLevels string) {
	n.Box, n.Info = box, info(box, infoLevels)
	c, ok := box.(mp4.ContainerBox)
	if !ok {
		return
	}
	children := c.GetChildren()
	childrenEnd := n.Size - uint64(len(trailingBytes(box)))
	var childrenSize uint64
	for _, child := range children {
		childrenSize += child.Size()
	}
	if childrenSize > childrenEnd {
		n.Err = fmt.Errorf("child box sizes add up to %d, beyond the end at %d", childrenSize, childrenEnd)
		return
	}
	off := childrenEnd - childrenSize
	for _, child := range children {
		cn := &Node{Type: child.Type(), Pos: n.Pos + off, Size: child.Size(), HdrLen: 8}
		n.Children = append(n.Children, cn)
		end := off + child.Size()
		if end > uint64(len(data)) || !hasHeader(data[off:end], cn) {
			cn.Box, cn.Info = child, info(child, infoLevels)
			cn.Err = errors.New("the box is not at the position given by the box sizes")
			n.ChildErr = true
			off = end
			continue
		}
		setBox(cn, child, data[off:end], infoLevels)
		if cn.Err != nil || cn.ChildErr {
			n.ChildErr = true
		}
		off = end
	}
}

// trailingBytes returns the bytes that box keeps after its children.
func trailingBytes(box mp4.Box) []byte {
	switch b := box.(type) {
	case *mp4.VisualSampleEntryBox:
		return b.TrailingBytes
	case *mp4.WaveBox:
		return b.RawTail
	case *mp4.UdtaBox:
		return b.TrailingBytes
	}
	return nil
}

// hasHeader tells if data starts with the header of the box of n, and sets the header length of n.
func hasHeader(data []byte, n *Node) bool {
	if len(data) < 8 || string(data[4:8]) != n.Type {
		return false
	}
	size := uint64(binary.BigEndian.Uint32(data))
	if size == 1 && len(data) >= 16 {
		size, n.HdrLen = binary.BigEndian.Uint64(data[8:16]), 16
	}
	return size == n.Size
}

// childOffset returns where the children start in the payload of a container box that could not
// be decoded, following the decoders of the mp4 package. It returns false for other box types.
func childOffset(boxType string, payload []byte) (int, bool) {
	switch boxType {
	case "dinf", "edts", "ilst", "ludt", "mdia", "mfra", "minf", "moof", "moov", "mvex", "schi", "sinf",
		"stbl", "traf", "trak", "tref", "trgr", "udta", "vexu", "desc", "\xa9ART", "\xa9nam", "\xa9too", "\xa9cpy":
		return 0, true
	case "dref", "stsd", "trep", "wvtt", "wvtc":
		return 8, true
	case "meta":
		if len(payload) >= 8 && string(payload[4:8]) == "hdlr" {
			return 0, true // QuickTime meta, without version and flags
		}
		return 4, true
	case "av01", "avc1", "avc3", "avs3", "dvh1", "dvhe", "encv", "hev1", "hvc1", "jpeg", "mjpg", "mp4v",
		"vp08", "vp09", "vvc1", "vvi1":
		return 78, true
	case ".mp3", "ac-3", "ac-4", "ec-3", "enca", "fLaC", "iamf", "lpcm", "mha1", "mha2", "mhm1", "mhm2",
		"mp4a", "Opus", "sowt", "twos":
		if len(payload) >= 10 {
			switch binary.BigEndian.Uint16(payload[8:10]) { // QuickTime sound description version
			case 1:
				return 28 + 16, true
			case 2:
				return 28 + 36, true
			}
		}
		return 28, true
	}
	return 0, false
}

// errChildInfo stops an Info method when it gets to its child boxes.
var errChildInfo = errors.New("info of child box")

// infoWriter keeps the lines written by an Info method, up to the first line of a child box.
type infoWriter struct {
	lines []string
	buf   []byte
	done  bool
}

func (w *infoWriter) Write(p []byte) (int, error) {
	if w.done {
		return 0, errChildInfo
	}
	w.buf = append(w.buf, p...)
	for {
		i := bytes.IndexByte(w.buf, '\n')
		if i < 0 {
			return len(p), nil
		}
		line := string(w.buf[:i])
		w.buf = w.buf[i+1:]
		if len(w.lines) > 0 && strings.HasPrefix(line, indentStep+"[") {
			w.done = true
			return len(p), errChildInfo
		}
		w.lines = append(w.lines, line)
	}
}

// info returns the Info lines of box, without the lines of its child boxes.
func info(box mp4.Box, infoLevels string) []string {
	w := infoWriter{}
	_ = box.Info(&w, infoLevels, "", indentStep)
	if len(w.buf) > 0 && !w.done {
		w.lines = append(w.lines, string(w.buf))
	}
	return w.lines
}

// Fprint writes the tree in the format of the Info methods of the mp4 boxes, with the position of
// each box added to its first line, and a line for each error.
func (t *Tree) Fprint(w io.Writer) error {
	return fprint(w, t.Boxes, "")
}

func fprint(w io.Writer, nodes []*Node, indent string) error {
	for _, n := range nodes {
		lines := n.Info
		if len(lines) == 0 {
			lines = []string{n.Name()}
			if n.HdrLen > 0 {
				lines[0] = fmt.Sprintf("[%s] size=%d", n.Name(), n.Size)
			}
		}
		if _, err := fmt.Fprintf(w, "%s%s pos=%d\n", indent, lines[0], n.Pos); err != nil {
			return err
		}
		for _, line := range lines[1:] {
			if _, err := fmt.Fprintf(w, "%s%s\n", indent, line); err != nil {
				return err
			}
		}
		if n.Err != nil {
			if _, err := fmt.Fprintf(w, "%s - error: %v\n", indent, n.Err); err != nil {
				return err
			}
		}
		if err := fprint(w, n.Children, indent+indentStep); err != nil {
			return err
		}
	}
	return nil
}

// BoxError is the error of a box that could not be decoded.
type BoxError struct {
	// Path is the box types from the top-level box down to the box, such as "moov/trak/edts/elst".
	Path string
	// Pos is the position of the box in the file.
	Pos uint64
	// Err is the error of the box.
	Err error
}

func (e *BoxError) Error() string {
	return fmt.Sprintf("%s at position %d: %v", e.Path, e.Pos, e.Err)
}

func (e *BoxError) Unwrap() error {
	return e.Err
}

// Errors returns the errors of the boxes that could not be decoded, in file order.
func (t *Tree) Errors() []*BoxError {
	return appendErrors(nil, t.Boxes, "")
}

func appendErrors(errs []*BoxError, nodes []*Node, parentPath string) []*BoxError {
	for _, n := range nodes {
		path := parentPath + n.Name()
		if n.Err != nil {
			errs = append(errs, &BoxError{Path: path, Pos: n.Pos, Err: n.Err})
		}
		errs = appendErrors(errs, n.Children, path+"/")
	}
	return errs
}

// Name returns the box type as UTF-8 text, or the number of bytes, as "(12 bytes)", for bytes without
// a box header. It is the name of the box in Fprint and in the paths of Errors.
func (n *Node) Name() string {
	if n.HdrLen == 0 {
		return fmt.Sprintf("(%d bytes)", n.Size)
	}
	return displayType(n.Type)
}

// MarshalJSON writes the node with the box type as UTF-8 text and the error as a string.
func (n *Node) MarshalJSON() ([]byte, error) {
	type jsonNode struct {
		Type     string   `json:"type"`
		Pos      uint64   `json:"pos"`
		Size     uint64   `json:"size"`
		HdrLen   int      `json:"hdrLen"`
		Info     []string `json:"info,omitempty"`
		Err      string   `json:"error,omitempty"`
		ChildErr bool     `json:"childError,omitempty"`
		Children []*Node  `json:"children,omitempty"`
	}
	j := jsonNode{Type: displayType(n.Type), Pos: n.Pos, Size: n.Size, HdrLen: n.HdrLen, Info: n.Info,
		ChildErr: n.ChildErr, Children: n.Children}
	if n.Err != nil {
		j.Err = n.Err.Error()
	}
	return json.Marshal(j)
}

// displayType returns the box type as UTF-8 text. Bytes from 0xa0 are Latin-1 characters, as in the
// ©-types of QuickTime metadata, and other bytes outside printable ASCII are written as \xNN.
func displayType(boxType string) string {
	var sb strings.Builder
	for i := 0; i < len(boxType); i++ {
		switch c := boxType[i]; {
		case c >= 0x20 && c < 0x7f:
			sb.WriteByte(c)
		case c >= 0xa0:
			sb.WriteRune(rune(c))
		default:
			fmt.Fprintf(&sb, "\\x%02x", c)
		}
	}
	return sb.String()
}
