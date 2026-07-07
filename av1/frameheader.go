package av1

import (
	"bytes"
	"fmt"

	"github.com/Eyevinn/mp4ff/bits"
)

// Constants from the AV1 specification (section 3).
const (
	numRefFrames      = 8    // NUM_REF_FRAMES
	refsPerFrame      = 7    // REFS_PER_FRAME
	superresDenomBits = 3    // SUPERRES_DENOM_BITS
	superresDenomMin  = 9    // SUPERRES_DENOM_MIN
	superresNum       = 8    // SUPERRES_NUM
	maxTileWidth      = 4096 // MAX_TILE_WIDTH (pixels)
	maxTileArea       = 4096 * 2304
	maxTileCols       = 64 // MAX_TILE_COLS
	maxTileRows       = 64 // MAX_TILE_ROWS
)

// FrameHeader holds the fields decoded from an AV1 uncompressed frame header up to and
// including tile_info() (spec 5.9.2). It is enough to know a frame's type, resolution and
// tile layout, but not (yet) to locate the tile data within an OBU.
type FrameHeader struct {
	ShowExistingFrame bool
	FrameType         FrameType
	ShowFrame         bool
	FrameIsIntra      bool
	// Resolution (post-superres coded size in FrameWidth; display size in UpscaledWidth).
	FrameWidth    uint32
	FrameHeight   uint32
	UpscaledWidth uint32
	RenderWidth   uint32
	RenderHeight  uint32
	// tile_info()
	TileCols            int
	TileRows            int
	TileColsLog2        int
	TileRowsLog2        int
	TileSizeBytes       int // bytes per tile-size field (1..4); valid when more than one tile
	ContextUpdateTileID int
}

// FrameHeaderDecoder parses a sequence of AV1 frame headers in decode order. It carries the
// reference-frame sizes needed because an inter frame may inherit its size from a previously
// decoded reference frame (frame_size_with_refs, spec 5.9.7).
type FrameHeaderDecoder struct {
	seq *SequenceHeader

	refValid         [numRefFrames]bool
	refFrameType     [numRefFrames]FrameType
	refUpscaledWidth [numRefFrames]uint32
	refFrameWidth    [numRefFrames]uint32
	refFrameHeight   [numRefFrames]uint32
	refRenderWidth   [numRefFrames]uint32
	refRenderHeight  [numRefFrames]uint32
	refOrderHint     [numRefFrames]uint32
	refFrameID       [numRefFrames]uint32
}

// NewFrameHeaderDecoder returns a decoder bound to a sequence header. The same decoder must
// be used for all frames of the sequence, fed in decode order, so that inter frames can
// resolve sizes inherited from reference frames.
func NewFrameHeaderDecoder(seq *SequenceHeader) (*FrameHeaderDecoder, error) {
	if seq == nil {
		return nil, fmt.Errorf("av1 frame header: nil sequence header")
	}
	return &FrameHeaderDecoder{seq: seq}, nil
}

