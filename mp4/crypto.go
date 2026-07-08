package mp4

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
	"encoding/hex"
	"fmt"

	"github.com/Eyevinn/mp4ff/av1"
	"github.com/Eyevinn/mp4ff/avc"
	"github.com/Eyevinn/mp4ff/hevc"
)

type cryptoDir int

const (
	minClearSize           = 96 // to generate same output as Bento4
	naluHdrLen             = 4
	dirEnc       cryptoDir = iota
	dirDec
)

// GetAVCProtectRanges for common encryption from a sample with 4-byte NALU lengths.
// THe spsMap and ppsMap are only needed for CBCS mode.
// For scheme cenc, protection ranges must be a multiple of 16 bytes leaving header and some more in the clear
// For scheme cbcs, protection range must start after the slice header.
func GetAVCProtectRanges(spsMap map[uint32]*avc.SPS, ppsMap map[uint32]*avc.PPS, sample []byte,
	scheme string) ([]SubSamplePattern, error) {
	var ssps []SubSamplePattern
	length := len(sample)
	if length < 4 {
		return nil, fmt.Errorf("less than 4 bytes, No NALUs")
	}
	var pos uint32 = 0
	clearStart := uint32(0)
	clearEnd := uint32(0)
	for pos < uint32(length-4) {
		naluLength := binary.BigEndian.Uint32(sample[pos : pos+4])
		pos += 4
		if int(pos+naluLength) > len(sample) {
			return nil, fmt.Errorf("NALU length fields are bad")
		}
		naluType := avc.GetNaluType(sample[pos])
		var bytesToProtect uint32 = 0
		clearEnd = pos + naluLength
		if avc.IsVideoNaluType(naluType) {
			nalu := sample[pos : pos+naluLength]
			switch scheme {
			case "cenc":
				if naluLength+naluHdrLen >= minClearSize+16 {
					// Calculate a multiple of 16 bytes to protect
					bytesToProtect = (naluLength + naluHdrLen - minClearSize) & 0xfffffff0
					if bytesToProtect > 0 {
						clearEnd -= bytesToProtect
					}
				}
			case "cbcs":
				sh, err := avc.ParseSliceHeader(nalu, spsMap, ppsMap)
				if err != nil {
					return nil, err
				}
				clearHeadSize := uint32(sh.Size)
				clearEnd = pos + clearHeadSize
				bytesToProtect = naluLength - clearHeadSize
			default:
				return nil, fmt.Errorf("unknown protect scheme %s", scheme)
			}
		}
		if bytesToProtect > 0 {
			ssps = AppendProtectRange(ssps, clearEnd-clearStart, bytesToProtect)
			clearStart = clearEnd + bytesToProtect
			clearEnd = clearStart
		}

		pos += naluLength
	}
	if clearEnd > clearStart {
		ssps = AppendProtectRange(ssps, clearEnd-clearStart, 0)
	}
	if len(ssps) == 0 {
		// Degenerate sample (e.g. only a NALU length field): mark all
		// bytes clear so every video sample carries a subsample entry,
		// keeping senc and saiz consistent within the fragment.
		ssps = AppendProtectRange(ssps, uint32(length), 0)
	}
	return ssps, nil
}

func GetHEVCProtectRanges(spsMap map[uint32]*hevc.SPS, ppsMap map[uint32]*hevc.PPS,
	sample []byte, scheme string) ([]SubSamplePattern, error) {
	var ssps []SubSamplePattern
	length := len(sample)
	if length < 4 {
		return nil, fmt.Errorf("less than 4 bytes, No NALUs")
	}
	var pos uint32 = 0
	clearStart := uint32(0)
	clearEnd := uint32(0)
	for pos < uint32(length-4) {
		naluLength := binary.BigEndian.Uint32(sample[pos : pos+4])
		pos += 4
		if int(pos+naluLength) > len(sample) {
			return nil, fmt.Errorf("NALU length fields are bad")
		}
		naluType := hevc.GetNaluType(sample[pos])
		var bytesToProtect uint32 = 0
		clearEnd = pos + naluLength
		if hevc.IsVideoNaluType(naluType) {
			nalu := sample[pos : pos+naluLength]
			switch scheme {
			case "cenc":
				if naluLength+naluHdrLen >= minClearSize+16 {
					// Calculate a multiple of 16 bytes to protect
					bytesToProtect = (naluLength + naluHdrLen - minClearSize) & 0xfffffff0
					if bytesToProtect > 0 {
						clearEnd -= bytesToProtect
					}
				}
			case "cbcs":
				sh, err := hevc.ParseSliceHeader(nalu, spsMap, ppsMap)
				if err != nil {
					return nil, err
				}
				clearHeadSize := uint32(sh.Size)
				clearEnd = pos + clearHeadSize
				bytesToProtect = naluLength - clearHeadSize
			default:
				return nil, fmt.Errorf("unknown protect scheme %s", scheme)
			}
		}
		if bytesToProtect > 0 {
			ssps = AppendProtectRange(ssps, clearEnd-clearStart, bytesToProtect)
			clearStart = clearEnd + bytesToProtect
			clearEnd = clearStart
		}

		pos += naluLength
	}
	if clearEnd > clearStart {
		ssps = AppendProtectRange(ssps, clearEnd-clearStart, 0)
	}
	if len(ssps) == 0 {
		// Degenerate sample (e.g. only a NALU length field): mark all
		// bytes clear so every video sample carries a subsample entry,
		// keeping senc and saiz consistent within the fragment.
		ssps = AppendProtectRange(ssps, uint32(length), 0)
	}
	return ssps, nil
}

