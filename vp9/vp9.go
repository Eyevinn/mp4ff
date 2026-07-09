// Package vp9 parses the VP9 uncompressed frame header.
//
// It reads enough of uncompressed_header() (VP9 Bitstream & Decoding Process Specification v0.6,
// §6.2) to detect key frames (random-access points) and, for key frames, the color configuration
// and picture size needed to build a vpcC (VPCodecConfigurationRecord) when muxing VP9 into mp4.
package vp9

import (
	"bytes"
	"fmt"

	"github.com/Eyevinn/mp4ff/bits"
)

// VP9 color_space values (color_config, spec §7.2.2).
const (
	CSUnknown  = 0
	CSBT601    = 1
	CSBT709    = 2
	CSSMPTE170 = 3
	CSSMPTE240 = 4
	CSBT2020   = 5
	CSReserved = 6
	CSRGB      = 7
)

const frameSyncCode = 0x498342

// Header is a parsed VP9 uncompressed frame header. For key frames the color-config and size
// fields are populated; for other frames only the leading flags are meaningful.
type Header struct {
	Profile           byte
	ShowExistingFrame bool
	KeyFrame          bool // frame_type == KEY_FRAME (0), and not a show_existing_frame
	ShowFrame         bool
	ErrorResilient    bool
	BitDepth          byte // 8, 10 or 12
	ColorSpace        byte
	ColorRange        bool
	SubsamplingX      byte
	SubsamplingY      byte
	Width             uint32
	Height            uint32
}

// ParseFrameHeader parses the uncompressed header at the start of a VP9 frame (an IVF frame
// payload, or the first coded frame of a superframe). It reads through frame_size() for key
// frames and stops early for show_existing_frame and non-key frames.
func ParseFrameHeader(frame []byte) (*Header, error) {
	r := bits.NewReader(bytes.NewReader(frame))
	h := &Header{}
	if r.Read(2) != 2 {
		return nil, fmt.Errorf("vp9: invalid frame_marker (not a VP9 frame)")
	}
	low := r.Read(1)
	high := r.Read(1)
	h.Profile = byte(high<<1 | low)
	if h.Profile == 3 {
		if r.Read(1) != 0 {
			return nil, fmt.Errorf("vp9: reserved_zero bit after profile is not zero")
		}
	}
	h.ShowExistingFrame = r.ReadFlag()
	if h.ShowExistingFrame {
		_ = r.Read(3) // frame_to_show_map_idx; header ends here
		return h, r.AccError()
	}
	h.KeyFrame = r.Read(1) == 0 // frame_type: 0 = KEY_FRAME
	h.ShowFrame = r.ReadFlag()
	h.ErrorResilient = r.ReadFlag()
	if !h.KeyFrame {
		return h, r.AccError()
	}
	if r.Read(24) != frameSyncCode {
		return nil, fmt.Errorf("vp9: invalid frame_sync_code")
	}
	h.parseColorConfig(r)
	h.Width = uint32(r.Read(16)) + 1
	h.Height = uint32(r.Read(16)) + 1
	return h, r.AccError()
}

func (h *Header) parseColorConfig(r *bits.Reader) {
	if h.Profile >= 2 {
		if r.ReadFlag() { // ten_or_twelve_bit
			h.BitDepth = 12
		} else {
			h.BitDepth = 10
		}
	} else {
		h.BitDepth = 8
	}
	h.ColorSpace = byte(r.Read(3))
	if h.ColorSpace != CSRGB {
		h.ColorRange = r.ReadFlag()
		if h.Profile == 1 || h.Profile == 3 {
			h.SubsamplingX = byte(r.Read(1))
			h.SubsamplingY = byte(r.Read(1))
			_ = r.Read(1) // reserved_zero
		} else {
			h.SubsamplingX, h.SubsamplingY = 1, 1 // 4:2:0 implied
		}
	} else {
		h.ColorRange = true // CS_RGB is always full range
		if h.Profile == 1 || h.Profile == 3 {
			h.SubsamplingX, h.SubsamplingY = 0, 0 // 4:4:4
			_ = r.Read(1)                         // reserved_zero
		}
	}
}