// ParseFrameHeader decodes the uncompressed header of an OBU_FRAME or OBU_FRAME_HEADER
// payload through tile_info() and updates the reference-frame state. temporalID and spatialID
// come from the OBU extension header (0 when absent) and are only used for buffer_removal_time.
func (d *FrameHeaderDecoder) ParseFrameHeader(temporalID, spatialID byte, payload []byte) (*FrameHeader, error) {
	if len(payload) == 0 {
		return nil, fmt.Errorf("av1 frame header: empty payload")
	}
	seq := d.seq
	r := bits.NewReader(bytes.NewReader(payload))
	fh := &FrameHeader{}

	idLen := 0
	if seq.FrameIDNumbersPresent {
		idLen = int(seq.AdditionalFrameIDLengthMinus1) + int(seq.DeltaFrameIDLengthMinus2) + 3
	}
	allFrames := uint((1 << numRefFrames) - 1)

	var showableFrame, errorResilientMode bool
	if seq.ReducedStillPictureHeader {
		fh.FrameType = FrameTypeKey
		fh.FrameIsIntra = true
		fh.ShowFrame = true
	} else {
		fh.ShowExistingFrame = r.ReadFlag()
		if fh.ShowExistingFrame {
			// A repeat of an already decoded frame carries no tile data.
			if err := r.AccError(); err != nil {
				return nil, fmt.Errorf("av1 frame header: %w", err)
			}
			return fh, nil
		}
		fh.FrameType = FrameType(r.Read(2))
		fh.FrameIsIntra = fh.FrameType == FrameTypeKey || fh.FrameType == FrameTypeIntraOnly
		fh.ShowFrame = r.ReadFlag()
		if fh.ShowFrame && seq.DecoderModelInfoPresent && !seq.EqualPictureInterval {
			_ = r.Read(int(seq.FramePresentationTimeLengthMinus1) + 1) // frame_presentation_time
		}
		if fh.ShowFrame {
			showableFrame = fh.FrameType != FrameTypeKey
		} else {
			showableFrame = r.ReadFlag()
		}
		if fh.FrameType == FrameTypeSwitch || (fh.FrameType == FrameTypeKey && fh.ShowFrame) {
			errorResilientMode = true
		} else {
			errorResilientMode = r.ReadFlag()
		}
	}
	_ = showableFrame

	disableCdfUpdate := r.ReadFlag()

	var allowScreenContentTools uint
	if seq.SeqForceScreenContentTools == selectScreenContentTools {
		allowScreenContentTools = r.Read(1)
	} else {
		allowScreenContentTools = uint(seq.SeqForceScreenContentTools)
	}
	forceIntegerMV := uint(0)
	if allowScreenContentTools != 0 {
		if seq.SeqForceIntegerMV == selectIntegerMV {
			forceIntegerMV = r.Read(1)
		} else {
			forceIntegerMV = uint(seq.SeqForceIntegerMV)
		}
	}
	if fh.FrameIsIntra {
		forceIntegerMV = 1
	}

	var currentFrameID uint
	if seq.FrameIDNumbersPresent {
		currentFrameID = r.Read(idLen)
	}

	var frameSizeOverride bool
	switch {
	case fh.FrameType == FrameTypeSwitch:
		frameSizeOverride = true
	case seq.ReducedStillPictureHeader:
		frameSizeOverride = false
	default:
		frameSizeOverride = r.ReadFlag()
	}

	orderHint := r.Read(int(seq.OrderHintBits))

	if !fh.FrameIsIntra && !errorResilientMode {
		_ = r.Read(3) // primary_ref_frame
	}

	if seq.DecoderModelInfoPresent {
		if r.ReadFlag() { // buffer_removal_time_present_flag
			for opNum := 0; opNum < len(seq.OperatingPointIdc); opNum++ {
				if !seq.DecoderModelPresentForOp[opNum] {
					continue
				}
				opPtIdc := seq.OperatingPointIdc[opNum]
				inTemporal := (opPtIdc >> temporalID) & 1
				inSpatial := (opPtIdc >> (spatialID + 8)) & 1
				if opPtIdc == 0 || (inTemporal != 0 && inSpatial != 0) {
					_ = r.Read(int(seq.BufferRemovalTimeLengthMinus1) + 1)
				}
			}
		}
	}

	var refreshFrameFlags uint
	if fh.FrameType == FrameTypeSwitch || (fh.FrameType == FrameTypeKey && fh.ShowFrame) {
		refreshFrameFlags = allFrames
	} else {
		refreshFrameFlags = r.Read(8)
	}

	if (!fh.FrameIsIntra || refreshFrameFlags != allFrames) && errorResilientMode && seq.EnableOrderHint {
		for i := 0; i < numRefFrames; i++ {
			_ = r.Read(int(seq.OrderHintBits)) // ref_order_hint[i]
		}
	}

	var frameWidth, frameHeight, upscaledWidth, renderWidth, renderHeight uint
	if fh.FrameIsIntra {
		frameWidth, frameHeight, upscaledWidth = d.frameSize(r, frameSizeOverride)
		renderWidth, renderHeight = renderSize(r, upscaledWidth, frameHeight)
		if allowScreenContentTools != 0 && upscaledWidth == frameWidth {
			_ = r.Read(1) // allow_intrabc
		}
	} else {
		var refFrameIdx [refsPerFrame]int
		frameRefsShortSignaling := false
		if seq.EnableOrderHint {
			frameRefsShortSignaling = r.ReadFlag()
			if frameRefsShortSignaling {
				lastFrameIdx := int(r.Read(3))
				goldFrameIdx := int(r.Read(3))
				// set_frame_refs() (spec 7.8) computes the remaining indices from order hints.
				// It reads no bits; the exact indices only affect inherited sizes, which agree
				// for constant-resolution streams. Full resolution is left for GetAV1ProtectRanges.
				for i := range refFrameIdx {
					refFrameIdx[i] = lastFrameIdx
				}
				refFrameIdx[3] = goldFrameIdx
			}
		}
		for i := 0; i < refsPerFrame; i++ {
			if !frameRefsShortSignaling {
				refFrameIdx[i] = int(r.Read(3))
			}
			if seq.FrameIDNumbersPresent {
				_ = r.Read(int(seq.DeltaFrameIDLengthMinus2) + 2) // delta_frame_id_minus_1
			}
		}
		if frameSizeOverride && !errorResilientMode {
			frameWidth, frameHeight, upscaledWidth, renderWidth, renderHeight = d.frameSizeWithRefs(r, refFrameIdx, frameSizeOverride)
		} else {
			frameWidth, frameHeight, upscaledWidth = d.frameSize(r, frameSizeOverride)
			renderWidth, renderHeight = renderSize(r, upscaledWidth, frameHeight)
		}
		if forceIntegerMV == 0 {
			_ = r.Read(1) // allow_high_precision_mv
		}
		if !r.ReadFlag() { // is_filter_switchable
			_ = r.Read(2) // interpolation_filter
		}
		_ = r.Read(1) // is_motion_mode_switchable
		if !errorResilientMode && seq.EnableRefFrameMvs {
			_ = r.Read(1) // use_ref_frame_mvs
		}
	}

	if !seq.ReducedStillPictureHeader && !disableCdfUpdate {
		_ = r.ReadFlag() // disable_frame_end_update_cdf
	}

	d.tileInfo(r, frameWidth, frameHeight, fh)

	if err := r.AccError(); err != nil {
		return nil, fmt.Errorf("av1 frame header: %w", err)
	}

	fh.FrameWidth = uint32(frameWidth)
	fh.FrameHeight = uint32(frameHeight)
	fh.UpscaledWidth = uint32(upscaledWidth)
	fh.RenderWidth = uint32(renderWidth)
	fh.RenderHeight = uint32(renderHeight)

	for i := 0; i < numRefFrames; i++ {
		if (refreshFrameFlags>>uint(i))&1 == 1 {
			d.refValid[i] = true
			d.refFrameType[i] = fh.FrameType
			d.refUpscaledWidth[i] = fh.UpscaledWidth
			d.refFrameWidth[i] = fh.FrameWidth
			d.refFrameHeight[i] = fh.FrameHeight
			d.refRenderWidth[i] = fh.RenderWidth
			d.refRenderHeight[i] = fh.RenderHeight
			d.refOrderHint[i] = uint32(orderHint)
			d.refFrameID[i] = uint32(currentFrameID)
		}
	}
	return fh, nil
}

