package mp4_test

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/Eyevinn/mp4ff/mp4"
	"github.com/go-test/deep"
)

// utf16LE encodes s in UTF-16LE.
func utf16LE(s string) []byte {
	var b []byte
	for _, u := range utf16.Encode([]rune(s)) {
		b = binary.LittleEndian.AppendUint16(b, u)
	}
	return b
}

type playReadyTestRecord struct {
	typ  uint16
	data []byte
}

// playReadyObject makes a PlayReady Object with records.
func playReadyObject(records ...playReadyTestRecord) []byte {
	var body []byte
	for _, r := range records {
		body = binary.LittleEndian.AppendUint16(body, r.typ)
		body = binary.LittleEndian.AppendUint16(body, uint16(len(r.data)))
		body = append(body, r.data...)
	}
	b := binary.LittleEndian.AppendUint32(nil, uint32(6+len(body)))
	b = binary.LittleEndian.AppendUint16(b, uint16(len(records)))
	return append(b, body...)
}

// header43 is a version 4.3 PlayReady Header, with two KIDs in VALUE attributes and a KID element in
// CUSTOMATTRIBUTES that is not a key ID.
const header43 = `<WRMHEADER xmlns="http://schemas.microsoft.com/DRM/2007/03/PlayReadyHeader" version="4.3.0.0">` +
	`<DATA><PROTECTINFO><KIDS><KID ALGID="AESCTR" VALUE="nWNX8IeSFTOL9VCZnElF9w=="></KID>` +
	`<KID ALGID="AESCBC" VALUE="MyIRAFVEd2aImaq7zN3u/w=="/></KIDS></PROTECTINFO>` +
	`<CUSTOMATTRIBUTES><KID>not a key ID</KID></CUSTOMATTRIBUTES></DATA></WRMHEADER>`

func TestDecodePlayReadyObject(t *testing.T) {
	_, pr := cencPsshs(t)
	bom := string(rune(0xfeff))
	header43Data := append(utf16LE(bom+header43), 0, 0) // with a byte order mark and a trailing NUL
	data43 := playReadyObject(playReadyTestRecord{mp4.PlayReadyRecordHeader, header43Data},
		playReadyTestRecord{mp4.PlayReadyRecordLicenseStore, []byte{0xab, 0xcd}},
		playReadyTestRecord{9, []byte{0x01}})
	cencHeader := `<WRMHEADER xmlns="http://schemas.microsoft.com/DRM/2007/03/PlayReadyHeader" ` +
		`version="4.0.0.0"><DATA><PROTECTINFO><KEYLEN>16</KEYLEN><ALGID>AESCTR</ALGID></PROTECTINFO>` +
		`<KID>nWNX8IeSFTOL9VCZnElF9w==</KID><LA_URL>https://lic.drmtoday.com/license-proxy-headerauth/` +
		`drmtoday/RightsManager.asmx</LA_URL><LUI_URL>https://foo.blah.com/</LUI_URL>` +
		`<CHECKSUM>ki0HbHtwJwU=</CHECKSUM></DATA></WRMHEADER>`

	cases := []struct {
		desc string
		data []byte
		want *mp4.PlayReadyObject
		info string
	}{
		{
			desc: "init_cenc.cmfv",
			data: pr.Data,
			want: &mp4.PlayReadyObject{Records: []mp4.PlayReadyRecord{{
				Offset:    6,
				Type:      mp4.PlayReadyRecordHeader,
				Data:      pr.Data[10:],
				HeaderXML: cencHeader,
				KIDs:      []mp4.UUID{uuid(t, "f057639d-9287-3315-8bf5-50999c4945f7")},
			}}},
			info: ` - record[1]: type=1 (PlayReady Header) length=776
   - xml: ` + cencHeader + `
   - KID[1]: f057639d-9287-3315-8bf5-50999c4945f7
`,
		},
		{
			desc: "version 4.3 header and other records",
			data: data43,
			want: &mp4.PlayReadyObject{Records: []mp4.PlayReadyRecord{
				{
					Offset:    6,
					Type:      mp4.PlayReadyRecordHeader,
					Data:      header43Data,
					HeaderXML: header43,
					KIDs: []mp4.UUID{uuid(t, "f057639d-9287-3315-8bf5-50999c4945f7"),
						uuid(t, "00112233-4455-6677-8899-aabbccddeeff")},
				},
				{Offset: 10 + len(header43Data), Type: mp4.PlayReadyRecordLicenseStore, Data: []byte{0xab, 0xcd}},
				{Offset: 16 + len(header43Data), Type: 9, Data: []byte{0x01}},
			}},
			info: fmt.Sprintf(` - record[1]: type=1 (PlayReady Header) length=%d
   - xml: %s
   - KID[1]: f057639d-9287-3315-8bf5-50999c4945f7
   - KID[2]: 00112233-4455-6677-8899-aabbccddeeff
 - record[2]: type=3 (Embedded License Store) length=2
   - data: abcd
 - record[3]: type=9 (Unknown) length=1
   - data: 01
`, len(header43Data), header43),
		},
		{
			desc: "no records",
			data: playReadyObject(),
			want: &mp4.PlayReadyObject{},
			info: "",
		},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			got, err := mp4.DecodePlayReadyObject(c.data)
			if err != nil {
				t.Fatal(err)
			}
			if diff := deep.Equal(got, c.want); diff != nil {
				t.Error(diff)
			}
			var buf bytes.Buffer
			if err := got.Info(&buf, ""); err != nil {
				t.Fatal(err)
			}
			if diff := deep.Equal(buf.String(), c.info); diff != nil {
				t.Error(diff)
			}
		})
	}
}

func TestDecodePlayReadyObjectErrors(t *testing.T) {
	header := func(xml string) []byte {
		return playReadyObject(playReadyTestRecord{mp4.PlayReadyRecordHeader, utf16LE(xml)})
	}
	withLength := func(b []byte, length uint32) []byte {
		b = bytes.Clone(b)
		binary.LittleEndian.PutUint32(b, length)
		return b
	}
	twoRecords := playReadyObject(playReadyTestRecord{3, []byte{1}}, playReadyTestRecord{3, []byte{2}})
	cases := []struct {
		desc string
		data []byte
		err  string
	}{
		{"too short", []byte{6, 0, 0, 0, 0}, "5 bytes are too few"},
		{"wrong length", withLength(header("<a/>"), 100), "length 100 is not the 18 bytes of the data"},
		{"record header beyond end", withLength(twoRecords[:13], 13), "record 2: header goes beyond the end"},
		{"record length beyond end", withLength(twoRecords[:15], 15), "record 2: length 1 goes beyond the end"},
		{"bytes after records", withLength(append(bytes.Clone(twoRecords), 0), 17), "1 bytes after the last record"},
		{"odd UTF-16", playReadyObject(playReadyTestRecord{1, []byte{'<', 0, 'a'}}), "3 bytes are an odd number"},
		{"bad XML", header("<a><b></a>"), "record 1: playready header: XML syntax error"},
		{"bad KID", header("<KID>AAAA</KID>"), `record 1: playready header: KID "AAAA" is not a 16-byte value`},
		{"bad KID value", header(`<KID VALUE="!"/>`), `KID "!" is not a 16-byte value in base64`},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			_, err := mp4.DecodePlayReadyObject(c.data)
			if err == nil || !strings.Contains(err.Error(), c.err) {
				t.Errorf("got error %v, want one with %q", err, c.err)
			}
		})
	}
}
