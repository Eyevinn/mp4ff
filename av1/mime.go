package av1

import "fmt"

// bitDepth derives the coded bit depth from seq_profile, high_bitdepth and twelve_bit,
// following the AV1 color_config() semantics (spec 5.5.2).
func bitDepth(seqProfile, highBitdepth, twelveBit byte) byte {
	if seqProfile == 2 && highBitdepth == 1 {
		if twelveBit == 1 {
			return 12
		}
		return 10
	}
	if highBitdepth == 1 {
		return 10
	}
	return 8
}

// mandatoryCodecString builds the "av01.P.LLT.DD" prefix common to all AV1 codec strings.
func mandatoryCodecString(sampleEntry string, seqProfile, seqLevelIdx0, seqTier0, bd byte) string {
	tier := "M"
	if seqTier0 == 1 {
		tier = "H"
	}
	return fmt.Sprintf("%s.%d.%02d%s.%02d", sampleEntry, seqProfile, seqLevelIdx0, tier, bd)
}

// CodecString returns the mandatory part of the RFC 6381 codecs parameter for AV1,
// e.g. "av01.0.09M.08", where sampleEntry is normally "av01". The optional
// color-configuration suffix requires the sequence header and is available through
// SequenceHeader.CodecString. Defined in the AV1 Codec ISO Media File Format Binding.
func (c *CodecConfRec) CodecString(sampleEntry string) string {
	bd := bitDepth(c.SeqProfile, c.HighBitdepth, c.TwelveBit)
	return mandatoryCodecString(sampleEntry, c.SeqProfile, c.SeqLevelIdx0, c.SeqTier0, bd)
}

// CodecString returns the full RFC 6381 codecs parameter for AV1, e.g.
// "av01.0.04M.10.0.110.01.01.01.0", where sampleEntry is normally "av01".
// The color-configuration suffix is omitted when all six fields hold their default
// values (non-monochrome, 4:2:0 with unknown chroma position, BT.709 primaries/
// transfer/matrix, studio range), yielding the short form "av01.0.04M.10".
// Defined in the AV1 Codec ISO Media File Format Binding.
func (s *SequenceHeader) CodecString(sampleEntry string) string {
	prefix := mandatoryCodecString(sampleEntry, s.SeqProfile, s.SeqLevelIdx0, s.SeqTier0, s.BitDepth)

	defaultColor := !s.MonoChrome &&
		s.SubsamplingX == 1 && s.SubsamplingY == 1 && s.ChromaSamplePosition == 0 &&
		s.ColorPrimaries == 1 && s.TransferCharacteristics == 1 &&
		s.MatrixCoefficients == 1 && !s.ColorRange
	if defaultColor {
		return prefix
	}
	return fmt.Sprintf("%s.%d.%d%d%d.%02d.%02d.%02d.%d",
		prefix, boolToDigit(s.MonoChrome),
		s.SubsamplingX, s.SubsamplingY, s.ChromaSamplePosition,
		s.ColorPrimaries, s.TransferCharacteristics, s.MatrixCoefficients,
		boolToDigit(s.ColorRange))
}

// CodecString - sub-parameter for MIME type "codecs" parameter like av01.0.04M.10
// where av01 is sampleEntry. Convenience function mirroring avc.CodecString and
// hevc.CodecString; see SequenceHeader.CodecString for details.
func CodecString(sampleEntry string, sh *SequenceHeader) string {
	return sh.CodecString(sampleEntry)
}

func boolToDigit(b bool) int {
	if b {
		return 1
	}
	return 0
}