// GetAV1ProtectRanges computes common-encryption protection ranges for an AV1 sample,
// following the AV1 Codec ISO Media File Format Binding: only the tile data (decode_tile)
// of Frame and Tile Group OBUs is protected, while OBU headers, sequence/frame headers,
// tile-group headers and tile-size fields are left clear.
//
// For cenc, each tile's protected bytes are the leading complete 16-byte blocks and the
// trailing partial block is left clear (matching shaka-packager); tiles smaller than 16
// bytes are left entirely clear. For cbcs, the whole tile is protected and the pattern
// cipher skips the final partial block.
//
// dec must be fed samples in decode order because inter frames may inherit their size from
// reference frames; getAV1ProtFunc creates one decoder per track for this reason.
func GetAV1ProtectRanges(dec *av1.FrameHeaderDecoder, sample []byte, scheme string) ([]SubSamplePattern, error) {
	if scheme != "cenc" && scheme != "cbcs" {
		return nil, fmt.Errorf("unknown protect scheme %s", scheme)
	}
	tiles, err := dec.GetTileRanges(sample)
	if err != nil {
		return nil, fmt.Errorf("av1 tile ranges: %w", err)
	}
	// Accumulate clear bytes and emit a (clear, protected) subsample per tile, aligning the
	// protected data to whole 16-byte blocks for cenc as shaka-packager's SubsampleOrganizer does.
	var ssps []SubSamplePattern
	clearAccum := 0
	prevEnd := 0
	for _, t := range tiles {
		cipher := t.Length
		clearAtEnd := 0
		if scheme == "cenc" {
			clearAtEnd = cipher % 16
			cipher -= clearAtEnd
		}
		clearAccum += t.Offset - prevEnd // OBU/frame/tile-group headers and tile-size fields
		if cipher == 0 {
			clearAccum += clearAtEnd // tile too small to encrypt: keep it clear
		} else {
			ssps = AppendProtectRange(ssps, uint32(clearAccum), uint32(cipher))
			clearAccum = clearAtEnd
		}
		prevEnd = t.Offset + t.Length
	}
	clearAccum += len(sample) - prevEnd
	if clearAccum > 0 || len(ssps) == 0 {
		// A trailing clear range, or an all-clear sample (so every video sample carries a
		// subsample entry, keeping senc and saiz consistent).
		ssps = AppendProtectRange(ssps, uint32(clearAccum), 0)
	}
	return ssps, nil
}

// AppendProtectRange appends a SubSamplePattern to a slice of SubSamplePattern, splitting into multiple if needed.
func AppendProtectRange(ssps []SubSamplePattern, nrClear, nrProtected uint32) []SubSamplePattern {
	for nrClear >= 65536 {
		ssps = append(ssps, SubSamplePattern{65535, 0})
		nrClear -= 65535
	}
	ssps = append(ssps, SubSamplePattern{uint16(nrClear), nrProtected})
	return ssps
}

func getAudioProtectRanges(sample []byte, scheme string) ([]SubSamplePattern, error) {
	return nil, nil
}

// CryptSampleCenc encrypts/decrypts cenc-schema sample in place provided key, iv, and subSamplePatterns.
func CryptSampleCenc(sample []byte, key []byte, iv []byte, subSamplePatterns []SubSamplePattern) error {
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}

	stream := cipher.NewCTR(block, iv)
	if len(subSamplePatterns) != 0 {
		var pos uint32 = 0
		for j := 0; j < len(subSamplePatterns); j++ {
			ss := subSamplePatterns[j]
			nrClear := uint32(ss.BytesOfClearData)
			if nrClear > 0 {
				pos += nrClear
			}
			nrEnc := ss.BytesOfProtectedData
			if nrEnc > 0 {
				stream.XORKeyStream(sample[pos:pos+nrEnc], sample[pos:pos+nrEnc])
				pos += nrEnc
			}
		}
	} else {
		stream.XORKeyStream(sample, sample)
	}
	return nil
}

// DecryptSampleCenc does in-place decryption of cbcs-schema encrypted sample.
// Each protected byte range is striped with with pattern defined by pattern in tenc.
func DecryptSampleCbcs(sample []byte, key []byte, iv []byte, subSamplePatterns []SubSamplePattern, tenc *TencBox) error {
	return cryptSampleCbcs(dirDec, sample, key, iv, subSamplePatterns, tenc)
}

// EncryptSampleCenc does in-place encryption using cbcs schema.
// Each protected byte range is striped with with pattern defined by pattern in tenc.
func EncryptSampleCbcs(sample []byte, key []byte, iv []byte, subSamplePatterns []SubSamplePattern, tenc *TencBox) error {
	return cryptSampleCbcs(dirEnc, sample, key, iv, subSamplePatterns, tenc)
}

// cryptSampleCbcs does either encryption of decryption of a sample using cbcs scheme.
func cryptSampleCbcs(dir cryptoDir, sample []byte, key []byte, iv []byte, subSamplePatterns []SubSamplePattern, tenc *TencBox) error {
	nrInCryptBlock := int(tenc.DefaultCryptByteBlock) * 16
	nrInSkipBlock := int(tenc.DefaultSkipByteBlock) * 16
	var pos uint32 = 0
	if len(subSamplePatterns) != 0 {
		for j := 0; j < len(subSamplePatterns); j++ {
			ss := subSamplePatterns[j]
			nrClear := uint32(ss.BytesOfClearData)
			pos += nrClear
			if ss.BytesOfProtectedData > 0 {
				err := cbcsCrypt(dir, sample[pos:pos+ss.BytesOfProtectedData], key,
					iv, nrInCryptBlock, nrInSkipBlock)
				if err != nil {
					return err
				}
			}
			pos += ss.BytesOfProtectedData
		}
	} else { // Full encryption as used for audio
		err := cbcsCrypt(dir, sample, key, iv, nrInCryptBlock, nrInSkipBlock)
		if err != nil {
			return err
		}
	}
	return nil
}

