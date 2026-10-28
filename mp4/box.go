package mp4

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"

	"github.com/Eyevinn/mp4ff/bits"
)

const (
	// boxHeaderSize - standard size + name header
	boxHeaderSize = 8
	largeSizeLen  = 8          // Length of largesize extension
	flagsMask     = 0x00ffffff // Flags for masks from full header
)

var decoders map[string]BoxDecoder

func init() {
	decoders = map[string]BoxDecoder{
		"\xa9ART": DecodeGenericContainerBox,
		"\xa9nam": DecodeGenericContainerBox,
		"\xa9too": DecodeGenericContainerBox,
		"\xa9cpy": DecodeGenericContainerBox,
		".mp3":    DecodeQuickTimeAudioSampleEntry,
		"ac-3":    DecodeAudioSampleEntry,
		"ac-4":    DecodeAudioSampleEntry,
		"agsm":    DecodeQuickTimeAudioSampleEntry,
		"alac":    DecodeQuickTimeAudioSampleEntry,
		"alaw":    DecodeQuickTimeAudioSampleEntry,
		"alis":    DecodeAlis,
		"alou":    DecodeLoudnessBaseBox,
		"ap4h":    DecodeQuickTimeVisualSampleEntry,
		"ap4x":    DecodeQuickTimeVisualSampleEntry,
		"apch":    DecodeQuickTimeVisualSampleEntry,
		"apcn":    DecodeQuickTimeVisualSampleEntry,
		"apco":    DecodeQuickTimeVisualSampleEntry,
		"apcs":    DecodeQuickTimeVisualSampleEntry,
		"av01":    DecodeVisualSampleEntry,
		"av1C":    DecodeAv1C,
		"avc1":    DecodeVisualSampleEntry,
		"avc3":    DecodeVisualSampleEntry,
		"av3c":    DecodeAv3c,
		"avcC":    DecodeAvcC,
		"avs3":    DecodeVisualSampleEntry,
		"blin":    DecodeBlin,
		"btrt":    DecodeBtrt,
		"cams":    DecodeCams,
		"cdat":    DecodeCdat,
		"cdsc":    DecodeTrefType,
		"chan":    DecodeChan,
		"chpl":    DecodeChpl,
		"clap":    DecodeClap,
		"clef":    DecodeTrackApertureDimensions,
		"clli":    DecodeClli,
		"co64":    DecodeCo64,
		"CoLL":    DecodeCoLL,
		"colr":    DecodeColr,
		"cslg":    DecodeCslg,
		"cstg":    DecodeTrackGroupType,
		"ctim":    DecodeCtim,
		"ctts":    DecodeCtts,
		"dac3":    DecodeDac3,
		"dac4":    DecodeDac4,
		"enof":    DecodeTrackApertureDimensions,
		"fl32":    DecodeQuickTimeAudioSampleEntry,
		"fl64":    DecodeQuickTimeAudioSampleEntry,
		"ftab":    DecodeFtab,
		"gmhd":    DecodeGmhd,
		"gmin":    DecodeGmin,
		"ima4":    DecodeQuickTimeAudioSampleEntry,
		"in24":    DecodeQuickTimeAudioSampleEntry,
		"in32":    DecodeQuickTimeAudioSampleEntry,
		"keys":    DecodeKeys,
		"MAC3":    DecodeQuickTimeAudioSampleEntry,
		"MAC6":    DecodeQuickTimeAudioSampleEntry,
		"mean":    DecodeMean,
		"mhaC":    DecodeMhaC,
		"data":    DecodeData,
		"dec3":    DecodeDec3,
		"dfLa":    DecodeDfLa,
		"dOps":    DecodeDops,
		"desc":    DecodeGenericContainerBox,
		"dinf":    DecodeDinf,
		"dpnd":    DecodeTrefType,
		"dref":    DecodeDref,
		"dvcC":    DecodeDoViConfig,
		"dvh1":    DecodeVisualSampleEntry,
		"dvhe":    DecodeVisualSampleEntry,
		"dvvC":    DecodeDoViConfig,
		"dvwC":    DecodeDoViConfig,
		"ec-3":    DecodeAudioSampleEntry,
		"edts":    DecodeEdts,
		"elng":    DecodeElng,
		"elst":    DecodeElst,
		"emeb":    DecodeEmeb,
		"emib":    DecodeEmib,
		"emsg":    DecodeEmsg,
		"enca":    DecodeAudioSampleEntry,
		"encv":    DecodeVisualSampleEntry,
		"esds":    DecodeEsds,
		"evte":    DecodeEvte,
		"eyes":    DecodeEyes,
		"fiel":    DecodeFiel,
		"fLaC":    DecodeAudioSampleEntry,
		"font":    DecodeTrefType,
		"free":    DecodeFree,
		"frma":    DecodeFrma,
		"ftyp":    DecodeFtyp,
		"hdlr":    DecodeHdlr,
		"hero":    DecodeHero,
		"hev1":    DecodeVisualSampleEntry,
		"hfov":    DecodeHfov,
		"hind":    DecodeTrefType,
		"hint":    DecodeTrefType,
		"hvc1":    DecodeVisualSampleEntry,
		"hvcC":    DecodeHvcC,
		"lhvC":    DecodeLhvC,
		"iacb":    DecodeIacb,
		"iamf":    DecodeAudioSampleEntry,
		"iden":    DecodeIden,
		"ID32":    DecodeID32,
		"ilst":    DecodeIlst,
		"iods":    DecodeUnknown,
		"ipir":    DecodeTrefType,
		"jpeg":    DecodeVisualSampleEntry,
		"jpgC":    DecodeJpgC,
		"kind":    DecodeKind,
		"labl":    DecodeLabl,
		"leva":    DecodeLeva,
		"lpcm":    DecodeQuickTimeAudioSampleEntry,
		"ludt":    DecodeLudt,
		"mdat":    DecodeMdat,
		"mdcv":    DecodeMdcv,
		"mehd":    DecodeMehd,
		"mdhd":    DecodeMdhd,
		"mdia":    DecodeMdia,
		"meta":    DecodeMeta,
		"mfhd":    DecodeMfhd,
		"mfra":    DecodeMfra,
		"mfro":    DecodeMfro,
		"mha1":    DecodeAudioSampleEntry,
		"mha2":    DecodeAudioSampleEntry,
		"mhm1":    DecodeAudioSampleEntry,
		"mhm2":    DecodeAudioSampleEntry,
		"mime":    DecodeMime,
		"minf":    DecodeMinf,
		"mjpg":    DecodeVisualSampleEntry,
		"moof":    DecodeMoof,
		"moov":    DecodeMoov,
		"mp4a":    DecodeAudioSampleEntry,
		"mp4v":    DecodeVisualSampleEntry,
		"mpod":    DecodeTrefType,
		"msrc":    DecodeTrackGroupType,
		"mvex":    DecodeMvex,
		"mvhd":    DecodeMvhd,
		"name":    DecodeName,
		"nmhd":    DecodeNmhd,
		"Opus":    DecodeAudioSampleEntry,
		"pasp":    DecodePasp,
		"payl":    DecodePayl,
		"prft":    DecodePrft,
		"prji":    DecodePrji,
		"prof":    DecodeTrackApertureDimensions,
		"proj":    DecodeProj,
		"pssh":    DecodePssh,
		"QDM2":    DecodeQuickTimeAudioSampleEntry,
		"QDMC":    DecodeQuickTimeAudioSampleEntry,
		"rsot":    DecodeRsot,
		"saio":    DecodeSaio,
		"saiz":    DecodeSaiz,
		"sbgp":    DecodeSbgp,
		"schi":    DecodeSchi,
		"schm":    DecodeSchm,
		"sdtp":    DecodeSdtp,
		"senc":    DecodeSenc,
		"sgpd":    DecodeSgpd,
		"sidx":    DecodeSidx,
		"silb":    DecodeSilb,
		"sinf":    DecodeSinf,
		"skip":    DecodeFree,
		"SmDm":    DecodeSmDm,
		"smhd":    DecodeSmhd,
		"sowt":    DecodeQuickTimeAudioSampleEntry,
		"ssix":    DecodeSsix,
		"stbl":    DecodeStbl,
		"stco":    DecodeStco,
		"ster":    DecodeTrackGroupType,
		"sthd":    DecodeSthd,
		"stpc":    DecodeStpp,
		"stpp":    DecodeStpp,
		"stri":    DecodeStri,
		"stsc":    DecodeStsc,
		"stsd":    DecodeStsd,
		"stss":    DecodeStss,
		"stsz":    DecodeStsz,
		"sttg":    DecodeSttg,
		"stts":    DecodeStts,
		"styp":    DecodeStyp,
		"subs":    DecodeSubs,
		"subt":    DecodeTrefType,
		"sync":    DecodeTrefType,
		"tapt":    DecodeTapt,
		"tcmi":    DecodeTcmi,
		"tenc":    DecodeTenc,
		"text":    DecodeText,
		"tfdt":    DecodeTfdt,
		"tfhd":    DecodeTfhd,
		"tfra":    DecodeTfra,
		"tkhd":    DecodeTkhd,
		"tlou":    DecodeLoudnessBaseBox,
		"tmcd":    DecodeTmcd,
		"traf":    DecodeTraf,
		"trak":    DecodeTrak,
		"tref":    DecodeTref,
		"trep":    DecodeTrep,
		"trex":    DecodeTrex,
		"trgr":    DecodeTrgr,
		"trun":    DecodeTrun,
		"ttma":    DecodeTtma,
		"ttmb":    DecodeTtmb,
		"ttmn":    DecodeTtmn,
		"twos":    DecodeQuickTimeAudioSampleEntry,
		"tx3g":    DecodeTx3g,
		"udta":    DecodeUdta,
		"ulaw":    DecodeQuickTimeAudioSampleEntry,
		"url ":    DecodeURLBox,
		"uuid":    DecodeUUIDBox,
		"vdep":    DecodeTrefType,
		"vexu":    DecodeVexu,
		"vlab":    DecodeVlab,
		"vmhd":    DecodeVmhd,
		"vp08":    DecodeVisualSampleEntry,
		"vp09":    DecodeVisualSampleEntry,
		"vpcC":    DecodeVppC,
		"vplx":    DecodeTrefType,
		"vsid":    DecodeVsid,
		"vtta":    DecodeVtta,
		"vvc1":    DecodeVisualSampleEntry,
		"vvcC":    DecodeVvcC,
		"vvi1":    DecodeVisualSampleEntry,
		"vttc":    DecodeVttc,
		"vttC":    DecodeVttC,
		"vtte":    DecodeVtte,
		"vttn":    DecodeVttn,
		"wave":    DecodeWave,
		"wide":    DecodeFree,
		"wvtc":    DecodeWvtt,
		"wvtt":    DecodeWvtt,
	}
}

