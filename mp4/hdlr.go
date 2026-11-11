package mp4

import (
	"fmt"
	"io"
	"strings"

	"github.com/Eyevinn/mp4ff/bits"
)

// HdlrBox - Handler Reference Box (hdlr - mandatory)
//
// Contained in: Media Box (mdia) or Meta Box (meta)
//
// This box describes the type of data contained in the trak.
// Most common hnadler types are: "vide" (video track), "soun" (audio track), "subt" (subtitle track),
// "text" (text track). "meta" (timed Metadata track), clcp (Closed Captions (QuickTime))
//
// A QuickTime handler, in mdia or minf, puts its component type in PreDefined, see QuickTimeComponentType,
// and has a counted string as name, see QuickTimeComponentName.
type HdlrBox struct {
	Version              byte
	Flags                uint32
	PreDefined           uint32    // Zero in ISO/IEC 14496-12. QuickTime: component type mhlr or dhlr
	HandlerType          string    // QuickTime: component subtype
	Reserved             [3]uint32 // Zero in ISO/IEC 14496-12. QuickTime: manufacturer (e.g. "appl"), flags, flags mask
	Name                 string    // Null-terminated UTF-8 string according to ISO/IEC 14496-12 Sec. 8.4.3.3
	LacksNullTermination bool      // This should be false, but we allow true as well
}

// Component types of a QuickTime handler, in HdlrBox.PreDefined.
const (
	// QuickTimeMediaHandler is the component type of the media handler, in mdia.
	QuickTimeMediaHandler = "mhlr"
	// QuickTimeDataHandler is the component type of the data handler, in minf, whose handler type is the
	// data reference type, such as alis.
	QuickTimeDataHandler = "dhlr"
)

// CreateHdlr - create mediaType-specific hdlr box
//
// mediaOrHdlrType is a media type or sample entry type with a known handler
// ("video", "audio", "subtitle", "subtitles", "stpp", "text", "wvtt", "meta"
// or "clcp"), or any other four-character handler type, which is used as is.
func CreateHdlr(mediaOrHdlrType string) (*HdlrBox, error) {
	hdlr := &HdlrBox{}
	switch mediaOrHdlrType {
	case "video", "vide":
		hdlr.HandlerType = "vide"
		hdlr.Name = "mp4ff video handler"
	case "audio", "soun":
		hdlr.HandlerType = "soun"
		hdlr.Name = "mp4ff audio handler"
	case "subtitle", "subtitles", "subt", "stpp":
		hdlr.HandlerType = "subt"
		hdlr.Name = "mp4ff subtitle handler"
	case "text", "wvtt":
		hdlr.HandlerType = "text"
		hdlr.Name = "mp4ff text handler"
	case "meta":
		hdlr.HandlerType = "meta"
		hdlr.Name = "mp4ff timed metadata handler"
	case "clcp":
		hdlr.HandlerType = "subt"
		hdlr.Name = "mp4ff closed captions handler"
	default:
		if len(mediaOrHdlrType) != 4 {
			return nil, fmt.Errorf("handler type is not four characters: %s", mediaOrHdlrType)
		}
		hdlr.HandlerType = mediaOrHdlrType
		hdlr.Name = fmt.Sprintf("mp4ff %s handler", mediaOrHdlrType)
	}
	return hdlr, nil
}

// DecodeHdlr - box-specific decode
func DecodeHdlr(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := readBoxBody(r, hdr)
	if err != nil {
		return nil, err
	}
	sr := bits.NewFixedSliceReader(data)
	return DecodeHdlrSR(hdr, startPos, sr)
}