// cbcsCrypt does one in-place CBC encryption/decryption. Full if nrInSkipBlock == 0.
// The normal case is that nrInCryptBlock == 16 and nrInSkipBlock == 144.
func cbcsCrypt(dir cryptoDir, data []byte, key []byte, iv []byte, nrInCryptBlock, nrInSkipBlock int) error {
	pos := 0
	size := len(data) // This is the bytes that we should stripe decrypt
	aesCbcCrypto, err := aes.NewCipher(key)
	if err != nil {
		return err
	}
	var cph cipher.BlockMode
	switch dir {
	case dirDec:
		cph = cipher.NewCBCDecrypter(aesCbcCrypto, iv)
	case dirEnc:
		cph = cipher.NewCBCEncrypter(aesCbcCrypto, iv)
	default:
		return fmt.Errorf("unknown crypto direction %d", dir)
	}

	if nrInSkipBlock == 0 {
		nrToCrypt := size & ^0xf // Drops 4 last bits -> multiple of 16
		cph.CryptBlocks(data[:nrToCrypt], data[:nrToCrypt])
		return nil
	}
	for size-pos >= nrInCryptBlock {
		cph.CryptBlocks(data[pos:pos+nrInCryptBlock], data[pos:pos+nrInCryptBlock])
		pos += nrInCryptBlock
		if size-pos < nrInSkipBlock {
			break
		}
		pos += nrInSkipBlock
	}
	return nil
}

// incrementIV increments the IV by the number of encrypted blocks and return a new IV.
func incrementIV(inIV []byte, subsamplePatterns []SubSamplePattern, sampleLen int) []byte {
	nrEncBlocks := 0
	if len(subsamplePatterns) == 0 {
		nrEncBlocks = (sampleLen + 15) / 16
	} else {
		for _, s := range subsamplePatterns {
			nrEncBlocks += int(s.BytesOfProtectedData / 16)
		}
	}
	iv := make([]byte, len(inIV))
	copy(iv, inIV)
	incrementIVInPlace(iv, nrEncBlocks)
	return iv
}

func incrementIVInPlace(iv []byte, nrSteps int) {
	rest := nrSteps
	for i := len(iv) - 1; i >= 0; i-- {
		sum := int(iv[i]) + rest
		if sum < 256 {
			iv[i] = byte(sum)
			break
		}
		iv[i] = byte(sum % 256)
		rest = sum / 256
	}
}

// sampleProtector computes the common-encryption protection ranges for each sample of one
// continuous decode sequence, fed in decode order. It may hold mutable state - AV1 accumulates
// reference-frame sizes across samples - so a protector must be used by a single goroutine and
// must not be reused across sequences. Obtain a fresh one per encryption run via
// InitProtectData.newProtector (see FragmentEncryptor).
type sampleProtector interface {
	protectRanges(sample []byte, scheme string) ([]SubSamplePattern, error)
}

// sampleProtectorFactory creates a fresh sampleProtector. It is immutable and safe to call
// concurrently; every returned protector is single-use.
type sampleProtectorFactory func() (sampleProtector, error)

// InitProtectData carries the immutable information needed to encrypt the fragments of a track
// once InitProtect (or ExtractInitProtectData) has prepared the init segment. It is safe to
// share between goroutines: all mutable, decode-order state lives in the FragmentEncryptor (and
// its sampleProtector) created for each encryption run, not here.
type InitProtectData struct {
	Tenc   *TencBox
	Trex   *TrexBox
	Scheme string
	// newProtector builds a fresh, single-use sampleProtector for one decode sequence.
	newProtector sampleProtectorFactory
}

