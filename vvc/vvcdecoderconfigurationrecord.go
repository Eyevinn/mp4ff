package vvc

import (
	"fmt"
	"io"

	"github.com/Eyevinn/mp4ff/bits"
)

/*
PTL represents profile-tier-level information (VvcPTLRecord) Section 11.2.4.1.2

	aligned(8) class VvcPTLRecord(num_sublayers) {
		bit(2) reserved = 0;
		unsigned int(6) num_bytes_constraint_info;
		unsigned int(7) general_profile_idc;
		unsigned int(1) general_tier_flag;
		unsigned int(8) general_level_idc;
		unsigned int(1) ptl_frame_only_constraint_flag;
		unsigned int(1) ptl_multi_layer_enabled_flag;
		unsigned int(8*num_bytes_constraint_info - 2) general_constraint_info;
		for (i=num_sublayers - 2; i >= 0; i--)
			unsigned int(1) ptl_sublayer_level_present_flag[i];
		for (j=num_sublayers; j<=8 && num_sublayers > 1; j++)
			bit(1) ptl_reserved_zero_bit = 0;
		for (i=num_sublayers-2; i >= 0; i--)
			if (ptl_sublayer_level_present_flag[i])
				unsigned int(8) sublayer_level_idc[i];
		unsigned int(8) ptl_num_sub_profiles;
		for (j=0; j < ptl_num_sub_profiles; j++)
		unsigned int(32) general_sub_profile_idc[j];
	}
*/
type PTL struct {
	NumBytesConstraintInfo      uint8
	GeneralProfileIDC           uint8
	GeneralTierFlag             bool
	GeneralLevelIDC             uint8
	PtlFrameOnlyConstraintFlag  bool
	PtlMultiLayerEnabledFlag    bool
	GeneralConstraintInfo       []byte
	PtlSublayerLevelPresentFlag []bool
	SublayerLevelIDC            []uint8
	PtlNumSubProfiles           uint8
	GeneralSubProfileIDC        []uint32
}

/*
DecConfRec represents VVC decoder configuration record, 11.2.4.2.2

	aligned(8) class VvcDecoderConfigurationRecord {
		bit(5) reserved = '11111'b;
		unsigned int(2) LengthSizeMinusOne;
		unsigned int(1) ptl_present_flag;
		if (ptl_present_flag) {
			unsigned int(9) ols_idx;
			unsigned int(3) num_sublayers;
			unsigned int(2) constant_frame_rate;
			unsigned int(2) chroma_format_idc;
			unsigned int(3) bit_depth_minus8;
			bit(5) reserved = '11111'b;
			VvcPTLRecord(num_sublayers) native_ptl;
			unsigned_int(16) max_picture_width;
			unsigned_int(16) max_picture_height;
			unsigned int(16) avg_frame_rate;
		}
		unsigned int(8) num_of_arrays;
		for (j=0; j < num_of_arrays; j++) {
			unsigned int(1) array_completeness;
			bit(2) reserved = 0;
			unsigned int(5) NAL_unit_type;
			if (NAL_unit_type != DCI_NUT && NAL_unit_type != OPI_NUT)
				unsigned int(16) num_nalus;
			for (i=0; i< num_nalus; i++) {
				unsigned int(16) nal_unit_length;
				bit(8*nal_unit_length) nal_unit;
			}
		}
	}
*/
type DecConfRec struct {
	LengthSizeMinusOne uint8
	PtlPresentFlag     bool
	OlsIdx             uint16
	NumSublayers       uint8
	ConstantFrameRate  uint8
	ChromaFormatIDC    uint8
	BitDepthMinus8     uint8
	NativePTL          PTL
	MaxPictureWidth    uint16
	MaxPictureHeight   uint16
	AvgFrameRate       uint16
	NaluArrays         []NaluArray
}

