package mp4

import (
	"testing"
)

func TestStpp(t *testing.T) {

	stpp := NewStppBox("The namespace", "schema location", "image/png,image/jpg")
	btrt := &BtrtBox{} //Note, we only handle btrt box when both optional fields are present
	stpp.AddChild(btrt)
	if stpp.Btrt != btrt {
		t.Error("Btrt link is broken")
	}
	boxDiffAfterEncodeAndDecode(t, stpp)

	stppWithoutOptionalFields := NewStppBox("The namespace", "", "")
	boxDiffAfterEncodeAndDecode(t, stppWithoutOptionalFields)
}
