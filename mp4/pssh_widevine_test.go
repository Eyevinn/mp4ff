package mp4_test

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"

	"github.com/Eyevinn/mp4ff/mp4"
	"github.com/go-test/deep"
)

// cencPsshs returns the Widevine and PlayReady pssh boxes of testdata/init_cenc.cmfv.
func cencPsshs(t *testing.T) (widevine, playReady *mp4.PsshBox) {
	t.Helper()
	f, err := mp4.ReadMP4File("testdata/init_cenc.cmfv")
	if err != nil {
		t.Fatal(err)
	}
	psshs := f.Init.Moov.Psshs
	if len(psshs) != 2 {
		t.Fatalf("got %d pssh boxes, want 2", len(psshs))
	}
	return psshs[0], psshs[1]
}

func uuid(t *testing.T, s string) mp4.UUID {
	t.Helper()
	u, err := mp4.NewUUIDFromString(s)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func ptr32(v uint32) *uint32 { return &v }

// Protobuf encoding of test messages.

func pbKey(num, wireType int) []byte {
	return binary.AppendUvarint(nil, uint64(num<<3|wireType))
}

func pbVarint(num int, v uint64) []byte {
	return binary.AppendUvarint(pbKey(num, 0), v)
}

func pbBytes(num int, b []byte) []byte {
	return append(binary.AppendUvarint(pbKey(num, 2), uint64(len(b))), b...)
}

func TestDecodeWidevinePsshData(t *testing.T) {
	wv, _ := cencPsshs(t)
	b64Pssh, err := mp4.PsshBoxesFromBase64("AAAASnBzc2gAAAAA7e+LqXnWSs6jyCfc1R0h7QAAACoSEDEuM2I0EEaatTa5ydDK/DESEDEu" +
		"M2I0EEaatTa5ydDK/DFI49yVmwY=")
	if err != nil {
		t.Fatal(err)
	}
	kid := uuid(t, "312e3362-3410-469a-b536-b9c9d0cafc31")
	entitledKey := bytes.Join([][]byte{
		pbBytes(1, uuid(t, "00112233-4455-6677-8899-aabbccddeeff")),
		pbBytes(2, kid),
		pbBytes(3, []byte{0xde, 0xad}),
		pbBytes(4, []byte{0xbe, 0xef}),
		pbVarint(5, 16),
	}, nil)
	allFields := bytes.Join([][]byte{
		pbVarint(1, 1),
		pbBytes(2, kid),
		pbBytes(3, []byte("provider")),
		pbBytes(4, []byte{0x01, 0x02}),
		pbBytes(5, []byte("HD")),
		pbBytes(6, []byte("policy")),
		pbVarint(7, 0),
		pbBytes(8, []byte{0x0a, 0x00}),
		pbVarint(9, 0x63626373),
		pbVarint(10, 30),
		pbVarint(11, 2),
		pbVarint(12, 7),
		pbBytes(13, []byte("group")),
		pbBytes(14, entitledKey),
		pbBytes(15, []byte("HDR")),
		pbVarint(20, 1),                          // unknown varint field
		pbBytes(21, []byte{1, 2, 3}),             // unknown length-delimited field
		append(pbKey(22, 1), make([]byte, 8)...), // unknown 64-bit field
		append(pbKey(23, 5), make([]byte, 4)...), // unknown 32-bit field
	}, nil)

	cases := []struct {
		desc string
		data []byte
		want *mp4.WidevinePsshData
		info string
	}{
		{
			desc: "init_cenc.cmfv",
			data: wv.Data,
			want: &mp4.WidevinePsshData{
				Algorithm: ptr32(1),
				KeyIDs:    []mp4.UUID{uuid(t, "f057639d-9287-3315-8bf5-50999c4945f7")},
				Provider:  "castlabs",
				ContentID: []byte("eyJhc3NldElkIjoidHYyX2Z5biJ9"),
				Policy:    "default",
			},
			info: ` - algorithm: 1 (AESCTR)
 - keyID[1]: f057639d-9287-3315-8bf5-50999c4945f7
 - provider: "castlabs"
 - contentID: "eyJhc3NldElkIjoidHYyX2Z5biJ9"
 - policy: "default"
`,
		},
		{
			desc: "two key IDs and protection scheme",
			data: b64Pssh[0].Data,
			want: &mp4.WidevinePsshData{
				KeyIDs:           []mp4.UUID{kid, kid},
				ProtectionScheme: ptr32(0x63656e63),
			},
			info: ` - keyID[1]: 312e3362-3410-469a-b536-b9c9d0cafc31
 - keyID[2]: 312e3362-3410-469a-b536-b9c9d0cafc31
 - protectionScheme: cenc
`,
		},
		{
			desc: "all fields",
			data: allFields,
			want: &mp4.WidevinePsshData{
				Algorithm:           ptr32(1),
				KeyIDs:              []mp4.UUID{kid},
				Provider:            "provider",
				ContentID:           []byte{0x01, 0x02},
				TrackType:           "HD",
				Policy:              "policy",
				CryptoPeriodIndex:   ptr32(0),
				GroupedLicense:      []byte{0x0a, 0x00},
				ProtectionScheme:    ptr32(0x63626373),
				CryptoPeriodSeconds: ptr32(30),
				Type:                ptr32(2),
				KeySequence:         ptr32(7),
				GroupIDs:            [][]byte{[]byte("group")},
				EntitledKeys: []mp4.WidevineEntitledKey{{
					EntitlementKeyID:        uuid(t, "00112233-4455-6677-8899-aabbccddeeff"),
					KeyID:                   kid,
					Key:                     []byte{0xde, 0xad},
					IV:                      []byte{0xbe, 0xef},
					EntitlementKeySizeBytes: ptr32(16),
				}},
				VideoFeature: "HDR",
			},
			info: ` - algorithm: 1 (AESCTR)
 - keyID[1]: 312e3362-3410-469a-b536-b9c9d0cafc31
 - provider: "provider"
 - contentID: 0102
 - trackType: "HD"
 - policy: "policy"
 - cryptoPeriodIndex: 0
 - groupedLicense: 0a00
 - protectionScheme: cbcs
 - cryptoPeriodSeconds: 30
 - type: 2 (ENTITLED_KEY)
 - keySequence: 7
 - groupID[1]: "group"
 - entitledKey[1]:
   - entitlementKeyID: 00112233-4455-6677-8899-aabbccddeeff
   - keyID: 312e3362-3410-469a-b536-b9c9d0cafc31
   - key: dead
   - iv: beef
   - entitlementKeySizeBytes: 16
 - videoFeature: "HDR"
`,
		},
		{
			desc: "unknown enum values and protection scheme",
			data: bytes.Join([][]byte{pbVarint(1, 5), pbVarint(9, 1), pbVarint(11, 9)}, nil),
			want: &mp4.WidevinePsshData{Algorithm: ptr32(5), ProtectionScheme: ptr32(1), Type: ptr32(9)},
			info: ` - algorithm: 5
 - protectionScheme: 0x00000001
 - type: 9
`,
		},
		{
			desc: "empty",
			data: nil,
			want: &mp4.WidevinePsshData{},
			info: "",
		},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			got, err := mp4.DecodeWidevinePsshData(c.data)
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

func TestDecodeWidevinePsshDataErrors(t *testing.T) {
	cases := []struct {
		desc string
		data []byte
		err  string
	}{
		{"bad key", []byte{0x80}, "bad field key"},
		{"bad varint", []byte{0x08, 0x80}, "field 1: bad varint"},
		{"bad length", []byte{0x12, 0x05, 0x01}, "field 2: bad length"},
		{"truncated 64-bit field", append(pbKey(22, 1), 0x00), "field 22: 1 bytes are too few"},
		{"wrong wire type", pbVarint(2, 1), "field 2: wrong wire type 0"},
		{"unsupported wire type", pbKey(3, 3), "field 3: unsupported wire type 3"},
		{"bad entitled key", pbBytes(14, []byte{0x08}), "entitled key: field 1: bad varint"},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			_, err := mp4.DecodeWidevinePsshData(c.data)
			if err == nil || !strings.HasSuffix(err.Error(), c.err) {
				t.Errorf("got error %v, want one that ends with %q", err, c.err)
			}
		})
	}
}

func TestPsshInfoLevels(t *testing.T) {
	cases := []struct {
		desc     string
		systemID string
		data     []byte
		levels   string
		want     string
	}{
		{"decoded at level 0", mp4.UUIDWidevine, pbVarint(9, 0x63656e63), "", `[pssh] size=38 version=0 flags=000000
 - systemID: edef8ba9-79d6-4ace-a3c8-27dcd51d21ed (Widevine)
`},
		{"decoded at level 1", mp4.UUIDWidevine, pbVarint(9, 0x63656e63), "pssh:1", `[pssh] size=38 version=0 flags=000000
 - systemID: edef8ba9-79d6-4ace-a3c8-27dcd51d21ed (Widevine)
 - WidevinePsshData:
   - protectionScheme: cenc
`},
		{"decoded at level 2", mp4.UUIDWidevine, pbVarint(9, 0x63656e63), "pssh:2", `[pssh] size=38 version=0 flags=000000
 - systemID: edef8ba9-79d6-4ace-a3c8-27dcd51d21ed (Widevine)
 - data: 48e3dc959b06
 - WidevinePsshData:
   - protectionScheme: cenc
`},
		{"decode error at level 1", mp4.UUIDWidevine, []byte{0x80}, "pssh:1", `[pssh] size=33 version=0 flags=000000
 - systemID: edef8ba9-79d6-4ace-a3c8-27dcd51d21ed (Widevine)
 - data: 80
 - WidevinePsshData: widevine pssh data: bad field key
`},
		{"other system at level 1", mp4.UUIDFairPlay, []byte{0x01}, "pssh:1", `[pssh] size=33 version=0 flags=000000
 - systemID: 94ce86fb-07ff-4f43-adb8-93d2fa968ca2 (FairPlay)
 - data: 01
`},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			pssh, err := mp4.NewPsshBox(c.systemID, nil, c.data)
			if err != nil {
				t.Fatal(err)
			}
			var buf bytes.Buffer
			if err := pssh.Info(&buf, c.levels, "", ""); err != nil {
				t.Fatal(err)
			}
			if diff := deep.Equal(buf.String(), c.want); diff != nil {
				t.Error(diff)
			}
		})
	}
}
