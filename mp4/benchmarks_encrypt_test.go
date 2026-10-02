package mp4_test

import (
	"bytes"
	"encoding/hex"
	"os"
	"testing"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/Eyevinn/mp4ff/mp4"
)

// BenchmarkEncryptFragments encrypts testdata/1.m4s as one fragment of 60 video samples and as 60 one-sample
// fragments, with one FragmentEncryptor. Decoding the clear fragments is not timed.
func BenchmarkEncryptFragments(b *testing.B) {
	key, _ := hex.DecodeString("00112233445566778899aabbccddeeff")
	kid, _ := mp4.NewUUIDFromString("11112222333344445555666677778888")
	iv8, _ := hex.DecodeString("7766554433221100")
	iv16, _ := hex.DecodeString("ffeeddccbbaa99887766554433221100")
	initData, err := os.ReadFile("testdata/init.mp4")
	if err != nil {
		b.Fatal(err)
	}
	seg, err := os.ReadFile("testdata/1.m4s")
	if err != nil {
		b.Fatal(err)
	}
	layouts := []struct {
		name string
		data []byte
	}{{"Fragment60", seg}, {"Fragments1", oneSampleFragments(b)}}
	for _, sc := range []struct {
		scheme string
		iv     []byte
	}{{"cenc", iv8}, {"cbcs", iv16}} {
		for _, layout := range layouts {
			b.Run(sc.scheme+"/"+layout.name, func(b *testing.B) {
				init, err := mp4.DecodeFile(bytes.NewReader(initData))
				if err != nil {
					b.Fatal(err)
				}
				ipd, err := mp4.InitProtect(init.Init, key, sc.iv, sc.scheme, kid, nil)
				if err != nil {
					b.Fatal(err)
				}
				work := make([]byte, len(layout.data))
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					b.StopTimer()
					copy(work, layout.data) // encryption is in place
					f, err := mp4.DecodeFileSR(bits.NewFixedSliceReader(work))
					if err != nil {
						b.Fatal(err)
					}
					b.StartTimer()
					e, err := ipd.NewFragmentEncryptor(key, sc.iv)
					if err != nil {
						b.Fatal(err)
					}
					for _, frag := range f.Segments[0].Fragments {
						if err := e.EncryptFragment(frag); err != nil {
							b.Fatal(err)
						}
					}
				}
			})
		}
	}
}