// RemoveBoxDecoder removes the decode of boxType. It will be treated as unknown instead.
//
// This is a global change, so use with care.
func RemoveBoxDecoder(boxType string) {
	delete(decoders, boxType)
	delete(decodersSR, boxType)
	setBoxType(boxType, nil)
}

// SetBoxDecoder sets decoder functions for a specific boxType.
//
// This is a global change, so use with care.
func SetBoxDecoder(boxType string, dec BoxDecoder, decSR BoxDecoderSR) {
	decoders[boxType] = dec
	decodersSR[boxType] = decSR
	setBoxType(boxType, decSR)
}

// BoxHeader - 8 or 16 bytes depending on size
type BoxHeader struct {
	Name   string
	Size   uint64
	Hdrlen int
}

// payloadLen returns the payload size, or math.MaxInt if it does not fit in an int.
func (b BoxHeader) payloadLen() int {
	return int(min(b.Size-uint64(b.Hdrlen), math.MaxInt))
}

// DecodeHeader decodes a box header (size + box type + possible largeSize)
func DecodeHeader(r io.Reader) (BoxHeader, error) {
	buf := make([]byte, boxHeaderSize)
	n, err := io.ReadFull(r, buf)
	if err != nil {
		return BoxHeader{}, err
	}
	if n != boxHeaderSize {
		return BoxHeader{}, fmt.Errorf("incomplete box header read: %d/%d", n, boxHeaderSize)
	}
	size := uint64(binary.BigEndian.Uint32(buf[0:4]))
	boxType, _ := lookupBoxType(buf[4:8])
	headerLen := boxHeaderSize
	switch size {
	case 1: // size 1 means large size in next 8 bytes
		if boxType != "mdat" {
			return BoxHeader{}, fmt.Errorf("extended size not supported for box type %s", boxType)
		}
		buf := make([]byte, largeSizeLen)
		_, err = io.ReadFull(r, buf)
		if err != nil {
			return BoxHeader{}, err
		}
		size = binary.BigEndian.Uint64(buf)
		headerLen += largeSizeLen
	case 0: // size 0 means to end of file
		return BoxHeader{}, fmt.Errorf("Size 0, meaning to end of file, not supported")
	}
	if uint64(headerLen) > size {
		return BoxHeader{}, fmt.Errorf("box header size %d exceeds box size %d", headerLen, size)
	}
	return BoxHeader{boxType, size, headerLen}, nil
}