// frameSize implements frame_size() (spec 5.9.5) plus superres_params().
func (d *FrameHeaderDecoder) frameSize(r *bits.Reader, override bool) (frameWidth, frameHeight, upscaledWidth uint) {
	seq := d.seq
	if override {
		frameWidth = r.Read(int(seq.FrameWidthBitsMinus1)+1) + 1
		frameHeight = r.Read(int(seq.FrameHeightBitsMinus1)+1) + 1
	} else {
		frameWidth = uint(seq.MaxFrameWidthMinus1) + 1
		frameHeight = uint(seq.MaxFrameHeightMinus1) + 1
	}
	upscaledWidth = frameWidth
	frameWidth = d.superresParams(r, upscaledWidth)
	return frameWidth, frameHeight, upscaledWidth
}

// superresParams implements superres_params() (spec 5.9.8), returning the downscaled FrameWidth.
func (d *FrameHeaderDecoder) superresParams(r *bits.Reader, upscaledWidth uint) uint {
	useSuperres := false
	if d.seq.EnableSuperres {
		useSuperres = r.ReadFlag()
	}
	if !useSuperres {
		return upscaledWidth
	}
	denom := r.Read(superresDenomBits) + superresDenomMin
	return (upscaledWidth*superresNum + (denom / 2)) / denom
}

// renderSize implements render_size() (spec 5.9.6).
func renderSize(r *bits.Reader, upscaledWidth, frameHeight uint) (renderWidth, renderHeight uint) {
	if r.ReadFlag() { // render_and_frame_size_different
		renderWidth = r.Read(16) + 1
		renderHeight = r.Read(16) + 1
	} else {
		renderWidth = upscaledWidth
		renderHeight = frameHeight
	}
	return renderWidth, renderHeight
}

// frameSizeWithRefs implements frame_size_with_refs() (spec 5.9.7).
func (d *FrameHeaderDecoder) frameSizeWithRefs(r *bits.Reader, refFrameIdx [refsPerFrame]int, override bool) (
	frameWidth, frameHeight, upscaledWidth, renderWidth, renderHeight uint) {
	foundRef := false
	for i := 0; i < refsPerFrame; i++ {
		if r.ReadFlag() { // found_ref
			idx := refFrameIdx[i]
			if idx < 0 || idx >= numRefFrames {
				idx = 0
			}
			upscaledWidth = uint(d.refUpscaledWidth[idx])
			frameHeight = uint(d.refFrameHeight[idx])
			renderWidth = uint(d.refRenderWidth[idx])
			renderHeight = uint(d.refRenderHeight[idx])
			foundRef = true
			break
		}
	}
	if !foundRef {
		frameWidth, frameHeight, upscaledWidth = d.frameSize(r, override)
		renderWidth, renderHeight = renderSize(r, upscaledWidth, frameHeight)
	} else {
		frameWidth = d.superresParams(r, upscaledWidth)
	}
	return frameWidth, frameHeight, upscaledWidth, renderWidth, renderHeight
}