// Size returns the size of the decoder configuration record
func (d *DecConfRec) Size() int {
	size := 1 // reserved + lengthSizeMinusOne + ptlPresentFlag
	if d.PtlPresentFlag {
		size += 2 // olsIdx + numSublayers + constantFrameRate + chromaFormatIDC
		size += 1 // bitDepthMinus8 + reserved

		// PTL fields (VvcPTLRecord)
		size += 1                                       // num_bytes_constraint_info
		size += 1                                       // general_profile_idc + general_tier_flag
		size += 1                                       // general_level_idc
		size += int(d.NativePTL.NumBytesConstraintInfo) // constraint info (includes frame_only/multi_layer flags)

		// Sublayer flags and levels
		numSublayers := int(d.NumSublayers)
		if numSublayers > 1 {
			// The spec requires exactly 8 bits total for flags + reserved bits = 1 byte
			size += 1

			// Sublayer level IDCs (only for present ones)
			for _, present := range d.NativePTL.PtlSublayerLevelPresentFlag {
				if present {
					size += 1
				}
			}
		}

		size += 1                                      // ptl_num_sub_profiles
		size += int(d.NativePTL.PtlNumSubProfiles) * 4 // general_sub_profile_idc array

		size += 6 // maxPictureWidth + maxPictureHeight + avgFrameRate
	}
	size += 1 // numOfArrays

	for _, array := range d.NaluArrays {
		size += 1 // arrayCompleteness + reserved + NALUnitType
		if array.NaluType != NALU_DCI && array.NaluType != NALU_OPI {
			size += 2 // numNalus
		}
		for _, nalu := range array.Nalus {
			size += 2 + len(nalu) // naluLength + nalu
		}
	}

	return size
}

// Encode writes the decoder configuration record to w
func (d *DecConfRec) Encode(w io.Writer) error {
	sw := bits.NewFixedSliceWriter(d.Size())
	err := d.EncodeSW(sw)
	if err != nil {
		return err
	}
	_, err = w.Write(sw.Bytes())
	return err
}

// EncodeSW writes the decoder configuration record to sw
func (d *DecConfRec) EncodeSW(sw bits.SliceWriter) error {
	// First byte: reserved (5 bits) + lengthSizeMinusOne (2 bits) + ptlPresentFlag (1 bit)
	firstByte := byte(0xF8) | (d.LengthSizeMinusOne << 1)
	if d.PtlPresentFlag {
		firstByte |= 0x01
	}
	sw.WriteUint8(firstByte)

	if d.PtlPresentFlag {
		// olsIdx (9 bits) + numSublayers (3 bits) + constantFrameRate (2 bits) + chromaFormatIDC (2 bits)
		sw.WriteUint16((d.OlsIdx << 7) | (uint16(d.NumSublayers) << 4) | (uint16(d.ConstantFrameRate) << 2) | uint16(d.ChromaFormatIDC))

		// bitDepthMinus8 (3 bits) + reserved (5 bits)
		sw.WriteUint8((d.BitDepthMinus8 << 5) | 0x1F)

		// PTL fields - VvcPTLRecord structure
		// First byte: reserved (2 bits) + num_bytes_constraint_info (6 bits)
		sw.WriteUint8((d.NativePTL.NumBytesConstraintInfo & 0x3F))

		// Second byte: general_profile_idc (7 bits) + general_tier_flag (1 bit)
		sw.WriteUint8((d.NativePTL.GeneralProfileIDC << 1) | boolToUint8(d.NativePTL.GeneralTierFlag))

		// general_level_idc (8 bits)
		sw.WriteUint8(d.NativePTL.GeneralLevelIDC)

		// ptl_frame_only_constraint_flag (1 bit) + ptl_multi_layer_enabled_flag (1 bit) + constraint info
		flagsByte := boolToUint8(d.NativePTL.PtlFrameOnlyConstraintFlag)<<7 | boolToUint8(d.NativePTL.PtlMultiLayerEnabledFlag)<<6
		if len(d.NativePTL.GeneralConstraintInfo) > 0 {
			flagsByte |= d.NativePTL.GeneralConstraintInfo[0] & 0x3F
			sw.WriteUint8(flagsByte)
			if len(d.NativePTL.GeneralConstraintInfo) > 1 {
				sw.WriteBytes(d.NativePTL.GeneralConstraintInfo[1:])
			}
		} else {
			sw.WriteUint8(flagsByte)
		}

		// Handle sublayer level present flags
		numSublayers := int(d.NumSublayers)
		if numSublayers > 1 {
			// The spec requires writing exactly 8 bits total:
			// - (numSublayers - 1) bits for ptl_sublayer_level_present_flag
			// - (9 - numSublayers) bits for ptl_reserved_zero_bit
			// This always totals to 8 bits when numSublayers > 1

			flagByte := uint8(0)

			// Write sublayer level present flags (from MSB to LSB)
			for i := numSublayers - 2; i >= 0; i-- {
				bitPos := (numSublayers - 2) - i
				if i < len(d.NativePTL.PtlSublayerLevelPresentFlag) && d.NativePTL.PtlSublayerLevelPresentFlag[i] {
					flagByte |= (0x80 >> bitPos)
				}
			}

			// The remaining bits are reserved zero bits (already 0)
			sw.WriteUint8(flagByte)

			// Write sublayer level IDCs
			for i := numSublayers - 2; i >= 0; i-- {
				if i < len(d.NativePTL.PtlSublayerLevelPresentFlag) && d.NativePTL.PtlSublayerLevelPresentFlag[i] {
					if i < len(d.NativePTL.SublayerLevelIDC) {
						sw.WriteUint8(d.NativePTL.SublayerLevelIDC[i])
					} else {
						sw.WriteUint8(0) // Default value
					}
				}
			}
		}

		// ptl_num_sub_profiles
		sw.WriteUint8(d.NativePTL.PtlNumSubProfiles)

		// general_sub_profile_idc
		for _, subProfile := range d.NativePTL.GeneralSubProfileIDC {
			sw.WriteUint32(subProfile)
		}

		// maxPictureWidth, maxPictureHeight, avgFrameRate
		sw.WriteUint16(d.MaxPictureWidth)
		sw.WriteUint16(d.MaxPictureHeight)
		sw.WriteUint16(d.AvgFrameRate)
	}

	// numOfArrays
	sw.WriteUint8(uint8(len(d.NaluArrays)))

	// NAL unit arrays
	for _, array := range d.NaluArrays {
		// arrayCompleteness (1 bit) + reserved (2 bits) + NALUnitType (5 bits)
		arrayByte := uint8(array.NaluType) & 0x1F
		if array.Complete {
			arrayByte |= 0x80
		}
		sw.WriteUint8(arrayByte)

		// NALU_DCI and NALU_OPI do not have numNalus field but default to 1 NALU
		if array.NaluType != NALU_DCI && array.NaluType != NALU_OPI {
			sw.WriteUint16(uint16(len(array.Nalus)))
		}
		for _, nalu := range array.Nalus {
			sw.WriteUint16(uint16(len(nalu)))
			sw.WriteBytes(nalu)
		}
	}

	return sw.AccError()
}

