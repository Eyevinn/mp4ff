package mp4

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
)

const (
	// BoxHeaderSize - standard size + name header
	BoxHeaderSize = 8
)

var (
	// ErrTruncatedHeader - could not read full header
	ErrTruncatedHeader = errors.New("truncated header")
	// ErrBadFormat - box structure not parsable
	ErrBadFormat = errors.New("bad format")
)

var decoders map[string]BoxDecoder

func init() {
	decoders = map[string]BoxDecoder{
		"ftyp": DecodeFtyp,
		"moov": DecodeMoov,
		"mvhd": DecodeMvhd,
		"iods": DecodeIods,
		"trak": DecodeTrak,
		"udta": DecodeUdta,
		"tkhd": DecodeTkhd,
		"edts": DecodeEdts,
		"elst": DecodeElst,
		"mdia": DecodeMdia,
		"minf": DecodeMinf,
		"mdhd": DecodeMdhd,
		"hdlr": DecodeHdlr,
		"vmhd": DecodeVmhd,
		"smhd": DecodeSmhd,
		"dinf": DecodeDinf,
		"dref": DecodeDref,
		"stbl": DecodeStbl,
		"stco": DecodeStco,
		"stsc": DecodeStsc,
		"stsz": DecodeStsz,
		"ctts": DecodeCtts,
		"stsd": DecodeStsd,
		"stts": DecodeStts,
		"stss": DecodeStss,
		"meta": DecodeMeta,
		"mdat": DecodeMdat,
		"free": DecodeFree,
		"styp": DecodeStyp,
		"moof": DecodeMoof,
		"mfhd": DecodeMfhd,
		"traf": DecodeTraf,
		"tfhd": DecodeTfhd,
		"tfdt": DecodeTfdt,
		"trun": DecodeTrun,
		"mvex": DecodeMvex,
		"trex": DecodeTrex,
	}
}

// BoxHeader - 8-byte header of a box
type BoxHeader struct {
	Type string
	Size uint32
}

// DecodeHeader decodes a box header (size + box type)
func DecodeHeader(r io.Reader) (BoxHeader, error) {
	buf := make([]byte, BoxHeaderSize)
	n, err := r.Read(buf)
	if n == 0 {
		return BoxHeader{}, nil
	}
	if err != nil {
		return BoxHeader{}, err
	}
	if n != BoxHeaderSize {
		return BoxHeader{}, ErrTruncatedHeader
	}
	return BoxHeader{string(buf[4:8]), binary.BigEndian.Uint32(buf[0:4])}, nil
}

// EncodeHeader encodes a box header to a writer
func EncodeHeader(b Box, w io.Writer) error {
	buf := make([]byte, BoxHeaderSize)
	binary.BigEndian.PutUint32(buf, uint32(b.Size()))
	strtobuf(buf[4:], b.Type(), 4)
	_, err := w.Write(buf)
	return err
}

// Box is the general interface
type Box interface {
	Type() string
	Size() int
	Encode(w io.Writer) error
}

// BoxDecoder is function signature of the Box Decode method
type BoxDecoder func(r io.Reader) (Box, error)

// DecodeBox decodes a box
func DecodeBox(h BoxHeader, r io.Reader) (Box, error) {
	var err error
	var b Box
	d, ok := decoders[h.Type]

	if !ok {
		log.Printf("Found unknown box type %v, size %v", h.Type, h.Size)
		b, err = DecodeUnknown(h.Type, io.LimitReader(r, int64(h.Size-BoxHeaderSize)))

	} else {
		log.Printf("Found supported box %v, size %v", h.Type, h.Size)
		b, err = d(io.LimitReader(r, int64(h.Size-BoxHeaderSize)))
	}
	if h.Size != uint32(b.Size()) {
		log.Printf("### Warning: %v size mismatch %d %d", h.Type, h.Size, b.Size())
	}
	//log.Printf("Box type %v, size %d", b.Type(), b.Size())
	if err != nil {
		log.Printf("Error while decoding %s : %s", h.Type, err)
		return nil, err
	}
	return b, nil
}

// DecodeContainer decodes a container box
func DecodeContainer(r io.Reader) ([]Box, error) {
	l := []Box{}
	for {
		h, err := DecodeHeader(r)
		if err == io.EOF || h.Size == 0 {
			return l, nil
		}
		if err != nil {
			return l, err
		}
		b, err := DecodeBox(h, r)
		if err != nil {
			return l, err
		}
		l = append(l, b)
	}
}

// Fixed16 - An 8.8 fixed point number
type Fixed16 uint16

func (f Fixed16) String() string {
	return fmt.Sprintf("%d.%d", uint16(f)>>8, uint16(f)&7)
}

func fixed16(bytes []byte) Fixed16 {
	return Fixed16(binary.BigEndian.Uint16(bytes))
}

func putFixed16(bytes []byte, i Fixed16) {
	binary.BigEndian.PutUint16(bytes, uint16(i))
}

// Fixed32 -  A 16.16 fixed point number
type Fixed32 uint32

func (f Fixed32) String() string {
	return fmt.Sprintf("%d.%d", uint32(f)>>16, uint32(f)&15)
}

func fixed32(bytes []byte) Fixed32 {
	return Fixed32(binary.BigEndian.Uint32(bytes))
}

func putFixed32(bytes []byte, i Fixed32) {
	binary.BigEndian.PutUint32(bytes, uint32(i))
}

func strtobuf(out []byte, str string, l int) {
	in := []byte(str)
	if l < len(in) {
		copy(out, in)
	} else {
		copy(out, in[0:l])
	}
}

func makebuf(b Box) []byte {
	return make([]byte, b.Size()-BoxHeaderSize)
}