// EncodeHeader - encode a box header to a writer
func EncodeHeader(b Box, w io.Writer) error {
	boxType, boxSize := b.Type(), b.Size()
	if boxSize >= 1<<32 {
		return fmt.Errorf("Box size %d is too big for normal 4-byte size field", boxSize)
	}
	buf := make([]byte, boxHeaderSize)
	binary.BigEndian.PutUint32(buf, uint32(boxSize))
	strtobuf(buf[4:], boxType, 4)
	_, err := w.Write(buf)
	return err
}

// EncodeHeaderWithSize - encode a box header to a writer and allow for largeSize
func EncodeHeaderWithSize(boxType string, boxSize uint64, largeSize bool, w io.Writer) error {
	if !largeSize && boxSize >= 1<<32 {
		return fmt.Errorf("Box size %d is too big for normal 4-byte size field", boxSize)
	}
	headerSize := boxHeaderSize
	if largeSize {
		headerSize += 8
	}
	buf := make([]byte, headerSize)
	if !largeSize {
		binary.BigEndian.PutUint32(buf, uint32(boxSize))
		strtobuf(buf[4:], boxType, 4)
	} else {
		binary.BigEndian.PutUint32(buf, 1) // signals large size
		strtobuf(buf[4:], boxType, 4)
		binary.BigEndian.PutUint64(buf[8:], boxSize)
	}
	_, err := w.Write(buf)
	return err
}

