package mp4

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
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
		"ctim": DecodeCtim,
		"ctts": DecodeCtts,
		"dinf": DecodeDinf,
		"dref": DecodeDref,
		"elng": DecodeElng,
		"esds": DecodeEsds,
		"edts": DecodeEdts,
		"elst": DecodeElst,
		"enca": DecodeAudioSampleEntry,
		"encv": DecodeVisualSampleEntry,
		"emsg": DecodeEmsg,
		"free": DecodeFree,
		"frma": DecodeFrma,
		"ftyp": DecodeFtyp,
		"hdlr": DecodeHdlr,
		"iden": DecodeIden,
		"iods": DecodeUnknown,
		"mdat": DecodeMdat,
		"mehd": DecodeMehd,
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
		"nmhd": DecodeNmhd,
		"payl": DecodePayl,
		"prft": DecodePrft,
		"pssh": DecodePssh,
		"saio": DecodeSaio,
		"saiz": DecodeSaiz,
		"sbgp": DecodeSbgp,
		"schi": DecodeSchi,
		"schm": DecodeSchm,
		"senc": DecodeSenc,
		"sgpd": DecodeSgpd,
		"sidx": DecodeSidx,
		"sinf": DecodeSinf,
		"smhd": DecodeSmhd,
		"sthd": DecodeSthd,
		"stbl": DecodeStbl,
		"stco": DecodeStco,
		"stpp": DecodeStpp,
		"stsc": DecodeStsc,
		"stsd": DecodeStsd,
		"stss": DecodeStss,
		"stsz": DecodeStsz,
		"sttg": DecodeSttg,
		"stts": DecodeStts,
		"styp": DecodeStyp,
		"subs": DecodeSubs,
		"tenc": DecodeTenc,
		"tfdt": DecodeTfdt,
		"tfhd": DecodeTfhd,
		"tkhd": DecodeTkhd,
		"traf": DecodeTraf,
		"trak": DecodeTrak,
		"trex": DecodeTrex,
		"trun": DecodeTrun,
		"url ": DecodeURLBox,
		"uuid": DecodeUUID,
		"vlab": DecodeVlab,
		"vmhd": DecodeVmhd,
		"vsid": DecodeVsid,
		"vtta": DecodeVtta,
		"vttc": DecodeVttc,
		"vttC": DecodeVttC,
		"vtte": DecodeVtte,
		"wvtt": DecodeWvtt,
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
	buf := make([]byte, boxHeaderSize)
	largeSize := false
	if boxSize < 1<<32 {
		binary.BigEndian.PutUint32(buf, uint32(boxSize))
	} else {
		largeSize = true
		binary.BigEndian.PutUint32(buf, 1)
	}
	strtobuf(buf[4:], boxType, 4)
	if largeSize {
		binary.BigEndian.PutUint64(buf, boxSize)
	}
	_, err := w.Write(buf)
	return err
}

// Box is the general interface to any ISOBMFF box or similar
type Box interface {
	// Type of box, normally 4 asccii characters, but is uint32 according to spec
	Type() string
	// Size of box including header and all children if any
	Size() uint64
	// Encode box to writer
	Encode(w io.Writer) error
	// Info - write box details
	//   spedificBoxLevels is a comma-separated list box:level or all:level where level >= 0.
	//   Higher levels give more details. 0 is default
	//   indent is indent at this box level.
	//   indentStep is how much to indent at each level
	Info(w io.Writer, specificBoxLevels, indent, indentStep string) error
}

// Informer - write box, segment or file details
type Informer interface {
	// Info - write details via Info method
	//   spedificBoxLevels is a comma-separated list box:level or all:level where level >= 0.
	//   Higher levels give more details. 0 is default
	//   indent is indent at this box level.
	//   indentStep is how much to indent at each level
	Info(w io.Writer, specificBoxLevels, indent, indentStep string) error
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
		b, err = DecodeUnknown(h, startPos, io.LimitReader(r, remainingLength))

	} else {
		b, err = d(h, startPos, io.LimitReader(r, remainingLength))
	}
	if err != nil {
		return nil, fmt.Errorf("decode %s: %w", h.name, err)
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
