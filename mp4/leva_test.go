package mp4_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/Eyevinn/mp4ff/mp4"
)

func TestLeva(t *testing.T) {
	leva := mp4.LevaBox{}
	lvl, err := mp4.NewLevaLevel(1, true, 0, "seig", 0, 0)
	if err != nil {
		t.Error(err)
	}
	leva.Levels = append(leva.Levels, lvl)
	lvl, err = mp4.NewLevaLevel(2, false, 1, "tele", 43, 0)
	if err != nil {
		t.Error(err)
	}
	leva.Levels = append(leva.Levels, lvl)
	lvl, err = mp4.NewLevaLevel(2, false, 2, "", 0, 0)
	if err != nil {
		t.Error(err)
	}
	leva.Levels = append(leva.Levels, lvl)
	lvl, err = mp4.NewLevaLevel(2, false, 3, "", 0, 0)
	if err != nil {
		t.Error(err)
	}
	leva.Levels = append(leva.Levels, lvl)
	lvl, err = mp4.NewLevaLevel(3, false, 4, "", 0, 44)
	if err != nil {
		t.Error(err)
	}
	leva.Levels = append(leva.Levels, lvl)
	boxDiffAfterEncodeAndDecode(t, &leva)

	var buf bytes.Buffer
	if err := leva.Info(&buf, "all:1", "", "  "); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"groupingType=seig", "groupingType=tele groupingTypeParameter=43"} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("Info output does not contain %q:\n%s", want, buf.String())
		}
	}
}

func TestLevaGroupingTypeNotFourCharacters(t *testing.T) {
	for _, assignmentType := range []byte{0, 1} {
		if _, err := mp4.NewLevaLevel(1, false, assignmentType, "sei", 0, 0); err == nil {
			t.Errorf("NewLevaLevel with assignmentType %d accepted a 3-character groupingType", assignmentType)
		}
	}

	leva := mp4.LevaBox{}
	lvl, err := mp4.NewLevaLevel(1, false, 0, "seig", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	lvl.GroupingType = "seigx"
	leva.Levels = append(leva.Levels, lvl)
	sw := bits.NewFixedSliceWriter(int(leva.Size()))
	if err := leva.EncodeSW(sw); err == nil {
		t.Error("EncodeSW accepted a 5-character groupingType")
	}
}