// InitProtect modifies the init segment to add protection information and return what is needed to encrypt fragments.
func InitProtect(init *InitSegment, key, iv []byte, scheme string, kid UUID, psshBoxes []*PsshBox) (*InitProtectData, error) {
	ipd := InitProtectData{Scheme: scheme}
	moov := init.Moov
	if len(moov.Traks) != 1 {
		return nil, fmt.Errorf("only one track supported")
	}
	stsd := moov.Trak.Mdia.Minf.Stbl.Stsd
	if len(stsd.Children) != 1 {
		return nil, fmt.Errorf("only one stsd child supported")
	}

	inputIVSize := len(iv)
	if len(iv) == 8 {
		// Convert to 16 bytes for use as cbcs constant IV
		iv8 := iv
		iv = make([]byte, 16)
		copy(iv, iv8)
	}
	var err error
	ipd.Trex = moov.Mvex.Trex
	sinf := SinfBox{}
	var mediaType string
	switch se := stsd.Children[0].(type) {
	case *VisualSampleEntryBox:
		mediaType = "video"
		veType := se.Type()
		se.SetType("encv")
		frma := FrmaBox{DataFormat: veType}
		sinf.AddChild(&frma)
		se.AddChild(&sinf)
		switch veType {
		case "avc1", "avc3":
			ipd.newProtector, err = newAVCProtectorFactory(se.AvcC)
			if err != nil {
				return nil, fmt.Errorf("get avc protector: %w", err)
			}
		case "hvc1", "hev1", "dvh1", "dvhe":
			ipd.newProtector, err = newHEVCProtectorFactory(se.HvcC)
			if err != nil {
				return nil, fmt.Errorf("get hevc protector: %w", err)
			}
		case "av01":
			ipd.newProtector, err = newAV1ProtectorFactory(se.Av1C)
			if err != nil {
				return nil, fmt.Errorf("get av1 protector: %w", err)
			}
		default:
			return nil, fmt.Errorf("visual sample entry type %s not yet supported", veType)
		}
	case *AudioSampleEntryBox:
		mediaType = "audio"
		aeType := se.Type()
		se.SetType("enca")
		frma := FrmaBox{DataFormat: aeType}
		sinf.AddChild(&frma)
		se.AddChild(&sinf)
		ipd.newProtector = audioProtectorFactory
	default:
		return nil, fmt.Errorf("sample entry type %s should not be encrypted", se.Type())
	}
	schi := SchiBox{}
	switch scheme {
	case "cenc":
		// The per-sample IV size follows the provided iv. CMAF
		// (ISO/IEC 23000-19 Section 8.2.3.1) requires 8-byte IVs for
		// the cenc scheme, so an 8-byte iv is the conformant choice;
		// 16-byte IVs are kept for backwards compatibility.
		var perSampleIVSize byte
		switch inputIVSize {
		case 8:
			perSampleIVSize = 8
		case 16:
			perSampleIVSize = 16
		default:
			return nil, fmt.Errorf("cenc iv must be 8 or 16 bytes, got %d", inputIVSize)
		}
		ipd.Tenc = &TencBox{Version: 0, DefaultIsProtected: 1, DefaultPerSampleIVSize: perSampleIVSize, DefaultKID: kid}
		sinf.AddChild(&SchmBox{SchemeType: "cenc", SchemeVersion: 65536})
	case "cbcs":
		switch mediaType {
		case "video":
			ipd.Tenc = &TencBox{Version: 1, DefaultCryptByteBlock: 1, DefaultSkipByteBlock: 9,
				DefaultIsProtected: 1, DefaultPerSampleIVSize: 0, DefaultKID: kid,
				DefaultConstantIV: iv}
		case "audio":
			ipd.Tenc = &TencBox{Version: 1, DefaultCryptByteBlock: 0, DefaultSkipByteBlock: 0,
				DefaultIsProtected: 1, DefaultPerSampleIVSize: 0, DefaultKID: kid,
				DefaultConstantIV: iv}
		}
		sinf.AddChild(&SchmBox{SchemeType: "cbcs", SchemeVersion: 65536})
	default:
		return nil, fmt.Errorf("unknown protection scheme %s", scheme)
	}
	schi.AddChild(ipd.Tenc)
	sinf.AddChild(&schi)
	for _, pssh := range psshBoxes {
		init.Moov.AddChild(pssh)
	}
	return &ipd, nil
}

func getAVCPSMaps(spss [][]byte, ppss [][]byte) (map[uint32]*avc.SPS, map[uint32]*avc.PPS, error) {
	spsMap := make(map[uint32]*avc.SPS, 1)
	for _, spsNalu := range spss {
		sps, err := avc.ParseSPSNALUnit(spsNalu, false)
		if err != nil {
			return nil, nil, err
		}
		spsMap[sps.ParameterID] = sps
	}
	ppsMap := make(map[uint32]*avc.PPS, 1)
	for _, ppsNalu := range ppss {
		pps, err := avc.ParsePPSNALUnit(ppsNalu, spsMap)
		if err != nil {
			return nil, nil, err
		}
		ppsMap[pps.PicParameterSetID] = pps
	}
	return spsMap, ppsMap, nil
}

// funcProtector adapts a stateless protection-range function to the sampleProtector interface.
// It carries no mutable state, so a single instance is safe to reuse and share.
type funcProtector func(sample []byte, scheme string) ([]SubSamplePattern, error)

func (f funcProtector) protectRanges(sample []byte, scheme string) ([]SubSamplePattern, error) {
	return f(sample, scheme)
}

// statelessFactory returns a factory that always hands back the same stateless protector.
func statelessFactory(p sampleProtector) sampleProtectorFactory {
	return func() (sampleProtector, error) { return p, nil }
}

// audioProtectorFactory protects full audio samples; it is stateless and codec-independent.
var audioProtectorFactory = statelessFactory(funcProtector(getAudioProtectRanges))

func newAVCProtectorFactory(avcC *AvcCBox) (sampleProtectorFactory, error) {
	spsMap, ppsMap, err := getAVCPSMaps(avcC.SPSnalus, avcC.PPSnalus)
	if err != nil {
		return nil, fmt.Errorf("get avc ps maps: %w", err)
	}
	// AVC subsample ranges are computed per sample from immutable parameter sets, so the
	// protector is stateless and can be shared across sequences and goroutines.
	p := funcProtector(func(sample []byte, scheme string) ([]SubSamplePattern, error) {
		return GetAVCProtectRanges(spsMap, ppsMap, sample, scheme)
	})
	return statelessFactory(p), nil
}

func getHEVCPSMaps(arrays []hevc.NaluArray) (map[uint32]*hevc.SPS, map[uint32]*hevc.PPS, error) {
	// First check SPS
	spsMap := make(map[uint32]*hevc.SPS, 1)
	for _, naluArray := range arrays {
		naluType := naluArray.NaluType()
		switch naluType {
		case hevc.NALU_SPS:
			for _, nalu := range naluArray.Nalus {
				sps, err := hevc.ParseSPSNALUnit(nalu)
				if err != nil {
					return nil, nil, err
				}
				spsMap[uint32(sps.SpsID)] = sps
			}
		}
	}
	// Then check PPS
	ppsMap := make(map[uint32]*hevc.PPS, 1)
	for _, naluArray := range arrays {
		naluType := naluArray.NaluType()
		switch naluType {
		case hevc.NALU_PPS:
			for _, nalu := range naluArray.Nalus {
				pps, err := hevc.ParsePPSNALUnit(nalu, spsMap)
				if err != nil {
					return nil, nil, err
				}
				ppsMap[pps.PicParameterSetID] = pps
			}
		}
	}
	return spsMap, ppsMap, nil
}

