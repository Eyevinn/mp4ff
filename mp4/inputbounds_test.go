package mp4_test

import (
	"bytes"
	"encoding/binary"
	"runtime"
	"testing"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/Eyevinn/mp4ff/mp4"
)

// The tests in this file give small inputs that declare much larger sizes or counts, or nest boxes deeply, and
// check that decoding and reading return errors without allocating memory for what is declared. The declared
// sizes are big enough to exceed maxTestAlloc but small enough to be allocated if the bounds were missing.

// maxTestAlloc is the most memory that one call on a small hostile input may allocate.
const maxTestAlloc = 8 << 20

// allocated returns the bytes allocated by f.
func allocated(f func()) uint64 {
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	f()
	runtime.ReadMemStats(&after)
	return after.TotalAlloc - before.TotalAlloc
}

func be32(v uint32) []byte { return binary.BigEndian.AppendUint32(nil, v) }

// boxOf builds a box with the given name and the concatenated payload parts.
func boxOf(name string, parts ...[]byte) []byte {
	return makeBox(name, bytes.Join(parts, nil))
}

func TestSencSampleCountBeyondPayload(t *testing.T) {
	decodeSenc := func(t *testing.T, sampleCount uint32, payload []byte) *mp4.SencBox {
		t.Helper()
		box, err := mp4.DecodeBoxSR(0, bits.NewFixedSliceReader(boxOf("senc", be32(0), be32(sampleCount), payload)))
		if err != nil {
			t.Fatal(err)
		}
		return box.(*mp4.SencBox)
	}
	t.Run("ivSizeGiven", func(t *testing.T) {
		senc := decodeSenc(t, 1<<22, make([]byte, 16))
		var err error
		if n := allocated(func() { err = senc.ParseReadBox(8, nil) }); n > maxTestAlloc {
			t.Errorf("allocated %d bytes", n)
		}
		if err == nil {
			t.Error("expected an error for more samples than IVs")
		}
	})
	t.Run("ivSizeInferred", func(t *testing.T) {
		senc := decodeSenc(t, 1<<22, make([]byte, 16))
		if n := allocated(func() { _ = senc.ParseReadBox(0, nil) }); n > maxTestAlloc {
			t.Errorf("allocated %d bytes", n)
		}
	})
	t.Run("inferredIVSizeTooBig", func(t *testing.T) {
		senc := decodeSenc(t, 1, make([]byte, 264))
		if err := senc.ParseReadBox(0, nil); err == nil {
			t.Error("expected an error for 264 bytes of IV for one sample")
		}
	})
	t.Run("ivsForAllSamples", func(t *testing.T) {
		senc := decodeSenc(t, 2, make([]byte, 16))
		if err := senc.ParseReadBox(8, nil); err != nil {
			t.Fatal(err)
		}
		if len(senc.IVs) != 2 {
			t.Errorf("got %d IVs, want 2", len(senc.IVs))
		}
	})
	t.Run("decodeFile", func(t *testing.T) {
		traf := boxOf("traf", boxOf("tfhd", be32(0x020000), be32(1)),
			boxOf("senc", be32(0), be32(1<<22), []byte{0}))
		data := append(boxOf("moof", boxOf("mfhd", be32(0), be32(1)), traf), boxOf("mdat")...)
		decoders := map[string]func(){
			"DecodeFile":   func() { _, _ = mp4.DecodeFile(bytes.NewReader(data)) },
			"DecodeFileSR": func() { _, _ = mp4.DecodeFileSR(bits.NewFixedSliceReader(data)) },
			"FragmentDecoder": func() {
				var d mp4.FragmentDecoder
				_, _ = d.DecodeSR(bits.NewFixedSliceReader(data), 0)
			},
		}
		for name, decode := range decoders {
			if n := allocated(decode); n > maxTestAlloc {
				t.Errorf("%s: allocated %d bytes", name, n)
			}
		}
	})
}

// streamFragment returns a moof with one trun of the given sample sizes, followed by the header of an mdat that
// declares mdatPayload bytes but no payload. With largeSize, the mdat header has a 64-bit size.
func streamFragment(sizes []uint32, mdatPayload uint64, largeSize bool) []byte {
	mfhd := boxOf("mfhd", be32(0), be32(1))
	tfhd := boxOf("tfhd", be32(0x020000), be32(1))
	trun := func(dataOffset uint32) []byte {
		parts := [][]byte{be32(0x000201), be32(uint32(len(sizes))), be32(dataOffset)}
		for _, s := range sizes {
			parts = append(parts, be32(s))
		}
		return boxOf("trun", parts...)
	}
	mdatHdr := append(be32(uint32(mdatPayload+8)), "mdat"...)
	if largeSize {
		mdatHdr = binary.BigEndian.AppendUint64(append(be32(1), "mdat"...), mdatPayload+16)
	}
	moofSize := len(boxOf("moof", mfhd, boxOf("traf", tfhd, trun(0))))
	moof := boxOf("moof", mfhd, boxOf("traf", tfhd, trun(uint32(moofSize+len(mdatHdr)))))
	return append(moof, mdatHdr...)
}

func TestStreamSampleDataBeyondStream(t *testing.T) {
	const size = 1 << 26
	cases := []struct {
		name string
		data []byte
		read func(sa mp4.SampleAccessor) error
	}{
		{"GetSamples", streamFragment([]uint32{size}, size, false),
			func(sa mp4.SampleAccessor) error { _, err := sa.GetSamples(1); return err }},
		{"GetSample", streamFragment([]uint32{size}, size, false),
			func(sa mp4.SampleAccessor) error { _, err := sa.GetSample(1, 1); return err }},
		{"AppendSamples", streamFragment([]uint32{size}, size, false),
			func(sa mp4.SampleAccessor) error { _, _, err := sa.AppendSamples(nil, nil, 1); return err }},
		{"GetSampleAfterBigSample", streamFragment([]uint32{size, 16}, size+16, true),
			func(sa mp4.SampleAccessor) error { _, err := sa.GetSample(1, 2); return err }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var err error
			n := allocated(func() {
				var sf *mp4.StreamFile
				sf, err = mp4.InitDecodeStream(bytes.NewReader(c.data),
					mp4.WithFragmentCallback(func(_ *mp4.Fragment, sa mp4.SampleAccessor) error { return c.read(sa) }))
				if err == nil {
					err = sf.ProcessFragments()
				}
			})
			if n > maxTestAlloc {
				t.Errorf("allocated %d bytes for a stream of %d bytes", n, len(c.data))
			}
			if err == nil {
				t.Error("expected an error for sample data beyond the stream")
			}
		})
	}
}

func TestStreamBoxBeyondStream(t *testing.T) {
	data := append(be32(1<<26), "moov"...)
	var err error
	if n := allocated(func() { _, err = mp4.InitDecodeStream(bytes.NewReader(data)) }); n > maxTestAlloc {
		t.Errorf("allocated %d bytes for a stream of %d bytes", n, len(data))
	}
	if err == nil {
		t.Error("expected an error for a box beyond the stream")
	}
}
