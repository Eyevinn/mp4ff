package mp4

import (
	"fmt"

	"github.com/Eyevinn/mp4ff/bits"
)

var decodersSR map[string]BoxDecoderSR

func init() {
	decodersSR = map[string]BoxDecoderSR{
		"\xa9ART": DecodeGenericContainerBoxSR,
		"\xa9cpy": DecodeGenericContainerBoxSR,
		"\xa9nam": DecodeGenericContainerBoxSR,
		"\xa9too": DecodeGenericContainerBoxSR,
		"ac-3":    DecodeAudioSampleEntrySR,
		"ac-4":    DecodeAudioSampleEntrySR,
		"alou":    DecodeLoudnessBaseBoxSR,
		"av01":    DecodeVisualSampleEntrySR,
		"av1C":    DecodeAv1CSR,
		"avc1":    DecodeVisualSampleEntrySR,
		"avc3":    DecodeVisualSampleEntrySR,
		"av3c":    DecodeAv3cSR,
		"avcC":    DecodeAvcCSR,
		"avs3":    DecodeVisualSampleEntrySR,
		"blin":    DecodeBlinSR,
		"btrt":    DecodeBtrtSR,
		"cams":    DecodeCamsSR,
		"cdat":    DecodeCdatSR,
		"cdsc":    DecodeTrefTypeSR,
		"clap":    DecodeClapSR,
		"clli":    DecodeClliSR,
		"co64":    DecodeCo64SR,
		"CoLL":    DecodeCoLLSR,
		"colr":    DecodeColrSR,
		"cslg":    DecodeCslgSR,
		"cstg":    DecodeTrackGroupTypeSR,
		"ctim":    DecodeCtimSR,
		"ctts":    DecodeCttsSR,
		"dac3":    DecodeDac3SR,
		"dac4":    DecodeDac4SR,
		"mhaC":    DecodeMhaCSR,
		"data":    DecodeDataSR,
		"dec3":    DecodeDec3SR,
		"dfLa":    DecodeDfLaSR,
		"dOps":    DecodeDopsSR,
		"desc":    DecodeGenericContainerBoxSR,
		"dinf":    DecodeDinfSR,
		"dpnd":    DecodeTrefTypeSR,
		"dref":    DecodeDrefSR,
		"dvcC":    DecodeDoViConfigSR,
		"dvh1":    DecodeVisualSampleEntrySR,
		"dvhe":    DecodeVisualSampleEntrySR,
		"dvvC":    DecodeDoViConfigSR,
		"dvwC":    DecodeDoViConfigSR,
		"ec-3":    DecodeAudioSampleEntrySR,
		"edts":    DecodeEdtsSR,
		"elng":    DecodeElngSR,
		"elst":    DecodeElstSR,
		"emeb":    DecodeEmebSR,
		"emib":    DecodeEmibSR,
		"emsg":    DecodeEmsgSR,
		"enca":    DecodeAudioSampleEntrySR,
		"encv":    DecodeVisualSampleEntrySR,
		"esds":    DecodeEsdsSR,
		"evte":    DecodeEvteSR,
		"eyes":    DecodeEyesSR,
		"fLaC":    DecodeAudioSampleEntrySR,
		"font":    DecodeTrefTypeSR,
		"free":    DecodeFreeSR,
		"frma":    DecodeFrmaSR,
		"ftyp":    DecodeFtypSR,
		"hdlr":    DecodeHdlrSR,
		"hero":    DecodeHeroSR,
		"hev1":    DecodeVisualSampleEntrySR,
		"hfov":    DecodeHfovSR,
		"hind":    DecodeTrefTypeSR,
		"hint":    DecodeTrefTypeSR,
		"hvc1":    DecodeVisualSampleEntrySR,
		"hvcC":    DecodeHvcCSR,
		"lhvC":    DecodeLhvCSR,
		"iacb":    DecodeIacbSR,
		"iamf":    DecodeAudioSampleEntrySR,
		"iden":    DecodeIdenSR,
		"ID32":    DecodeID32SR,
		"ilst":    DecodeIlstSR,
		"iods":    DecodeUnknownSR,
		"ipir":    DecodeTrefTypeSR,
		"kind":    DecodeKindSR,
		"leva":    DecodeLevaSR,
		"ludt":    DecodeLudtSR,
		"mdat":    DecodeMdatSR,
		"mdcv":    DecodeMdcvSR,
		"mehd":    DecodeMehdSR,
		"mdhd":    DecodeMdhdSR,
		"mdia":    DecodeMdiaSR,
		"meta":    DecodeMetaSR,
		"mfhd":    DecodeMfhdSR,
		"mfra":    DecodeMfraSR,
		"mfro":    DecodeMfroSR,
		"mha1":    DecodeAudioSampleEntrySR,
		"mha2":    DecodeAudioSampleEntrySR,
		"mhm1":    DecodeAudioSampleEntrySR,
		"mhm2":    DecodeAudioSampleEntrySR,
		"mime":    DecodeMimeSR,
		"minf":    DecodeMinfSR,
		"moof":    DecodeMoofSR,
		"moov":    DecodeMoovSR,
		"mp4a":    DecodeAudioSampleEntrySR,
		"mpod":    DecodeTrefTypeSR,
		"msrc":    DecodeTrackGroupTypeSR,
		"mvex":    DecodeMvexSR,
		"mvhd":    DecodeMvhdSR,
		"nmhd":    DecodeNmhdSR,
		"Opus":    DecodeAudioSampleEntrySR,
		"pasp":    DecodePaspSR,
		"payl":    DecodePaylSR,
		"prft":    DecodePrftSR,
		"prji":    DecodePrjiSR,
		"proj":    DecodeProjSR,
		"pssh":    DecodePsshSR,
		"saio":    DecodeSaioSR,
		"saiz":    DecodeSaizSR,
		"sbgp":    DecodeSbgpSR,
		"schi":    DecodeSchiSR,
		"schm":    DecodeSchmSR,
		"sdtp":    DecodeSdtpSR,
		"senc":    DecodeSencSR,
		"sgpd":    DecodeSgpdSR,
		"sidx":    DecodeSidxSR,
		"silb":    DecodeSilbSR,
		"sinf":    DecodeSinfSR,
		"skip":    DecodeFreeSR,
		"SmDm":    DecodeSmDmSR,
		"smhd":    DecodeSmhdSR,
		"ssix":    DecodeSsixSR,
		"stbl":    DecodeStblSR,
		"stco":    DecodeStcoSR,
		"ster":    DecodeTrackGroupTypeSR,
		"sthd":    DecodeSthdSR,
		"stpp":    DecodeStppSR,
		"stri":    DecodeStriSR,
		"stsc":    DecodeStscSR,
		"stsd":    DecodeStsdSR,
		"stss":    DecodeStssSR,
		"stsz":    DecodeStszSR,
		"sttg":    DecodeSttgSR,
		"stts":    DecodeSttsSR,
		"styp":    DecodeStypSR,
		"subs":    DecodeSubsSR,
		"subt":    DecodeTrefTypeSR,
		"sync":    DecodeTrefTypeSR,
		"tenc":    DecodeTencSR,
		"tfdt":    DecodeTfdtSR,
		"tfhd":    DecodeTfhdSR,
		"tfra":    DecodeTfraSR,
		"tkhd":    DecodeTkhdSR,
		"tlou":    DecodeLoudnessBaseBoxSR,
		"traf":    DecodeTrafSR,
		"trak":    DecodeTrakSR,
		"tref":    DecodeTrefSR,
		"trep":    DecodeTrepSR,
		"trex":    DecodeTrexSR,
		"trgr":    DecodeTrgrSR,
		"trun":    DecodeTrunSR,
		"udta":    DecodeUdtaSR,
		"url ":    DecodeURLBoxSR,
		"uuid":    DecodeUUIDBoxSR,
		"vdep":    DecodeTrefTypeSR,
		"vexu":    DecodeVexuSR,
		"vlab":    DecodeVlabSR,
		"vmhd":    DecodeVmhdSR,
		"vp08":    DecodeVisualSampleEntrySR,
		"vp09":    DecodeVisualSampleEntrySR,
		"vpcC":    DecodeVppCSR,
		"vplx":    DecodeTrefTypeSR,
		"vsid":    DecodeVsidSR,
		"vtta":    DecodeVttaSR,
		"vvc1":    DecodeVisualSampleEntrySR,
		"vvcC":    DecodeVvcCSR,
		"vvi1":    DecodeVisualSampleEntrySR,
		"vttc":    DecodeVttcSR,
		"vttC":    DecodeVttCSR,
		"vtte":    DecodeVtteSR,
		"wvtt":    DecodeWvttSR,
	}
}

