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
	largeSizeLen  = 8          // Length of largesize exension
	flagsMask     = 0x00ffffff // Flags for masks from full header
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
		"avc1": DecodeAvcX,
		"avc3": DecodeAvcX,
		"avcC": DecodeAvcC,
		"ctts": DecodeCtts,
		"dinf": DecodeDinf,
		"dref": DecodeDref,
		"edts": DecodeEdts,
		"elst": DecodeElst,
		"free": DecodeFree,
		"ftyp": DecodeFtyp,
		"hdlr": DecodeHdlr,
		"iods": DecodeUnknown,
		"mdat": DecodeMdat,
		"mdhd": DecodeMdhd,
		"mdia": DecodeMdia,
		"meta": DecodeUnknown,
		"mfhd": DecodeMfhd,
		"minf": DecodeMinf,
		"moof": DecodeMoof,
		"moov": DecodeMoov,
		"mvex": DecodeMvex,
		"mvhd": DecodeMvhd,
		"prft": DecodePrft,
		"smhd": DecodeSmhd,
		"stbl": DecodeStbl,
		"stco": DecodeStco,
		"stsc": DecodeStsc,
		"stsd": DecodeStsd,
		"stss": DecodeStss,
		"stsz": DecodeStsz,
		"stts": DecodeStts,
		"styp": DecodeStyp,
		"tfdt": DecodeTfdt,
		"tfhd": DecodeTfhd,
		"tkhd": DecodeTkhd,
		"traf": DecodeTraf,
		"trak": DecodeTrak,
		"trex": DecodeTrex,
		"trun": DecodeTrun,
		"vmhd": DecodeVmhd,
	}
}

// boxHeader - 8 or 16 bytes depending on size
type boxHeader struct {
	name   string
	size   uint64
	hdrlen int
}

// decodeHeader decodes a box header (size + box type)
func decodeHeader(r io.Reader) (*boxHeader, error) {
	buf := make([]byte, boxHeaderSize)
	n, err := r.Read(buf)
	if err != nil {
		return nil, err
	}
	if n != boxHeaderSize {
		return nil, ErrTruncatedHeader
	}
	size := uint64(binary.BigEndian.Uint32(buf[0:4]))
	headerLen := boxHeaderSize
	if size == 1 {
		buf := make([]byte, largeSizeLen)
		n, err := r.Read(buf)
		if err != nil {
			return nil, err
		}
		if n != largeSizeLen {
			return nil, errors.New("Could not read largeSize")
		}
		size = binary.BigEndian.Uint64(buf)
		headerLen += largeSizeLen
	} else if size == 0 {
		return nil, errors.New("Size to end of file not supported")
	}
	return &boxHeader{string(buf[4:8]), size, headerLen}, nil
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
type BoxDecoder func(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error)

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
		b, err = DecodeUnknown(h, startPos, io.LimitReader(r, remainingLength))

	} else {
		log.Printf("Found supported box %v, size %v", h.name, h.size)
		b, err = d(h, startPos, io.LimitReader(r, remainingLength))
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
