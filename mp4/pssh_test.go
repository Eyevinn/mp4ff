package mp4

import (
	"bytes"
	"strings"
	"testing"

	"github.com/go-test/deep"
)

func TestPsshFromBase64(t *testing.T) {
	b64 := "AAAASnBzc2gAAAAA7e+LqXnWSs6jyCfc1R0h7QAAACoSEDEuM2I0EEaatTa5ydDK/DESEDEuM2I0EEaatTa5ydDK/DFI49yVmwY="
	expected := "[pssh] size=74 version=0 flags=000000\n - systemID: edef8ba9-79d6-4ace-a3c8-27dcd51d21ed (Widevine)\n"
	psshs, err := PsshBoxesFromBase64(b64)
	if err != nil {
		t.Fatal(err)
	}
	if len(psshs) != 1 {
		t.Errorf("Expected 1 pssh, got %d", len(psshs))
	}
	var buf bytes.Buffer
	err = psshs[0].Info(&buf, "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	d := deep.Equal(buf.String(), expected)
	if len(d) > 0 {
		for _, l := range d {
			t.Error(l)
		}
	}
}

func TestEncodeDecodePSSH(t *testing.T) {
	hPR := strings.ReplaceAll(UUIDPlayReady, "-", "")
	pr, err := NewUUIDFromString(hPR)
	if err != nil {
		t.Fatal(err)
	}
	kid := "00112233445566778899aabbccddeeff"
	ku, err := NewUUIDFromString(kid)
	if err != nil {
		t.Fatal(err)
	}
	pssh := &PsshBox{
		Version:  0,
		SystemID: pr,
		Data:     []byte("some data"),
	}
	boxDiffAfterEncodeAndDecode(t, pssh)
	pssh = &PsshBox{
		Version:  1,
		SystemID: pr,
		KIDs:     []UUID{ku},
		Data:     []byte("some data"),
	}
	boxDiffAfterEncodeAndDecode(t, pssh)
}

func TestPsshUUIDs(t *testing.T) {
	cases := []struct {
		hexUUIDs     string
		expectedName string
	}{
		{"edef8ba9-79d6-4ace-a3c8-27dcd51d21ed", "Widevine"},
		{"9a04f079-9840-4286-ab92-e65be0885f95", "PlayReady"},
		{"94CE86FB-07FF-4F43-ADB8-93D2FA968CA2", "FairPlay"},
		{"9a27dd82-fde2-4725-8cbc-4234aa06ec09", "Verimatrix VCAS"},
		{"1077efec-c0b2-4d02-ace3-3c1e52e2fb4b", "W3C Common PSSH box"},
		{"00000000-0000-0000-0000-000000000000", "Unknown"},
	}

	for _, c := range cases {
		u, err := NewUUIDFromString(c.hexUUIDs)
		if err != nil {
			t.Fatal(err)
		}
		if ProtectionSystemName(u) != c.expectedName {
			t.Errorf("Expected %s, got %s", c.expectedName, ProtectionSystemName(u))
		}
	}
}
