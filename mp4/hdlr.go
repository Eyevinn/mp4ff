package mp4

import (
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
)

// HdlrBox - Handler Reference Box (hdlr - mandatory)
//
// Contained in: Media Box (mdia) or Meta Box (meta)
//
// This box describes the type of data contained in the trak.
// HandlerType can be : "vide" (video track), "soun" (audio track), "subt" (subtitle track)
// Other types are: "hint" (hint track), "meta" (timed Metadata track), "auxv" (auxiliary video track).
type HdlrBox struct {
	Version     byte
	Flags       uint32
	PreDefined  uint32
	HandlerType string
	Name        string
}

// CreateHdlr - create mediaType-specific hdlr box
func CreateHdlr(mediaType string) (*HdlrBox, error) {
	hdlr := &HdlrBox{}
	switch mediaType {
	case "video":
		hdlr.HandlerType = "vide"
		hdlr.Name = "Edgeware Video Handler"
	case "audio":
		hdlr.HandlerType = "soun"
		hdlr.Name = "Edgeware Audio Handler"
	case "subtitle":
		hdlr.HandlerType = "subt"
		hdlr.Name = "Edgeware Subtitle Handler"
	default:
		return nil, fmt.Errorf("Unkown mediaType %s", mediaType)
	}
	return hdlr, nil
}

// DecodeHdlr - box-specific decode
func DecodeHdlr(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := ioutil.ReadAll(r)

	if err != nil {
		return nil, err
	}
	versionAndFlags := binary.BigEndian.Uint32(data[0:4])
	return &HdlrBox{
		Version:     byte(versionAndFlags >> 24),
		Flags:       versionAndFlags & flagsMask,
		PreDefined:  binary.BigEndian.Uint32(data[4:8]),
		HandlerType: string(data[8:12]),
		Name:        string(data[24:]),
	}, nil
}

// Type - box type
func (b *HdlrBox) Type() string {
	return "hdlr"
}

// Size - calculated size of box
func (b *HdlrBox) Size() uint64 {
	return uint64(boxHeaderSize + 24 + len(b.Name))
}

// Encode - write box to w
func (b *HdlrBox) Encode(w io.Writer) error {
	err := EncodeHeader(b, w)
	if err != nil {
		return err
	}
	buf := makebuf(b)
	versionAndFlags := (uint32(b.Version) << 24) | b.Flags
	binary.BigEndian.PutUint32(buf[0:], versionAndFlags)
	binary.BigEndian.PutUint32(buf[4:], b.PreDefined)
	strtobuf(buf[8:], b.HandlerType, 4)
	strtobuf(buf[24:], b.Name, len(b.Name))
	_, err = w.Write(buf)
	return err
}

func (b *HdlrBox) Dump(w io.Writer, indent, indentStep string) error {
	_, err := fmt.Fprintf(w, "%s%s size=%d\n%s - Handler type: %s\n%s - Handler name: %s\n",
		indent, b.Type(), b.Size(), indent, b.HandlerType, indent, b.Name)
	return err
}
