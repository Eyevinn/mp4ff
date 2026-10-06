package mp4_test

import (
	"bytes"
	"testing"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/Eyevinn/mp4ff/mp4"
)

// TestVersionsAbove1RoundTrip checks that tkhd, mvhd and subs boxes of a version above 1, which
// their decoders read with the version 0 layout, also have that layout's size and encoding. The
// tkhd and mvhd encoders wrote the version 1 layout for any version but 0, overflowing the
// version 0 size, and subs counted version 1 sizes for its subsamples, so its decoded size did
// not match the bytes read.
func TestVersionsAbove1RoundTrip(t *testing.T) {
	subs := &mp4.SubsBox{Entries: []mp4.SubsEntry{{SampleDelta: 1, SubSamples: []mp4.SubsSample{{SubsampleSize: 50}}}}}
	for _, b := range []mp4.Box{mp4.CreateTkhd(), mp4.CreateMvhd(), subs} {
		var buf bytes.Buffer
		if err := b.Encode(&buf); err != nil {
			t.Fatal(err)
		}
		data := buf.Bytes()
		data[8] = 2 // version 2, laid out as version 0
		for _, decode := range []func() (mp4.Box, error){
			func() (mp4.Box, error) { return mp4.DecodeBox(0, bytes.NewReader(data)) },
			func() (mp4.Box, error) { return mp4.DecodeBoxSR(0, bits.NewFixedSliceReader(data)) },
		} {
			box, err := decode()
			if err != nil {
				t.Errorf("%s version 2: %v", b.Type(), err)
				continue
			}
			if box.Size() != uint64(len(data)) {
				t.Errorf("%s version 2: size %d, want %d", b.Type(), box.Size(), len(data))
			}
			var out bytes.Buffer
			if err := box.Encode(&out); err != nil {
				t.Errorf("%s version 2: encode: %v", b.Type(), err)
				continue
			}
			if !bytes.Equal(out.Bytes(), data) {
				t.Errorf("%s version 2: re-encoded box differs from the decoded one", b.Type())
			}
		}
	}
}
