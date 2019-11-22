package mp4

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
)

const (
	// boxHeaderSize - standard size + name header
	boxHeaderSize = 8
	largeSizeLen  = 8 // Length of largesize exension
)

var (
	// ErrTruncatedHeader - could not read full header
	ErrTruncatedHeader = errors.New("truncated header")
	// ErrBadFormat - box structure not parsable
	ErrBadFormat = errors.New("bad format")
)

// headerLength - header length including potential largesize
func headerLength(contentSize uint64) uint64 {
	hdrlen := boxHeaderSize

	if contentSize > 429496729-8 { // 2**32 - 8
		hdrlen += largeSizeLen
	}
	return uint64(hdrlen)
}

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
		"prft": DecodePrft,
	}
}

// boxHeader - 8 or 16 bytes depending on size
type boxHeader struct {
	name   string
	size   uint64
	hdrlen int
}

// decodeHeader decodes a box header (size + box type)
func decodeHeader(r io.Reader) (boxHeader, error) {
	buf := make([]byte, boxHeaderSize)
	n, err := r.Read(buf)
	if err != nil {
		return boxHeader{}, err
	}
	if n != boxHeaderSize {
		return boxHeader{}, ErrTruncatedHeader
	}
	size := uint64(binary.BigEndian.Uint32(buf[0:4]))
	headerLen := boxHeaderSize
	if size == 1 {
		buf := make([]byte, largeSizeLen)
		n, err := r.Read(buf)
		if err != nil {
			return boxHeader{}, err
		}
		if n != largeSizeLen {
			return boxHeader{}, errors.New("Could not read largeSize")
		}
		size = binary.BigEndian.Uint64(buf)
		headerLen += largeSizeLen
	} else if size == 0 {
		return boxHeader{}, errors.New("Size to end of file not supported")
	}
	return boxHeader{string(buf[4:8]), size, headerLen}, nil
}

// EncodeHeader encodes a box header to a writer
func EncodeHeader(b Box, w io.Writer) error {
	fmt.Printf("Writing %v size %d\n", b.Type(), b.Size())
	buf := make([]byte, boxHeaderSize)
	// Todo. Handle largesize extension
	binary.BigEndian.PutUint32(buf, uint32(b.Size()))
	strtobuf(buf[4:], b.Type(), 4)
	_, err := w.Write(buf)
	return err
}

// Box is the general interface
type Box interface {
	Type() string
	Size() uint64
	Encode(w io.Writer) error
}

// BoxDecoder is function signature of the Box Decode method
type BoxDecoder func(size uint64, startPos uint64, r io.Reader) (Box, error)

// DecodeBox decodes a box
func DecodeBox(startPos uint64, r io.Reader) (Box, error) {
	var err error
	var b Box

	h, err := decodeHeader(r)
	if err != nil {
		return nil, err
	}

	d, ok := decoders[h.name]

	remainingLength := int64(h.size) - int64(h.hdrlen)

	if !ok {
		log.Printf("Found unknown box type %v, size %v", h.name, h.size)
		b, err = DecodeUnknown(h.name, h.size, startPos, io.LimitReader(r, remainingLength))

	} else {
		log.Printf("Found supported box %v, size %v", h.name, h.size)
		b, err = d(h.size, startPos, io.LimitReader(r, remainingLength))
	}
	if h.size != b.Size() {
		log.Printf("### Mismatch size %d %d for %s", h.size, b.Size(), b.Type())
	}
	//log.Printf("Box type %v, size %d %d", b.Type(), b.Size())
	if err != nil {
		log.Printf("Error while decoding %s : %s", h.name, err)
		return nil, err
	}
	return b, nil
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
	return make([]byte, b.Size()-boxHeaderSize)
}
