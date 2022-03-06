package mp4

import (
	"fmt"

	"github.com/edgeware/mp4ff/bits"
)

var decodersSR map[string]BoxDecoderSR

func init() {
	decodersSR = map[string]BoxDecoderSR{
		"ac-3":    DecodeAudioSampleEntrySR,
		"avc1":    DecodeVisualSampleEntrySR,
		"avc3":    DecodeVisualSampleEntrySR,
		"avcC":    DecodeAvcCSR,
		"btrt":    DecodeBtrtSR,
		"cdat":    DecodeCdatSR,
		"cdsc":    DecodeTrefTypeSR,
		"clap":    DecodeClapSR,
		"cslg":    DecodeCslgSR,
		"co64":    DecodeCo64SR,
		"ctim":    DecodeCtimSR,
		"ctts":    DecodeCttsSR,
		"dac3":    DecodeDac3SR,
		"data":    DecodeDataSR,
		"dec3":    DecodeDec3SR,
		"dinf":    DecodeDinfSR,
		"dpnd":    DecodeTrefTypeSR,
		"dref":    DecodeDrefSR,
		"ec-3":    DecodeAudioSampleEntrySR,
		"elng":    DecodeElngSR,
		"esds":    DecodeEsdsSR,
		"edts":    DecodeEdtsSR,
		"elst":    DecodeElstSR,
		"enca":    DecodeAudioSampleEntrySR,
		"encv":    DecodeVisualSampleEntrySR,
		"emsg":    DecodeEmsgSR,
		"font":    DecodeTrefTypeSR,
		"free":    DecodeFreeSR,
		"frma":    DecodeFrmaSR,
		"ftyp":    DecodeFtypSR,
		"hdlr":    DecodeHdlrSR,
		"hev1":    DecodeVisualSampleEntrySR,
		"hind":    DecodeTrefTypeSR,
		"hint":    DecodeTrefTypeSR,
		"hvcC":    DecodeHvcCSR,
		"hvc1":    DecodeVisualSampleEntrySR,
		"iden":    DecodeIdenSR,
		"ilst":    DecodeIlstSR,
		"iods":    DecodeUnknownSR,
		"ipir":    DecodeTrefTypeSR,
		"kind":    DecodeKindSR,
		"mdat":    DecodeMdatSR,
		"mehd":    DecodeMehdSR,
		"mdhd":    DecodeMdhdSR,
		"mdia":    DecodeMdiaSR,
		"meta":    DecodeMetaSR,
		"mfhd":    DecodeMfhdSR,
		"mfra":    DecodeMfraSR,
		"mfro":    DecodeMfroSR,
		"mime":    DecodeMimeSR,
		"minf":    DecodeMinfSR,
		"moof":    DecodeMoofSR,
		"moov":    DecodeMoovSR,
		"mpod":    DecodeTrefTypeSR,
		"mvex":    DecodeMvexSR,
		"mvhd":    DecodeMvhdSR,
		"mp4a":    DecodeAudioSampleEntrySR,
		"nmhd":    DecodeNmhdSR,
		"pasp":    DecodePaspSR,
		"payl":    DecodePaylSR,
		"prft":    DecodePrftSR,
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
		"sinf":    DecodeSinfSR,
		"skip":    DecodeFreeSR,
		"smhd":    DecodeSmhdSR,
		"sthd":    DecodeSthdSR,
		"stbl":    DecodeStblSR,
		"stco":    DecodeStcoSR,
		"stpp":    DecodeStppSR,
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
		"traf":    DecodeTrafSR,
		"trak":    DecodeTrakSR,
		"tref":    DecodeTrefSR,
		"trep":    DecodeTrepSR,
		"trex":    DecodeTrexSR,
		"trun":    DecodeTrunSR,
		"udta":    DecodeUdtaSR,
		"url ":    DecodeURLBoxSR,
		"uuid":    DecodeUUIDBoxSR,
		"vdep":    DecodeTrefTypeSR,
		"vlab":    DecodeVlabSR,
		"vmhd":    DecodeVmhdSR,
		"vplx":    DecodeTrefTypeSR,
		"vsid":    DecodeVsidSR,
		"vtta":    DecodeVttaSR,
		"vttc":    DecodeVttcSR,
		"vttC":    DecodeVttCSR,
		"vtte":    DecodeVtteSR,
		"wvtt":    DecodeWvttSR,
		"\xa9too": DecodeCTooSR,
	}
}

// BoxDecoderSR is function signature of the Box DecodeSR method
type BoxDecoderSR func(hdr BoxHeader, startPos uint64, sw bits.SliceReader) (Box, error)

// DecodeBoxSR - decode a box from SliceReader
func DecodeBoxSR(startPos uint64, sr bits.SliceReader) (Box, error) {
	var err error
	var b Box

	h, err := DecodeHeaderSR(sr)
	if err != nil {
		return nil, err
	}

	d, ok := decodersSR[h.Name]

	if !ok {
		b, err = DecodeUnknownSR(h, startPos, sr)
	} else {
		b, err = d(h, startPos, sr)
	}
	if err != nil {
		return nil, fmt.Errorf("decode %s: %w", h.Name, err)
	}

	return b, nil
}

// DecodeHeaderSR - decode a box header (size + box type + possible largeSize) from sr
func DecodeHeaderSR(sr bits.SliceReader) (BoxHeader, error) {
	size := uint64(sr.ReadUint32())
	boxType := sr.ReadFixedLengthString(4)
	headerLen := boxHeaderSize
	if size == 1 {
		size = sr.ReadUint64()
		headerLen += largeSizeLen
	} else if size == 0 {
		return BoxHeader{}, fmt.Errorf("Size 0, meaning to end of file, not supported")
	}
	return BoxHeader{boxType, size, headerLen}, nil
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
		if err != nil {
			return nil, err
		}
		switch boxType {
		case "mdat":
			if f.isFragmented {
				if lastBoxType != "moof" {
					return nil, fmt.Errorf("Does not support %v between moof and mdat", lastBoxType)
				}
			}
		case "moof":
			moof := box.(*MoofBox)
			for _, traf := range moof.Trafs {
				if ok, parsed := traf.ContainsSencBox(); ok && !parsed {
					defaultIVSize := byte(0) // Should get this from tenc in sinf
					if f.Moov != nil {
						trackID := traf.Tfhd.TrackID
						sinf := f.Moov.GetSinf(trackID)
						if sinf != nil && sinf.Schi != nil && sinf.Schi.Tenc != nil {
							defaultIVSize = sinf.Schi.Tenc.DefaultPerSampleIVSize
						}
					}
					err = traf.ParseReadSenc(defaultIVSize, moof.StartPos)
					if err != nil {
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
