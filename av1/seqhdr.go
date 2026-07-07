package av1

import (
	"bytes"
	"fmt"

	"github.com/Eyevinn/mp4ff/bits"
)

// AV1 constants used in sequence_header_obu() (spec 3 and 6.4.2).
const (
	selectScreenContentTools = 2
	selectIntegerMV          = 2

	cpBT709       = 1  // CP_BT_709
	tcSRGB        = 13 // TC_SRGB
	mcIdentity    = 0  // MC_IDENTITY
	cpUnspecified = 2  // CP_UNSPECIFIED
	tcUnspecified = 2  // TC_UNSPECIFIED
	mcUnspecified = 2  // MC_UNSPECIFIED
	cspUnknown    = 0  // CSP_UNKNOWN
)

// SequenceHeader is the parsed content of an AV1 Sequence Header OBU (spec 5.5).
// Only the fields needed for stream description (codec string, resolution, color) are kept;
// the full syntax is walked so that later fields are read at the correct bit offset.
type SequenceHeader struct {
	SeqProfile                byte
	StillPicture              bool
	ReducedStillPictureHeader bool
	SeqLevelIdx0              byte // seq_level_idx of operating point 0
	SeqTier0                  byte // seq_tier of operating point 0
	MaxFrameWidthMinus1       uint32
	MaxFrameHeightMinus1      uint32
	// color_config()
	BitDepth                byte // 8, 10 or 12
	MonoChrome              bool
	ColorPrimaries          byte
	TransferCharacteristics byte
	MatrixCoefficients      byte
	ColorRange              bool
	SubsamplingX            byte
	SubsamplingY            byte
	ChromaSamplePosition    byte
	// timing_info() (only present when TimingInfoPresent)
	TimingInfoPresent        bool
	NumUnitsInDisplayTick    uint32
	TimeScale                uint32
	EqualPictureInterval     bool
	NumTicksPerPictureMinus1 uint64
	// Fields needed for frame-header parsing (see FrameHeaderDecoder).
	FrameIDNumbersPresent             bool
	DeltaFrameIDLengthMinus2          byte
	AdditionalFrameIDLengthMinus1     byte
	FrameWidthBitsMinus1              byte
	FrameHeightBitsMinus1             byte
	Use128x128Superblock              bool
	EnableSuperres                    bool
	EnableOrderHint                   bool
	OrderHintBits                     byte // 0 when order hints are disabled
	EnableRefFrameMvs                 bool
	SeqForceScreenContentTools        byte // 0, 1, or SELECT_SCREEN_CONTENT_TOOLS (2)
	SeqForceIntegerMV                 byte // 0, 1, or SELECT_INTEGER_MV (2)
	EnableWarpedMotion                bool
	EnableCdef                        bool
	EnableRestoration                 bool
	FilmGrainParamsPresent            bool
	SeparateUVDeltaQ                  bool
	DecoderModelInfoPresent           bool
	BufferRemovalTimeLengthMinus1     byte
	FramePresentationTimeLengthMinus1 byte
	OperatingPointIdc                 []uint16
	DecoderModelPresentForOp          []bool
}

// Width returns the maximum frame width in pixels.
func (s *SequenceHeader) Width() uint32 { return s.MaxFrameWidthMinus1 + 1 }

// Height returns the maximum frame height in pixels.
func (s *SequenceHeader) Height() uint32 { return s.MaxFrameHeightMinus1 + 1 }

