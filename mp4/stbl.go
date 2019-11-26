package mp4

import "io"

// StblBox - Sample Table Box (stbl - mandatory)
//
// Contained in : Media Information Box (minf)
//
// Status: partially decoded (anything other than stsd, stts, stsc, stss, stsz, stco, ctts is ignored)
//
// The table contains all information relevant to data samples (times, chunks, sizes, ...)
type StblBox struct {
	Stsd  *StsdBox
	Stts  *SttsBox
	Stss  *StssBox
	Stsc  *StscBox
	Stsz  *StszBox
	Stco  *StcoBox
	Ctts  *CttsBox
	boxes []Box
}

// DecodeStbl - box-specific decode
func DecodeStbl(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	l, err := DecodeContainerChildren(hdr, startPos, r)
	if err != nil {
		return nil, err
	}
	s := &StblBox{}
	s.boxes = l
	for _, b := range l {
		switch b.Type() {
		case "stsd":
			s.Stsd = b.(*StsdBox)
		case "stts":
			s.Stts = b.(*SttsBox)
		case "stsc":
			s.Stsc = b.(*StscBox)
		case "stss":
			s.Stss = b.(*StssBox)
		case "stsz":
			s.Stsz = b.(*StszBox)
		case "stco":
			s.Stco = b.(*StcoBox)
		case "ctts":
			s.Ctts = b.(*CttsBox)
		}
	}
	return s, nil
}

// Type - box-specific type
func (b *StblBox) Type() string {
	return "stbl"
}

// Size - box-specific size
func (b *StblBox) Size() uint64 {
	return containerSize(b.boxes)
}

// Dump - box-specific dump
func (b *StblBox) Dump() {
	if b.Stsc != nil {
		b.Stsc.Dump()
	}
	if b.Stts != nil {
		b.Stts.Dump()
	}
	if b.Stsz != nil {
		b.Stsz.Dump()
	}
	if b.Stss != nil {
		b.Stss.Dump()
	}
	if b.Stco != nil {
		b.Stco.Dump()
	}
}

// Encode - box-specific encode
func (b *StblBox) Encode(w io.Writer) error {
	err := EncodeHeader(b, w)
	if err != nil {
		return err
	}
	err = b.Stsd.Encode(w)
	if err != nil {
		return err
	}
	err = b.Stts.Encode(w)
	if err != nil {
		return err
	}
	if b.Stss != nil {
		err = b.Stss.Encode(w)
		if err != nil {
			return err
		}
	}
	err = b.Stsc.Encode(w)
	if err != nil {
		return err
	}
	err = b.Stsz.Encode(w)
	if err != nil {
		return err
	}
	err = b.Stco.Encode(w)
	if err != nil {
		return err
	}
	if b.Ctts != nil {
		return b.Ctts.Encode(w)
	}
	return nil
}
