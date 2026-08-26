package mp4_test

import (
	"encoding/hex"
	"os"
	"testing"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/Eyevinn/mp4ff/mp4"
)

// BenchmarkEncryptFragment measures in-place encryption of one AVC video
// fragment, the hot path of a JIT packager. The fragment is decoded once;
// each iteration restores the pristine mdat payload and removes the
// encryption boxes added by the previous iteration, so the loop measures
// only the encryption work itself.
func BenchmarkEncryptFragment(b *testing.B) {
	key, _ := hex.DecodeString("00112233445566778899aabbccddeeff")
	iv, _ := hex.DecodeString("7766554433221100")
	kid, _ := mp4.NewUUIDFromString("11112222333344445555666677778888")

	for _, scheme := range []string{"cenc", "cbcs"} {
		b.Run(scheme, func(b *testing.B) {
			rawInit, err := os.ReadFile("testdata/init.mp4")
			if err != nil {
				b.Fatal(err)
			}
			initFile, err := mp4.DecodeFileSR(bits.NewFixedSliceReader(rawInit))
			if err != nil {
				b.Fatal(err)
			}
			ipd, err := mp4.InitProtect(initFile.Init, key, iv, scheme, kid, nil)
			if err != nil {
				b.Fatal(err)
			}
			rawSeg, err := os.ReadFile("testdata/1.m4s")
			if err != nil {
				b.Fatal(err)
			}
			segFile, err := mp4.DecodeFileSR(bits.NewFixedSliceReader(rawSeg))
			if err != nil {
				b.Fatal(err)
			}
			frag := segFile.Segments[0].Fragments[0]
			pristineMdat := make([]byte, len(frag.Mdat.Data))
			copy(pristineMdat, frag.Mdat.Data)

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				copy(frag.Mdat.Data, pristineMdat)
				frag.Moof.Traf.RemoveEncryptionBoxes()
				if _, err := mp4.EncryptFragment(frag, key, iv, ipd); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
