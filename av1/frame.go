package av1

import (
	"bytes"
	"fmt"

	"github.com/Eyevinn/mp4ff/bits"
)

// FrameType as signalled in the AV1 uncompressed frame header (spec 6.8.2).
type FrameType uint8

const (
	FrameTypeKey       FrameType = 0
	FrameTypeInter     FrameType = 1
	FrameTypeIntraOnly FrameType = 2
	FrameTypeSwitch    FrameType = 3
)

func (ft FrameType) String() string {
	switch ft {
	case FrameTypeKey:
		return "KEY_FRAME"
	case FrameTypeInter:
		return "INTER_FRAME"
	case FrameTypeIntraOnly:
		return "INTRA_ONLY_FRAME"
	case FrameTypeSwitch:
		return "SWITCH_FRAME"
	default:
		return fmt.Sprintf("Unknown(%d)", uint8(ft))
	}
}

// FrameInfo holds the frame-level fields decoded from the start of an AV1
// uncompressed frame header (OBU_FRAME or OBU_FRAME_HEADER payload).
type FrameInfo struct {
	ShowExistingFrame bool
	FrameType         FrameType
	ShowFrame         bool
	FrameIsIntra      bool
}

// ParseFrameHeaderStart decodes the leading fields of an uncompressed frame header
// (show_existing_frame, frame_type, show_frame) — enough to classify a frame as a
// random-access point. It follows uncompressed_header() in the AV1 spec (5.9.2) up to
// show_frame; no fields are read before show_existing_frame, so only the sequence
// header's reduced_still_picture_header flag is needed for context.
func ParseFrameHeaderStart(payload []byte, sh *SequenceHeader) (FrameInfo, error) {
	if sh == nil {
		return FrameInfo{}, fmt.Errorf("av1 frame header: nil sequence header")
	}
	if len(payload) == 0 {
		return FrameInfo{}, fmt.Errorf("av1 frame header: empty payload")
	}
	if sh.ReducedStillPictureHeader {
		return FrameInfo{FrameType: FrameTypeKey, ShowFrame: true, FrameIsIntra: true}, nil
	}
	r := bits.NewReader(bytes.NewReader(payload))
	fi := FrameInfo{}
	fi.ShowExistingFrame = r.ReadFlag()
	if fi.ShowExistingFrame {
		if err := r.AccError(); err != nil {
			return FrameInfo{}, fmt.Errorf("av1 frame header: %w", err)
		}
		return fi, nil
	}
	fi.FrameType = FrameType(r.Read(2))
	fi.FrameIsIntra = fi.FrameType == FrameTypeKey || fi.FrameType == FrameTypeIntraOnly
	fi.ShowFrame = r.ReadFlag()
	if err := r.AccError(); err != nil {
		return FrameInfo{}, fmt.Errorf("av1 frame header: %w", err)
	}
	return fi, nil
}

// IsRAPSample reports whether an AV1 sample (a temporal unit of concatenated OBUs) is a
// random-access point, i.e. it contains a key frame. A key frame resets all reference
// state, so any sample carrying one can start decoding.
//
// If the sample itself carries a sequence header OBU (as RAP samples normally do) it is
// used; otherwise the provided sh is used, which may be nil only when an in-band sequence
// header is guaranteed to be present.
func IsRAPSample(sample []byte, sh *SequenceHeader) (bool, error) {
	obus, err := SplitOBUs(sample)
	if err != nil {
		return false, err
	}
	for _, o := range obus {
		if o.Header.Type == OBUSequenceHeader {
			if parsed, perr := ParseSequenceHeader(o.Payload); perr == nil {
				sh = parsed
			}
		}
	}
	for _, o := range obus {
		if o.Header.Type != OBUFrame && o.Header.Type != OBUFrameHeader {
			continue
		}
		fi, err := ParseFrameHeaderStart(o.Payload, sh)
		if err != nil {
			return false, err
		}
		if !fi.ShowExistingFrame && fi.FrameType == FrameTypeKey {
			return true, nil
		}
	}
	return false, nil
}
