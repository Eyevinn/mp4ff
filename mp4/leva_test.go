package mp4

import "testing"

func TestLeva(t *testing.T) {
	leva := LevaBox{}
	lvl, err := NewLevaLevel(1, true, 0, 42, 0, 0)
	if err != nil {
		t.Error(err)
	}
	leva.Levels = append(leva.Levels, lvl)
	lvl, err = NewLevaLevel(2, false, 1, 42, 43, 0)
	if err != nil {
		t.Error(err)
	}
	leva.Levels = append(leva.Levels, lvl)
	lvl, err = NewLevaLevel(2, false, 2, 0, 0, 0)
	if err != nil {
		t.Error(err)
	}
	leva.Levels = append(leva.Levels, lvl)
	lvl, err = NewLevaLevel(2, false, 3, 0, 0, 0)
	if err != nil {
		t.Error(err)
	}
	leva.Levels = append(leva.Levels, lvl)
	lvl, err = NewLevaLevel(3, false, 4, 0, 0, 44)
	if err != nil {
		t.Error(err)
	}
	leva.Levels = append(leva.Levels, lvl)
	boxDiffAfterEncodeAndDecode(t, &leva)
}
