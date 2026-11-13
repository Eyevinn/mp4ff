package mp4

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode/utf16"
)

// PlayReady Object record types.
const (
	PlayReadyRecordHeader       = 1
	PlayReadyRecordLicenseStore = 3
)

// PlayReadyObject is the data of a PlayReady pssh box: a PlayReady Object, as defined in the PlayReady Header
// Specification, https://learn.microsoft.com/en-us/playready/specifications/playready-header-specification.
// It starts with a 6-byte header, the length of the object and the number of records, and then has the records.
type PlayReadyObject struct {
	Records []PlayReadyRecord
}

// PlayReadyRecord is a record of a PlayReady Object.
type PlayReadyRecord struct {
	// Offset is the position of the record in the PlayReady Object. The record starts with a 4-byte header, its
	// type and the length of Data.
	Offset int
	// Type is PlayReadyRecordHeader or PlayReadyRecordLicenseStore.
	Type uint16
	// Data is the value of the record.
	Data []byte
	// HeaderXML is the PlayReady Header in a record of type PlayReadyRecordHeader, decoded from UTF-16LE.
	HeaderXML string
	// KIDs are the key IDs in the PlayReady Header. The header has them in base64, with the first three fields of
	// the GUID little-endian. Here they are in the byte order of the KIDs of pssh and tenc boxes.
	KIDs []UUID
}

// DecodePlayReadyObject decodes the data of a PlayReady pssh box.
func DecodePlayReadyObject(data []byte) (*PlayReadyObject, error) {
	if len(data) < 6 {
		return nil, fmt.Errorf("playready object: %d bytes are too few", len(data))
	}
	length, count := binary.LittleEndian.Uint32(data), binary.LittleEndian.Uint16(data[4:])
	if int64(length) != int64(len(data)) {
		return nil, fmt.Errorf("playready object: length %d is not the %d bytes of the data", length, len(data))
	}
	p := &PlayReadyObject{}
	off := 6
	for i := range int(count) {
		if off+4 > len(data) {
			return nil, fmt.Errorf("playready object: record %d: header goes beyond the end of the data", i+1)
		}
		r := PlayReadyRecord{Offset: off, Type: binary.LittleEndian.Uint16(data[off:])}
		size := int(binary.LittleEndian.Uint16(data[off+2:]))
		if off+4+size > len(data) {
			return nil, fmt.Errorf("playready object: record %d: length %d goes beyond the end of the data", i+1, size)
		}
		r.Data = data[off+4 : off+4+size]
		off += 4 + size
		if r.Type == PlayReadyRecordHeader {
			var err error
			if r.HeaderXML, err = utf16LEText(r.Data); err != nil {
				return nil, fmt.Errorf("playready object: record %d: %w", i+1, err)
			}
			if r.KIDs, err = playReadyKIDs(r.HeaderXML); err != nil {
				return nil, fmt.Errorf("playready object: record %d: %w", i+1, err)
			}
		}
		p.Records = append(p.Records, r)
	}
	if off != len(data) {
		return nil, fmt.Errorf("playready object: %d bytes after the last record", len(data)-off)
	}
	return p, nil
}

// utf16LEText decodes the UTF-16LE text in b, without a byte order mark and trailing NUL characters.
func utf16LEText(b []byte) (string, error) {
	if len(b)%2 != 0 {
		return "", fmt.Errorf("%d bytes are an odd number for UTF-16 text", len(b))
	}
	u := make([]uint16, len(b)/2)
	for i := range u {
		u[i] = binary.LittleEndian.Uint16(b[2*i:])
	}
	s := strings.TrimPrefix(string(utf16.Decode(u)), "\ufeff")
	return strings.TrimRight(s, "\x00"), nil
}

// playReadyKIDs returns the key IDs in the PlayReady Header XML x. A KID element has the key ID as text in header
// version 4.0, and in a VALUE attribute in versions 4.1 to 4.3. KID elements in CUSTOMATTRIBUTES are not key IDs.
func playReadyKIDs(x string) ([]UUID, error) {
	var kids []UUID
	d := xml.NewDecoder(strings.NewReader(x))
	// The text is already UTF-8, whatever the XML declaration says.
	d.CharsetReader = func(_ string, r io.Reader) (io.Reader, error) { return r, nil }
	inKID, custom := false, 0
	for {
		t, err := d.Token()
		if errors.Is(err, io.EOF) {
			return kids, nil
		}
		if err != nil {
			return nil, fmt.Errorf("playready header: %w", err)
		}
		switch t := t.(type) {
		case xml.StartElement:
			if t.Name.Local == "CUSTOMATTRIBUTES" {
				custom++
			}
			inKID = t.Name.Local == "KID" && custom == 0
			for _, a := range t.Attr {
				if inKID && a.Name.Local == "VALUE" {
					kid, err := playReadyKID(a.Value)
					if err != nil {
						return nil, err
					}
					kids = append(kids, kid)
					inKID = false
				}
			}
		case xml.CharData:
			if s := strings.TrimSpace(string(t)); inKID && s != "" {
				kid, err := playReadyKID(s)
				if err != nil {
					return nil, err
				}
				kids = append(kids, kid)
			}
		case xml.EndElement:
			inKID = false
			if t.Name.Local == "CUSTOMATTRIBUTES" {
				custom--
			}
		}
	}
}

// playReadyKID decodes a PlayReady key ID: a GUID in base64 with its first three fields little-endian.
func playReadyKID(s string) (UUID, error) {
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil || len(b) != 16 {
		return nil, fmt.Errorf("playready header: KID %q is not a 16-byte value in base64", s)
	}
	return UUID{b[3], b[2], b[1], b[0], b[5], b[4], b[7], b[6],
		b[8], b[9], b[10], b[11], b[12], b[13], b[14], b[15]}, nil
}

// TypeName returns the name of the type of r, such as "PlayReady Header".
func (r *PlayReadyRecord) TypeName() string {
	switch r.Type {
	case PlayReadyRecordHeader:
		return "PlayReady Header"
	case PlayReadyRecordLicenseStore:
		return "Embedded License Store"
	}
	return "Unknown"
}

// Info writes the records of p to w, every line starting with indent.
func (p *PlayReadyObject) Info(w io.Writer, indent string) error {
	bd := &infoDumper{w: w, indent: indent}
	for i := range p.Records {
		r := &p.Records[i]
		bd.write(" - record[%d]: type=%d (%s) length=%d", i+1, r.Type, r.TypeName(), len(r.Data))
		if bd.err == nil {
			bd.err = r.Info(w, indent+"  ")
		}
	}
	return bd.err
}

// Info writes the fields of r to w, one " - name: value" line each, every line starting with indent: the XML and
// key IDs of a PlayReady Header, and the data of other records in hex.
func (r *PlayReadyRecord) Info(w io.Writer, indent string) error {
	bd := &infoDumper{w: w, indent: indent}
	if r.Type != PlayReadyRecordHeader {
		bd.write(" - data: %s", hex.EncodeToString(r.Data))
		return bd.err
	}
	bd.write(" - xml: %s", r.HeaderXML)
	for i, kid := range r.KIDs {
		bd.write(" - KID[%d]: %s", i+1, kid)
	}
	return bd.err
}
