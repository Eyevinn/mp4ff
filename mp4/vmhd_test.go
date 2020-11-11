package mp4

import "testing"

func TestVmhd(t *testing.T) {

	vmhd := CreateVmhd()

	boxDiffAfterEncodeAndDecode(t, vmhd)
}
