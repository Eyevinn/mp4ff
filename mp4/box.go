package mp4

import (
	"encoding/binary"
	"fmt"
	"io"

	"github.com/pkg/errors"

	log "github.com/sirupsen/logrus"
)

const (
	// boxHeaderSize - standard size + name header
	boxHeaderSize = 8
	largeSizeLen  = 8          // Length of largesize exension
	flagsMask     = 0x00ffffff // Flags for masks from full header
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
		"avc1": DecodeVisualSampleEntry,
		"avc3": DecodeVisualSampleEntry,
		"avcC": DecodeAvcC,
		"btrt": DecodeBtrt,
		"ctts": DecodeCtts,
		"dinf": DecodeDinf,
		"dref": DecodeDref,
		"elng": DecodeElng,
		"esds": DecodeEsds,
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
		"mp4a": DecodeAudioSampleEntry,
		"prft": DecodePrft,
		"senc": DecodeSenc,
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
		"url ": DecodeURLBox,
		"uuid": DecodeUUID,
		"vmhd": DecodeVmhd,
	}
}

// boxHeader - 8 or 16 bytes depending on size
type boxHeader struct {
	name   string
	size   uint64
	hdrlen int
}

// decodeHeader decodes a box header (size + box type + possiible largeSize)
func decodeHeader(r io.Reader) (*boxHeader, error) {
	buf := make([]byte, boxHeaderSize)
	n, err := r.Read(buf)
	if err != nil {
		return nil, err
	}
	if n != boxHeaderSize {
		return nil, errors.New("Could not read full 8B header")
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
		return nil, errors.New("Size 0, meaning to end of file, not supported")
	}
	return &boxHeader{string(buf[4:8]), size, headerLen}, nil
}

// EncodeHeader encodes a box header to a writer
func EncodeHeader(b Box, w io.Writer) error {
	boxType, boxSize := b.Type(), b.Size()
	log.Debugf("Writing %v size %d\n", boxType, boxSize)
	buf := make([]byte, boxHeaderSize)
	largeSize := false
	if boxSize < 1<<32 {
		binary.BigEndian.PutUint32(buf, uint32(boxSize))
	} else {
		largeSize = true
		binary.BigEndian.PutUint32(buf, 1)
	}
	strtobuf(buf[4:], b.Type(), 4)
	if largeSize {
		binary.BigEndian.PutUint64(buf, boxSize)
	}
	_, err := w.Write(buf)
	return err
}

// Box is the general interface
type Box interface {
	Type() string
	Size() uint64
	Encode(w io.Writer) error
	Dump(w io.Writer, indent, indentStep string) error
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
		log.Debugf("Found unknown box type %v, size %v", h.name, h.size)
		b, err = DecodeUnknown(h, startPos, io.LimitReader(r, remainingLength))

	} else {
		log.Debugf("Found supported box %v, size %v", h.name, h.size)
		b, err = d(h, startPos, io.LimitReader(r, remainingLength))
	}
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("couldn't decode %s", h.name))
	}

	return b, nil
}

// Fixed16 - An 8.8 fixed point number
type Fixed16 uint16

func (f Fixed16) String() string {
	return fmt.Sprintf("%d.%d", uint16(f)>>8, uint16(f)&7)
}

// Fixed32 -  A 16.16 fixed point number
type Fixed32 uint32

func (f Fixed32) String() string {
	return fmt.Sprintf("%d.%d", uint32(f)>>16, uint32(f)&15)
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