// ParseSequenceHeader parses the payload of a Sequence Header OBU (excluding the OBU header
// and size field). Follows sequence_header_obu() in the AV1 spec (section 5.5).
func ParseSequenceHeader(payload []byte) (*SequenceHeader, error) {
	if len(payload) == 0 {
		return nil, fmt.Errorf("av1 seqhdr: empty payload")
	}
	r := bits.NewReader(bytes.NewReader(payload))
	s := &SequenceHeader{}

	s.SeqProfile = byte(r.Read(3))
	s.StillPicture = r.ReadFlag()
	s.ReducedStillPictureHeader = r.ReadFlag()

	var decoderModelInfoPresent bool
	var bufferDelayLengthMinus1 uint

	if s.ReducedStillPictureHeader {
		s.SeqLevelIdx0 = byte(r.Read(5))
		s.SeqTier0 = 0
		s.OperatingPointIdc = []uint16{0}
		s.DecoderModelPresentForOp = []bool{false}
		s.SeqForceScreenContentTools = selectScreenContentTools
		s.SeqForceIntegerMV = selectIntegerMV
	} else {
		s.TimingInfoPresent = r.ReadFlag()
		if s.TimingInfoPresent {
			// timing_info()
			s.NumUnitsInDisplayTick = uint32(r.Read(32))
			s.TimeScale = uint32(r.Read(32))
			s.EqualPictureInterval = r.ReadFlag()
			if s.EqualPictureInterval {
				s.NumTicksPerPictureMinus1 = readUVLC(r)
			}
			decoderModelInfoPresent = r.ReadFlag()
			s.DecoderModelInfoPresent = decoderModelInfoPresent
			if decoderModelInfoPresent {
				// decoder_model_info()
				bufferDelayLengthMinus1 = r.Read(5)
				_ = r.Read(32) // num_units_in_decoding_tick
				s.BufferRemovalTimeLengthMinus1 = byte(r.Read(5))
				s.FramePresentationTimeLengthMinus1 = byte(r.Read(5))
			}
		}
		initialDisplayDelayPresent := r.ReadFlag()
		operatingPointsCntMinus1 := int(r.Read(5))
		s.OperatingPointIdc = make([]uint16, operatingPointsCntMinus1+1)
		s.DecoderModelPresentForOp = make([]bool, operatingPointsCntMinus1+1)
		for i := 0; i <= operatingPointsCntMinus1; i++ {
			s.OperatingPointIdc[i] = uint16(r.Read(12))
			seqLevelIdx := byte(r.Read(5))
			var seqTier byte
			if seqLevelIdx > 7 {
				seqTier = byte(r.Read(1))
			}
			if i == 0 {
				s.SeqLevelIdx0 = seqLevelIdx
				s.SeqTier0 = seqTier
			}
			if decoderModelInfoPresent {
				if r.ReadFlag() { // decoder_model_present_for_this_op[i]
					s.DecoderModelPresentForOp[i] = true
					// operating_parameters_info(i)
					n := int(bufferDelayLengthMinus1) + 1
					_ = r.Read(n) // decoder_buffer_delay[op]
					_ = r.Read(n) // encoder_buffer_delay[op]
					_ = r.Read(1) // low_delay_mode_flag[op]
				}
			}
			if initialDisplayDelayPresent {
				if r.ReadFlag() { // initial_display_delay_present_for_this_op[i]
					_ = r.Read(4) // initial_display_delay_minus_1[i]
				}
			}
		}
	}

	s.FrameWidthBitsMinus1 = byte(r.Read(4))
	s.FrameHeightBitsMinus1 = byte(r.Read(4))
	s.MaxFrameWidthMinus1 = uint32(r.Read(int(s.FrameWidthBitsMinus1) + 1))
	s.MaxFrameHeightMinus1 = uint32(r.Read(int(s.FrameHeightBitsMinus1) + 1))

	if !s.ReducedStillPictureHeader {
		s.FrameIDNumbersPresent = r.ReadFlag()
	}
	if s.FrameIDNumbersPresent {
		s.DeltaFrameIDLengthMinus2 = byte(r.Read(4))
		s.AdditionalFrameIDLengthMinus1 = byte(r.Read(3))
	}

	s.Use128x128Superblock = r.ReadFlag()
	_ = r.Read(1) // enable_filter_intra
	_ = r.Read(1) // enable_intra_edge_filter

	if !s.ReducedStillPictureHeader {
		_ = r.Read(1) // enable_interintra_compound
		_ = r.Read(1) // enable_masked_compound
		s.EnableWarpedMotion = r.ReadFlag()
		_ = r.Read(1) // enable_dual_filter
		s.EnableOrderHint = r.ReadFlag()
		if s.EnableOrderHint {
			_ = r.Read(1) // enable_jnt_comp
			s.EnableRefFrameMvs = r.ReadFlag()
		}
		if r.ReadFlag() { // seq_choose_screen_content_tools
			s.SeqForceScreenContentTools = selectScreenContentTools
		} else {
			s.SeqForceScreenContentTools = byte(r.Read(1))
		}
		if s.SeqForceScreenContentTools > 0 {
			if r.ReadFlag() { // seq_choose_integer_mv
				s.SeqForceIntegerMV = selectIntegerMV
			} else {
				s.SeqForceIntegerMV = byte(r.Read(1))
			}
		} else {
			s.SeqForceIntegerMV = selectIntegerMV
		}
		if s.EnableOrderHint {
			s.OrderHintBits = byte(r.Read(3)) + 1
		}
	}

	s.EnableSuperres = r.ReadFlag()
	s.EnableCdef = r.ReadFlag()
	s.EnableRestoration = r.ReadFlag()

	s.parseColorConfig(r)
	s.FilmGrainParamsPresent = r.ReadFlag()

	if err := r.AccError(); err != nil {
		return nil, fmt.Errorf("av1 seqhdr: %w", err)
	}
	return s, nil
}