// IsKeyFrame reports whether the VP9 frame is a key frame (a random-access point).
func IsKeyFrame(frame []byte) (bool, error) {
	h, err := ParseFrameHeader(frame)
	if err != nil {
		return false, err
	}
	return h.KeyFrame, nil
}

// VpcCChromaSubsampling returns the vpcC chroma_subsampling code (0..3) for the header's
// subsampling. VP9 does not signal chroma sample position, so 4:2:0 uses the WebM binding default
// of 1 (colocated with luma).
func (h *Header) VpcCChromaSubsampling() byte {
	switch {
	case h.SubsamplingX == 1 && h.SubsamplingY == 1:
		return 1 // 4:2:0 colocated (WebM default)
	case h.SubsamplingX == 1 && h.SubsamplingY == 0:
		return 2 // 4:2:2
	case h.SubsamplingX == 0 && h.SubsamplingY == 0:
		return 3 // 4:4:4
	default:
		return 1
	}
}

// CICP returns the colour_primaries, transfer_characteristics and matrix_coefficients
// (ISO/IEC 23091-2 code points) derived from the VP9 color_space. Unknown/reserved map to
// 2 (unspecified).
func (h *Header) CICP() (primaries, transfer, matrix byte) {
	switch h.ColorSpace {
	case CSBT709:
		return 1, 1, 1
	case CSBT601, CSSMPTE170:
		return 6, 6, 6
	case CSSMPTE240:
		return 7, 7, 7
	case CSBT2020:
		transfer = 14 // BT.2020 10-bit
		if h.BitDepth == 12 {
			transfer = 15
		}
		return 9, transfer, 9
	case CSRGB:
		return 1, 13, 0 // sRGB primaries (BT.709), sRGB transfer, Identity matrix (needs 4:4:4)
	default: // CSUnknown, CSReserved
		return 2, 2, 2
	}
}

// levelEntry is one row of the VP9 level table (spec Annex A / webmproject.org/vp9/levels).
type levelEntry struct {
	id             byte
	maxSampleRate  uint64 // max luma sample rate (samples/s)
	maxPictureSize uint64 // max luma picture size (samples)
	maxDimension   uint32 // max width or height
}

var levelTable = []levelEntry{
	{10, 829440, 36864, 512},
	{11, 2764800, 73728, 768},
	{20, 4608000, 122880, 960},
	{21, 9216000, 245760, 1344},
	{30, 20736000, 552960, 2048},
	{31, 36864000, 983040, 2752},
	{40, 83558400, 2228224, 4160},
	{41, 160432128, 2228224, 4160},
	{50, 311951360, 8912896, 8384},
	{51, 588251136, 8912896, 8384},
	{52, 1176502272, 8912896, 8384},
	{60, 1176502272, 35651584, 16832},
	{61, 2353004544, 35651584, 16832},
	{62, 4706009088, 35651584, 16832},
}

// Level returns the vpcC level (e.g. 31 for level 3.1) as the smallest VP9 level that can carry
// width x height at frameRate frames per second. It returns 0 (undefined) when the size is
// unknown, and the highest defined level (62) when the content exceeds the table.
func Level(width, height uint32, frameRate float64) byte {
	if width == 0 || height == 0 {
		return 0
	}
	pic := uint64(width) * uint64(height)
	rate := uint64(float64(pic) * frameRate)
	maxDim := width
	if height > maxDim {
		maxDim = height
	}
	for _, l := range levelTable {
		if l.maxPictureSize >= pic && l.maxSampleRate >= rate && l.maxDimension >= maxDim {
			return l.id
		}
	}
	return levelTable[len(levelTable)-1].id
}