// BoxDecoderSR is function signature of the Box DecodeSR method
type BoxDecoderSR func(hdr BoxHeader, startPos uint64, sw bits.SliceReader) (Box, error)

// DecodeBoxSR - decode a box from SliceReader
func DecodeBoxSR(startPos uint64, sr bits.SliceReader) (Box, error) {
	h, err := DecodeHeaderSR(sr)
	if err != nil {
		return nil, err
	}
	return DecodeBoxBodySR(startPos, h, sr)
}

// DecodeHeaderSR - decode a box header (size + box type + possible largeSize) from sr
func DecodeHeaderSR(sr bits.SliceReader) (BoxHeader, error) {
	if sr.NrRemainingBytes() < boxHeaderSize {
		return BoxHeader{}, fmt.Errorf("not enough bytes to read box header, need %d, have %d", boxHeaderSize, sr.NrRemainingBytes())
	}
	size := uint64(sr.ReadUint32())
	boxType := sr.ReadFixedLengthString(4)
	headerLen := boxHeaderSize
	switch size {
	case 1: // size 1 means large size in next 8 bytes
		if boxType != "mdat" {
			return BoxHeader{}, fmt.Errorf("extended size not supported for box type %s", boxType)
		}
		size = sr.ReadUint64()
		headerLen += largeSizeLen
	case 0: // size 0 means to end of file
		return BoxHeader{}, fmt.Errorf("Size 0, meaning to end of file, not supported")
	}
	if uint64(headerLen) > size {
		return BoxHeader{}, fmt.Errorf("box header size %d exceeds box size %d", headerLen, size)
	}
	return BoxHeader{boxType, size, headerLen}, sr.AccError()
}

