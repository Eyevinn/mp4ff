package mp4

import "testing"

func TestMfra(t *testing.T) {
	mfra := &MfraBox{}
	tfra := &TfraBox{
		TrackID:               1,
		LengthSizeOfTrafNum:   0,
		LengthSizeOfTrunNum:   1,
		LengthSizeOfSampleNum: 2,
	}
	te := TfraEntry{
		Time:        3145,
		MoofOffset:  1892,
		TrafNumber:  1,
		TrunNumber:  2,
		SampleDelta: 1,
	}
	tfra.Entries = append(tfra.Entries, te)
	err := mfra.AddChild(tfra)
	if err != nil {
		t.Error(err)
	}
	mfro := &MfroBox{
		ParentSize: 12345,
	}
	err = mfra.AddChild(mfro)
	if err != nil {
		t.Error(err)
	}
	boxDiffAfterEncodeAndDecode(t, mfra)
}
