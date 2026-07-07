package av1

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/Eyevinn/mp4ff/bits"
)

// Real sequence header OBU payload from AOM fate-suite av1-1-b8-23-film_grain-50.ivf.
const filmGrainSeqHdr = "00000004457e3e7dfcc060"

func TestParseSequenceHeaderFilmGrain(t *testing.T) {
	payload, _ := hex.DecodeString(filmGrainSeqHdr)
	sh, err := ParseSequenceHeader(payload)
	if err != nil {
		t.Fatal(err)
	}
	if sh.SeqProfile != 0 {
		t.Errorf("SeqProfile: got %d, want 0", sh.SeqProfile)
	}
	if sh.Width() != 352 || sh.Height() != 288 {
		t.Errorf("resolution: got %dx%d, want 352x288", sh.Width(), sh.Height())
	}
	if sh.BitDepth != 8 {
		t.Errorf("BitDepth: got %d, want 8", sh.BitDepth)
	}
	if sh.MonoChrome {
		t.Error("MonoChrome: got true, want false")
	}
	if sh.SubsamplingX != 1 || sh.SubsamplingY != 1 {
		t.Errorf("subsampling: got %d%d, want 11 (4:2:0)", sh.SubsamplingX, sh.SubsamplingY)
	}
	if got, want := sh.CodecString("av01"), "av01.0.00M.08.0.110.02.02.02.0"; got != want {
		t.Errorf("CodecString: got %s, want %s", got, want)
	}
}

// TestSequenceHeaderMatchesConfigRecord verifies that the sequence header parsed out of an
// av1C record's configOBUs agrees with the record's own fixed header fields.
func TestSequenceHeaderMatchesConfigRecord(t *testing.T) {
	byteData, _ := hex.DecodeString(av1DecoderConfigRecord)
	rec, err := DecodeAV1CodecConfRec(byteData)
	if err != nil {
		t.Fatal(err)
	}
	sh, err := rec.SequenceHeader()
	if err != nil {
		t.Fatal(err)
	}
	if sh.SeqProfile != rec.SeqProfile {
		t.Errorf("SeqProfile: seqhdr %d, record %d", sh.SeqProfile, rec.SeqProfile)
	}
	if sh.SeqLevelIdx0 != rec.SeqLevelIdx0 {
		t.Errorf("SeqLevelIdx0: seqhdr %d, record %d", sh.SeqLevelIdx0, rec.SeqLevelIdx0)
	}
	if sh.SeqTier0 != rec.SeqTier0 {
		t.Errorf("SeqTier0: seqhdr %d, record %d", sh.SeqTier0, rec.SeqTier0)
	}
	wantBitDepth := bitDepth(rec.SeqProfile, rec.HighBitdepth, rec.TwelveBit)
	if sh.BitDepth != wantBitDepth {
		t.Errorf("BitDepth: seqhdr %d, record-derived %d", sh.BitDepth, wantBitDepth)
	}
	if boolToByte(sh.MonoChrome) != rec.MonoChrome {
		t.Errorf("MonoChrome: seqhdr %v, record %d", sh.MonoChrome, rec.MonoChrome)
	}
	if sh.SubsamplingX != rec.ChromaSubsamplingX || sh.SubsamplingY != rec.ChromaSubsamplingY {
		t.Errorf("subsampling: seqhdr %d%d, record %d%d",
			sh.SubsamplingX, sh.SubsamplingY, rec.ChromaSubsamplingX, rec.ChromaSubsamplingY)
	}
}

func TestParseSequenceHeaderErrors(t *testing.T) {
	if _, err := ParseSequenceHeader(nil); err == nil {
		t.Error("expected error for empty payload")
	}
	// A single byte cannot hold a full sequence header; parsing must run off the end.
	if _, err := ParseSequenceHeader([]byte{0x00}); err == nil {
		t.Error("expected error for truncated payload")
	}
}

func TestReadUVLC(t *testing.T) {
	// 0b0110_0000: one leading zero, done bit, then value bit 1 -> 1 + (1<<1) - 1 = 2
	r := bits.NewReader(bytes.NewReader([]byte{0x60}))
	if got := readUVLC(r); got != 2 {
		t.Errorf("readUVLC: got %d, want 2", got)
	}
	// 32 leading zeros followed by the done bit clamp to 2^32-1 with the whole
	// zero run and done bit consumed, so the next bit read is the trailing 1.
	r = bits.NewReader(bytes.NewReader([]byte{0, 0, 0, 0, 0xc0}))
	if got := readUVLC(r); got != 1<<32-1 {
		t.Errorf("readUVLC: got %d, want 2^32-1", got)
	}
	if !r.ReadFlag() {
		t.Error("readUVLC left the reader misaligned after the done bit")
	}
}

func boolToByte(b bool) byte {
	if b {
		return 1
	}
	return 0
}

// TestSequenceHeaderFateVectors parses the sequence header of every AV1 IVF test vector in
// MP4FF_AV1_TESTVECTORS_DIR and checks the coded resolution against the IVF display size.
// Skipped when the dir is unset.
func TestSequenceHeaderFateVectors(t *testing.T) {
	dir := os.Getenv("MP4FF_AV1_TESTVECTORS_DIR")
	if dir == "" {
		t.Skip("set MP4FF_AV1_TESTVECTORS_DIR to an AV1 IVF test-vector directory to run")
	}
	files, _ := filepath.Glob(filepath.Join(dir, "*.ivf"))
	if len(files) == 0 {
		t.Skipf("no *.ivf files in %s", dir)
	}
	for _, f := range files {
		f := f
		t.Run(filepath.Base(f), func(t *testing.T) {
			data, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			ivfW := uint32(binary.LittleEndian.Uint16(data[12:14]))
			ivfH := uint32(binary.LittleEndian.Uint16(data[14:16]))
			sh := firstSequenceHeader(t, ivfFrames(t, data))
			if sh == nil {
				t.Skip("no sequence header found")
			}
			if sh.Width() != ivfW || sh.Height() != ivfH {
				t.Errorf("resolution: seqhdr %dx%d, IVF header %dx%d",
					sh.Width(), sh.Height(), ivfW, ivfH)
			}
		})
	}
}

func firstSequenceHeader(t *testing.T, frames [][]byte) *SequenceHeader {
	t.Helper()
	for _, frame := range frames {
		obus, err := SplitOBUs(frame)
		if err != nil {
			t.Fatal(err)
		}
		for _, o := range obus {
			if o.Header.Type == OBUSequenceHeader {
				sh, err := ParseSequenceHeader(o.Payload)
				if err != nil {
					t.Fatal(err)
				}
				return sh
			}
		}
	}
	return nil
}