// DecodeHdlrSR - box-specific decode
func DecodeHdlrSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	versionAndFlags := sr.ReadUint32()
	h := HdlrBox{
		Version:     byte(versionAndFlags >> 24),
		Flags:       versionAndFlags & flagsMask,
		PreDefined:  sr.ReadUint32(),
		HandlerType: sr.ReadFixedLengthString(4),
	}
	for i := range h.Reserved {
		h.Reserved[i] = sr.ReadUint32()
	}
	nrBytesLeft := hdr.payloadLen() - 24
	if nrBytesLeft > 0 {
		bytesLeft := sr.ReadBytes(nrBytesLeft)
		lastChar := bytesLeft[len(bytesLeft)-1]
		if lastChar != 0 {
			h.LacksNullTermination = true
			h.Name = string(bytesLeft)
		} else {
			h.Name = string(bytesLeft[:len(bytesLeft)-1])
		}
	} else {
		h.LacksNullTermination = true
	}
	return &h, sr.AccError()
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
	sw := bits.NewFixedSliceWriter(int(b.Size()))
	err := b.EncodeSW(sw)
	if err != nil {
		return err
	}
	_, err = w.Write(sw.Bytes())
	return err
}

// EncodeSW - box-specific encode to slicewriter
func (b *HdlrBox) EncodeSW(sw bits.SliceWriter) error {
	err := EncodeHeaderSW(b, sw)
	if err != nil {
		return err
	}
	versionAndFlags := (uint32(b.Version) << 24) | b.Flags
	sw.WriteUint32(versionAndFlags)
	sw.WriteUint32(b.PreDefined)
	sw.WriteString(b.HandlerType, false)
	for _, r := range b.Reserved {
		sw.WriteUint32(r)
	}
	sw.WriteString(b.Name, !b.LacksNullTermination)
	return sw.AccError()
}

// QuickTimeComponentType returns the QuickTime component type in PreDefined, QuickTimeMediaHandler or
// QuickTimeDataHandler, or "" if PreDefined holds neither, as in a handler of ISO/IEC 14496-12, where it is 0.
func (b *HdlrBox) QuickTimeComponentType() string {
	switch t := string([]byte{byte(b.PreDefined >> 24), byte(b.PreDefined >> 16), byte(b.PreDefined >> 8),
		byte(b.PreDefined)}); t {
	case QuickTimeMediaHandler, QuickTimeDataHandler:
		return t
	}
	return ""
}

// QuickTimeComponentName returns Name read as the counted string of a QuickTime handler: a byte with the length,
// followed by that many bytes. Zero bytes may follow the string, and an empty Name is an empty string.
// ok is false if Name is not such a string.
func (b *HdlrBox) QuickTimeComponentName() (name string, ok bool) {
	data := b.Name
	if !b.LacksNullTermination {
		data += "\x00"
	}
	if data == "" {
		return "", true
	}
	end := 1 + int(data[0])
	if end > len(data) || strings.Trim(data[end:], "\x00") != "" {
		return "", false
	}
	return data[1:end], true
}

// Info - write box-specific information
//
// For a QuickTime handler, it writes the component fields, and the name as a counted string if it is one.
func (b *HdlrBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, int(b.Version), b.Flags)
	componentType := b.QuickTimeComponentType()
	switch componentType {
	case QuickTimeMediaHandler:
		bd.write(" - componentType: %s (media handler)", componentType)
	case QuickTimeDataHandler:
		bd.write(" - componentType: %s (data handler)", componentType)
	default:
		if b.PreDefined != 0 {
			bd.write(" - preDefined: %08x", b.PreDefined)
		}
	}
	bd.write(" - handlerType: %s", b.HandlerType)
	if componentType == "" {
		if b.Reserved != [3]uint32{} {
			bd.write(" - reserved: %08x %08x %08x", b.Reserved[0], b.Reserved[1], b.Reserved[2])
		}
		bd.write(" - handlerName: %q", b.Name)
		return bd.err
	}
	bd.write(" - componentManufacturer: %s, componentFlags: %08x, componentFlagsMask: %08x",
		quickTimeVendorString(b.Reserved[0]), b.Reserved[1], b.Reserved[2])
	if name, ok := b.QuickTimeComponentName(); ok {
		bd.write(" - componentName: %q", name)
	} else {
		bd.write(" - handlerName: %q", b.Name)
	}
	return bd.err
}