func newHEVCProtectorFactory(hvcC *HvcCBox) (sampleProtectorFactory, error) {
	spsMap, ppsMap, err := getHEVCPSMaps(hvcC.NaluArrays)
	if err != nil {
		return nil, fmt.Errorf("get hevc ps maps: %w", err)
	}
	// Like AVC, HEVC ranges come from immutable parameter sets - parse them once and share a
	// single stateless protector (previously they were re-parsed for every sample).
	p := funcProtector(func(sample []byte, scheme string) ([]SubSamplePattern, error) {
		return GetHEVCProtectRanges(spsMap, ppsMap, sample, scheme)
	})
	return statelessFactory(p), nil
}

// av1Protector holds the mutable AV1 frame-header decoder for one decode sequence. Its
// reference-frame state accumulates across the samples of the sequence (in decode order),
// so it must not be shared between goroutines or reused across sequences.
type av1Protector struct {
	dec *av1.FrameHeaderDecoder
}

func (p *av1Protector) protectRanges(sample []byte, scheme string) ([]SubSamplePattern, error) {
	return GetAV1ProtectRanges(p.dec, sample, scheme)
}

func newAV1ProtectorFactory(av1C *Av1CBox) (sampleProtectorFactory, error) {
	seqHdr, err := av1C.SequenceHeader()
	if err != nil {
		return nil, fmt.Errorf("av1 sequence header: %w", err)
	}
	// Validate the sequence header up front so InitProtect fails fast. The header is immutable
	// and shared, but each sequence gets its own decoder so reference-frame state never leaks
	// between segments or concurrent encryption runs.
	if _, err := av1.NewFrameHeaderDecoder(seqHdr); err != nil {
		return nil, err
	}
	return func() (sampleProtector, error) {
		dec, err := av1.NewFrameHeaderDecoder(seqHdr)
		if err != nil {
			return nil, err
		}
		return &av1Protector{dec: dec}, nil
	}, nil
}

// FragmentEncryptor encrypts the fragments of one continuous decode sequence (a CMAF segment, or
// any independently decodable run that begins at a random-access point). It owns the mutable,
// decode-order state - most importantly the AV1 reference-frame decoder - so it must be used by a
// single goroutine, and only for the fragments of one sequence, fed in decode order. Create a new
// FragmentEncryptor for each sequence and for each concurrent request.
type FragmentEncryptor struct {
	ipd     *InitProtectData
	key     []byte
	iv      []byte
	iv8Mode bool
	prot    sampleProtector
}

// NewFragmentEncryptor validates the key/iv against the scheme and builds a fresh sample protector
// for one decode sequence. The iv is copied, so the caller may reuse its buffer afterwards.
func (ipd *InitProtectData) NewFragmentEncryptor(key, iv []byte) (*FragmentEncryptor, error) {
	if ipd == nil {
		return nil, fmt.Errorf("no protection data")
	}
	// cenc with 8-byte per-sample IVs (the size CMAF requires): the 8-byte IV is the high half of
	// the 16-byte CTR counter, the low half starts at zero for each sample, and the IV increments
	// by one per sample.
	iv8Mode := ipd.Scheme == "cenc" && ipd.Tenc != nil && ipd.Tenc.DefaultPerSampleIVSize == 8
	if iv8Mode {
		if len(iv) != 8 {
			return nil, fmt.Errorf("cenc with 8-byte per-sample IVs needs an 8-byte iv, got %d bytes", len(iv))
		}
	} else {
		if len(iv) == 8 {
			// Convert to 16 bytes
			iv8 := iv
			iv = make([]byte, 16)
			copy(iv, iv8)
		}
		if len(iv) != 16 {
			return nil, fmt.Errorf("iv must be 16 bytes")
		}
	}
	if ipd.newProtector == nil {
		return nil, fmt.Errorf("no sample protector for scheme %s", ipd.Scheme)
	}
	prot, err := ipd.newProtector()
	if err != nil {
		return nil, fmt.Errorf("new sample protector: %w", err)
	}
	ivCopy := make([]byte, len(iv))
	copy(ivCopy, iv)
	return &FragmentEncryptor{ipd: ipd, key: key, iv: ivCopy, iv8Mode: iv8Mode, prot: prot}, nil
}

// IV returns the next initialization vector. For cenc it advances as fragments are encrypted, so
// it can be chained into the next sequence; for cbcs it is the constant IV.
func (e *FragmentEncryptor) IV() []byte {
	return e.iv
}

