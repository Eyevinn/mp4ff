package vvc

import (
	"fmt"
)

// NaluType - VVC NAL unit type according to ISO/IEC 23090-3 Table 5
type NaluType uint8

// VVC NAL unit types (0-31)
const (
	// VCL NAL unit types
	NALU_TRAIL      = NaluType(0)  // Coded slice of a trailing picture or subpicture
	NALU_STSA       = NaluType(1)  // Coded slice of an STSA picture or subpicture
	NALU_RADL       = NaluType(2)  // Coded slice of a RADL picture or subpicture
	NALU_RASL       = NaluType(3)  // Coded slice of a RASL picture or subpicture
	NALU_RSV_VCL_4  = NaluType(4)  // Reserved non-IRAP VCL NAL unit type
	NALU_RSV_VCL_5  = NaluType(5)  // Reserved non-IRAP VCL NAL unit type
	NALU_RSV_VCL_6  = NaluType(6)  // Reserved non-IRAP VCL NAL unit type
	NALU_IDR_W_RADL = NaluType(7)  // Coded slice of an IDR picture or subpicture
	NALU_IDR_N_LP   = NaluType(8)  // Coded slice of an IDR picture or subpicture
	NALU_CRA        = NaluType(9)  // Coded slice of a CRA picture or subpicture
	NALU_GDR        = NaluType(10) // Coded slice of a GDR picture or subpicture
	NALU_RSV_IRAP   = NaluType(11) // Reserved IRAP VCL NAL unit type

	// Non-VCL NAL unit types
	NALU_OPI         = NaluType(12) // Operating point information
	NALU_DCI         = NaluType(13) // Decoding capability information
	NALU_VPS         = NaluType(14) // Video parameter set
	NALU_SPS         = NaluType(15) // Sequence parameter set
	NALU_PPS         = NaluType(16) // Picture parameter set
	NALU_PREFIX_APS  = NaluType(17) // Adaptation parameter set
	NALU_SUFFIX_APS  = NaluType(18) // Adaptation parameter set
	NALU_PH          = NaluType(19) // Picture header
	NALU_AUD         = NaluType(20) // AU delimiter
	NALU_EOS         = NaluType(21) // End of sequence
	NALU_EOB         = NaluType(22) // End of bitstream
	NALU_SEI_PREFIX  = NaluType(23) // Supplemental enhancement information
	NALU_SEI_SUFFIX  = NaluType(24) // Supplemental enhancement information
	NALU_FD          = NaluType(25) // Filler data
	NALU_RSV_NVCL_26 = NaluType(26) // Reserved non-VCL NAL unit type
	NALU_RSV_NVCL_27 = NaluType(27) // Reserved non-VCL NAL unit type
	NALU_UNSPEC_28   = NaluType(28) // Unspecified non-VCL NAL unit type
	NALU_UNSPEC_29   = NaluType(29) // Unspecified non-VCL NAL unit type
	NALU_UNSPEC_30   = NaluType(30) // Unspecified non-VCL NAL unit type
	NALU_UNSPEC_31   = NaluType(31) // Unspecified non-VCL NAL unit type
)

func (n NaluType) String() string {
	switch n {
	case NALU_TRAIL:
		return "TRAIL_0"
	case NALU_STSA:
		return "STSA_1"
	case NALU_RADL:
		return "RADL_2"
	case NALU_RASL:
		return "RASL_3"
	case NALU_RSV_VCL_4:
		return "RSV_VCL_4"
	case NALU_RSV_VCL_5:
		return "RSV_VCL_5"
	case NALU_RSV_VCL_6:
		return "RSV_VCL_6"
	case NALU_IDR_W_RADL:
		return "IDR_W_RADL_7"
	case NALU_IDR_N_LP:
		return "IDR_N_LP_8"
	case NALU_CRA:
		return "CRA_9"
	case NALU_GDR:
		return "GDR_10"
	case NALU_RSV_IRAP:
		return "RSV_IRAP_11"
	case NALU_OPI:
		return "OPI_12"
	case NALU_DCI:
		return "DCI_13"
	case NALU_VPS:
		return "VPS_14"
	case NALU_SPS:
		return "SPS_15"
	case NALU_PPS:
		return "PPS_16"
	case NALU_PREFIX_APS:
		return "PREFIX_APS_17"
	case NALU_SUFFIX_APS:
		return "SUFFIX_APS_18"
	case NALU_PH:
		return "PH_19"
	case NALU_AUD:
		return "AUD_20"
	case NALU_EOS:
		return "EOS_21"
	case NALU_EOB:
		return "EOB_22"
	case NALU_SEI_PREFIX:
		return "SEI_PREFIX_23"
	case NALU_SEI_SUFFIX:
		return "SEI_SUFFIX_24"
	case NALU_FD:
		return "FD_25"
	case NALU_RSV_NVCL_26:
		return "RSV_NVCL_26"
	case NALU_RSV_NVCL_27:
		return "RSV_NVCL_27"
	case NALU_UNSPEC_28:
		return "UNSPEC_28"
	case NALU_UNSPEC_29:
		return "UNSPEC_29"
	case NALU_UNSPEC_30:
		return "UNSPEC_30"
	case NALU_UNSPEC_31:
		return "UNSPEC_31"
	default:
		return fmt.Sprintf("Unknown(%d)", n)
	}
}

// NaluTypeName returns the name of the NAL unit type (backward compatibility)
func NaluTypeName(naluType uint8) string {
	return NaluType(naluType).String()
}

// NaluArray represents an array of NAL units of the same type
type NaluArray struct {
	NaluType NaluType
	Complete bool
	Nalus    [][]byte
}

// NewNaluArray creates a new NaluArray
func NewNaluArray(complete bool, naluType NaluType, nalus [][]byte) NaluArray {
	return NaluArray{
		NaluType: naluType,
		Complete: complete,
		Nalus:    nalus,
	}
}

// NaluTypeName returns the NAL unit type name
func (n NaluArray) NaluTypeName() string {
	return n.NaluType.String()
}

// NaluHeader is VVC NAL unit header
type NaluHeader struct {
	NuhLayerId         uint8 // NAL unit header layer ID
	NaluType           NaluType
	NuhTemporalIdPlus1 uint8 // NAL unit header temporal ID plus 1
}

// ParseNaluHeader parses the NAL unit header from raw bytes
func ParseNaluHeader(rawBytes []byte) (NaluHeader, error) {
	if len(rawBytes) < 2 {
		return NaluHeader{}, fmt.Errorf("NaluHeader: not enough bytes to parse header")
	}
	if forbiddenZeroBit := rawBytes[0] & 0x80; forbiddenZeroBit != 0 {
		return NaluHeader{}, fmt.Errorf("NaluHeader: forbidden zero bit is set")
	}
	if reservedZeroBit := rawBytes[0] & 0x40; reservedZeroBit != 0 {
		return NaluHeader{}, fmt.Errorf("NaluHeader: reserved zero bit is set")
	}
	return NaluHeader{
		NuhLayerId:         rawBytes[0] & 0x3f,         // 6 bits for layer ID
		NaluType:           NaluType(rawBytes[1] >> 3), // 5 bits for NALU type
		NuhTemporalIdPlus1: rawBytes[1] & 0x07,         // 3 bits for temporal ID plus 1
	}, nil
}
