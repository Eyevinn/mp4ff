package mp4

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"unicode/utf16"

	"github.com/Eyevinn/mp4ff/bits"
)

// UserDataTextBox is a QuickTime user data text atom, whose type starts with ©, such as ©nam or ©swr in a udta
// box. It holds a text in one or more languages.
//
// iTunes metadata in an ilst box uses the same types for items that hold data boxes. A box whose type starts
// with © decodes as a UserDataTextBox only if its payload is a list of texts with valid language codes, not all
// empty, and otherwise as a container box for ©ART, ©nam, ©too and ©cpy, and as an UnknownBox for other types.
type UserDataTextBox struct {
	name  string
	Texts []UserDataText
}

// UserDataText is a text of a user data text atom, in one language.
//
// In the atom, each text has a 16-bit size and a 16-bit language code before it. The size is the number of
// bytes of the text, as QuickTime and ffmpeg write it. The QuickTime File Format specification says that it
// includes the four bytes of the size and the language code.
type UserDataText struct {
	// Language is a Macintosh language code if it is below 0x400 or is 0x7FFF, which means an unspecified
	// Macintosh language, and otherwise an ISO 639-2/T code packed as in mdhd.
	Language uint16
	// Text is the text: in Macintosh text encoding for a Macintosh language code, and otherwise in UTF-8, or in
	// big-endian UTF-16 if it starts with a byte order mark.
	Text string
}

// unspecifiedMacLanguage is the language code of a text in an unspecified Macintosh language.
const unspecifiedMacLanguage = 0x7fff

// NewUserDataTextBox returns a user data text atom of type boxType, which should start with ©, with texts.
func NewUserDataTextBox(boxType string, texts ...UserDataText) *UserDataTextBox {
	return &UserDataTextBox{name: boxType, Texts: texts}
}

// isUserDataTextType tells if boxType starts with ©, as the types of user data text atoms do.
func isUserDataTextType(boxType string) bool {
	return len(boxType) == 4 && boxType[0] == 0xa9
}

// parseUserDataTexts returns the texts in payload, and false if payload is not a list of texts with valid
// language codes, or if all the texts are empty, as in four zero bytes.
func parseUserDataTexts(payload []byte) ([]UserDataText, bool) {
	var texts []UserDataText
	nonEmpty := false
	for pos := 0; pos < len(payload); {
		if pos+4 > len(payload) {
			return nil, false
		}
		size := int(binary.BigEndian.Uint16(payload[pos:]))
		lang := binary.BigEndian.Uint16(payload[pos+2:])
		end := pos + 4 + size
		if end > len(payload) || !validUserDataLanguage(lang) {
			return nil, false
		}
		texts = append(texts, UserDataText{Language: lang, Text: string(payload[pos+4 : end])})
		nonEmpty = nonEmpty || size > 0
		pos = end
	}
	return texts, nonEmpty
}

// validUserDataLanguage tells if lang is a Macintosh language code or a packed ISO 639-2/T code of three letters.
func validUserDataLanguage(lang uint16) bool {
	if lang < 0x400 || lang == unspecifiedMacLanguage {
		return true
	}
	if lang&0x8000 != 0 {
		return false
	}
	for shift := 10; shift >= 0; shift -= 5 {
		if c := lang >> shift & 0x1f; c < 1 || c > 26 {
			return false
		}
	}
	return true
}

// decodeUserDataTextOr decodes the payload of a box whose type starts with © from r: as a UserDataTextBox if
// it is a list of texts, and otherwise with d, or as an UnknownBox if d is nil.
func decodeUserDataTextOr(hdr BoxHeader, startPos uint64, d BoxDecoder, r io.Reader) (Box, error) {
	data, err := readBoxBody(r, hdr)
	if err != nil {
		return nil, err
	}
	if texts, ok := parseUserDataTexts(data); ok {
		return &UserDataTextBox{name: hdr.Name, Texts: texts}, nil
	}
	if d == nil {
		return DecodeUnknown(hdr, startPos, bytes.NewReader(data))
	}
	return d(hdr, startPos, bytes.NewReader(data))
}

