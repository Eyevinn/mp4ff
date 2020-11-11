package mp4

import (
	"testing"
)

func TestDref(t *testing.T) {
	dref := CreateDref()
	boxDiffAfterEncodeAndDecode(t, dref)
}
