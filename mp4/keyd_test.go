package mp4_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Eyevinn/mp4ff/mp4"
)

func TestKeyd(t *testing.T) {
	keyd := &mp4.KeydBox{Namespace: "mdta", Value: "com.example.speed"}
	boxDiffAfterEncodeAndDecode(t, keyd)
	// A key without value, which is the shortest one.
	boxDiffAfterEncodeAndDecode(t, &mp4.KeydBox{Namespace: "uiso"})

	var info strings.Builder
	if err := keyd.Info(&info, "", "", "  "); err != nil {
		t.Fatal(err)
	}
	if want := "[keyd] size=29\n - key: mdta \"com.example.speed\"\n"; info.String() != want {
		t.Errorf("got info:\n%swant:\n%s", info.String(), want)
	}

	bad := &mp4.KeydBox{Namespace: "mdt", Value: "x"}
	if err := bad.Encode(&bytes.Buffer{}); err == nil || err.Error() != `keyd: namespace "mdt" is not 4 bytes` {
		t.Errorf("error for a namespace of 3 bytes: %v", err)
	}

	// A payload that is too short for the namespace.
	decodesAsUnknown(t, []byte{0, 0, 0, 11, 'k', 'e', 'y', 'd', 'm', 'd', 't'})
}
