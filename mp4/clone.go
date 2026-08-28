package mp4

import (
	"fmt"

	"github.com/Eyevinn/mp4ff/bits"
)

// CloneBox returns a deep copy of a box, made by encoding it and decoding the
// bytes back into a fresh box tree. Nothing in the returned box aliases the
// original, so either of the two can be mutated without affecting the other.
//
// The typical use is protecting a shared box tree from functions that modify
// boxes in place: a caller that builds init segments from a cached moov can
// hand InitProtect a clone of the shared stsd instead of the original, since
// InitProtect rewrites the sample entry (avc1 -> encv etc.) in place.
//
// The clone is what a fresh decode of the box would give, so decode-time
// state is reset: StartPos fields reflect a decode at position 0, and a senc
// box needs its usual deferred parsing again. A lazy mdat (payload not held
// in memory) cannot be cloned this way and returns an error.
func CloneBox(b Box) (Box, error) {
	if m, ok := b.(*MdatBox); ok && m.IsLazy() {
		return nil, fmt.Errorf("cannot clone lazy mdat box")
	}
	size := b.Size()
	sw := bits.NewFixedSliceWriter(int(size))
	if err := b.EncodeSW(sw); err != nil {
		return nil, fmt.Errorf("encode %s: %w", b.Type(), err)
	}
	sr := bits.NewFixedSliceReader(sw.Bytes())
	clone, err := DecodeBoxSR(0, sr)
	if err != nil {
		return nil, fmt.Errorf("decode %s: %w", b.Type(), err)
	}
	return clone, nil
}
