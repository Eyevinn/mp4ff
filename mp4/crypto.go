package mp4

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"maps"
	"slices"

	"github.com/Eyevinn/mp4ff/av1"
	"github.com/Eyevinn/mp4ff/avc"
	"github.com/Eyevinn/mp4ff/hevc"
	"github.com/Eyevinn/mp4ff/vp9"
)

type cryptoDir int

const (
	minClearSize           = 96 // to generate same output as Bento4
	naluHdrLen             = 4
	dirEnc       cryptoDir = iota
	dirDec
)

// GetAVCProtectRanges for common encryption from a sample with 4-byte NALU lengths.
// The spsMap and ppsMap are only needed for cbcs.
// For scheme cenc, protection ranges must be a multiple of 16 bytes leaving header and some more in the clear
// For scheme cbcs, protection range must start after the slice header. An SPS or PPS in the sample replaces the one
// with the same ID for the slices after it; spsMap and ppsMap are not changed.
func GetAVCProtectRanges(spsMap map[uint32]*avc.SPS, ppsMap map[uint32]*avc.PPS, sample []byte,
	scheme string) ([]SubSamplePattern, error) {
	ps := parameterSets[avc.SPS, avc.PPS]{sps: spsMap, pps: ppsMap, parser: avcParameterSetParser}
	return appendAVCProtectRanges(nil, &ps, sample, scheme)
}

// parameterSets are the SPSs and PPSs that slice headers are parsed with: those of the decoder configuration
// record, each replaced by an SPS or PPS with the same ID that a sample carries. The maps it starts from may be
// shared, so the first replacement copies them.
type parameterSets[S, P any] struct {
	sps map[uint32]*S
	pps map[uint32]*P
	// spsNalus and ppsNalus hold the NAL units of sps and pps, so that a repeated one is recognized without parsing
	spsNalus map[uint32][]byte
	ppsNalus map[uint32][]byte
	owned    bool
	parser   *parameterSetParser[S, P]
}

// parameterSetParser parses the SPS and PPS NAL units of a codec, returning each with its ID.
type parameterSetParser[S, P any] struct {
	sps func(nalu []byte) (*S, uint32, error)
	pps func(nalu []byte, spsMap map[uint32]*S) (*P, uint32, error)
}

var avcParameterSetParser = &parameterSetParser[avc.SPS, avc.PPS]{
	sps: func(nalu []byte) (*avc.SPS, uint32, error) {
		sps, err := avc.ParseSPSNALUnit(nalu, false)
		if err != nil {
			return nil, 0, err
		}
		return sps, sps.ParameterID, nil
	},
	pps: func(nalu []byte, spsMap map[uint32]*avc.SPS) (*avc.PPS, uint32, error) {
		pps, err := avc.ParsePPSNALUnit(nalu, spsMap)
		if err != nil {
			return nil, 0, err
		}
		return pps, pps.PicParameterSetID, nil
	},
}

var hevcParameterSetParser = &parameterSetParser[hevc.SPS, hevc.PPS]{
	sps: func(nalu []byte) (*hevc.SPS, uint32, error) {
		sps, err := hevc.ParseSPSNALUnit(nalu)
		if err != nil {
			return nil, 0, err
		}
		return sps, uint32(sps.SpsID), nil
	},
	pps: func(nalu []byte, spsMap map[uint32]*hevc.SPS) (*hevc.PPS, uint32, error) {
		pps, err := hevc.ParsePPSNALUnit(nalu, spsMap)
		if err != nil {
			return nil, 0, err
		}
		return pps, pps.PicParameterSetID, nil
	},
}

// newParameterSets parses the SPS and PPS NAL units of a decoder configuration record.
func newParameterSets[S, P any](parser *parameterSetParser[S, P], spss, ppss [][]byte) (parameterSets[S, P], error) {
	ps := parameterSets[S, P]{
		sps:      make(map[uint32]*S, len(spss)),
		pps:      make(map[uint32]*P, len(ppss)),
		spsNalus: make(map[uint32][]byte, len(spss)),
		ppsNalus: make(map[uint32][]byte, len(ppss)),
		owned:    true,
		parser:   parser,
	}
	for _, nalu := range spss {
		if err := ps.addSPS(nalu); err != nil {
			return parameterSets[S, P]{}, err
		}
	}
	for _, nalu := range ppss {
		if err := ps.addPPS(nalu); err != nil {
			return parameterSets[S, P]{}, err
		}
	}
	return ps, nil
}

// takeSPS takes an SPS NAL unit of a sample into ps, unless it repeats the current one.
func (ps *parameterSets[S, P]) takeSPS(nalu []byte) error {
	if containsNalu(ps.spsNalus, nalu) {
		return nil
	}
	ps.own()
	return ps.addSPS(nalu)
}

// takePPS takes a PPS NAL unit of a sample into ps, unless it repeats the current one.
func (ps *parameterSets[S, P]) takePPS(nalu []byte) error {
	if containsNalu(ps.ppsNalus, nalu) {
		return nil
	}
	ps.own()
	return ps.addPPS(nalu)
}

func (ps *parameterSets[S, P]) addSPS(nalu []byte) error {
	sps, id, err := ps.parser.sps(nalu)
	if err != nil {
		return fmt.Errorf("sps: %w", err)
	}
	ps.sps[id] = sps
	ps.spsNalus[id] = bytes.Clone(nalu)
	return nil
}

