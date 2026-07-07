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
	primaryRefNone    = 7    // PRIMARY_REF_NONE
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
	// HeaderBytes is the byte length of the uncompressed frame header including
	// byte alignment, i.e. the offset within an OBU_FRAME payload where the tile
	// group data begins. Zero for a show_existing_frame.
	HeaderBytes int
}

// NumTiles returns the number of tiles in the frame.
func (fh *FrameHeader) NumTiles() int { return fh.TileCols * fh.TileRows }

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

	// lastFrameHeader is the most recently parsed frame header, used to associate a
	// separate OBU_TILE_GROUP with the OBU_FRAME_HEADER that precedes it.
	lastFrameHeader *FrameHeader
}

// AV1 global motion types (spec 6.8.20).
const (
	gmIdentity    = 0
	gmTranslation = 1
	gmRotZoom     = 2
	gmAffine      = 3
)

// Segmentation feature bit widths and signedness (spec Tables in 5.9.14).
var (
	segFeatureBits   = [8]int{8, 6, 6, 6, 6, 3, 0, 0}
	segFeatureSigned = [8]bool{true, true, true, true, true, false, false, false}
)

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

	primaryRefFrame := uint(primaryRefNone)
	if !fh.FrameIsIntra && !errorResilientMode {
		primaryRefFrame = r.Read(3)
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
	var refFrameIdx [refsPerFrame]int
	allowIntrabc := false
	allowHighPrecisionMv := false
	if fh.FrameIsIntra {
		frameWidth, frameHeight, upscaledWidth = d.frameSize(r, frameSizeOverride)
		renderWidth, renderHeight = renderSize(r, upscaledWidth, frameHeight)
		if allowScreenContentTools != 0 && upscaledWidth == frameWidth {
			allowIntrabc = r.ReadFlag() // allow_intrabc
		}
	} else {
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
			allowHighPrecisionMv = r.ReadFlag() // allow_high_precision_mv
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

	// Continue through the rest of uncompressed_header() so that the byte offset of
	// the tile data (needed by GetTileRanges) can be determined. None of the tail
	// affects resolution; it only advances the bit position.
	d.parseHeaderTail(r, fh, frameWidth, upscaledWidth, allowIntrabc, allowHighPrecisionMv,
		errorResilientMode, showableFrame, primaryRefFrame, orderHint, refFrameIdx)

	r.ByteAlign()
	if err := r.AccError(); err != nil {
		return nil, fmt.Errorf("av1 frame header: %w", err)
	}
	fh.HeaderBytes = r.NrBytesRead()

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
	d.lastFrameHeader = fh
	return fh, nil
}

// parseHeaderTail parses uncompressed_header() from quantization_params() to the end
// (spec 5.9.2). It reads no resolution fields; its only purpose is to advance the reader
// to the byte-aligned start of the tile data.
func (d *FrameHeaderDecoder) parseHeaderTail(r *bits.Reader, fh *FrameHeader,
	frameWidth, upscaledWidth uint, allowIntrabc, allowHighPrecisionMv, errorResilientMode, showableFrame bool,
	primaryRefFrame, orderHint uint, refFrameIdx [refsPerFrame]int) {
	seq := d.seq
	numPlanes := 3
	if seq.MonoChrome {
		numPlanes = 1
	}

	// quantization_params()
	baseQIdx := int(r.Read(8))
	deltaQYDc := readDeltaQ(r)
	deltaQUDc, deltaQUAc, deltaQVDc, deltaQVAc := 0, 0, 0, 0
	if numPlanes > 1 {
		diffUVDelta := false
		if seq.SeparateUVDeltaQ {
			diffUVDelta = r.ReadFlag()
		}
		deltaQUDc = readDeltaQ(r)
		deltaQUAc = readDeltaQ(r)
		if diffUVDelta {
			deltaQVDc = readDeltaQ(r)
			deltaQVAc = readDeltaQ(r)
		} else {
			deltaQVDc, deltaQVAc = deltaQUDc, deltaQUAc
		}
	}
	if r.ReadFlag() { // using_qmatrix
		_ = r.Read(4) // qm_y
		_ = r.Read(4) // qm_u
		if seq.SeparateUVDeltaQ {
			_ = r.Read(4) // qm_v
		}
	}

	// segmentation_params()
	var featureEnabled [8][8]bool
	var featureData [8][8]int
	if r.ReadFlag() { // segmentation_enabled
		segmentationUpdateData := true
		if primaryRefFrame != primaryRefNone {
			if r.ReadFlag() { // segmentation_update_map
				_ = r.Read(1) // segmentation_temporal_update
			}
			segmentationUpdateData = r.ReadFlag()
		}
		if segmentationUpdateData {
			for i := 0; i < 8; i++ {
				for j := 0; j < 8; j++ {
					if r.ReadFlag() { // feature_enabled
						featureEnabled[i][j] = true
						bits := segFeatureBits[j]
						if segFeatureSigned[j] {
							featureData[i][j] = readSU(r, bits+1)
						} else {
							featureData[i][j] = int(r.Read(bits))
						}
					}
				}
			}
		}
	}

	// delta_q_params()
	deltaQPresent := false
	if baseQIdx > 0 {
		deltaQPresent = r.ReadFlag()
	}
	if deltaQPresent {
		_ = r.Read(2) // delta_q_res
	}

	// delta_lf_params()
	if deltaQPresent && !allowIntrabc {
		if r.ReadFlag() { // delta_lf_present
			_ = r.Read(2) // delta_lf_res
			_ = r.Read(1) // delta_lf_multi
		}
	}

	// CodedLossless / AllLossless (spec 5.9.2)
	codedLossless := true
	for seg := 0; seg < 8; seg++ {
		qindex := baseQIdx
		if featureEnabled[seg][0] { // SEG_LVL_ALT_Q
			qindex = baseQIdx + featureData[seg][0]
			if qindex < 0 {
				qindex = 0
			} else if qindex > 255 {
				qindex = 255
			}
		}
		lossless := qindex == 0 && deltaQYDc == 0 && deltaQUAc == 0 && deltaQUDc == 0 &&
			deltaQVAc == 0 && deltaQVDc == 0
		if !lossless {
			codedLossless = false
		}
	}
	allLossless := codedLossless && frameWidth == upscaledWidth

	// loop_filter_params()
	if !codedLossless && !allowIntrabc {
		lfl0 := r.Read(6)
		lfl1 := r.Read(6)
		if numPlanes > 1 && (lfl0 != 0 || lfl1 != 0) {
			_ = r.Read(6) // loop_filter_level[2]
			_ = r.Read(6) // loop_filter_level[3]
		}
		_ = r.Read(3)     // loop_filter_sharpness
		if r.ReadFlag() { // loop_filter_delta_enabled
			if r.ReadFlag() { // loop_filter_delta_update
				for i := 0; i < 8; i++ {
					if r.ReadFlag() { // update_ref_delta
						_ = readSU(r, 7)
					}
				}
				for i := 0; i < 2; i++ {
					if r.ReadFlag() { // update_mode_delta
						_ = readSU(r, 7)
					}
				}
			}
		}
	}

	// cdef_params()
	if !codedLossless && !allowIntrabc && seq.EnableCdef {
		_ = r.Read(2) // cdef_damping_minus_3
		cdefBits := r.Read(2)
		for i := 0; i < (1 << cdefBits); i++ {
			_ = r.Read(4) // cdef_y_pri_delta
			_ = r.Read(2) // cdef_y_sec_delta
			if numPlanes > 1 {
				_ = r.Read(4) // cdef_uv_pri_delta
				_ = r.Read(2) // cdef_uv_sec_delta
			}
		}
	}

	// lr_params()
	if !allLossless && !allowIntrabc && seq.EnableRestoration {
		usesLr := false
		usesChromaLr := false
		for i := 0; i < numPlanes; i++ {
			if r.Read(2) != 0 { // lr_type != RESTORE_NONE
				usesLr = true
				if i > 0 {
					usesChromaLr = true
				}
			}
		}
		if usesLr {
			if seq.Use128x128Superblock {
				_ = r.Read(1) // lr_unit_shift
			} else if r.ReadFlag() { // lr_unit_shift
				_ = r.Read(1) // lr_unit_extra_shift
			}
			if seq.SubsamplingX == 1 && seq.SubsamplingY == 1 && usesChromaLr {
				_ = r.Read(1) // lr_uv_shift
			}
		}
	}

	// read_tx_mode()
	if !codedLossless {
		_ = r.Read(1) // tx_mode_select
	}

	// frame_reference_mode()
	referenceSelect := false
	if !fh.FrameIsIntra {
		referenceSelect = r.ReadFlag()
	}

	// skip_mode_params()
	if d.skipModeAllowed(fh, errorResilientMode, referenceSelect, refFrameIdx, orderHint) {
		_ = r.Read(1) // skip_mode_present
	}

	if !fh.FrameIsIntra && !errorResilientMode && seq.EnableWarpedMotion {
		_ = r.Read(1) // allow_warped_motion
	}
	_ = r.Read(1) // reduced_tx_set

	// global_motion_params()
	if !fh.FrameIsIntra {
		globalMotionParams(r, allowHighPrecisionMv)
	}

	// film_grain_params()
	filmGrainParams(r, seq, fh, showableFrame)
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

// readSU reads an n-bit two's-complement signed value su(n) (spec 4.10.6).
func readSU(r *bits.Reader, n int) int {
	value := int(r.Read(n))
	signMask := 1 << (n - 1)
	if value&signMask != 0 {
		value -= 2 * signMask
	}
	return value
}

// readDeltaQ implements read_delta_q() (spec 5.9.13).
func readDeltaQ(r *bits.Reader) int {
	if r.ReadFlag() { // delta_coded
		return readSU(r, 7) // su(1+6)
	}
	return 0
}

// getRelativeDist implements get_relative_dist() (spec 5.9.3).
func (d *FrameHeaderDecoder) getRelativeDist(a, b uint32) int {
	if !d.seq.EnableOrderHint {
		return 0
	}
	diff := int(a) - int(b)
	m := 1 << (int(d.seq.OrderHintBits) - 1)
	return (diff & (m - 1)) - (diff & m)
}

// skipModeAllowed implements the skipModeAllowed computation of skip_mode_params() (spec 5.9.22).
func (d *FrameHeaderDecoder) skipModeAllowed(fh *FrameHeader, errorResilientMode, referenceSelect bool,
	refFrameIdx [refsPerFrame]int, orderHint uint) bool {
	_ = errorResilientMode
	if fh.FrameIsIntra || !referenceSelect || !d.seq.EnableOrderHint {
		return false
	}
	oh := uint32(orderHint)
	forwardIdx, backwardIdx := -1, -1
	var forwardHint, backwardHint uint32
	for i := 0; i < refsPerFrame; i++ {
		refHint := d.refOrderHint[refFrameIdx[i]]
		if d.getRelativeDist(refHint, oh) < 0 {
			if forwardIdx < 0 || d.getRelativeDist(refHint, forwardHint) > 0 {
				forwardIdx, forwardHint = i, refHint
			}
		} else if d.getRelativeDist(refHint, oh) > 0 {
			if backwardIdx < 0 || d.getRelativeDist(refHint, backwardHint) < 0 {
				backwardIdx, backwardHint = i, refHint
			}
		}
	}
	if forwardIdx < 0 {
		return false
	}
	if backwardIdx >= 0 {
		return true
	}
	secondForwardIdx := -1
	var secondForwardHint uint32
	for i := 0; i < refsPerFrame; i++ {
		refHint := d.refOrderHint[refFrameIdx[i]]
		if d.getRelativeDist(refHint, forwardHint) < 0 {
			if secondForwardIdx < 0 || d.getRelativeDist(refHint, secondForwardHint) > 0 {
				secondForwardIdx, secondForwardHint = i, refHint
			}
		}
	}
	return secondForwardIdx >= 0
}

// globalMotionParams implements global_motion_params() for inter frames (spec 5.9.24).
// Only the number of bits consumed matters here, which is independent of the reference
// global-motion parameters, so those are not tracked.
func globalMotionParams(r *bits.Reader, allowHighPrecisionMv bool) {
	for ref := 1; ref <= 7; ref++ { // LAST_FRAME .. ALTREF_FRAME
		gmType := gmIdentity
		if r.ReadFlag() { // is_global
			if r.ReadFlag() { // is_rot_zoom
				gmType = gmRotZoom
			} else if r.ReadFlag() { // is_translation
				gmType = gmTranslation
			} else {
				gmType = gmAffine
			}
		}
		if gmType >= gmRotZoom {
			readGlobalParam(r, gmType, 2, allowHighPrecisionMv)
			readGlobalParam(r, gmType, 3, allowHighPrecisionMv)
			if gmType == gmAffine {
				readGlobalParam(r, gmType, 4, allowHighPrecisionMv)
				readGlobalParam(r, gmType, 5, allowHighPrecisionMv)
			}
		}
		if gmType >= gmTranslation {
			readGlobalParam(r, gmType, 0, allowHighPrecisionMv)
			readGlobalParam(r, gmType, 1, allowHighPrecisionMv)
		}
	}
}

// readGlobalParam implements read_global_param() (spec 5.9.25) for bit consumption only.
func readGlobalParam(r *bits.Reader, gmType, idx int, allowHighPrecisionMv bool) {
	absBits := 12 // GM_ABS_ALPHA_BITS
	if idx < 2 {
		if gmType == gmTranslation {
			hp := 1
			if allowHighPrecisionMv {
				hp = 0
			}
			absBits = 9 - hp // GM_ABS_TRANS_ONLY_BITS - !allow_high_precision_mv
		} else {
			absBits = 12 // GM_ABS_TRANS_BITS
		}
	}
	mx := 1 << absBits
	// decode_signed_subexp_with_ref(-mx, mx+1, r) -> decode_subexp(2*mx+1); the reference r
	// affects the decoded value but not the number of bits read.
	decodeSubexp(r, 2*mx+1)
}

// decodeSubexp implements decode_subexp() (spec 5.9.27) for bit consumption only.
func decodeSubexp(r *bits.Reader, numSyms int) {
	i, mk, k := 0, 0, 3
	for {
		b2 := k
		if i != 0 {
			b2 = k + i - 1
		}
		a := 1 << b2
		if numSyms <= mk+3*a {
			_ = readNS(r, uint(numSyms-mk)) // subexp_final_bits
			return
		}
		if r.ReadFlag() { // subexp_more_bits
			i++
			mk += a
		} else {
			_ = r.Read(b2) // subexp_bits
			return
		}
	}
}

// filmGrainParams implements film_grain_params() (spec 5.9.30).
func filmGrainParams(r *bits.Reader, seq *SequenceHeader, fh *FrameHeader, showableFrame bool) {
	if !seq.FilmGrainParamsPresent || (!fh.ShowFrame && !showableFrame) {
		return
	}
	if !r.ReadFlag() { // apply_grain
		return
	}
	_ = r.Read(16) // grain_seed
	updateGrain := true
	if fh.FrameType == FrameTypeInter {
		updateGrain = r.ReadFlag()
	}
	if !updateGrain {
		_ = r.Read(3) // film_grain_params_ref_idx
		return
	}
	numYPoints := int(r.Read(4))
	for i := 0; i < numYPoints; i++ {
		_ = r.Read(8) // point_y_value
		_ = r.Read(8) // point_y_scaling
	}
	chromaScalingFromLuma := false
	if !seq.MonoChrome {
		chromaScalingFromLuma = r.ReadFlag()
	}
	numCbPoints, numCrPoints := 0, 0
	subsampledNoLuma := seq.SubsamplingX == 1 && seq.SubsamplingY == 1 && numYPoints == 0
	if !seq.MonoChrome && !chromaScalingFromLuma && !subsampledNoLuma {
		numCbPoints = int(r.Read(4))
		for i := 0; i < numCbPoints; i++ {
			_ = r.Read(8) // point_cb_value
			_ = r.Read(8) // point_cb_scaling
		}
		numCrPoints = int(r.Read(4))
		for i := 0; i < numCrPoints; i++ {
			_ = r.Read(8) // point_cr_value
			_ = r.Read(8) // point_cr_scaling
		}
	}
	_ = r.Read(2) // grain_scaling_minus_8
	arCoeffLag := int(r.Read(2))
	numPosLuma := 2 * arCoeffLag * (arCoeffLag + 1)
	numPosChroma := numPosLuma
	if numYPoints > 0 {
		numPosChroma = numPosLuma + 1
		for i := 0; i < numPosLuma; i++ {
			_ = r.Read(8) // ar_coeffs_y_plus_128
		}
	}
	if chromaScalingFromLuma || numCbPoints > 0 {
		for i := 0; i < numPosChroma; i++ {
			_ = r.Read(8) // ar_coeffs_cb_plus_128
		}
	}
	if chromaScalingFromLuma || numCrPoints > 0 {
		for i := 0; i < numPosChroma; i++ {
			_ = r.Read(8) // ar_coeffs_cr_plus_128
		}
	}
	_ = r.Read(2) // ar_coeff_shift_minus_6
	_ = r.Read(2) // grain_scale_shift
	if numCbPoints > 0 {
		_ = r.Read(8) // cb_mult
		_ = r.Read(8) // cb_luma_mult
		_ = r.Read(9) // cb_offset
	}
	if numCrPoints > 0 {
		_ = r.Read(8) // cr_mult
		_ = r.Read(8) // cr_luma_mult
		_ = r.Read(9) // cr_offset
	}
	_ = r.Read(1) // overlap_flag
	_ = r.Read(1) // clip_to_restricted_range
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
