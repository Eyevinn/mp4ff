package sei

import (
	"bytes"
	"fmt"

	"github.com/Eyevinn/mp4ff/bits"
)

// RecoveryPointHevcSEI carries the data of an HEVC SEI 6 RecoveryPoint message.
// Defined in ISO/IEC 23008-2 Annex D.2.8 (syntax) and D.3.8 (semantics).
// The corresponding AVC message in RecoveryPointAvcSEI has a different syntax.
type RecoveryPointHevcSEI struct {
	// RecoveryPocCnt specifies the recovery point of decoded pictures in output order.
	RecoveryPocCnt int `json:"recovery_poc_cnt"`
	// ExactMatchFlag indicates whether pictures at and after the recovery point exactly match.
	ExactMatchFlag bool `json:"exact_match_flag"`
	// BrokenLinkFlag indicates the presence of a broken link at the recovery point location.
	BrokenLinkFlag bool `json:"broken_link_flag"`
}

// DecodeRecoveryPointHevcSEI decodes an HEVC SEI 6 RecoveryPoint message.
func DecodeRecoveryPointHevcSEI(sd *SEIData) (SEIMessage, error) {
	buf := bytes.NewBuffer(sd.Payload())
	br := bits.NewReader(buf)
	rp := RecoveryPointHevcSEI{
		RecoveryPocCnt: br.ReadSignedGolomb(),
		ExactMatchFlag: br.ReadFlag(),
		BrokenLinkFlag: br.ReadFlag(),
	}
	return &rp, br.AccError()
}

// Type returns the SEI payload type.
func (s *RecoveryPointHevcSEI) Type() uint {
	return SEIRecoveryPointType
}

// Size is the size in bytes of the raw SEI message rbsp payload.
func (s *RecoveryPointHevcSEI) Size() uint {
	nrBits := seNrBits(s.RecoveryPocCnt) + 1 + 1
	return uint((nrBits + 7) / 8)
}

// Payload returns the SEI raw rbsp payload.
func (s *RecoveryPointHevcSEI) Payload() []byte {
	sw := bits.NewFixedSliceWriter(int(s.Size()))
	sw.WriteSignedGolomb(s.RecoveryPocCnt)
	sw.WriteFlag(s.ExactMatchFlag)
	sw.WriteFlag(s.BrokenLinkFlag)
	sw.WriteFlag(true) // Final 1 and then byte align
	sw.FlushBits()
	return sw.Bytes()
}

// String returns a string representation of RecoveryPointHevcSEI.
func (s *RecoveryPointHevcSEI) String() string {
	return fmt.Sprintf("%s, size=%d, recoveryPocCnt=%d, exactMatch=%t, brokenLink=%t",
		SEIType(s.Type()), s.Size(), s.RecoveryPocCnt, s.ExactMatchFlag, s.BrokenLinkFlag)
}
