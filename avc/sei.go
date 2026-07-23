package avc

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/Eyevinn/mp4ff/sei"
)

var (
	ErrNotSEINalu = errors.New("not an SEI NAL unit")
)

// CreateSEINalu creates an SEI NAL unit (header + EBSP payload) from SEI messages.
// It is the inverse of ParseSEINalu.
func CreateSEINalu(msgs []sei.SEIMessage) ([]byte, error) {
	buf := bytes.Buffer{}
	// AVC NAL unit header (1 byte): forbidden_zero_bit(1)=0 | nal_ref_idc(2)=0 |
	// nal_unit_type(5)=NALU_SEI(6). The byte value equals NALU_SEI = 0x06.
	buf.WriteByte(byte(NALU_SEI))
	if err := sei.WriteSEIMessages(&buf, msgs); err != nil {
		return nil, fmt.Errorf("writing SEI messages: %w", err)
	}
	return buf.Bytes(), nil
}

// ParseSEINalu - parse SEI NAL unit (incl header) and return messages given SPS.
// Returns sei.ErrRbspTrailingBitsMissing if the NALU is missing the trailing bits.
func ParseSEINalu(nalu []byte, sps *SPS) ([]sei.SEIMessage, error) {
	if len(nalu) < 1 { // AVC NAL unit header is 1 byte
		return nil, ErrNotSEINalu
	}
	if GetNaluType(nalu[0]) != NALU_SEI {
		return nil, ErrNotSEINalu
	}
	seiBytes := nalu[1:] // Skip NALU header
	buf := bytes.NewReader(seiBytes)
	seiDatas, err := sei.ExtractSEIData(buf)
	missingRbspTrailingBits := false
	if err != nil {
		if errors.Is(err, sei.ErrRbspTrailingBitsMissing) {
			missingRbspTrailingBits = true
		} else {
			return nil, fmt.Errorf("extracting SEI data: %w", err)
		}
	}

	seiMsgs := make([]sei.SEIMessage, 0, len(seiDatas))
	var seiMsg sei.SEIMessage
	for _, seiData := range seiDatas {
		switch {
		case seiData.Type() == sei.SEIPicTimingType && sps != nil && sps.VUI != nil:
			var cbpDbpDelay *sei.CbpDbpDelay
			var timeOffsetLen byte = 0
			hrdParams := sps.VUI.VclHrdParameters
			if hrdParams == nil {
				hrdParams = sps.VUI.NalHrdParameters
			}
			if hrdParams != nil {
				cbpDbpDelay = &sei.CbpDbpDelay{
					CpbRemovalDelayLengthMinus1: byte(hrdParams.CpbRemovalDelayLengthMinus1),
					DpbOutputDelayLengthMinus1:  byte(hrdParams.DpbOutputDelayLengthMinus1),
				}
				timeOffsetLen = byte(hrdParams.TimeOffsetLength)
			}
			seiMsg, err = sei.DecodePicTimingAvcSEIHRD(&seiData, cbpDbpDelay, timeOffsetLen)
		default:
			seiMsg, err = sei.DecodeSEIMessage(&seiData, sei.AVC)
		}
		if err != nil {
			return nil, fmt.Errorf("sei decode: %w", err)
		}
		seiMsgs = append(seiMsgs, seiMsg)
	}
	if missingRbspTrailingBits {
		return seiMsgs, sei.ErrRbspTrailingBitsMissing
	}
	return seiMsgs, nil
}