// parseColorConfig implements color_config() (spec 5.5.2).
func (s *SequenceHeader) parseColorConfig(r *bits.Reader) {
	highBitdepth := r.Read(1)
	switch {
	case s.SeqProfile == 2 && highBitdepth == 1:
		if r.Read(1) == 1 { // twelve_bit
			s.BitDepth = 12
		} else {
			s.BitDepth = 10
		}
	case highBitdepth == 1:
		s.BitDepth = 10
	default:
		s.BitDepth = 8
	}

	if s.SeqProfile == 1 {
		s.MonoChrome = false
	} else {
		s.MonoChrome = r.ReadFlag()
	}

	if r.ReadFlag() { // color_description_present_flag
		s.ColorPrimaries = byte(r.Read(8))
		s.TransferCharacteristics = byte(r.Read(8))
		s.MatrixCoefficients = byte(r.Read(8))
	} else {
		s.ColorPrimaries = cpUnspecified
		s.TransferCharacteristics = tcUnspecified
		s.MatrixCoefficients = mcUnspecified
	}

	if s.MonoChrome {
		s.ColorRange = r.ReadFlag()
		s.SubsamplingX = 1
		s.SubsamplingY = 1
		s.ChromaSamplePosition = cspUnknown
		return
	}

	if s.ColorPrimaries == cpBT709 && s.TransferCharacteristics == tcSRGB && s.MatrixCoefficients == mcIdentity {
		// sRGB
		s.ColorRange = true
		s.SubsamplingX = 0
		s.SubsamplingY = 0
	} else {
		s.ColorRange = r.ReadFlag()
		switch s.SeqProfile {
		case 0:
			s.SubsamplingX = 1
			s.SubsamplingY = 1
		case 1:
			s.SubsamplingX = 0
			s.SubsamplingY = 0
		default: // profile 2
			if s.BitDepth == 12 {
				s.SubsamplingX = byte(r.Read(1))
				if s.SubsamplingX == 1 {
					s.SubsamplingY = byte(r.Read(1))
				} else {
					s.SubsamplingY = 0
				}
			} else {
				s.SubsamplingX = 1
				s.SubsamplingY = 0
			}
		}
		if s.SubsamplingX == 1 && s.SubsamplingY == 1 {
			s.ChromaSamplePosition = byte(r.Read(2))
		}
	}

	s.SeparateUVDeltaQ = r.ReadFlag()
}

// readUVLC reads a variable length unsigned integer uvlc() (spec 4.10.3).
func readUVLC(r *bits.Reader) uint64 {
	leadingZeros := 0
	for !r.ReadFlag() {
		if r.AccError() != nil {
			return 0
		}
		leadingZeros++
	}
	if leadingZeros >= 32 {
		return (1 << 32) - 1
	}
	value := uint64(r.Read(leadingZeros))
	return value + (1 << uint(leadingZeros)) - 1
}