// EncryptFragment encrypts one fragment in place and advances the internal IV. Call it once per
// fragment of the sequence, in decode order.
func (e *FragmentEncryptor) EncryptFragment(f *Fragment) error {
	ipd := e.ipd
	iv := e.iv
	if len(f.Moof.Trafs) != 1 {
		return fmt.Errorf("only one traf supported")
	}
	traf := f.Moof.Traf
	if len(traf.Truns) != 1 {
		return fmt.Errorf("only one trun supported")
	}
	nrSamples := int(f.Moof.Traf.Trun.SampleCount())
	saiz := NewSaizBox(nrSamples)
	_ = traf.AddChild(saiz)
	saio := NewSaioBox()
	_ = traf.AddChild(saio)
	var senc *SencBox
	switch ipd.Scheme {
	case "cenc":
		senc = NewSencBox(nrSamples, nrSamples)
	case "cbcs":
		senc = NewSencBox(0, nrSamples)
	default:
		return fmt.Errorf("unknown scheme %s", ipd.Scheme)
	}
	_ = traf.AddChild(senc)
	fss, err := f.GetFullSamples(ipd.Trex)
	if err != nil {
		return fmt.Errorf("get full samples: %w", err)
	}

	for _, fs := range fss {
		sample := fs.Data
		subsamplePatterns, err := e.prot.protectRanges(sample, ipd.Scheme)
		if err != nil {
			return fmt.Errorf("get protect ranges: %w", err)
		}
		switch ipd.Scheme {
		case "cenc":
			ctrIV := iv
			if e.iv8Mode {
				ctrIV = make([]byte, 16)
				copy(ctrIV, iv)
			}
			if err := CryptSampleCenc(sample, e.key, ctrIV, subsamplePatterns); err != nil {
				return fmt.Errorf("crypt sample cenc: %w", err)
			}
			// Store IVs in the senc box and advance the IV for the next sample
			if err := senc.AddSample(SencSample{IV: iv, SubSamples: subsamplePatterns}); err != nil {
				return fmt.Errorf("senc add sample: %w", err)
			}
			if err := saiz.AddSampleInfo(iv, subsamplePatterns); err != nil {
				return fmt.Errorf("saiz add sample info: %w", err)
			}
			if e.iv8Mode {
				nextIV := make([]byte, 8)
				copy(nextIV, iv)
				incrementIVInPlace(nextIV, 1)
				iv = nextIV
			} else {
				iv = incrementIV(iv, subsamplePatterns, len(sample))
			}
		case "cbcs":
			if err := EncryptSampleCbcs(sample, e.key, iv, subsamplePatterns, ipd.Tenc); err != nil {
				return fmt.Errorf("crypt sample cbcs: %w", err)
			}
			// iv is constant and not sent to senc
			if err := senc.AddSample(SencSample{IV: nil, SubSamples: subsamplePatterns}); err != nil {
				return fmt.Errorf("senc add sample: %w", err)
			}
			if err := saiz.AddSampleInfo(nil, subsamplePatterns); err != nil {
				return fmt.Errorf("saiz add sample info: %w", err)
			}
		default:
			return fmt.Errorf("unknown scheme %s", ipd.Scheme)
		}
	}
	e.iv = iv
	if len(senc.IVs) == 0 && len(senc.SubSamples) == 0 {
		// No sample auxiliary information (full-sample encryption with a constant IV): CMAF
		// (ISO/IEC 23000-19 Section 8.2.2.1) recommends omitting the senc, saiz, and saio boxes.
		_ = f.Moof.Traf.RemoveEncryptionBoxes()
		return nil
	}
	moof := f.Moof
	offset := uint64(8)
	sencDataOffset := uint64(0) // Offset to the senc box data to be set in saio
	for _, c := range moof.Children {
		if c.Type() != "traf" {
			offset += c.Size()
			continue
		}
		traf := c.(*TrafBox)
		offset += 8
		for _, tc := range traf.Children {
			if tc.Type() == "senc" {
				sencDataOffset = offset + 12 + 4 // 12 for full box and 4 for sample count
			}
			offset += tc.Size()
		}
		break
	}
	saio.Offset[0] = int64(sencDataOffset)
	return nil
}

// EncryptFragments encrypts the fragments of one continuous decode sequence in place, in decode
// order, sharing a single sample protector, and returns the next IV. The fragments must form one
// decodable run starting at a random-access point. This is the correct entry point for AV1, whose
// protection ranges depend on reference-frame state that spans the sequence's fragments.
func EncryptFragments(frags []*Fragment, key, iv []byte, ipd *InitProtectData) ([]byte, error) {
	enc, err := ipd.NewFragmentEncryptor(key, iv)
	if err != nil {
		return nil, err
	}
	for i, f := range frags {
		if err := enc.EncryptFragment(f); err != nil {
			return nil, fmt.Errorf("fragment %d: %w", i, err)
		}
	}
	return enc.IV(), nil
}

// EncryptFragment encrypts a single self-contained fragment (a one-fragment decode sequence that
// begins at a random-access point) in place and returns the next IV, which can be chained into the
// next call. For a multi-fragment decode sequence use EncryptFragments (or a FragmentEncryptor) so
// that AV1 reference-frame state is carried across the fragments rather than reset for each one.
func EncryptFragment(f *Fragment, key, iv []byte, ipd *InitProtectData) ([]byte, error) {
	return EncryptFragments([]*Fragment{f}, key, iv, ipd)
}

type DecryptInfo struct {
	Psshs      []*PsshBox
	TrackInfos []DecryptTrackInfo
}

type DecryptTrackInfo struct {
	TrackID uint32
	Sinf    *SinfBox
	Trex    *TrexBox
	Psshs   []*PsshBox
}

func (d DecryptInfo) findTrackInfo(trackID uint32) DecryptTrackInfo {
	for _, ti := range d.TrackInfos {
		if ti.TrackID == trackID {
			return ti
		}
	}
	return DecryptTrackInfo{}
}

// normalizePiffScheme rewrites a PIFF-style sinf so it looks like cenc/cbcs to
// the rest of the decryption pipeline. Per PIFF 1.1 §5.3.3, the PIFF
// TrackEncryptionBox (UUID 8974dbce-7be7-4c51-84f9-7148f9882554) carries
// default_AlgorithmID, default_IV_size and default_KID. AlgorithmID values are
// listed in PIFF 1.1 §5.3.2: 1=AES 128-bit CTR (equivalent to cenc),
// 2=AES 128-bit CBC (equivalent to cbcs).
func normalizePiffScheme(sinf *SinfBox) error {
	if sinf == nil || sinf.Schi == nil || sinf.Schi.Tenc == nil {
		return fmt.Errorf("piff scheme without piff tenc uuid")
	}
	var algorithmID uint32
	for _, c := range sinf.Schi.Children {
		if u, ok := c.(*UUIDBox); ok && u.SubType() == "piff-tenc" {
			algorithmID = u.PiffTenc.AlgorithmID
			break
		}
	}
	switch algorithmID {
	case 1:
		sinf.Schm.SchemeType = "cenc"
	case 2:
		sinf.Schm.SchemeType = "cbcs"
	default:
		return fmt.Errorf("piff unsupported algorithmID %d", algorithmID)
	}
	return nil
}