// tileInfo implements tile_info() (spec 5.9.15).
func (d *FrameHeaderDecoder) tileInfo(r *bits.Reader, frameWidth, frameHeight uint, fh *FrameHeader) {
	miCols := 2 * ((frameWidth + 7) >> 3)
	miRows := 2 * ((frameHeight + 7) >> 3)
	var sbCols, sbRows, sbShift int
	if d.seq.Use128x128Superblock {
		sbCols = int((miCols + 31) >> 5)
		sbRows = int((miRows + 31) >> 5)
		sbShift = 5
	} else {
		sbCols = int((miCols + 15) >> 4)
		sbRows = int((miRows + 15) >> 4)
		sbShift = 4
	}
	sbSize := sbShift + 2
	maxTileWidthSb := maxTileWidth >> sbSize
	maxTileAreaSb := maxTileArea >> (2 * sbSize)
	minLog2TileCols := tileLog2(maxTileWidthSb, sbCols)
	maxLog2TileCols := tileLog2(1, minInt(sbCols, maxTileCols))
	maxLog2TileRows := tileLog2(1, minInt(sbRows, maxTileRows))
	minLog2Tiles := maxInt(minLog2TileCols, tileLog2(maxTileAreaSb, sbRows*sbCols))

	if r.ReadFlag() { // uniform_tile_spacing_flag
		fh.TileColsLog2 = minLog2TileCols
		for fh.TileColsLog2 < maxLog2TileCols {
			if r.ReadFlag() { // increment_tile_cols_log2
				fh.TileColsLog2++
			} else {
				break
			}
		}
		tileWidthSb := (sbCols + (1 << fh.TileColsLog2) - 1) >> fh.TileColsLog2
		fh.TileCols = ceilDiv(sbCols, tileWidthSb)

		minLog2TileRows := maxInt(minLog2Tiles-fh.TileColsLog2, 0)
		fh.TileRowsLog2 = minLog2TileRows
		for fh.TileRowsLog2 < maxLog2TileRows {
			if r.ReadFlag() { // increment_tile_rows_log2
				fh.TileRowsLog2++
			} else {
				break
			}
		}
		tileHeightSb := (sbRows + (1 << fh.TileRowsLog2) - 1) >> fh.TileRowsLog2
		fh.TileRows = ceilDiv(sbRows, tileHeightSb)
	} else {
		widestTileSb := 0
		startSb := 0
		i := 0
		for ; startSb < sbCols; i++ {
			maxWidth := minInt(sbCols-startSb, maxTileWidthSb)
			widthInSbs := int(readNS(r, uint(maxWidth))) + 1
			if widthInSbs > widestTileSb {
				widestTileSb = widthInSbs
			}
			startSb += widthInSbs
		}
		fh.TileCols = i
		fh.TileColsLog2 = tileLog2(1, fh.TileCols)

		var maxTileAreaSb2 int
		if minLog2Tiles > 0 {
			maxTileAreaSb2 = (sbRows * sbCols) >> (minLog2Tiles + 1)
		} else {
			maxTileAreaSb2 = sbRows * sbCols
		}
		maxTileHeightSb := maxInt(maxTileAreaSb2/maxInt(widestTileSb, 1), 1)
		startSb = 0
		j := 0
		for ; startSb < sbRows; j++ {
			maxHeight := minInt(sbRows-startSb, maxTileHeightSb)
			heightInSbs := int(readNS(r, uint(maxHeight))) + 1
			startSb += heightInSbs
		}
		fh.TileRows = j
		fh.TileRowsLog2 = tileLog2(1, fh.TileRows)
	}

	if fh.TileColsLog2 > 0 || fh.TileRowsLog2 > 0 {
		fh.ContextUpdateTileID = int(r.Read(fh.TileRowsLog2 + fh.TileColsLog2))
		fh.TileSizeBytes = int(r.Read(2)) + 1
	} else {
		fh.TileSizeBytes = 1
	}
}

// readNS reads a non-symmetric unsigned encoded value ns(n) (spec 4.10.7).
func readNS(r *bits.Reader, n uint) uint {
	if n <= 1 {
		return 0
	}
	w := uint(floorLog2(n)) + 1
	m := (uint(1) << w) - n
	v := r.Read(int(w) - 1)
	if v < m {
		return v
	}
	extraBit := r.Read(1)
	return (v << 1) - m + extraBit
}

func floorLog2(x uint) int {
	s := 0
	for x != 0 {
		x >>= 1
		s++
	}
	return s - 1
}

// tileLog2 returns the smallest k such that (blkSize << k) >= target (spec 5.9.16).
func tileLog2(blkSize, target int) int {
	k := 0
	for (blkSize << k) < target {
		k++
	}
	return k
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func ceilDiv(a, b int) int {
	if b == 0 {
		return 0
	}
	return (a + b - 1) / b
}
