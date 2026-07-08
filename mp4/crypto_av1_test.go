package mp4_test

import (
	"encoding/hex"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/Eyevinn/mp4ff/mp4"
)

// decodeAV1Fragments returns the fragments of the multi-tile AV1 test segment, decoded fresh
// from disk each call (EncryptFragment mutates fragments in place).
func decodeAV1Fragments(t *testing.T, raw []byte) []*mp4.Fragment {
	t.Helper()
	sr := bits.NewFixedSliceReader(raw)
	f, err := mp4.DecodeFileSR(sr)
	if err != nil {
		t.Fatal(err)
	}
	var frags []*mp4.Fragment
	for _, s := range f.Segments {
		frags = append(frags, s.Fragments...)
	}
	if len(frags) == 0 {
		t.Fatal("no fragments in AV1 test segment")
	}
	return frags
}

func av1SencSignature(t *testing.T, frags []*mp4.Fragment) string {
	t.Helper()
	var b strings.Builder
	for _, f := range frags {
		senc := f.Moof.Traf.Senc
		if senc == nil {
			t.Fatal("expected senc box after encryption")
		}
		for _, ss := range senc.SubSamples {
			for _, p := range ss {
				b.WriteString(strconv.Itoa(int(p.BytesOfClearData)))
				b.WriteByte(':')
				b.WriteString(strconv.Itoa(int(p.BytesOfProtectedData)))
				b.WriteByte(',')
			}
			b.WriteByte('|')
		}
	}
	return b.String()
}

// TestAV1ConcurrentEncryptionSharesInitProtectData verifies the crypto redesign's core property:
// a single InitProtectData is immutable and safe to share across concurrent encryptions. Each
// EncryptFragments call builds its own FragmentEncryptor (and its own AV1 frame-header decoder),
// so concurrent AV1 encryptions must be race-free (run with -race) and yield identical output.
func TestAV1ConcurrentEncryptionSharesInitProtectData(t *testing.T) {
	initRaw, err := os.ReadFile("testdata/av1_multitile_init.mp4")
	if err != nil {
		t.Fatal(err)
	}
	segRaw, err := os.ReadFile("testdata/av1_multitile_seg.m4s")
	if err != nil {
		t.Fatal(err)
	}
	key, _ := hex.DecodeString("00112233445566778899aabbccddeeff")
	iv, _ := hex.DecodeString("7766554433221100")
	kid, _ := mp4.NewUUIDFromString("11112222333344445555666677778888")

	initSeg, err := mp4.DecodeFileSR(bits.NewFixedSliceReader(initRaw))
	if err != nil {
		t.Fatal(err)
	}
	// One shared, immutable InitProtectData for all goroutines.
	ipd, err := mp4.InitProtect(initSeg.Init, key, iv, "cenc", kid, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Reference result from a single-threaded encryption.
	refFrags := decodeAV1Fragments(t, segRaw)
	if _, err := mp4.EncryptFragments(refFrags, key, iv, ipd); err != nil {
		t.Fatal(err)
	}
	want := av1SencSignature(t, refFrags)
	if want == "" {
		t.Fatal("empty senc signature")
	}

	const n = 16
	var wg sync.WaitGroup
	got := make([]string, n)
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			frags := decodeAV1Fragments(t, segRaw)
			if _, err := mp4.EncryptFragments(frags, key, iv, ipd); err != nil {
				errs[i] = err
				return
			}
			got[i] = av1SencSignature(t, frags)
		}(i)
	}
	wg.Wait()
	for i := 0; i < n; i++ {
		if errs[i] != nil {
			t.Fatalf("goroutine %d: %v", i, errs[i])
		}
		if got[i] != want {
			t.Fatalf("goroutine %d produced different senc layout than the single-threaded run", i)
		}
	}
}