// EncodeHeaderSW - encode a box header to a SliceWriter
func EncodeHeaderSW(b Box, sw bits.SliceWriter) error {
	boxType, boxSize := b.Type(), b.Size()
	if boxSize >= 1<<32 {
		return fmt.Errorf("Box size %d is too big for normal 4-byte size field", boxSize)
	}
	sw.WriteUint32(uint32(boxSize))
	sw.WriteString(boxType, false)
	return nil
}

// EncodeHeaderWithSizeSW - encode a box header to a SliceWriter and allow for largeSize
func EncodeHeaderWithSizeSW(boxType string, boxSize uint64, largeSize bool, sw bits.SliceWriter) error {
	if !largeSize && boxSize >= 1<<32 {
		return fmt.Errorf("Box size %d is too big for normal 4-byte size field", boxSize)
	}
	if !largeSize {
		sw.WriteUint32(uint32(boxSize))
		sw.WriteString(boxType, false)
	} else {
		sw.WriteUint32(1) // signals large size
		sw.WriteString(boxType, false)
		sw.WriteUint64(boxSize)
	}
	return sw.AccError()
}

// Box is the general interface to any ISOBMFF box or similar
type Box interface {
	// Type of box, normally 4 asccii characters, but is uint32 according to spec
	Type() string
	// Size of box including header and all children if any
	Size() uint64
	// Encode box to writer
	Encode(w io.Writer) error
	// Encode box to SliceWriter
	EncodeSW(sw bits.SliceWriter) error
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
type BoxDecoder func(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error)

// DecodeBox decodes a box
func DecodeBox(startPos uint64, r io.Reader) (Box, error) {
	h, err := DecodeHeader(r)
	if err != nil {
		return nil, err
	}
	return DecodeBoxBody(startPos, h, r)
}

// DecodeBoxBody decodes a box body from an io.Reader given BoxHeader
func DecodeBoxBody(startPos uint64, hdr BoxHeader, r io.Reader) (Box, error) {
	var err error
	var b Box

	d, ok := decoders[hdr.Name]

	switch {
	case isUserDataTextType(hdr.Name):
		b, err = decodeUserDataTextOr(hdr, startPos, d, r)
	case !ok:
		b, err = DecodeUnknown(hdr, startPos, r)
	default:
		b, err = d(hdr, startPos, r)
	}
	if err != nil {
		return nil, fmt.Errorf("decode %s pos %d: %w", hdr.Name, startPos, err)
	}

	return b, nil
}

// DecodeBoxLazyMdat decodes a box but doesn't read the payload of mdat, free or skip into memory
func DecodeBoxLazyMdat(startPos uint64, r io.ReadSeeker) (Box, error) {
	h, err := DecodeHeader(r)
	if err != nil {
		return nil, err
	}
	return DecodeBoxBodyLazily(startPos, h, r)

}

// DecodeBoxBodyLazily decodes the body of a box whose header h has just been read from r.
// The payload of mdat, free and skip is not read, but seeked past.
func DecodeBoxBodyLazily(startPos uint64, h BoxHeader, r io.ReadSeeker) (Box, error) {
	var err error
	var b Box

	d, ok := decoders[h.Name]

	switch {
	case isUserDataTextType(h.Name):
		b, err = decodeUserDataTextOr(h, startPos, d, r)
	case !ok:
		b, err = DecodeUnknown(h, startPos, r)
	default:
		switch h.Name {
		case "mdat":
			b, err = DecodeMdatLazily(h, startPos)
			if err == nil {
				err = seekPastBoxBody(h, r)
			}
		case "free", "skip", "wide":
			b = &FreeBox{Name: h.Name, lazyPayloadSize: h.Size - uint64(h.Hdrlen)}
			err = seekPastBoxBody(h, r)
		default:
			b, err = d(h, startPos, r)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("decode box %q: %w", h.Name, err)
	}

	return b, nil
}

// seekPastBoxBody seeks r from the end of header h to the end of the box.
// A box size above math.MaxInt64 is an error, since it would make the seek go backwards.
func seekPastBoxBody(h BoxHeader, r io.ReadSeeker) error {
	if h.Size > math.MaxInt64 {
		return fmt.Errorf("box size %d is larger than the largest file offset", h.Size)
	}
	_, err := r.Seek(int64(h.Size-uint64(h.Hdrlen)), io.SeekCurrent)
	return err
}

// Fixed16 - An 8.8 fixed point number
type Fixed16 uint16

func (f Fixed16) String() string {
	return fmt.Sprintf("%d.%d", uint16(f)>>8, uint16(f)&0xff)
}

// Fixed32 -  A 16.16 fixed point number
type Fixed32 uint32

func (f Fixed32) String() string {
	return fmt.Sprintf("%d.%d", uint32(f)>>16, uint32(f)&0xffff)
}

func strtobuf(out []byte, in string, l int) {
	if l < len(in) {
		copy(out, in)
	} else {
		copy(out, in[0:l])
	}
}

func makebuf(b Box) []byte {
	return make([]byte, b.Size()-boxHeaderSize)
}

// readBoxBody reads complete box body. Returns error if not possible
func readBoxBody(r io.Reader, h BoxHeader) ([]byte, error) {
	hdrLen := uint64(h.Hdrlen)
	if hdrLen == h.Size {
		return nil, nil
	}
	bodyLen := h.Size - hdrLen
	body, err := io.ReadAll(io.LimitReader(r, int64(bodyLen)))
	if err != nil {
		return nil, err
	}
	if len(body) != int(bodyLen) {
		return nil, fmt.Errorf("read box body length %d does not match expected length %d", len(body), bodyLen)
	}
	return body, nil
}