// DecryptInit modifies init segment in place and returns decryption info and a clean init segment.
func DecryptInit(init *InitSegment) (DecryptInfo, error) {
	moov := init.Moov
	di := DecryptInfo{
		TrackInfos: make([]DecryptTrackInfo, 0, len(moov.Traks)),
	}
	for _, trak := range moov.Traks {
		trackID := trak.Tkhd.TrackID
		stsd := trak.Mdia.Minf.Stbl.Stsd
		var encv *VisualSampleEntryBox
		var enca *AudioSampleEntryBox
		var schemeType string
		var err error

		for _, child := range stsd.Children {
			var sinf *SinfBox
			switch child.Type() {
			case "encv":
				encv = child.(*VisualSampleEntryBox)
				sinf, err = encv.RemoveEncryption()
				if err != nil {
					return di, err
				}
				schemeType = sinf.Schm.SchemeType
			case "enca":
				enca = child.(*AudioSampleEntryBox)
				sinf, err = enca.RemoveEncryption()
				if err != nil {
					return di, err
				}
				schemeType = sinf.Schm.SchemeType
			default:
				continue
			}
			if schemeType == "piff" {
				if err := normalizePiffScheme(sinf); err != nil {
					return di, err
				}
				schemeType = sinf.Schm.SchemeType
			}
			di.TrackInfos = append(di.TrackInfos, DecryptTrackInfo{
				TrackID: trackID,
				Sinf:    sinf,
			})
		}
		if schemeType != "" && schemeType != "cenc" && schemeType != "cbcs" {
			return di, fmt.Errorf("scheme type %s not supported", schemeType)
		}
		if schemeType == "" {
			// Should be track in the clear
			di.TrackInfos = append(di.TrackInfos, DecryptTrackInfo{
				TrackID: trackID,
				Sinf:    nil,
			})
		}
	}

	for _, trex := range moov.Mvex.Trexs {
		for i := range di.TrackInfos {
			if di.TrackInfos[i].TrackID == trex.TrackID {
				di.TrackInfos[i].Trex = trex
				break
			}
		}
	}
	di.Psshs = moov.RemovePsshs()
	return di, nil
}

// DecryptSegment decrypts a media segment in place
func DecryptSegment(seg *MediaSegment, di DecryptInfo, key []byte) error {
	return DecryptSegmentWithKeys(seg, di, key, nil, false)
}

// DecryptSegmentWithKeys decrypts a media segment in place using either a legacy key
// or keys selected by KID. KID values are expected as 32-char lowercase hex without dashes.
// If strictKIDMode is true, encrypted tracks must have a matching key in keysByKID.
func DecryptSegmentWithKeys(seg *MediaSegment, di DecryptInfo, key []byte, keysByKID map[string][]byte, strictKIDMode bool) error {
	for _, frag := range seg.Fragments {
		for _, traf := range frag.Moof.Trafs {
			hasSenc, _ := traf.ContainsSencBox()
			if hasSenc {
				ti := di.findTrackInfo(traf.Tfhd.TrackID)
				if ti.Sinf == nil {
					return fmt.Errorf("no decrypt info for trackID=%d which has senc box", traf.Tfhd.TrackID)
				}
			}
		}
	}
	for _, frag := range seg.Fragments {
		err := DecryptFragmentWithKeys(frag, di, key, keysByKID, strictKIDMode)
		if err != nil {
			return err
		}
	}
	if len(seg.Sidxs) > 0 {
		seg.Sidx = nil // drop sidx inside segment, since not modified properly
		seg.Sidxs = nil
	}
	return nil
}

// DecryptFragment decrypts a fragment in place
func DecryptFragment(frag *Fragment, di DecryptInfo, key []byte) error {
	return DecryptFragmentWithKeys(frag, di, key, nil, false)
}

func getTrackKIDHex(ti DecryptTrackInfo) (string, error) {
	if ti.Sinf == nil || ti.Sinf.Schi == nil || ti.Sinf.Schi.Tenc == nil {
		return "", fmt.Errorf("missing tenc for trackID=%d", ti.TrackID)
	}
	kid := ti.Sinf.Schi.Tenc.DefaultKID
	if len(kid) != 16 {
		return "", fmt.Errorf("bad kid length %d for trackID=%d", len(kid), ti.TrackID)
	}
	return hex.EncodeToString(kid), nil
}

func getTrackKey(ti DecryptTrackInfo, key []byte, keysByKID map[string][]byte, strictKIDMode bool) ([]byte, error) {
	if len(keysByKID) == 0 {
		return key, nil
	}
	kidHex, err := getTrackKIDHex(ti)
	if err != nil {
		return nil, err
	}
	mappedKey, ok := keysByKID[kidHex]
	if !ok {
		if strictKIDMode {
			return nil, fmt.Errorf("requested key was not found for kid=%s", kidHex)
		}
		return key, nil
	}
	return mappedKey, nil
}

