package mp4_test

import (
	"strings"
	"testing"

	"github.com/Eyevinn/mp4ff/mp4"
)

func TestDtyp(t *testing.T) {
	cases := []struct {
		desc      string
		dtyp      *mp4.DtypBox
		wellKnown uint32
		isKnown   bool
		wantInfo  string
	}{
		{
			desc:      "well-known type with a name",
			dtyp:      &mp4.DtypBox{Namespace: mp4.DtypNamespaceWellKnown, Value: []byte{0, 0, 0, mp4.DataTypeUTF8}},
			wellKnown: mp4.DataTypeUTF8,
			isKnown:   true,
			wantInfo:  "[dtyp] size=16\n - namespace: 0\n - dataType: 1 (UTF-8)\n",
		},
		{
			desc:      "well-known type without a name",
			dtyp:      &mp4.DtypBox{Namespace: mp4.DtypNamespaceWellKnown, Value: []byte{0, 0, 1, 0}},
			wellKnown: 256,
			isKnown:   true,
			wantInfo:  "[dtyp] size=16\n - namespace: 0\n - dataType: 256\n",
		},
		{
			desc:     "well-known namespace with a value that is not 4 bytes",
			dtyp:     &mp4.DtypBox{Namespace: mp4.DtypNamespaceWellKnown, Value: []byte{0, 0, 1}},
			wantInfo: "[dtyp] size=15\n - namespace: 0\n - dataType: 000001\n",
		},
		{
			desc:     "name",
			dtyp:     &mp4.DtypBox{Namespace: mp4.DtypNamespaceName, Value: []byte("com.example.type")},
			wantInfo: "[dtyp] size=28\n - namespace: 1\n - dataType: \"com.example.type\"\n",
		},
		{
			desc:     "other namespace without value",
			dtyp:     &mp4.DtypBox{Namespace: 2},
			wantInfo: "[dtyp] size=12\n - namespace: 2\n - dataType: \n",
		},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			boxDiffAfterEncodeAndDecode(t, c.dtyp)
			if wellKnown, ok := c.dtyp.WellKnownType(); wellKnown != c.wellKnown || ok != c.isKnown {
				t.Errorf("WellKnownType gives %d, %v", wellKnown, ok)
			}
			var info strings.Builder
			if err := c.dtyp.Info(&info, "", "", "  "); err != nil {
				t.Fatal(err)
			}
			if info.String() != c.wantInfo {
				t.Errorf("got info:\n%swant:\n%s", info.String(), c.wantInfo)
			}
		})
	}

	// A payload that is too short for the namespace.
	decodesAsUnknown(t, []byte{0, 0, 0, 11, 'd', 't', 'y', 'p', 0, 0, 0})
}