// isUserDataTextSR tells if the payload of the box of hdr, which is next in sr, is a list of texts. It does not
// move the read position of sr.
func isUserDataTextSR(hdr BoxHeader, sr bits.SliceReader) bool {
	if !isUserDataTextType(hdr.Name) || sr.NrRemainingBytes() < hdr.payloadLen() {
		return false
	}
	pos := sr.GetPos()
	_, ok := parseUserDataTexts(sr.ReadBytes(hdr.payloadLen()))
	sr.SetPos(pos)
	return ok
}

// DecodeUserDataText decodes a user data text atom.
func DecodeUserDataText(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := readBoxBody(r, hdr)
	if err != nil {
		return nil, err
	}
	return DecodeUserDataTextSR(hdr, startPos, bits.NewFixedSliceReader(data))
}

// DecodeUserDataTextSR decodes a user data text atom.
func DecodeUserDataTextSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	texts, ok := parseUserDataTexts(sr.ReadBytes(hdr.payloadLen()))
	if err := sr.AccError(); err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("%s: the payload is not a list of user data texts", hdr.Name)
	}
	return &UserDataTextBox{name: hdr.Name, Texts: texts}, nil
}

// Type returns the box type, such as ©nam.
func (b *UserDataTextBox) Type() string {
	return b.name
}

// Size returns the size of the box.
func (b *UserDataTextBox) Size() uint64 {
	size := uint64(boxHeaderSize)
	for _, t := range b.Texts {
		size += 4 + uint64(len(t.Text))
	}
	return size
}

// Encode writes the box to w.
func (b *UserDataTextBox) Encode(w io.Writer) error {
	sw := bits.NewFixedSliceWriter(int(b.Size()))
	if err := b.EncodeSW(sw); err != nil {
		return err
	}
	_, err := w.Write(sw.Bytes())
	return err
}

// EncodeSW writes the box to sw.
func (b *UserDataTextBox) EncodeSW(sw bits.SliceWriter) error {
	for _, t := range b.Texts {
		if len(t.Text) > 0xffff {
			return fmt.Errorf("%s: text of %d bytes is longer than 65535", b.name, len(t.Text))
		}
	}
	if err := EncodeHeaderSW(b, sw); err != nil {
		return err
	}
	for _, t := range b.Texts {
		sw.WriteUint16(uint16(len(t.Text)))
		sw.WriteUint16(t.Language)
		sw.WriteString(t.Text, false)
	}
	return sw.AccError()
}

// Info writes the texts with their languages.
func (b *UserDataTextBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, -1, 0)
	for _, t := range b.Texts {
		bd.write(" - text: language=%s %q", t.LanguageString(), t.UTF8())
	}
	return bd.err
}

// LanguageString returns the language as a three-letter ISO 639-2/T code, as "mac:N" for a Macintosh language
// code N, or as "mac:unspecified".
func (t UserDataText) LanguageString() string {
	switch {
	case t.Language == unspecifiedMacLanguage:
		return "mac:unspecified"
	case t.Language < 0x400:
		return fmt.Sprintf("mac:%d", t.Language)
	}
	return fmt.Sprintf("%c%c%c", byte(t.Language>>10&0x1f)+charOffset, byte(t.Language>>5&0x1f)+charOffset,
		byte(t.Language&0x1f)+charOffset)
}

// UTF8 returns the text in UTF-8. A text that starts with a UTF-16 byte order mark is converted from big-endian
// UTF-16, and other texts are returned as they are, so that Macintosh-encoded text keeps its bytes.
func (t UserDataText) UTF8() string {
	b := []byte(t.Text)
	if len(b) < 2 || b[0] != 0xfe || b[1] != 0xff {
		return t.Text
	}
	u := make([]uint16, 0, (len(b)-2)/2)
	for i := 2; i+1 < len(b); i += 2 {
		u = append(u, binary.BigEndian.Uint16(b[i:]))
	}
	return string(utf16.Decode(u))
}