func (ps *parameterSets[S, P]) addPPS(nalu []byte) error {
	pps, id, err := ps.parser.pps(nalu, ps.sps)
	if err != nil {
		return fmt.Errorf("pps: %w", err)
	}
	ps.pps[id] = pps
	ps.ppsNalus[id] = bytes.Clone(nalu)
	return nil
}

// own makes the maps of ps its own copies, so that changing them leaves the ones it started from unchanged.
func (ps *parameterSets[S, P]) own() {
	if ps.owned {
		return
	}
	ps.sps = cloneMap(ps.sps)
	ps.pps = cloneMap(ps.pps)
	ps.spsNalus = cloneMap(ps.spsNalus)
	ps.ppsNalus = cloneMap(ps.ppsNalus)
	ps.owned = true
}

// cloneMap returns a copy of m, which is non-nil also when m is nil.
func cloneMap[K comparable, V any](m map[K]V) map[K]V {
	c := make(map[K]V, len(m)+1)
	maps.Copy(c, m)
	return c
}

func containsNalu(nalus map[uint32][]byte, nalu []byte) bool {
	for _, n := range nalus {
		if bytes.Equal(n, nalu) {
			return true
		}
	}
	return false
}

// appendAVCProtectRanges is GetAVCProtectRanges appending to ssps, with the parameter sets in ps, which takes in the
// SPSs and PPSs of the sample for cbcs.
func appendAVCProtectRanges(ssps []SubSamplePattern, ps *parameterSets[avc.SPS, avc.PPS], sample []byte,
	scheme string) ([]SubSamplePattern, error) {
	origLen := len(ssps)
	length := len(sample)
	if length < 4 {
		return nil, fmt.Errorf("less than 4 bytes, No NALUs")
	}
	var pos uint32 = 0
	clearStart := uint32(0)
	clearEnd := uint32(0)
	for pos+4 < uint32(length) {
		naluLength := binary.BigEndian.Uint32(sample[pos : pos+4])
		pos += 4
		// uint64 arithmetic so that a length field close to 2^32 cannot wrap
		if uint64(pos)+uint64(naluLength) > uint64(len(sample)) {
			return nil, fmt.Errorf("NALU length fields are bad")
		}
		naluType := avc.GetNaluType(sample[pos])
		nalu := sample[pos : pos+naluLength]
		var bytesToProtect uint32 = 0
		clearEnd = pos + naluLength
		if scheme == "cbcs" && naluLength > 0 {
			var err error
			switch naluType {
			case avc.NALU_SPS:
				err = ps.takeSPS(nalu)
			case avc.NALU_PPS:
				err = ps.takePPS(nalu)
			}
			if err != nil {
				return nil, fmt.Errorf("parameter set in sample: %w", err)
			}
		}
		if avc.IsVideoNaluType(naluType) {
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
				shSize, err := avc.SliceHeaderSize(nalu, ps.sps, ps.pps)
				if err != nil {
					return nil, err
				}
				clearHeadSize := uint32(shSize)
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
	if len(ssps) == origLen {
		// Degenerate sample (e.g. only a NALU length field): mark all
		// bytes clear so every video sample carries a subsample entry,
		// keeping senc and saiz consistent within the fragment.
		ssps = AppendProtectRange(ssps, uint32(length), 0)
	}
	return ssps, nil
}

// GetHEVCProtectRanges for common encryption from a sample with 4-byte NALU lengths.
// The spsMap and ppsMap are only needed for cbcs, where the protection range must start after the slice header.
// An SPS or PPS in the sample replaces the one with the same ID for the slices after it; spsMap and ppsMap are not
// changed.
func GetHEVCProtectRanges(spsMap map[uint32]*hevc.SPS, ppsMap map[uint32]*hevc.PPS,
	sample []byte, scheme string) ([]SubSamplePattern, error) {
	ps := parameterSets[hevc.SPS, hevc.PPS]{sps: spsMap, pps: ppsMap, parser: hevcParameterSetParser}
	return appendHEVCProtectRanges(nil, &ps, sample, scheme)
}

// appendHEVCProtectRanges is GetHEVCProtectRanges appending to ssps, with the parameter sets in ps, which takes in
// the SPSs and PPSs of the sample for cbcs.
func appendHEVCProtectRanges(ssps []SubSamplePattern, ps *parameterSets[hevc.SPS, hevc.PPS], sample []byte,
	scheme string) ([]SubSamplePattern, error) {
	origLen := len(ssps)
	length := len(sample)
	if length < 4 {
		return nil, fmt.Errorf("less than 4 bytes, No NALUs")
	}
	var pos uint32 = 0
	clearStart := uint32(0)
	clearEnd := uint32(0)
	for pos+4 < uint32(length) {
		naluLength := binary.BigEndian.Uint32(sample[pos : pos+4])
		pos += 4
		// uint64 arithmetic so that a length field close to 2^32 cannot wrap
		if uint64(pos)+uint64(naluLength) > uint64(len(sample)) {
			return nil, fmt.Errorf("NALU length fields are bad")
		}
		naluType := hevc.GetNaluType(sample[pos])
		nalu := sample[pos : pos+naluLength]
		var bytesToProtect uint32 = 0
		clearEnd = pos + naluLength
		if scheme == "cbcs" && naluLength > 0 {
			var err error
			switch naluType {
			case hevc.NALU_SPS:
				err = ps.takeSPS(nalu)
			case hevc.NALU_PPS:
				err = ps.takePPS(nalu)
			}
			if err != nil {
				return nil, fmt.Errorf("parameter set in sample: %w", err)
			}
		}
		if hevc.IsVideoNaluType(naluType) {
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
				sh, err := hevc.ParseSliceHeader(nalu, ps.sps, ps.pps)
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
	if len(ssps) == origLen {
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
	return appendAV1ProtectRanges(nil, dec, sample, scheme)
}

// appendAV1ProtectRanges is GetAV1ProtectRanges appending to ssps.
func appendAV1ProtectRanges(ssps []SubSamplePattern, dec *av1.FrameHeaderDecoder, sample []byte, scheme string) (
	[]SubSamplePattern, error) {
	origLen := len(ssps)
	if scheme != "cenc" && scheme != "cbcs" {
		return nil, fmt.Errorf("unknown protect scheme %s", scheme)
	}
	tiles, err := dec.GetTileRanges(sample)
	if err != nil {
		return nil, fmt.Errorf("av1 tile ranges: %w", err)
	}
	// Accumulate clear bytes and emit a (clear, protected) subsample per tile, aligning the
	// protected data to whole 16-byte blocks for cenc as shaka-packager's SubsampleOrganizer does.
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
	if clearAccum > 0 || len(ssps) == origLen {
		// A trailing clear range, or an all-clear sample (so every video sample carries a
		// subsample entry, keeping senc and saiz consistent).
		ssps = AppendProtectRange(ssps, uint32(clearAccum), 0)
	}
	return ssps, nil
}

// vp9ClearPrefixBytes is the number of leading bytes of each VP9 frame that are left clear so that
// the uncompressed frame header stays readable.
const vp9ClearPrefixBytes = 96

// GetVP9ProtectRanges computes common-encryption protection ranges for a VP9 sample (a frame or a
// superframe). Only the cenc scheme is supported.
//
// Each frame's first 96 bytes are left clear, followed by a protected range of whole 16-byte
// blocks. Bytes that do not fill a whole block are added to that frame's clear prefix. This keeps
// every uncompressed frame header clear and each frame's protected data block-aligned, as required
// by the VP Codec ISO Media File Format Binding's Common Encryption section. A superframe index
// (VP9 Bitstream Specification Annex B) is left clear as a trailing subsample. A sample with no
// whole block to protect yields a single all-clear subsample.
//
// Each frame must start with a valid VP9 uncompressed frame header. Superframe sizes must account
// for all frame data before the index.
func GetVP9ProtectRanges(sample []byte, scheme string) ([]SubSamplePattern, error) {
	return appendVP9ProtectRanges(nil, sample, scheme)
}

// appendVP9ProtectRanges is GetVP9ProtectRanges appending to ssps.
func appendVP9ProtectRanges(ssps []SubSamplePattern, sample []byte, scheme string) ([]SubSamplePattern, error) {
	if scheme != "cenc" {
		return nil, fmt.Errorf("vp9 protection requires scheme cenc, got %s", scheme)
	}
	if len(sample) == 0 {
		return nil, fmt.Errorf("empty vp9 sample")
	}
	var frameSizes [8]int
	frameSizes[0] = len(sample)
	nrFrames := 1
	indexLen := vp9SuperframeIndexLen(sample)
	if indexLen > 0 {
		marker := sample[len(sample)-1]
		bytesPerFrameSize := int((marker>>3)&0x03) + 1
		nrFrames = int(marker&0x07) + 1
		indexPos := len(sample) - indexLen + 1
		remaining := len(sample) - indexLen
		for i := 0; i < nrFrames; i++ {
			var size uint32
			for j := 0; j < bytesPerFrameSize; j++ {
				size |= uint32(sample[indexPos]) << (8 * j)
				indexPos++
			}
			if size == 0 || uint64(size) > uint64(remaining) {
				return nil, fmt.Errorf("vp9 superframe frame %d: size %d exceeds remaining data %d or is zero",
					i, size, remaining)
			}
			frameSizes[i] = int(size)
			remaining -= int(size)
		}
		if remaining != 0 {
			return nil, fmt.Errorf("vp9 superframe sizes leave %d bytes before index", remaining)
		}
	}

	pos, clearBytes := 0, 0
	for _, size := range frameSizes[:nrFrames] {
		if _, err := vp9.ParseFrameHeader(sample[pos : pos+size]); err != nil {
			return nil, fmt.Errorf("vp9 frame header at offset %d: %w", pos, err)
		}
		protected := max(0, size-vp9ClearPrefixBytes) &^ 0x0f // whole 16-byte blocks
		clearBytes += size - protected
		if protected > 0 {
			ssps = AppendProtectRange(ssps, uint32(clearBytes), uint32(protected))
			clearBytes = 0
		}
		pos += size
	}
	clearBytes += indexLen
	if clearBytes > 0 {
		ssps = AppendProtectRange(ssps, uint32(clearBytes), 0)
	}
	return ssps, nil
}

// vp9SuperframeIndexLen returns the size of the superframe index at the end of sample, or 0 if
// there is none. The index starts and ends with the same marker byte
// 0b110[bytes_per_framesize_minus_1:2][frames_in_superframe_minus_1:3].
func vp9SuperframeIndexLen(sample []byte) int {
	if len(sample) < 2 {
		return 0
	}
	marker := sample[len(sample)-1]
	if marker&0xe0 != 0xc0 {
		return 0
	}
	bytesPerFrameSize := int((marker>>3)&0x03) + 1
	nrFrames := int(marker&0x07) + 1
	indexLen := 2 + nrFrames*bytesPerFrameSize
	if indexLen > len(sample) || sample[len(sample)-indexLen] != marker {
		return 0
	}
	return indexLen
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

// CryptSampleCenc encrypts/decrypts cenc-schema sample in place provided key, iv, and subSamplePatterns.
func CryptSampleCenc(sample []byte, key []byte, iv []byte, subSamplePatterns []SubSamplePattern) error {
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}
	return cryptSampleCencBlock(block, sample, iv, subSamplePatterns)
}

// cryptSampleCencBlock is CryptSampleCenc with an already-expanded AES block,
// so that a caller encrypting many samples under one key does not pay the
// key expansion (and its allocation) per sample.
func cryptSampleCencBlock(block cipher.Block, sample []byte, iv []byte, subSamplePatterns []SubSamplePattern) error {
	stream := cipher.NewCTR(block, iv)
	if len(subSamplePatterns) != 0 {
		var pos uint32 = 0
		for j := 0; j < len(subSamplePatterns); j++ {
			start, end, err := subSampleRange(len(sample), pos, j, subSamplePatterns[j])
			if err != nil {
				return err
			}
			if end > start {
				stream.XORKeyStream(sample[start:end], sample[start:end])
			}
			pos = end
		}
	} else {
		stream.XORKeyStream(sample, sample)
	}
	return nil
}

// subSampleRange returns the protected byte range [start, end) of the
// subsample pattern nr (zero-based) starting at pos in sample. An error is
// returned if the pattern reaches outside the sample, since the byte counts
// come from a senc box that nothing ties to the actual sample sizes.
func subSampleRange(sampleLen int, pos uint32, nr int, ss SubSamplePattern) (start, end uint32, err error) {
	// uint64 arithmetic so that big byte counts cannot wrap
	startPos := uint64(pos) + uint64(ss.BytesOfClearData)
	endPos := startPos + uint64(ss.BytesOfProtectedData)
	if endPos > uint64(sampleLen) {
		return 0, 0, fmt.Errorf("subsample %d range %d-%d is outside sample of size %d",
			nr+1, startPos, endPos, sampleLen)
	}
	return uint32(startPos), uint32(endPos), nil
}

// DecryptSampleCbcs does in-place decryption of cbcs-schema encrypted sample.
// Each protected byte range is striped with the pattern defined in tenc.
func DecryptSampleCbcs(sample []byte, key []byte, iv []byte, subSamplePatterns []SubSamplePattern, tenc *TencBox) error {
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}
	return cryptSampleCbcs(dirDec, sample, block, iv, subSamplePatterns, tenc)
}

// EncryptSampleCbcs does in-place encryption using cbcs schema.
// Each protected byte range is striped with the pattern defined in tenc.
func EncryptSampleCbcs(sample []byte, key []byte, iv []byte, subSamplePatterns []SubSamplePattern, tenc *TencBox) error {
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}
	return cryptSampleCbcs(dirEnc, sample, block, iv, subSamplePatterns, tenc)
}

// cryptSampleCbcs does either encryption of decryption of a sample using cbcs scheme.
// The block is the already-expanded AES cipher for the content key, so that a caller
// crypting many samples (or subsample ranges) under one key expands the key only once.
func cryptSampleCbcs(dir cryptoDir, sample []byte, block cipher.Block, iv []byte,
	subSamplePatterns []SubSamplePattern, tenc *TencBox) error {
	nrInCryptBlock := int(tenc.DefaultCryptByteBlock) * 16
	nrInSkipBlock := int(tenc.DefaultSkipByteBlock) * 16
	var pos uint32 = 0
	if len(subSamplePatterns) != 0 {
		for j := 0; j < len(subSamplePatterns); j++ {
			start, end, err := subSampleRange(len(sample), pos, j, subSamplePatterns[j])
			if err != nil {
				return err
			}
			if end > start {
				err = cbcsCrypt(dir, sample[start:end], block, iv, nrInCryptBlock, nrInSkipBlock)
				if err != nil {
					return err
				}
			}
			pos = end
		}
	} else { // Full encryption as used for audio
		err := cbcsCrypt(dir, sample, block, iv, nrInCryptBlock, nrInSkipBlock)
		if err != nil {
			return err
		}
	}
	return nil
}

// cbcsCrypt does one in-place CBC encryption/decryption. Full if nrInSkipBlock == 0.
// The normal case is that nrInCryptBlock == 16 and nrInSkipBlock == 144.
func cbcsCrypt(dir cryptoDir, data []byte, block cipher.Block, iv []byte, nrInCryptBlock, nrInSkipBlock int) error {
	var cph cipher.BlockMode
	switch dir {
	case dirDec:
		cph = cipher.NewCBCDecrypter(block, iv)
	case dirEnc:
		cph = cipher.NewCBCEncrypter(block, iv)
	default:
		return fmt.Errorf("unknown crypto direction %d", dir)
	}
	cbcsStripe(cph, data, nrInCryptBlock, nrInSkipBlock)
	return nil
}

// cbcMode is a CBC mode whose IV can be reset, as those of the standard library can, so that one
// instance serves every subsample instead of a new one being created for each.
type cbcMode interface {
	cipher.BlockMode
	SetIV(iv []byte)
}

// cryptSampleCbcsMode is cryptSampleCbcs with a reusable CBC mode, which is reset to iv at the
// start of every protected range.
func cryptSampleCbcsMode(cph cbcMode, sample []byte, iv []byte, subSamplePatterns []SubSamplePattern,
	tenc *TencBox) error {
	nrInCryptBlock := int(tenc.DefaultCryptByteBlock) * 16
	nrInSkipBlock := int(tenc.DefaultSkipByteBlock) * 16
	if len(subSamplePatterns) == 0 { // Full encryption as used for audio
		cph.SetIV(iv)
		cbcsStripe(cph, sample, nrInCryptBlock, nrInSkipBlock)
		return nil
	}
	var pos uint32 = 0
	for j := 0; j < len(subSamplePatterns); j++ {
		start, end, err := subSampleRange(len(sample), pos, j, subSamplePatterns[j])
		if err != nil {
			return err
		}
		if end > start {
			cph.SetIV(iv)
			cbcsStripe(cph, sample[start:end], nrInCryptBlock, nrInSkipBlock)
		}
		pos = end
	}
	return nil
}

// cbcsStripe encrypts or decrypts data in place with cph, in the pattern of nrInCryptBlock processed
// bytes followed by nrInSkipBlock clear bytes, or all full 16-byte blocks if nrInSkipBlock == 0.
func cbcsStripe(cph cipher.BlockMode, data []byte, nrInCryptBlock, nrInSkipBlock int) {
	pos := 0
	size := len(data) // This is the bytes that we should stripe decrypt
	if nrInSkipBlock == 0 {
		nrToCrypt := size & ^0xf // Drops 4 last bits -> multiple of 16
		cph.CryptBlocks(data[:nrToCrypt], data[:nrToCrypt])
		return
	}
	for size-pos >= nrInCryptBlock {
		cph.CryptBlocks(data[pos:pos+nrInCryptBlock], data[pos:pos+nrInCryptBlock])
		pos += nrInCryptBlock
		if size-pos < nrInSkipBlock {
			break
		}
		pos += nrInSkipBlock
	}
}

// nrEncBlocks returns the number of 16-byte blocks that cenc encrypts in a sample.
func nrEncBlocks(subsamplePatterns []SubSamplePattern, sampleLen int) int {
	if len(subsamplePatterns) == 0 {
		return (sampleLen + 15) / 16
	}
	n := 0
	for _, s := range subsamplePatterns {
		n += int(s.BytesOfProtectedData / 16)
	}
	return n
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
	// appendProtectRanges appends the subsample patterns of sample to dst.
	appendProtectRanges(dst []SubSamplePattern, sample []byte, scheme string) ([]SubSamplePattern, error)
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

// InitProtect modifies the init segment to add protection information and returns what is needed to encrypt fragments.
// The stsd sample entry is rewritten in place (avc1 -> encv etc.), so if the init segment shares boxes
// with another structure (for example a trak of a cached progressive file), clone the shared parts
// first (see [CloneBox]) or the other structure is silently modified too.
// For cbcs, AVC and HEVC slice headers are parsed with the SPS and PPS of the sample entry until a sample carries an
// SPS or PPS with the same ID, as avc3 and hev1 streams may; from there on, that one applies.
func InitProtect(init *InitSegment, key, iv []byte, scheme string, kid UUID, psshBoxes []*PsshBox) (*InitProtectData, error) {
	ipd := InitProtectData{Scheme: scheme}
	moov := init.Moov
	if moov == nil {
		return nil, fmt.Errorf("init segment has no moov box")
	}
	if len(moov.Traks) != 1 {
		return nil, fmt.Errorf("only one track supported")
	}
	stbl, err := moov.Trak.getSampleTable()
	if err != nil {
		return nil, err
	}
	stsd := stbl.Stsd
	if stsd == nil {
		return nil, fmt.Errorf("track has no stsd box")
	}
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
	if moov.Mvex != nil {
		ipd.Trex = moov.Mvex.Trex
	}
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
			if se.AvcC == nil {
				return nil, fmt.Errorf("%s sample entry has no avcC box", veType)
			}
			ipd.newProtector, err = newAVCProtectorFactory(se.AvcC)
			if err != nil {
				return nil, fmt.Errorf("get avc protector: %w", err)
			}
		case "hvc1", "hev1", "dvh1", "dvhe":
			if se.HvcC == nil {
				return nil, fmt.Errorf("%s sample entry has no hvcC box", veType)
			}
			ipd.newProtector, err = newHEVCProtectorFactory(se.HvcC)
			if err != nil {
				return nil, fmt.Errorf("get hevc protector: %w", err)
			}
		case "av01":
			if se.Av1C == nil {
				return nil, fmt.Errorf("%s sample entry has no av1C box", veType)
			}
			ipd.newProtector, err = newAV1ProtectorFactory(se.Av1C)
			if err != nil {
				return nil, fmt.Errorf("get av1 protector: %w", err)
			}
		case "vp09":
			if scheme != "cenc" {
				return nil, fmt.Errorf("vp09 protection requires scheme cenc, got %s", scheme)
			}
			ipd.newProtector = vp9ProtectorFactory
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

// funcProtector adapts a stateless protection-range function to the sampleProtector interface.
// It carries no mutable state, so a single instance is safe to reuse and share.
type funcProtector func(dst []SubSamplePattern, sample []byte, scheme string) ([]SubSamplePattern, error)

func (f funcProtector) appendProtectRanges(dst []SubSamplePattern, sample []byte, scheme string) (
	[]SubSamplePattern, error) {
	return f(dst, sample, scheme)
}

// statelessFactory returns a factory that always hands back the same stateless protector.
func statelessFactory(p sampleProtector) sampleProtectorFactory {
	return func() (sampleProtector, error) { return p, nil }
}

// audioProtectorFactory protects full audio samples; it is stateless and codec-independent.
var audioProtectorFactory = statelessFactory(funcProtector(
	func(dst []SubSamplePattern, sample []byte, scheme string) ([]SubSamplePattern, error) {
		return dst, nil // full-sample encryption: no subsamples
	}))

// vp9ProtectorFactory protects VP9 samples; the ranges depend on each sample alone, so it is stateless.
var vp9ProtectorFactory = statelessFactory(funcProtector(appendVP9ProtectRanges))

// avcProtector parses the slice headers of a sequence with the parameter sets of the decoder configuration record,
// each replaced by one with the same ID that the samples carry, from that sample on.
type avcProtector struct {
	ps parameterSets[avc.SPS, avc.PPS]
}

func (p *avcProtector) appendProtectRanges(dst []SubSamplePattern, sample []byte, scheme string) (
	[]SubSamplePattern, error) {
	return appendAVCProtectRanges(dst, &p.ps, sample, scheme)
}

func newAVCProtectorFactory(avcC *AvcCBox) (sampleProtectorFactory, error) {
	ps, err := newParameterSets(avcParameterSetParser, avcC.SPSnalus, avcC.PPSnalus)
	if err != nil {
		return nil, fmt.Errorf("get avc ps maps: %w", err)
	}
	// Every protector starts from the same parameter set maps and copies them before it changes them.
	ps.owned = false
	return func() (sampleProtector, error) {
		return &avcProtector{ps: ps}, nil
	}, nil
}

// hevcProtector parses the slice headers of a sequence with the parameter sets of the decoder configuration record,
// each replaced by one with the same ID that the samples carry, from that sample on.
type hevcProtector struct {
	ps parameterSets[hevc.SPS, hevc.PPS]
}

func (p *hevcProtector) appendProtectRanges(dst []SubSamplePattern, sample []byte, scheme string) (
	[]SubSamplePattern, error) {
	return appendHEVCProtectRanges(dst, &p.ps, sample, scheme)
}

func newHEVCProtectorFactory(hvcC *HvcCBox) (sampleProtectorFactory, error) {
	var spss, ppss [][]byte
	for _, naluArray := range hvcC.NaluArrays {
		switch naluArray.NaluType() {
		case hevc.NALU_SPS:
			spss = append(spss, naluArray.Nalus...)
		case hevc.NALU_PPS:
			ppss = append(ppss, naluArray.Nalus...)
		}
	}
	ps, err := newParameterSets(hevcParameterSetParser, spss, ppss)
	if err != nil {
		return nil, fmt.Errorf("get hevc ps maps: %w", err)
	}
	// Every protector starts from the same parameter set maps and copies them before it changes them.
	ps.owned = false
	return func() (sampleProtector, error) {
		return &hevcProtector{ps: ps}, nil
	}, nil
}

// av1Protector holds the mutable AV1 frame-header decoder for one decode sequence. Its
// reference-frame state accumulates across the samples of the sequence (in decode order),
// so it must not be shared between goroutines or reused across sequences.
type av1Protector struct {
	dec *av1.FrameHeaderDecoder
}

func (p *av1Protector) appendProtectRanges(dst []SubSamplePattern, sample []byte, scheme string) (
	[]SubSamplePattern, error) {
	return appendAV1ProtectRanges(dst, p.dec, sample, scheme)
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
	block   cipher.Block // AES block for the key, expanded once for the whole sequence
	iv      []byte
	iv8Mode bool
	prot    sampleProtector
	cbc     cbcMode // reusable CBC encrypter for cbcs, created at first use

	// Scratch reused for every fragment
	samples []FullSample
	iv16    [16]byte
	ivNext  [16]byte
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
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("new aes cipher: %w", err)
	}
	ivCopy := make([]byte, len(iv))
	copy(ivCopy, iv)
	return &FragmentEncryptor{ipd: ipd, block: block, iv: ivCopy, iv8Mode: iv8Mode, prot: prot}, nil
}

// IV returns a copy of the next initialization vector. For cenc it advances as fragments are encrypted, so
// it can be chained into the next sequence; for cbcs it is the constant IV.
func (e *FragmentEncryptor) IV() []byte {
	return bytes.Clone(e.iv)
}

// EncryptFragment encrypts one fragment in place and advances the internal IV. Call it once per
// fragment of the sequence, in decode order. A fragment built with AddFullSamples references the
// caller's sample buffers instead of copies, so those buffers are encrypted.
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
	saiz := f.takeSaiz(nrSamples)
	_ = traf.AddChild(saiz)
	saio := f.takeSaio()
	_ = traf.AddChild(saio)
	var senc *SencBox
	switch ipd.Scheme {
	case "cenc":
		senc = f.takeSenc(nrSamples, nrSamples)
	case "cbcs":
		senc = f.takeSenc(0, nrSamples)
	default:
		return fmt.Errorf("unknown scheme %s", ipd.Scheme)
	}
	_ = traf.AddChild(senc)
	fss, err := f.AppendFullSamples(e.samples[:0], ipd.Trex)
	if err != nil {
		return fmt.Errorf("get full samples: %w", err)
	}
	e.samples = fss

	// The per-sample IVs and subsample patterns stay referenced by senc and saiz until the fragment
	// is encoded, so they get fresh storage per fragment: one array each instead of one per sample.
	var ivs []byte
	if ipd.Scheme == "cenc" {
		ivs = slices.Grow(senc.ivStore[:0], len(fss)*len(iv))
	}
	ssps := slices.Grow(senc.sspStore[:0], 2*len(fss))
	// The 16-byte CTR IV for an 8-byte per-sample IV: the high half is the sample IV, the low
	// half stays zero. NewCTR copies the IV, so one buffer serves every sample.
	iv16 := e.iv16[:]
	clear(iv16)
	for _, fs := range fss {
		sample := fs.Data
		start := len(ssps)
		ssps, err = e.prot.appendProtectRanges(ssps, sample, ipd.Scheme)
		if err != nil {
			return fmt.Errorf("get protect ranges: %w", err)
		}
		subsamplePatterns := ssps[start:len(ssps):len(ssps)]
		if len(subsamplePatterns) == 0 {
			subsamplePatterns = nil
		}
		switch ipd.Scheme {
		case "cenc":
			ivStart := len(ivs)
			ivs = append(ivs, iv...)
			iv = ivs[ivStart:len(ivs):len(ivs)] // the sample's own copy, kept by senc and saiz
			ctrIV := iv
			if e.iv8Mode {
				copy(iv16, iv)
				ctrIV = iv16
			}
			if err := cryptSampleCencBlock(e.block, sample, ctrIV, subsamplePatterns); err != nil {
				return fmt.Errorf("crypt sample cenc: %w", err)
			}
			// Store IVs in the senc box and advance the IV for the next sample
			if err := senc.AddSample(SencSample{IV: iv, SubSamples: subsamplePatterns}); err != nil {
				return fmt.Errorf("senc add sample: %w", err)
			}
			if err := saiz.AddSampleInfo(iv, subsamplePatterns); err != nil {
				return fmt.Errorf("saiz add sample info: %w", err)
			}
			// Advance the IV in scratch; the next sample copies it into ivs
			next := e.iv16[:len(iv)]
			if e.iv8Mode {
				next = e.ivNext[:8]
			}
			copy(next, iv)
			if e.iv8Mode {
				incrementIVInPlace(next, 1)
			} else {
				incrementIVInPlace(next, nrEncBlocks(subsamplePatterns, len(sample)))
			}
			iv = next
		case "cbcs":
			if e.cbc == nil {
				if m, ok := cipher.NewCBCEncrypter(e.block, iv).(cbcMode); ok {
					e.cbc = m
				}
			}
			if e.cbc != nil {
				err = cryptSampleCbcsMode(e.cbc, sample, iv, subsamplePatterns, ipd.Tenc)
			} else {
				err = cryptSampleCbcs(dirEnc, sample, e.block, iv, subsamplePatterns, ipd.Tenc)
			}
			if err != nil {
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
	e.iv = append(e.iv[:0], iv...)
	clear(e.samples) // do not keep the fragment's sample data alive until the next call
	senc.ivStore, senc.sspStore = ivs, ssps
	if len(senc.IVs) == 0 && len(senc.SubSamples) == 0 {
		// No sample auxiliary information (full-sample encryption with a constant IV): CMAF
		// (ISO/IEC 23000-19 Section 8.2.2.1) recommends omitting the senc, saiz, and saio boxes.
		_ = f.Moof.Traf.RemoveEncryptionBoxes()
		f.spareSenc, f.spareSaiz, f.spareSaio = senc, saiz, saio
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
// protection ranges depend on reference-frame state that spans the sequence's fragments. A fragment
// built with AddFullSamples references the caller's sample buffers instead of copies, so those
// buffers are encrypted.
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
// A fragment built with AddFullSamples references the caller's sample buffers instead of copies,
// so those buffers are encrypted.
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
	if moov == nil {
		return DecryptInfo{}, fmt.Errorf("init segment has no moov box")
	}
	di := DecryptInfo{
		TrackInfos: make([]DecryptTrackInfo, 0, len(moov.Traks)),
	}
	for i, trak := range moov.Traks {
		if trak.Tkhd == nil {
			return di, fmt.Errorf("track %d has no tkhd box", i+1)
		}
		trackID := trak.Tkhd.TrackID
		stbl, err := trak.getSampleTable()
		if err != nil {
			return di, fmt.Errorf("trackID=%d: %w", trackID, err)
		}
		stsd := stbl.Stsd
		if stsd == nil {
			return di, fmt.Errorf("trackID=%d: track has no stsd box", trackID)
		}
		var schemeType string

		for _, child := range stsd.Children {
			var sinf *SinfBox
			switch child.Type() {
			case "encv":
				sinf, err = child.(*VisualSampleEntryBox).RemoveEncryption()
			case "enca":
				sinf, err = child.(*AudioSampleEntryBox).RemoveEncryption()
			default:
				continue
			}
			if err != nil {
				return di, err
			}
			if sinf.Schm == nil {
				return di, fmt.Errorf("missing schm for trackID=%d", trackID)
			}
			schemeType = sinf.Schm.SchemeType
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

	if moov.Mvex != nil {
		for _, trex := range moov.Mvex.Trexs {
			for i := range di.TrackInfos {
				if di.TrackInfos[i].TrackID == trex.TrackID {
					di.TrackInfos[i].Trex = trex
					break
				}
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
			if ti.Sinf.Schm == nil {
				return fmt.Errorf("missing schm for trackID=%d", ti.TrackID)
			}
			schemeType := ti.Sinf.Schm.SchemeType
			if schemeType != "cenc" && schemeType != "cbcs" {
				return fmt.Errorf("scheme type %s not supported", schemeType)
			}
			if ti.Sinf.Schi == nil || ti.Sinf.Schi.Tenc == nil {
				return fmt.Errorf("missing tenc for trackID=%d", ti.TrackID)
			}
			tenc := ti.Sinf.Schi.Tenc
			hasSenc, isParsed := traf.ContainsSencBox()
			if !hasSenc {
				// A missing senc is fine for full-sample encryption with a
				// constant IV (no sample auxiliary information); CMAF
				// (ISO/IEC 23000-19 Section 8.2.2.1) recommends omitting
				// senc, saiz, and saio in that case.
				if tenc.DefaultPerSampleIVSize != 0 || len(tenc.DefaultConstantIV) == 0 {
					return fmt.Errorf("no senc box in traf")
				}
			}
			if hasSenc && !isParsed {
				err := traf.ParseReadSenc(tenc.DefaultPerSampleIVSize, moof.StartPos)
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
	if frag.Mdat != nil && frag.Mdat.StartPos > frag.Moof.StartPos {
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
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}

	// Subsample information must cover exactly the samples to decrypt. Using
	// it for the wrong sample would decrypt the wrong byte ranges, and a senc
	// with fewer entries than samples used to panic.
	if senc != nil && len(senc.SubSamples) != 0 && len(senc.SubSamples) != len(samples) {
		return fmt.Errorf("senc has subsample information for %d samples, but %d samples to decrypt",
			len(senc.SubSamples), len(samples))
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
			if err := cryptSampleCencBlock(block, samples[i].Data, iv, subSamplePatterns); err != nil {
				return err
			}
		case "cbcs":
			err := cryptSampleCbcs(dirDec, samples[i].Data, block, iv, subSamplePatterns, tenc)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// ExtractInitProtectData extracts protection data from init segment.
// It returns an error if no sample entry has a sinf box with schm and tenc boxes.
// For cbcs, it has the same parameter set limitation as [InitProtect].
func ExtractInitProtectData(inSeg *InitSegment) (*InitProtectData, error) {
	moov := inSeg.Moov
	if moov == nil {
		return nil, fmt.Errorf("init segment has no moov box")
	}
	if len(moov.Traks) != 1 {
		return nil, fmt.Errorf("only one track supported")
	}
	stbl, err := moov.Trak.getSampleTable()
	if err != nil {
		return nil, err
	}
	if stbl.Stsd == nil {
		return nil, fmt.Errorf("track has no stsd box")
	}
	ipd := InitProtectData{}
	if moov.Mvex != nil {
		ipd.Trex = moov.Mvex.Trex
	}
	var sinf *SinfBox
	isVP9 := false
	for _, c := range stbl.Stsd.Children {
		switch box := c.(type) {
		case *VisualSampleEntryBox:
			if box.Sinf == nil {
				continue
			}
			sinf = box.Sinf
			frma := sinf.Frma
			if frma == nil {
				return nil, fmt.Errorf("sinf box has no frma box")
			}
			switch frma.DataFormat {
			case "avc1", "avc3":
				if box.AvcC == nil {
					return nil, fmt.Errorf("%s sample entry has no avcC box", frma.DataFormat)
				}
				ipd.newProtector, err = newAVCProtectorFactory(box.AvcC)
				if err != nil {
					return nil, fmt.Errorf("get AVC protector: %w", err)
				}
			case "hvc1", "hev1", "dvh1", "dvhe":
				if box.HvcC == nil {
					return nil, fmt.Errorf("%s sample entry has no hvcC box", frma.DataFormat)
				}
				ipd.newProtector, err = newHEVCProtectorFactory(box.HvcC)
				if err != nil {
					return nil, fmt.Errorf("get HEVC protector: %w", err)
				}
			case "av01":
				if box.Av1C == nil {
					return nil, fmt.Errorf("%s sample entry has no av1C box", frma.DataFormat)
				}
				ipd.newProtector, err = newAV1ProtectorFactory(box.Av1C)
				if err != nil {
					return nil, fmt.Errorf("get AV1 protector: %w", err)
				}
			case "vp09":
				ipd.newProtector = vp9ProtectorFactory
				isVP9 = true
			default:
				return nil, fmt.Errorf("unsupported video codec descriptor %s", frma.DataFormat)
			}
		case *AudioSampleEntryBox:
			if box.Sinf == nil {
				continue
			}
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

	switch {
	case sinf == nil:
		return nil, fmt.Errorf("no sample entry with a sinf box")
	case ipd.Scheme == "":
		return nil, fmt.Errorf("sinf box has no schm box")
	case ipd.Tenc == nil:
		return nil, fmt.Errorf("sinf box has no tenc box")
	}
	if isVP9 && ipd.Scheme != "cenc" {
		return nil, fmt.Errorf("vp09 protection requires scheme cenc, got %s", ipd.Scheme)
	}
	return &ipd, nil
}
