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
// clcp (Closed Captions (QuickTime))
type HdlrBox struct {
	Version              byte
	Flags                uint32
	PreDefined           uint32
	HandlerType          string
	Name                 string // Null-terminated UTF-8 string according to ISO/IEC 14496-12 Sec. 8.4.3.3
	LacksNullTermination bool   // This should be true, but we allow false as well
}

// CreateHdlr - create mediaType-specific hdlr box
func CreateHdlr(mediaOrHdlrType string) (*HdlrBox, error) {
	hdlr := &HdlrBox{}
	switch mediaOrHdlrType {
	case "video", "vide":
		hdlr.HandlerType = "vide"
		hdlr.Name = "mp4ff video handler"
	case "audio", "soun":
		hdlr.HandlerType = "soun"
		hdlr.Name = "mp4ff audio handler"
	case "subtitle", "subt":
		hdlr.HandlerType = "subt"
		hdlr.Name = "mp4ff subtitle handler"
	case "clcp":
		hdlr.HandlerType = "subt"
		hdlr.Name = "mp4ff closed captions handler"
	default:
		if len(mediaOrHdlrType) != 4 {
			return nil, fmt.Errorf("Unkown media or hdlr type %s", mediaOrHdlrType)
		}
		hdlr.HandlerType = mediaOrHdlrType
		hdlr.Name = fmt.Sprintf("mp4ff %s handler", mediaOrHdlrType)

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
	h := &HdlrBox{
		Version:     byte(versionAndFlags >> 24),
		Flags:       versionAndFlags & flagsMask,
		PreDefined:  binary.BigEndian.Uint32(data[4:8]),
		HandlerType: string(data[8:12]),
	}
	if len(data) > 24 {
		endPoint := len(data) - 1
		lastChar := data[endPoint]
		if lastChar != 0 {
			endPoint++
			h.LacksNullTermination = true
		}
		h.Name = string(data[24:endPoint])
	} else {
		h.LacksNullTermination = true
	}
	return h, nil
}

// Type - box type
func (b *HdlrBox) Type() string {
	return "hdlr"
}

// Size - calculated size of box
func (b *HdlrBox) Size() uint64 {
	size := uint64(boxHeaderSize + 24 + len(b.Name) + 1)
	if b.LacksNullTermination {
		size--
	}
	return size
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
	if !b.LacksNullTermination {
		buf[len(buf)-1] = 0 // null-termination of string
	}
	_, err = w.Write(buf)
	return err
}

func (b *HdlrBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, int(b.Version), b.Flags)
	bd.write(" - handlerType: %s", b.HandlerType)
	bd.write(" - handlerName: %q", b.Name)
	return bd.err
}