// DecodeVVCDecConfRec decodes a VVC decoder configuration record
func DecodeVVCDecConfRec(data []byte) (DecConfRec, error) {
	sr := bits.NewFixedSliceReader(data)

	d := DecConfRec{}

	// First byte: reserved (5 bits) + lengthSizeMinusOne (2 bits) + ptlPresentFlag (1 bit)
	firstByte := sr.ReadUint8()
	d.LengthSizeMinusOne = (firstByte >> 1) & 0x03
	d.PtlPresentFlag = (firstByte & 0x01) != 0

	if d.PtlPresentFlag {
		// olsIdx (9 bits) + numSublayers (3 bits) + constantFrameRate (2 bits) + chromaFormatIDC (2 bits)
		combined := sr.ReadUint16()
		d.OlsIdx = combined >> 7
		d.NumSublayers = uint8((combined >> 4) & 0x07)
		d.ConstantFrameRate = uint8((combined >> 2) & 0x03)
		d.ChromaFormatIDC = uint8(combined & 0x03)

		// bitDepthMinus8 (3 bits) + reserved (5 bits)
		bitDepthByte := sr.ReadUint8()
		d.BitDepthMinus8 = bitDepthByte >> 5

		// PTL fields - VvcPTLRecord structure
		// First byte: reserved (2 bits) + num_bytes_constraint_info (6 bits)
		ptlFirstByte := sr.ReadUint8()
		d.NativePTL.NumBytesConstraintInfo = ptlFirstByte & 0x3F

		// Validate num_bytes_constraint_info - must be > 0 according to VVC spec
		if d.NativePTL.NumBytesConstraintInfo == 0 {
			return d, fmt.Errorf("invalid VVC PTL: num_bytes_constraint_info must be > 0")
		}

		// Second byte: general_profile_idc (7 bits) + general_tier_flag (1 bit)
		ptlSecondByte := sr.ReadUint8()
		d.NativePTL.GeneralProfileIDC = ptlSecondByte >> 1
		d.NativePTL.GeneralTierFlag = (ptlSecondByte & 0x01) != 0

		// general_level_idc (8 bits)
		d.NativePTL.GeneralLevelIDC = sr.ReadUint8()

		// ptl_frame_only_constraint_flag + ptl_multi_layer_enabled_flag + general_constraint_info
		// These flags are always present, followed by (8*num_bytes_constraint_info - 2) bits of constraint info
		constraintInfoBytes := int(d.NativePTL.NumBytesConstraintInfo)
		firstConstraintByte := sr.ReadUint8()
		d.NativePTL.PtlFrameOnlyConstraintFlag = (firstConstraintByte & 0x80) != 0
		d.NativePTL.PtlMultiLayerEnabledFlag = (firstConstraintByte & 0x40) != 0

		// Read remaining constraint info bytes
		d.NativePTL.GeneralConstraintInfo = make([]byte, constraintInfoBytes)
		d.NativePTL.GeneralConstraintInfo[0] = firstConstraintByte & 0x3F // Mask to get the last 6 bits
		if constraintInfoBytes > 1 {
			remainingConstraintBytes := sr.ReadBytes(constraintInfoBytes - 1)
			copy(d.NativePTL.GeneralConstraintInfo[1:], remainingConstraintBytes)
		}

		// Handle sublayer level present flags
		numSublayers := int(d.NumSublayers)
		if numSublayers > 1 {
			d.NativePTL.PtlSublayerLevelPresentFlag = make([]bool, numSublayers-1)

			// The spec requires reading exactly 8 bits total:
			// - (numSublayers - 1) bits for ptl_sublayer_level_present_flag
			// - (9 - numSublayers) bits for ptl_reserved_zero_bit
			// This always totals to 8 bits when numSublayers > 1

			flagByte := sr.ReadUint8()

			// Read sublayer level present flags (from MSB to LSB)
			for i := numSublayers - 2; i >= 0; i-- {
				bitPos := (numSublayers - 2) - i
				d.NativePTL.PtlSublayerLevelPresentFlag[i] = (flagByte & (0x80 >> bitPos)) != 0
			}

			// The remaining bits in flagByte are reserved zero bits (already consumed)

			// Read sublayer level IDCs
			d.NativePTL.SublayerLevelIDC = make([]uint8, numSublayers-1)
			for i := numSublayers - 2; i >= 0; i-- {
				if len(d.NativePTL.PtlSublayerLevelPresentFlag) > i && d.NativePTL.PtlSublayerLevelPresentFlag[i] {
					d.NativePTL.SublayerLevelIDC[i] = sr.ReadUint8()
				}
			}
		}

		// ptl_num_sub_profiles
		d.NativePTL.PtlNumSubProfiles = sr.ReadUint8()

		// general_sub_profile_idc
		if d.NativePTL.PtlNumSubProfiles > 0 {
			d.NativePTL.GeneralSubProfileIDC = make([]uint32, d.NativePTL.PtlNumSubProfiles)
			for i := 0; i < int(d.NativePTL.PtlNumSubProfiles); i++ {
				d.NativePTL.GeneralSubProfileIDC[i] = sr.ReadUint32()
			}
		}

		// maxPictureWidth, maxPictureHeight, avgFrameRate
		d.MaxPictureWidth = sr.ReadUint16()
		d.MaxPictureHeight = sr.ReadUint16()
		d.AvgFrameRate = sr.ReadUint16()
	}

	// numOfArrays
	numOfArrays := sr.ReadUint8()

	// NAL unit arrays
	for i := 0; i < int(numOfArrays); i++ {
		// arrayCompleteness (1 bit) + reserved (2 bits) + NALUnitType (5 bits)
		arrayByte := sr.ReadUint8()
		array := NaluArray{
			Complete: (arrayByte & 0x80) != 0,
			NaluType: NaluType(arrayByte & 0x1F),
		}

		numNalus := uint16(1) // Default to 1 NALU for DCI and OPI
		if array.NaluType != NALU_DCI && array.NaluType != NALU_OPI {
			numNalus = sr.ReadUint16()
		}
		array.Nalus = make([][]byte, numNalus)
		for j := 0; j < int(numNalus); j++ {
			naluLength := sr.ReadUint16()
			array.Nalus[j] = sr.ReadBytes(int(naluLength))
		}

		d.NaluArrays = append(d.NaluArrays, array)
	}

	if err := sr.AccError(); err != nil {
		return d, err
	}

	return d, nil
}

// Helper function to convert bool to uint8
func boolToUint8(b bool) uint8 {
	if b {
		return 1
	}
	return 0
}
