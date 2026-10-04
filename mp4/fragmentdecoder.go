package mp4

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"slices"

	"github.com/Eyevinn/mp4ff/bits"
)

// FragmentDecoder decodes the fragments of a fragmented stream one at a time from slices. Each box is decoded into
// the box that had the same type at the same place in the previous fragment, so that the boxes and their storage
// are reused. This is done for styp, sidx, moof, mfhd, traf, tfhd, tfdt, trun and mdat, so after the first
// fragment, decoding a stream that repeats a layout of these boxes allocates nothing. Other boxes, such as prft,
// emsg and senc, are decoded as by DecodeBoxSR, which allocates them anew.
//
// The mdat data of a decoded fragment views the slice it was decoded from, as with DecodeFileSR, so sample data
// can be extracted without copying with Fragment.Samples or Fragment.AppendFullSamples.
type FragmentDecoder struct {
	// Moov is the movie box of the init segment, if known. It gives the per-sample IV size for parsing senc boxes.
	Moov *MoovBox

	frag Fragment
	// boxes holds the top-level boxes of the last fragment, in order, and spareBoxes the storage for the next.
	boxes, spareBoxes []Box
	// cands holds the boxes that the next fragment can decode into: those of the last fragment and those of
	// earlier fragments that the last one did not have, such as the styp that only the first fragment of a segment
	// has. Each is reused at most once per fragment, by type. spareCands is the storage for the next list.
	cands, spareCands []boxDecoderInto
}

// maxReusedBoxes bounds the number of top-level boxes kept for reuse.
const maxReusedBoxes = 64

// DecodeSR decodes the next fragment from sr: the boxes before its moof (such as styp, sidx, prft and emsg), the moof
// and its mdat. startPos is the position of sr's next byte in the stream, from which box positions and data offsets
// are computed. It returns io.EOF if sr has no bytes left, and io.ErrUnexpectedEOF if sr ends inside a fragment.
// Boxes after the last mdat of a stream, such as an mfra, count as the start of a fragment.
//
// The fragment and its boxes are reused by the next call, so nothing taken from them, including sample data, may be
// used after it. The returned fragment holds the emsg and prft boxes, the moof and the mdat. Boxes returns all
// top-level boxes, including those such as styp and sidx that belong to the segment rather than the fragment.
func (d *FragmentDecoder) DecodeSR(sr bits.SliceReader, startPos uint64) (*Fragment, error) {
	if sr.NrRemainingBytes() == 0 {
		return nil, io.EOF
	}
	f := &d.frag
	cands := d.cands
	var used uint64 // bit i is set when cands[i] has been reused
	boxes := d.spareBoxes[:0]
	next := d.spareCands[:0]
	pos := startPos
	var moof *MoofBox
	var mdat *MdatBox
	for mdat == nil {
		left := sr.NrRemainingBytes()
		if left < boxHeaderSize {
			return nil, io.ErrUnexpectedEOF
		}
		rh, err := readRawHeaderSR(sr)
		if err != nil {
			if sr.AccError() != nil { // the 64-bit size of an mdat is cut off
				return nil, io.ErrUnexpectedEOF
			}
			return nil, err
		}
		if rh.size > uint64(left) {
			return nil, io.ErrUnexpectedEOF
		}
		var box Box
		reused := false
		for i, c := range cands {
			if i >= maxReusedBoxes {
				break
			}
			if used&(1<<i) != 0 {
				continue
			}
			if reused, err = decodeIntoIfType(c, rh, pos, sr); reused || err != nil {
				used |= 1 << i
				box = c
				next = append(next, c)
				break
			}
		}
		if !reused && err == nil {
			hdr, dec := rh.lookup()
			box, err = decodeBoxBodySR(pos, hdr, dec, sr)
			if r, ok := box.(boxDecoderInto); ok && err == nil {
				next = append(next, r)
			}
		}
		if err != nil {
			return nil, err
		}
		switch b := box.(type) {
		case *MoofBox:
			if moof != nil {
				return nil, errors.New("moof without mdat")
			}
			b.StartPos = pos
			moof = b
		case *MdatBox:
			if moof == nil {
				return nil, fmt.Errorf("mdat without moof at position %d", pos)
			}
			mdat = b
		}
		boxes = append(boxes, box)
		pos += rh.size
	}
	for i, c := range cands { // keep the boxes this fragment did not have
		if i < maxReusedBoxes && used&(1<<i) == 0 && len(next) < maxReusedBoxes {
			next = append(next, c)
		}
	}
	clear(d.boxes)
	clear(cands)
	d.boxes, d.spareBoxes = boxes, d.boxes[:0]
	d.cands, d.spareCands = next, cands[:0]

	if err := d.parseSenc(moof); err != nil {
		return nil, err
	}

	clear(f.Emsgs)
	clear(f.Prfts)
	clear(f.Children)
	*f = Fragment{Emsgs: f.Emsgs[:0], Prfts: f.Prfts[:0], Children: f.Children[:0], EncOptimize: f.EncOptimize}
	for _, b := range boxes {
		switch b.(type) {
		case *EmsgBox, *PrftBox, *MoofBox, *MdatBox:
			f.AddChild(b)
		}
	}
	f.StartPos = moof.StartPos
	return f, nil
}

