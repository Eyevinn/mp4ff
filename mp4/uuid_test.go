package mp4_test

import (
	"bytes"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/Eyevinn/mp4ff/mp4"
)

func TestUUIDVariants(t *testing.T) {

	testInputs := []struct {
		expectedSubType string
		rawData         string
	}{
		{
			"tfxd", "0000002c757569646d1d9b0542d544e680e2141daff757b201000000000105c649bda4000000000000054600",
		},
		{
			"tfrf", "0000002d75756964d4807ef2ca3946958e5426cb9e46a79f0100000001000105c649c2ea000000000000054600",
		},
		{
			"unknown", "0000002c757569646e1d9b0542d544e680e2141daff757b201000000000105c649bda4000000000000054600",
		},
	}

	for _, ti := range testInputs {
		inRawBox, _ := hex.DecodeString(ti.rawData)
		inbuf := bytes.NewBuffer(inRawBox)
		hdr, err := mp4.DecodeHeader(inbuf)
		if err != nil {
			t.Error(err)
		}
		uuidRead, err := mp4.DecodeUUIDBox(hdr, 0, inbuf)
		if err != nil {
			t.Error(err)
		}
		uBox := uuidRead.(*mp4.UUIDBox)
		if uBox.SubType() != ti.expectedSubType {
			t.Errorf("got subtype %s instead of %s", uBox.SubType(), ti.expectedSubType)
		}

		outbuf := &bytes.Buffer{}

		err = uuidRead.Encode(outbuf)
		if err != nil {
			t.Error(err)
		}

		outRawBox := outbuf.Bytes()

		if !bytes.Equal(inRawBox, outRawBox) {
			for i := 0; i < len(inRawBox); i++ {
				t.Logf("%3d %02x %02x\n", i, inRawBox[i], outRawBox[i])
			}
			t.Errorf("%s: Non-matching in and out binaries", ti.expectedSubType)
		}
	}
}

func TestSetUUID(t *testing.T) {
	testCases := []struct {
		uuidStr    string
		expected   mp4.UUID
		shouldFail bool
	}{
		{
			uuidStr:    "6d1d9b05-42d5-44e6-80e2-141daff757b2",
			shouldFail: false,
		},
		{
			uuidStr:    "6d1d9b05-42d5-44e6-80e2-141daff757",
			shouldFail: true,
		},
	}
	for i, tc := range testCases {
		u := mp4.UUIDBox{}
		err := u.SetUUID(tc.uuidStr)
		if tc.shouldFail {
			if err == nil {
				t.Errorf("case %d did not fail as expected", i)
			}
			continue
		}
		if u.UUID() != tc.uuidStr {
			t.Errorf("got %s instead of %s", u.UUID(), tc.uuidStr)
		}
	}
}

func TestUUIDEncodeDecoder(t *testing.T) {

	tfrf := mp4.NewTfrfBox(1, []uint64{0}, []uint64{1000000})
	boxDiffAfterEncodeAndDecode(t, tfrf)

	tfxd := mp4.NewTfxdBox(0, 1_000_000)
	boxDiffAfterEncodeAndDecode(t, tfxd)
}

func TestUnpackKey(t *testing.T) {
	cases := []struct {
		desc        string
		keyStr      string
		expected    []byte
		expectedErr string
	}{
		{
			desc:   "valid hex key",
			keyStr: "00112233445566778899aabbccddeeff",
			expected: []byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77,
				0x88, 0x99, 0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff},
			expectedErr: "",
		},
		{
			desc:        "invalid hex key",
			keyStr:      "0011223x445566778899aabbccddeeff",
			expectedErr: "bad hex 001122...ddeeff: encoding/hex: invalid byte: U+0078 'x'",
		},
		{
			desc:        "wrong length key",
			keyStr:      "00112233445566778899aab",
			expectedErr: "cannot decode key 00112233445566778899aab",
		},
		{
			desc:   "good uuid",
			keyStr: "00112233-4455-6677-8899-aabbccddeeff",
			expected: []byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77,
				0x88, 0x99, 0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff},
			expectedErr: "",
		},
		{
			desc:        "bad uuid, misplaced dashes",
			keyStr:      "00----112233445566778899aabbccddeeff",
			expectedErr: "bad uuid format: 00----...ddeeff",
		},
		{
			desc:        "bad uuid too many dashes",
			keyStr:      "00112233-4-55-6677-8899-aabbccddeeff",
			expectedErr: "bad uuid format: 001122...ddeeff",
		},
		{
			desc:        "bad hex in uuid",
			keyStr:      "0011223x-4455-6677-8899-aabbccddeeff",
			expectedErr: "bad uuid 001122...ddeeff: encoding/hex: invalid byte: U+0078 'x'",
		},
		{
			desc:        "valid base64 key",
			keyStr:      "ABEiM0RVZneImaq7zN3u/w=-",
			expectedErr: "bad base64 ABEiM0...3u/w=-: illegal base64 data at input byte 22",
		},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			key, err := mp4.UnpackKey(c.keyStr)
			if c.expectedErr != "" {
				if err == nil {
					t.Error("expected error but got nil")
				}
				if err.Error() != c.expectedErr {
					t.Errorf("error %q not matching expected error %q", err, c.expectedErr)
				}
				return
			}
			if !bytes.Equal(key, c.expected) {
				t.Errorf("got %x instead of %x", key, c.expected)
			}
		})
	}

}

