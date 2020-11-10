package mp4

import (
	"testing"
)

func TestStpp(t *testing.T) {

	stpp := NewStppBox("The namespace", "schema location", "image/png,image/jpg")
	btrt := &BtrtBox{}
	stpp.AddChild(btrt)
	if stpp.Btrt != btrt {
		t.Error("Btrt link is broken")
	}
	boxDiffAfterEncodeAndDecode(t, stpp)
}