// Boxes returns the top-level boxes of the last fragment that DecodeSR decoded, in order.
func (d *FragmentDecoder) Boxes() []Box {
	return d.boxes
}

// decodeBodySRInto decodes the body of a box with header hdr from sr into b, as decodeBoxBodySR decodes a new box.
func decodeBodySRInto(startPos uint64, hdr BoxHeader, sr bits.SliceReader, b boxDecoderInto) (Box, error) {
	maxSize := uint64(sr.NrRemainingBytes()) + uint64(hdr.Hdrlen)
	if hdr.Size > maxSize && hdr.Name != "mdat" {
		return nil, fmt.Errorf("decode box %q, size %d too big (max %d)", hdr.Name, hdr.Size, maxSize)
	}
	if err := b.decodeSRInto(hdr, startPos, sr); err != nil {
		return nil, fmt.Errorf("decode %s pos %d: %w", hdr.Name, startPos, err)
	}
	return b, nil
}

// parseSenc parses senc boxes as DecodeFileSR does.
func (d *FragmentDecoder) parseSenc(moof *MoofBox) error {
	for _, traf := range moof.Trafs {
		ok, parsed := traf.ContainsSencBox()
		if !ok || parsed {
			continue
		}
		defaultIVSize := byte(0)
		if d.Moov != nil {
			trackID := traf.Tfhd.TrackID
			if !d.Moov.IsEncrypted(trackID) {
				continue
			}
			sinf := d.Moov.GetSinf(trackID)
			if sinf != nil && sinf.Schi != nil && sinf.Schi.Tenc != nil {
				defaultIVSize = sinf.Schi.Tenc.DefaultPerSampleIVSize
			}
		}
		if err := traf.ParseReadSenc(defaultIVSize, moof.StartPos); err != nil && d.Moov != nil {
			return err
		}
	}
	return nil
}

// boxDecoderInto is a box that can be decoded into, reusing its storage. Its decodeSRInto must decode as the decoder
// of its type in decodersSR does. Those decoders build a new box directly rather than through decodeSRInto, since
// that is measurably slower for DecodeFileSR.
type boxDecoderInto interface {
	Box
	decodeSRInto(hdr BoxHeader, startPos uint64, sr bits.SliceReader) error
}

// decodeBoxSRInto decodes the next box from sr into prev if prev has the same type and can be decoded into, and
// returns prev. Otherwise it decodes a new box, as DecodeBoxSR. It also returns the size of the box: that of its
// header for a box decoded into, which the caller checks against the bytes read, and Size for a new box, as
// DecodeContainerChildrenSR does.
func decodeBoxSRInto(startPos uint64, sr bits.SliceReader, prev Box) (Box, uint64, error) {
	rh, err := readRawHeaderSR(sr)
	if err != nil {
		return nil, 0, err
	}
	if prev != nil {
		reused, err := decodeIntoIfType(prev, rh, startPos, sr)
		if err != nil {
			return nil, 0, err
		}
		if reused {
			return prev, rh.size, nil
		}
	}
	hdr, d := rh.lookup()
	b, err := decodeBoxBodySR(startPos, hdr, d, sr)
	if err != nil {
		return nil, 0, err
	}
	return b, b.Size(), nil
}