func TestSphericalVideoV1(t *testing.T) {
	// Sample spherical video v1 XML metadata
	xmlData := `<?xml version="1.0"?><rdf:SphericalVideo
xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
xmlns:GSpherical="http://ns.google.com/videos/1.0/spherical/">` +
		`<GSpherical:Spherical>true</GSpherical:Spherical>` +
		`<GSpherical:Stitched>true</GSpherical:Stitched>` +
		`<GSpherical:StitchingSoftware>Spherical Metadata Tool</GSpherical:StitchingSoftware>` +
		`<GSpherical:ProjectionType>equirectangular</GSpherical:ProjectionType>` +
		`<GSpherical:StereoMode>top-bottom</GSpherical:StereoMode>` +
		`</rdf:SphericalVideo>`

	// Create a UUID box with spherical v1 UUID
	uuidBytes, _ := hex.DecodeString("ffcc8263f8554a938814587a02521fdd")

	// Create the raw box data: size + type + uuid + xml
	size := uint32(8 + 16 + len(xmlData))
	var buf bytes.Buffer

	// Write manually since we're building from scratch
	buf.Write([]byte{byte(size >> 24), byte(size >> 16), byte(size >> 8), byte(size)})
	buf.Write([]byte{'u', 'u', 'i', 'd'})
	buf.Write(uuidBytes)
	buf.Write([]byte(xmlData))

	reader := bytes.NewReader(buf.Bytes())
	hdr, err := mp4.DecodeHeader(reader)
	if err != nil {
		t.Fatal(err)
	}

	box, err := mp4.DecodeUUIDBox(hdr, 0, reader)
	if err != nil {
		t.Fatal(err)
	}

	uuidBox := box.(*mp4.UUIDBox)

	if uuidBox.SubType() != "spherical-v1" {
		t.Errorf("got subtype %s instead of spherical-v1", uuidBox.SubType())
	}

	expectedUUID := "ffcc8263-f855-4a93-8814-587a02521fdd"
	if uuidBox.UUID() != expectedUUID {
		t.Errorf("got uuid %s instead of %s", uuidBox.UUID(), expectedUUID)
	}

	if uuidBox.SphericalV1 == nil {
		t.Fatal("SphericalV1 data is nil")
	}

	s := uuidBox.SphericalV1
	if s.Spherical != "true" {
		t.Errorf("got Spherical %s instead of true", s.Spherical)
	}
	if s.Stitched != "true" {
		t.Errorf("got Stitched %s instead of true", s.Stitched)
	}
	if s.StitchingSoftware != "Spherical Metadata Tool" {
		t.Errorf("got StitchingSoftware %s instead of Spherical Metadata Tool", s.StitchingSoftware)
	}
	if s.ProjectionType != "equirectangular" {
		t.Errorf("got ProjectionType %s instead of equirectangular", s.ProjectionType)
	}
	if s.StereoMode != "top-bottom" {
		t.Errorf("got StereoMode %s instead of top-bottom", s.StereoMode)
	}

	// Test encode/decode round-trip
	var outBuf bytes.Buffer
	err = uuidBox.Encode(&outBuf)
	if err != nil {
		t.Fatal(err)
	}

	if uint64(outBuf.Len()) != uuidBox.Size() {
		t.Errorf("encoded size %d doesn't match Size() %d", outBuf.Len(), uuidBox.Size())
	}

	// Test with all optional fields
	fullXML := `<?xml version="1.0"?><rdf:SphericalVideo
xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
xmlns:GSpherical="http://ns.google.com/videos/1.0/spherical/">` +
		`<GSpherical:Spherical>true</GSpherical:Spherical>` +
		`<GSpherical:Stitched>true</GSpherical:Stitched>` +
		`<GSpherical:StitchingSoftware>Test Software</GSpherical:StitchingSoftware>` +
		`<GSpherical:ProjectionType>equirectangular</GSpherical:ProjectionType>` +
		`<GSpherical:StereoMode>mono</GSpherical:StereoMode>` +
		`<GSpherical:SourceCount>6</GSpherical:SourceCount>` +
		`<GSpherical:InitialViewHeadingDegrees>90</GSpherical:InitialViewHeadingDegrees>` +
		`<GSpherical:InitialViewPitchDegrees>0</GSpherical:InitialViewPitchDegrees>` +
		`<GSpherical:InitialViewRollDegrees>0</GSpherical:InitialViewRollDegrees>` +
		`<GSpherical:Timestamp>1400454971</GSpherical:Timestamp>` +
		`<GSpherical:FullPanoWidthPixels>1920</GSpherical:FullPanoWidthPixels>` +
		`<GSpherical:FullPanoHeightPixels>1080</GSpherical:FullPanoHeightPixels>` +
		`<GSpherical:CroppedAreaImageWidthPixels>1920</GSpherical:CroppedAreaImageWidthPixels>` +
		`<GSpherical:CroppedAreaImageHeightPixels>1080</GSpherical:CroppedAreaImageHeightPixels>` +
		`<GSpherical:CroppedAreaLeftPixels>0</GSpherical:CroppedAreaLeftPixels>` +
		`<GSpherical:CroppedAreaTopPixels>0</GSpherical:CroppedAreaTopPixels>` +
		`</rdf:SphericalVideo>`

	fullSize := uint32(8 + 16 + len(fullXML))
	buf.Reset()
	buf.Write([]byte{byte(fullSize >> 24), byte(fullSize >> 16), byte(fullSize >> 8), byte(fullSize)})
	buf.Write([]byte{'u', 'u', 'i', 'd'})
	buf.Write(uuidBytes)
	buf.Write([]byte(fullXML))

	reader = bytes.NewReader(buf.Bytes())
	hdr, err = mp4.DecodeHeader(reader)
	if err != nil {
		t.Fatal(err)
	}

	box, err = mp4.DecodeUUIDBox(hdr, 0, reader)
	if err != nil {
		t.Fatal(err)
	}

	uuidBox = box.(*mp4.UUIDBox)
	s = uuidBox.SphericalV1

	// Verify all optional fields are parsed
	if s.SourceCount != "6" {
		t.Errorf("got SourceCount %s instead of 6", s.SourceCount)
	}
	if s.InitialViewHeadingDegrees != "90" {
		t.Errorf("got InitialViewHeadingDegrees %s instead of 90", s.InitialViewHeadingDegrees)
	}
	if s.Timestamp != "1400454971" {
		t.Errorf("got Timestamp %s instead of 1400454971", s.Timestamp)
	}
	if s.FullPanoWidthPixels != "1920" {
		t.Errorf("got FullPanoWidthPixels %s instead of 1920", s.FullPanoWidthPixels)
	}
	if s.CroppedAreaLeftPixels != "0" {
		t.Errorf("got CroppedAreaLeftPixels %s instead of 0", s.CroppedAreaLeftPixels)
	}

	// Test Info method output
	var infoBuf bytes.Buffer
	err = uuidBox.Info(&infoBuf, "uuid:1", "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	infoStr := infoBuf.String()
	if !strings.Contains(infoStr, "spherical-v1") {
		t.Error("Info output should contain spherical-v1")
	}
	if !strings.Contains(infoStr, "Spherical: true") {
		t.Error("Info output should contain Spherical: true")
	}
}

func TestSphericalVideoV1Errors(t *testing.T) {
	// Test with malformed XML
	uuidBytes, _ := hex.DecodeString("ffcc8263f8554a938814587a02521fdd")
	badXML := `<broken xml`

	size := uint32(8 + 16 + len(badXML))
	var buf bytes.Buffer
	buf.Write([]byte{byte(size >> 24), byte(size >> 16), byte(size >> 8), byte(size)})
	buf.Write([]byte{'u', 'u', 'i', 'd'})
	buf.Write(uuidBytes)
	buf.Write([]byte(badXML))

	reader := bytes.NewReader(buf.Bytes())
	hdr, err := mp4.DecodeHeader(reader)
	if err != nil {
		t.Fatal(err)
	}

	_, err = mp4.DecodeUUIDBox(hdr, 0, reader)
	if err == nil {
		t.Error("expected error for malformed XML, got nil")
	}
}