// DecryptFragmentWithKeys decrypts a fragment in place using either a legacy key
// or keys selected by KID. KID values are expected as 32-char lowercase hex without dashes.
// If strictKIDMode is true, encrypted tracks must have a matching key in keysByKID.
func DecryptFragmentWithKeys(frag *Fragment, di DecryptInfo, key []byte, keysByKID map[string][]byte, strictKIDMode bool) error {
	moof := frag.Moof
	var nrBytesRemoved uint64 = 0
	for _, traf := range moof.Trafs {
		ti := di.findTrackInfo(traf.Tfhd.TrackID)
		if ti.Sinf != nil {
			schemeType := ti.Sinf.Schm.SchemeType
			if schemeType != "cenc" && schemeType != "cbcs" {
				return fmt.Errorf("scheme type %s not supported", schemeType)
			}
			tenc := ti.Sinf.Schi.Tenc
			hasSenc, isParsed := traf.ContainsSencBox()
			if !hasSenc {
				// A missing senc is fine for full-sample encryption with a
				// constant IV (no sample auxiliary information); CMAF
				// (ISO/IEC 23000-19 Section 8.2.2.1) recommends omitting
				// senc, saiz, and saio in that case.
				if tenc == nil || tenc.DefaultPerSampleIVSize != 0 || len(tenc.DefaultConstantIV) == 0 {
					return fmt.Errorf("no senc box in traf")
				}
			}
			if hasSenc && !isParsed {
				defaultPerSampleIVSize := ti.Sinf.Schi.Tenc.DefaultPerSampleIVSize
				err := traf.ParseReadSenc(defaultPerSampleIVSize, moof.StartPos)
				if err != nil {
					return fmt.Errorf("parseReadSenc: %w", err)
				}
			}
			trackKey, err := getTrackKey(ti, key, keysByKID, strictKIDMode)
			if err != nil {
				return err
			}
			samples, err := frag.GetFullSamples(ti.Trex)
			if err != nil {
				return err
			}
			var senc *SencBox
			switch {
			case traf.Senc != nil:
				senc = traf.Senc
			case traf.UUIDSenc != nil:
				senc = traf.UUIDSenc.Senc
			}

			err = decryptSamplesInPlace(schemeType, samples, trackKey, tenc, senc)
			if err != nil {
				return err
			}
			nrBytesRemoved += traf.RemoveEncryptionBoxes()
		}
	}
	_, psshBytesRemoved := moof.RemovePsshs()
	nrBytesRemoved += psshBytesRemoved
	for _, traf := range moof.Trafs {
		for _, trun := range traf.Truns {
			trun.DataOffset -= int32(nrBytesRemoved)
		}
	}
	if frag.Mdat.StartPos > frag.Moof.StartPos {
		frag.Mdat.StartPos -= nrBytesRemoved
	}

	return nil
}

// decryptSample - decrypt samples inplace
func decryptSamplesInPlace(schemeType string, samples []FullSample, key []byte, tenc *TencBox, senc *SencBox) error {

	// TODO. Interpret saio and saiz to get to the right place
	// Saio tells where the IV starts relative to moof start
	// It typically ends up inside senc (16 bytes after start)

	iv := make([]byte, 16)
	if tenc.DefaultConstantIV != nil {
		copy(iv, tenc.DefaultConstantIV)
	}

	for i := range samples {
		if senc != nil && len(senc.IVs) == len(samples) {
			if len(senc.IVs[i]) < 16 {
				for i := 0; i < 16; i++ {
					iv[i] = 0
				}
			}
			copy(iv, senc.IVs[i])
		}
		if len(iv) == 0 {
			return fmt.Errorf("iv has length 0")
		}

		var subSamplePatterns []SubSamplePattern
		if senc != nil && len(senc.SubSamples) != 0 {
			subSamplePatterns = senc.SubSamples[i]
		}
		switch schemeType {
		case "cenc":
			err := CryptSampleCenc(samples[i].Data, key, iv, subSamplePatterns)
			if err != nil {
				return err
			}
		case "cbcs":
			err := DecryptSampleCbcs(samples[i].Data, key, iv, subSamplePatterns, tenc)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// ExtractInitProtectData extracts protection data from init segment
func ExtractInitProtectData(inSeg *InitSegment) (*InitProtectData, error) {
	if len(inSeg.Moov.Traks) != 1 {
		return nil, fmt.Errorf("only one track supported")
	}
	ipd := InitProtectData{}
	ipd.Trex = inSeg.Moov.Mvex.Trex
	stsd := inSeg.Moov.Trak.Mdia.Minf.Stbl.Stsd
	var sinf *SinfBox
	var err error
	for _, c := range stsd.Children {
		switch box := c.(type) {
		case *VisualSampleEntryBox:
			sinf = box.Sinf
			frma := sinf.Frma
			switch frma.DataFormat {
			case "avc1":
				ipd.newProtector, err = newAVCProtectorFactory(box.AvcC)
				if err != nil {
					return nil, fmt.Errorf("get AVC protector: %w", err)
				}
			case "hvc1", "dvh1", "dvhe":
				ipd.newProtector, err = newHEVCProtectorFactory(box.HvcC)
				if err != nil {
					return nil, fmt.Errorf("get HEVC protector: %w", err)
				}
			case "av01":
				ipd.newProtector, err = newAV1ProtectorFactory(box.Av1C)
				if err != nil {
					return nil, fmt.Errorf("get AV1 protector: %w", err)
				}
			default:
				return nil, fmt.Errorf("unsupported video codec descriptor %s", frma.DataFormat)
			}
		case *AudioSampleEntryBox:
			sinf = box.Sinf
			ipd.newProtector = audioProtectorFactory
		default:
			continue
		}
		for _, c2 := range sinf.Children {
			switch box := c2.(type) {
			case *SchmBox:
				ipd.Scheme = box.SchemeType
			case *SchiBox:
				ipd.Tenc = box.Tenc
			default:
				continue
			}
		}
	}

	return &ipd, nil
}
