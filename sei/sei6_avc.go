package sei

import (
	"bytes"
	"fmt"

	"github.com/Eyevinn/mp4ff/bits"
)

// RecoveryPointAvcSEI carries the data of an AVC SEI 6 RecoveryPoint message.
// Defined in ISO/IEC 14496-10 Annex D.1.8 (syntax) and D.2.8 (semantics).
// The corresponding HEVC message in RecoveryPointHevcSEI has a different syntax.
type RecoveryPointAvcSEI struct {
	// RecoveryFrameCnt specifies the recovery point of output pictures in output order.
	RecoveryFrameCnt uint `json:"recovery_frame_cnt"`
	// ExactMatchFlag indicates whether pictures at and after the recovery point exactly match.
	ExactMatchFlag bool `json:"exact_match_flag"`
	// BrokenLinkFlag indicates the presence of a broken link at the recovery point location.
	BrokenLinkFlag bool `json:"broken_link_flag"`
	// ChangingSliceGroupIdc is in the range 0 to 2, inclusive.
	ChangingSliceGroupIdc uint8 `json:"changing_slice_group_idc"`
}

// DecodeRecoveryPointAvcSEI decodes an AVC SEI 6 RecoveryPoint message.
func DecodeRecoveryPointAvcSEI(sd *SEIData) (SEIMessage, error) {
	buf := bytes.NewBuffer(sd.Payload())
	br := bits.NewReader(buf)
	rp := RecoveryPointAvcSEI{
		RecoveryFrameCnt:      br.ReadExpGolomb(),
		ExactMatchFlag:        br.ReadFlag(),
		BrokenLinkFlag:        br.ReadFlag(),
		ChangingSliceGroupIdc: uint8(br.Read(2)),
	}
	return &rp, br.AccError()
}

// Type returns the SEI payload type.
func (s *RecoveryPointAvcSEI) Type() uint {
	return SEIRecoveryPointType
}

// Size is the size in bytes of the raw SEI message rbsp payload.
func (s *RecoveryPointAvcSEI) Size() uint {
	nrBits := ueNrBits(s.RecoveryFrameCnt) + 1 + 1 + 2
	return uint((nrBits + 7) / 8)
}

// Payload returns the SEI raw rbsp payload.
func (s *RecoveryPointAvcSEI) Payload() []byte {
	sw := bits.NewFixedSliceWriter(int(s.Size()))
	sw.WriteExpGolomb(s.RecoveryFrameCnt)
	sw.WriteFlag(s.ExactMatchFlag)
	sw.WriteFlag(s.BrokenLinkFlag)
	sw.WriteBits(uint(s.ChangingSliceGroupIdc), 2)
	sw.WriteFlag(true) // Final 1 and then byte align
	sw.FlushBits()
	return sw.Bytes()
}

// String returns a string representation of RecoveryPointAvcSEI.
func (s *RecoveryPointAvcSEI) String() string {
	return fmt.Sprintf("%s, size=%d, recoveryFrameCnt=%d, exactMatch=%t, brokenLink=%t, changingSliceGroupIdc=%d",
		SEIType(s.Type()), s.Size(), s.RecoveryFrameCnt, s.ExactMatchFlag, s.BrokenLinkFlag, s.ChangingSliceGroupIdc)
}

// ueNrBits returns the number of bits of the unsigned exponential Golomb code ue(v) for value v.
func ueNrBits(v uint) int {
	leadingZeroBits := 0
	x := v + 1
	for x > 1 {
		x >>= 1
		leadingZeroBits++
	}
	return 2*leadingZeroBits + 1
}

// seNrBits returns the number of bits of the signed exponential Golomb code se(v) for value v.
func seNrBits(v int) int {
	var codeNum uint
	if v <= 0 {
		codeNum = uint(-2 * v)
	} else {
		codeNum = uint(2*v - 1)
	}
	return ueNrBits(codeNum)
}