// DecodeBoxBodySR - decode box body from SliceReader given BoxHeader
func DecodeBoxBodySR(startPos uint64, hdr BoxHeader, sr bits.SliceReader) (Box, error) {
	maxSize := uint64(sr.NrRemainingBytes()) + uint64(hdr.Hdrlen)
	// In the following, we do not block mdat to allow for the case
	// that the first kiloBytes of a file are fetched and parsed to
	// get the init part of a file. In the future, a new decode option that
	// stops before the mdat starts is a better alternative.
	if hdr.Size > maxSize && hdr.Name != "mdat" {
		return nil, fmt.Errorf("decode box %q, size %d too big (max %d)", hdr.Name, hdr.Size, maxSize)
	}

	d, ok := decodersSR[hdr.Name]

	var b Box
	var err error
	if !ok {
		b, err = DecodeUnknownSR(hdr, startPos, sr)
	} else {
		b, err = d(hdr, startPos, sr)
	}
	if err != nil {
		return nil, fmt.Errorf("decode %s pos %d: %w", hdr.Name, startPos, err)
	}

	return b, nil
}

// DecodeFile - parse and decode a file from reader r with optional file options.
// For example, the file options overwrite the default decode or encode mode.
func DecodeFileSR(sr bits.SliceReader, options ...Option) (*File, error) {
	f := NewFile()

	// apply options to change the default decode or encode mode
	f.ApplyOptions(options...)

	var boxStartPos uint64 = 0
	lastBoxType := ""

	if f.fileDecMode == DecModeLazyMdat {
		return nil, fmt.Errorf("no support for lazy mdat in DecodeFileSR")
	}

LoopBoxes:
	for {
		var box Box
		var err error
		if sr.NrRemainingBytes() == 0 {
			break LoopBoxes
		}

		box, err = DecodeBoxSR(boxStartPos, sr)
		if err != nil {
			return nil, err
		}
		boxType, boxSize := box.Type(), box.Size()
		switch boxType {
		case "mdat":
			if f.isFragmented {
				if lastBoxType != "moof" {
					return nil, fmt.Errorf("does not support %v between moof and mdat", lastBoxType)
				}
			} else {
				if f.Mdat != nil {
					oldPayloadSize := f.Mdat.Size() - f.Mdat.HeaderSize()
					newMdat := box.(*MdatBox)
					newPayloadSize := newMdat.Size() - newMdat.HeaderSize()
					if oldPayloadSize > 0 && newPayloadSize > 0 {
						return nil, fmt.Errorf("only one non-empty mdat box supported (payload sizes %d and %d)",
							oldPayloadSize, newPayloadSize)
					}
				}
			}
		case "moof":
			moof := box.(*MoofBox)
			for _, traf := range moof.Trafs {
				if ok, parsed := traf.ContainsSencBox(); ok && !parsed {
					defaultIVSize := byte(0)
					if f.Moov != nil {
						trackID := traf.Tfhd.TrackID
						if !f.Moov.IsEncrypted(trackID) {
							continue
						}
						sinf := f.Moov.GetSinf(trackID)
						if sinf != nil && sinf.Schi != nil && sinf.Schi.Tenc != nil {
							defaultIVSize = sinf.Schi.Tenc.DefaultPerSampleIVSize
						}
					}
					err = traf.ParseReadSenc(defaultIVSize, moof.StartPos)
					if err != nil {
						if f.Moov == nil {
							// No moov and heuristic failed.
							// Leave senc deferred for caller to parse later with init info.
							continue
						}
						return nil, err
					}
				}
			}
		}
		f.AddChild(box, boxStartPos)
		lastBoxType = boxType
		boxStartPos += boxSize
	}
	return f, nil
}