// decodeIntoIfType decodes the box with header h into b if b is of that type and can be decoded into, and reports
// whether it did. The type switch on concrete types avoids an interface conversion for every box.
func decodeIntoIfType(b Box, h rawHeader, startPos uint64, sr bits.SliceReader) (bool, error) {
	var r boxDecoderInto
	var name string
	switch c := b.(type) {
	case *MoofBox:
		r, name = c, "moof"
	case *TrafBox:
		r, name = c, "traf"
	case *MfhdBox:
		r, name = c, "mfhd"
	case *TfhdBox:
		r, name = c, "tfhd"
	case *TfdtBox:
		r, name = c, "tfdt"
	case *TrunBox:
		r, name = c, "trun"
	case *MdatBox:
		r, name = c, "mdat"
	case *SidxBox:
		r, name = c, "sidx"
	case *StypBox:
		r, name = c, "styp"
	default:
		return false, nil
	}
	if string(h.typ) != name {
		return false, nil
	}
	_, err := decodeBodySRInto(startPos, BoxHeader{name, h.size, h.hdrLen}, sr, r)
	return true, err
}

// maxReadChunk bounds how much ReadFragmentBytes grows its buffer ahead of the data it has read.
const maxReadChunk = 1 << 20

// ReadFragmentBytes reads the next fragment from r and appends its bytes to dst: the boxes before its moof, such
// as styp, sidx, prft and emsg, the moof, and the mdat that follows it. It returns the extended slice, so that one
// buffer can serve fragment after fragment, or a buffer can be taken from a pool for each fragment that must
// outlive the next. Decode the appended bytes with FragmentDecoder.DecodeSR, giving the position of the fragment
// in the stream.
//
// The buffer grows only as data arrives, so a box size that the stream does not back with data cannot make it
// allocate much more than it has read. ReadFragmentBytes returns io.EOF if r ends before the first byte of a
// fragment, and io.ErrUnexpectedEOF if it ends inside one. Boxes after the last mdat of a stream, such as an mfra,
// count as the start of a fragment. On error, dst is returned with its original length.
func ReadFragmentBytes(r io.Reader, dst []byte) ([]byte, error) {
	start := len(dst)
	sawMoof := false
	for {
		boxStart := len(dst)
		var err error
		dst, err = readAppend(r, dst, boxHeaderSize)
		if err != nil {
			if errors.Is(err, io.EOF) && boxStart == start {
				return dst[:start], io.EOF
			}
			return dst[:start], unexpectedEOF(err)
		}
		size := uint64(binary.BigEndian.Uint32(dst[boxStart:]))
		isMoof := string(dst[boxStart+4:boxStart+8]) == "moof"
		isMdat := string(dst[boxStart+4:boxStart+8]) == "mdat"
		hdrLen := uint64(boxHeaderSize)
		switch size {
		case 1:
			if dst, err = readAppend(r, dst, largeSizeLen); err != nil {
				return dst[:start], unexpectedEOF(err)
			}
			size = binary.BigEndian.Uint64(dst[boxStart+boxHeaderSize:])
			hdrLen += largeSizeLen
		case 0:
			return dst[:start], fmt.Errorf("box at %d: size 0, meaning to end of file, not supported", boxStart-start)
		}
		if size < hdrLen {
			return dst[:start], fmt.Errorf("box at %d: size %d is smaller than its header", boxStart-start, size)
		}
		switch {
		case isMdat && !sawMoof:
			return dst[:start], fmt.Errorf("box at %d: mdat without preceding moof", boxStart-start)
		case isMoof && sawMoof:
			return dst[:start], fmt.Errorf("box at %d: moof without mdat", boxStart-start)
		}
		for n := size - hdrLen; n > 0; {
			c := n
			if c > maxReadChunk {
				c = maxReadChunk
			}
			if dst, err = readAppend(r, dst, int(c)); err != nil {
				return dst[:start], unexpectedEOF(err)
			}
			n -= c
		}
		sawMoof = sawMoof || isMoof
		if isMdat {
			return dst, nil
		}
	}
}

// readAppend reads n bytes from r and appends them to dst.
func readAppend(r io.Reader, dst []byte, n int) ([]byte, error) {
	l := len(dst)
	dst = slices.Grow(dst, n)[:l+n]
	if _, err := io.ReadFull(r, dst[l:]); err != nil {
		return dst[:l], err
	}
	return dst, nil
}

// unexpectedEOF turns io.EOF, which is only expected before a fragment, into io.ErrUnexpectedEOF.
func unexpectedEOF(err error) error {
	if errors.Is(err, io.EOF) {
		return io.ErrUnexpectedEOF
	}
	return err
}
