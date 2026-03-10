package main

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Eyevinn/mp4ff/avc"
	"github.com/Eyevinn/mp4ff/hevc"
	"github.com/Eyevinn/mp4ff/internal"
	"github.com/Eyevinn/mp4ff/mp4"
)

const appName = "mp4ff-mvhevc"

var usg = `%s handles MV-HEVC (Multi-View HEVC) MP4 files.

Subcommands:
  info <input.mp4>                                Display MV-HEVC metadata
  add [-fps <rate>] <input.hevc|mp4> <output.mp4> Mux into MP4 with MV-HEVC metadata

Usage of %s:
`

func main() {
	if err := run(os.Args, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string, w io.Writer) error {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, usg, appName, appName)
		return fmt.Errorf("need subcommand: info or add")
	}

	switch args[1] {
	case "info":
		return runInfo(args[1:], w)
	case "add":
		return runAdd(args[1:], w)
	case "-version", "--version", "version":
		fmt.Fprintf(w, "%s %s\n", appName, internal.GetVersion())
		return nil
	default:
		fmt.Fprintf(os.Stderr, usg, appName, appName)
		return fmt.Errorf("unknown subcommand: %s", args[1])
	}
}

// mvhevcInput holds parsed MV-HEVC data from either Annex B or MP4 input.
type mvhevcInput struct {
	baseVPS, baseSPS, basePPS, baseSEI [][]byte
	enhSPS, enhPPS                     [][]byte
	vps                                *hevc.VPS
	width, height                      uint16
	timeScale                          uint32
	sampleDur                          uint32
	samples                            []mvhevcSample

	// Spatial video metadata (vexu/hfov)
	addSpatial  bool
	stereoFlags byte
	heroEye     byte
	baseline    uint32 // micrometers
	hfov        uint32 // thousandths of a degree
	projType    string
}

// mvhevcSample holds data for a single access unit (sample).
type mvhevcSample struct {
	data   []byte // length-prefixed NALUs concatenated
	size   uint32
	isSync bool
}

func runInfo(args []string, w io.Writer) error {
	fs := flag.NewFlagSet("info", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "%s info <input.mp4>\n", appName)
	}
	if err := fs.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return fmt.Errorf("need input file")
	}

	ifd, err := os.Open(fs.Arg(0))
	if err != nil {
		return fmt.Errorf("could not open input file: %w", err)
	}
	defer ifd.Close()

	parsedMp4, err := mp4.DecodeFile(ifd, mp4.WithDecodeMode(mp4.DecModeLazyMdat))
	if err != nil {
		return fmt.Errorf("could not decode MP4: %w", err)
	}

	if parsedMp4.Moov == nil {
		return fmt.Errorf("no moov box found")
	}

	for i, trak := range parsedMp4.Moov.Traks {
		fmt.Fprintf(w, "Track %d (ID=%d):\n", i+1, trak.Tkhd.TrackID)

		stbl := trak.Mdia.Minf.Stbl
		if stbl == nil || stbl.Stsd == nil {
			continue
		}

		// Check for HEVC visual sample entries
		for _, child := range stbl.Stsd.Children {
			vse, ok := child.(*mp4.VisualSampleEntryBox)
			if !ok {
				continue
			}
			fmt.Fprintf(w, "  Sample entry: %s (%dx%d)\n",
				vse.Type(), vse.Width, vse.Height)

			if vse.HvcC != nil {
				fmt.Fprintf(w, "  hvcC (base layer config):\n")
				hdcr := vse.HvcC.DecConfRec
				fmt.Fprintf(w, "    Profile: space=%d tier=%t idc=%d level=%d\n",
					hdcr.GeneralProfileSpace, hdcr.GeneralTierFlag,
					hdcr.GeneralProfileIDC, hdcr.GeneralLevelIDC)
				fmt.Fprintf(w, "    Chroma: %d  BitDepth: luma=%d chroma=%d\n",
					hdcr.ChromaFormatIDC,
					hdcr.BitDepthLumaMinus8+8,
					hdcr.BitDepthChromaMinus8+8)
				fmt.Fprintf(w, "    NumTemporalLayers: %d  LengthSize: %d\n",
					hdcr.NumTemporalLayers, hdcr.LengthSizeMinusOne+1)

				vpsNalus := hdcr.GetNalusForType(hevc.NALU_VPS)
				if len(vpsNalus) > 0 {
					vps, err := hevc.ParseVPSNALUnit(vpsNalus[0])
					if err != nil {
						fmt.Fprintf(w, "    VPS parse error: %v\n", err)
					} else {
						fmt.Fprintf(w,
							"    VPS: layers=%d views=%d multiLayer=%t\n",
							vps.GetNumLayers(), vps.GetNumViews(),
							vps.IsMultiLayer())
					}
				}

				for _, array := range hdcr.NaluArrays {
					fmt.Fprintf(w, "    %s: %d nalus (complete=%d)\n",
						array.NaluType(), len(array.Nalus),
						array.Complete())
				}
			}

			if vse.LhvC != nil {
				fmt.Fprintf(w, "  lhvC (enhancement layer config):\n")
				hdcr := vse.LhvC.DecConfRec
				fmt.Fprintf(w, "    NumTemporalLayers: %d  LengthSize: %d\n",
					hdcr.NumTemporalLayers, hdcr.LengthSizeMinusOne+1)
				for _, array := range hdcr.NaluArrays {
					fmt.Fprintf(w, "    %s: %d nalus (complete=%d)\n",
						array.NaluType(), len(array.Nalus),
						array.Complete())
					for _, nalu := range array.Nalus {
						fmt.Fprintf(w, "      %s\n",
							hex.EncodeToString(nalu))
					}
				}
			}

			if vse.Vexu != nil {
				fmt.Fprintf(w, "  vexu (Spatial Video):\n")
				if vse.Vexu.Eyes != nil {
					eyes := vse.Vexu.Eyes
					if eyes.Stri != nil {
						fmt.Fprintf(w,
							"    stri: left=%t right=%t reversed=%t\n",
							eyes.Stri.HasLeftEye(),
							eyes.Stri.HasRightEye(),
							eyes.Stri.EyeViewsReversed())
					}
					if eyes.Hero != nil {
						fmt.Fprintf(w,
							"    hero: %s (%d)\n",
							eyes.Hero.HeroEyeName(),
							eyes.Hero.HeroEye)
					}
					if eyes.Cams != nil && eyes.Cams.Blin != nil {
						fmt.Fprintf(w,
							"    baseline: %d um (%.1f mm)\n",
							eyes.Cams.Blin.Baseline,
							float64(eyes.Cams.Blin.Baseline)/1000.0)
					}
				}
				if vse.Vexu.Proj != nil && vse.Vexu.Proj.Prji != nil {
					fmt.Fprintf(w, "    projection: %s\n",
						vse.Vexu.Proj.Prji.ProjectionType)
				}
			}

			if vse.Hfov != nil {
				fmt.Fprintf(w,
					"  hfov: %d/1000 degrees (%.1f)\n",
					vse.Hfov.FieldOfView,
					float64(vse.Hfov.FieldOfView)/1000.0)
			}
		}

		// Check for oinf/linf sample groups
		for _, child := range stbl.Children {
			sgpd, ok := child.(*mp4.SgpdBox)
			if !ok {
				continue
			}
			switch sgpd.GroupingType {
			case "oinf":
				for _, entry := range sgpd.SampleGroupEntries {
					oinf, ok := entry.(*mp4.OinfSampleGroupEntry)
					if !ok {
						continue
					}
					fmt.Fprintf(w, "  oinf (Operating Points Information):\n")
					fmt.Fprintf(w, "    ScalabilityMask: 0x%04x\n",
						oinf.ScalabilityMask)
					fmt.Fprintf(w, "    ProfileTierLevels: %d\n",
						len(oinf.ProfileTierLevels))
					for j, ptl := range oinf.ProfileTierLevels {
						fmt.Fprintf(w,
							"      PTL[%d]: space=%d tier=%t profile=%d level=%d\n",
							j, ptl.GeneralProfileSpace, ptl.GeneralTierFlag,
							ptl.GeneralProfileIDC, ptl.GeneralLevelIDC)
					}
					fmt.Fprintf(w, "    OperatingPoints: %d\n",
						len(oinf.OperatingPoints))
					for j, op := range oinf.OperatingPoints {
						fmt.Fprintf(w,
							"      OP[%d]: olsIdx=%d maxTid=%d layers=%d dims=%dx%d-%dx%d\n",
							j, op.OutputLayerSetIdx, op.MaxTemporalID,
							len(op.Layers),
							op.MinPicWidth, op.MinPicHeight,
							op.MaxPicWidth, op.MaxPicHeight)
						for k, l := range op.Layers {
							fmt.Fprintf(w,
								"        layer[%d]: ptlIdx=%d layerId=%d output=%t\n",
								k, l.PtlIdx, l.LayerID, l.IsOutputLayer)
						}
					}
					fmt.Fprintf(w, "    DependencyLayers: %d\n",
						len(oinf.DependencyLayers))
					for j, dep := range oinf.DependencyLayers {
						fmt.Fprintf(w,
							"      Dep[%d]: layerId=%d dependsOn=%v dimIds=%v\n",
							j, dep.LayerID, dep.DependsOnLayers,
							dep.DimensionIds)
					}
				}
			case "linf":
				for _, entry := range sgpd.SampleGroupEntries {
					linf, ok := entry.(*mp4.LinfSampleGroupEntry)
					if !ok {
						continue
					}
					fmt.Fprintf(w, "  linf (Layer Information):\n")
					for j, l := range linf.Layers {
						fmt.Fprintf(w,
							"    Layer[%d]: layerId=%d minTid=%d maxTid=%d subFlags=0x%02x\n",
							j, l.LayerID, l.MinTemporalID,
							l.MaxTemporalID, l.SubLayerPresenceFlags)
					}
				}
			}
		}

		// Check for trgr
		if trak.Trgr != nil {
			fmt.Fprintf(w, "  trgr (Track Group):\n")
			for _, child := range trak.Trgr.Children {
				if cstg, ok := child.(*mp4.TrackGroupTypeBox); ok {
					fmt.Fprintf(w, "    %s: trackGroupID=%d\n",
						cstg.Type(), cstg.TrackGroupID)
				}
			}
		}
	}
	return nil
}

func runAdd(args []string, w io.Writer) error {
	fs := flag.NewFlagSet("add", flag.ContinueOnError)
	var fps float64
	var spatial bool
	var baseline, hfovVal uint
	var heroStr string
	var reversed bool
	fs.Float64Var(&fps, "fps", 0,
		"Frame rate (required for .hevc input, e.g., 23.976, 24, 30, 60)")
	fs.BoolVar(&spatial, "spatial", false,
		"Add Apple spatial video metadata (vexu/hfov)")
	fs.UintVar(&baseline, "baseline", 63500,
		"Camera baseline in micrometers (default: 63500 = 63.5mm)")
	fs.UintVar(&hfovVal, "hfov", 63500,
		"Horizontal FOV in 1/1000 degrees (default: 63500 = 63.5)")
	fs.StringVar(&heroStr, "hero", "left",
		"Hero eye: left, right, or none")
	fs.BoolVar(&reversed, "reversed", false,
		"Eye views are reversed (right eye is base layer)")
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr,
			"%s add [-fps <rate>] [-spatial] <input.hevc|mp4> <output.mp4>\n",
			appName)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if fs.NArg() != 2 {
		fs.Usage()
		return fmt.Errorf("need input and output files")
	}

	inPath := fs.Arg(0)
	outPath := fs.Arg(1)

	var inp *mvhevcInput
	var err error

	if isMp4Input(inPath) {
		inp, err = parseMp4Input(inPath, w)
	} else {
		if fps <= 0 {
			return fmt.Errorf("-fps is required for Annex B input")
		}
		inp, err = parseAnnexBInput(inPath, fps, w)
	}
	if err != nil {
		return err
	}

	if spatial {
		inp.addSpatial = true
		inp.baseline = uint32(baseline)
		inp.hfov = uint32(hfovVal)
		inp.projType = "rect"

		var stereoFlags byte = 0x03 // hasLeft | hasRight
		if reversed {
			stereoFlags |= 0x08
		}
		inp.stereoFlags = stereoFlags

		switch heroStr {
		case "left":
			inp.heroEye = 1
		case "right":
			inp.heroEye = 2
		case "none":
			inp.heroEye = 0
		default:
			return fmt.Errorf("invalid -hero value: %s", heroStr)
		}
	}

	return buildAndWriteMp4(inp, outPath, w)
}

// isMp4Input returns true if the file extension suggests an MP4 container.
func isMp4Input(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".mp4" || ext == ".m4v" || ext == ".mov"
}

// parseAnnexBInput reads an Annex B HEVC bitstream and extracts MV-HEVC data.
func parseAnnexBInput(inPath string, fps float64, w io.Writer) (*mvhevcInput, error) {
	data, err := os.ReadFile(inPath)
	if err != nil {
		return nil, fmt.Errorf("could not read input: %w", err)
	}

	fmt.Fprintf(w, "Parsing Annex B bitstream (%d bytes)...\n", len(data))

	allNalus := avc.ExtractNalusFromByteStream(data)
	if len(allNalus) == 0 {
		return nil, fmt.Errorf("no NALUs found in bitstream")
	}
	fmt.Fprintf(w, "Found %d NALUs\n", len(allNalus))

	var baseVPS, baseSPS, basePPS, baseSEI [][]byte
	var enhSPS, enhPPS [][]byte

	type sampleNalu struct {
		nalu    []byte
		layerID byte
	}
	var videoNalus []sampleNalu
	var currentAU []sampleNalu
	lastIsVideo := false

	for _, nalu := range allNalus {
		if len(nalu) < 2 {
			continue
		}
		info := hevc.ParseNaluHeader(nalu)
		naluType := info.Type
		layerID := info.LayerID

		switch {
		case naluType == hevc.NALU_VPS:
			if layerID == 0 {
				baseVPS = append(baseVPS, nalu)
			}
		case naluType == hevc.NALU_SPS:
			if layerID == 0 {
				baseSPS = append(baseSPS, nalu)
			} else {
				enhSPS = append(enhSPS, nalu)
			}
		case naluType == hevc.NALU_PPS:
			if layerID == 0 {
				basePPS = append(basePPS, nalu)
			} else {
				enhPPS = append(enhPPS, nalu)
			}
		case naluType == hevc.NALU_SEI_PREFIX ||
			naluType == hevc.NALU_SEI_SUFFIX:
			if layerID == 0 {
				baseSEI = append(baseSEI, nalu)
			}
		case naluType <= 31: // Video NALUs (slice types)
			sn := sampleNalu{nalu: nalu, layerID: layerID}
			if lastIsVideo {
				if layerID == 0 && len(currentAU) > 0 {
					videoNalus = append(videoNalus, currentAU...)
					videoNalus = append(videoNalus,
						sampleNalu{nalu: nil}) // sentinel
					currentAU = nil
				}
			}
			currentAU = append(currentAU, sn)
			lastIsVideo = true
		default:
			lastIsVideo = false
		}
	}
	if len(currentAU) > 0 {
		videoNalus = append(videoNalus, currentAU...)
	}

	// Group NALUs into samples (access units)
	var samples []mvhevcSample
	var curData []byte
	var curSize uint32
	var curSync bool
	for _, sn := range videoNalus {
		if sn.nalu == nil {
			if curSize > 0 {
				samples = append(samples, mvhevcSample{
					data: curData, size: curSize, isSync: curSync,
				})
				curData = nil
				curSize = 0
				curSync = false
			}
			continue
		}
		naluType := hevc.GetNaluType(sn.nalu[0])
		lenBuf := make([]byte, 4)
		binary.BigEndian.PutUint32(lenBuf, uint32(len(sn.nalu)))
		curData = append(curData, lenBuf...)
		curData = append(curData, sn.nalu...)
		curSize += 4 + uint32(len(sn.nalu))
		if sn.layerID == 0 && (naluType == hevc.NALU_IDR_W_RADL ||
			naluType == hevc.NALU_IDR_N_LP ||
			naluType == hevc.NALU_CRA) {
			curSync = true
		}
	}
	if curSize > 0 {
		samples = append(samples, mvhevcSample{
			data: curData, size: curSize, isSync: curSync,
		})
	}

	fmt.Fprintf(w, "Samples: %d\n", len(samples))
	if len(samples) == 0 {
		return nil, fmt.Errorf("no video samples found")
	}
	fmt.Fprintf(w, "Base VPS: %d, SPS: %d, PPS: %d, SEI: %d\n",
		len(baseVPS), len(baseSPS), len(basePPS), len(baseSEI))
	fmt.Fprintf(w, "Enhancement SPS: %d, PPS: %d\n", len(enhSPS), len(enhPPS))

	if len(baseVPS) == 0 {
		return nil, fmt.Errorf("no VPS found in bitstream")
	}
	vps, err := hevc.ParseVPSNALUnit(baseVPS[0])
	if err != nil {
		return nil, fmt.Errorf("VPS parse error: %w", err)
	}
	fmt.Fprintf(w, "VPS: layers=%d views=%d multiLayer=%t\n",
		vps.GetNumLayers(), vps.GetNumViews(), vps.IsMultiLayer())

	if len(baseSPS) == 0 {
		return nil, fmt.Errorf("no SPS found in bitstream")
	}
	parsedSPS, err := hevc.ParseSPSNALUnit(baseSPS[0])
	if err != nil {
		return nil, fmt.Errorf("SPS parse error: %w", err)
	}
	imgW, imgH := parsedSPS.ImageSize()

	timeScale, sampleDur := fpsToTimescale(fps)
	fmt.Fprintf(w, "Timescale: %d, SampleDur: %d (%.3f fps)\n",
		timeScale, sampleDur, fps)

	return &mvhevcInput{
		baseVPS: baseVPS, baseSPS: baseSPS,
		basePPS: basePPS, baseSEI: baseSEI,
		enhSPS: enhSPS, enhPPS: enhPPS,
		vps:       vps,
		width:     uint16(imgW),
		height:    uint16(imgH),
		timeScale: timeScale,
		sampleDur: sampleDur,
		samples:   samples,
	}, nil
}

// parseMp4Input reads an MV-HEVC MP4 file and extracts its data.
func parseMp4Input(inPath string, w io.Writer) (*mvhevcInput, error) {
	ifd, err := os.Open(inPath)
	if err != nil {
		return nil, fmt.Errorf("could not open input: %w", err)
	}
	defer ifd.Close()

	parsedMp4, err := mp4.DecodeFile(ifd,
		mp4.WithDecodeMode(mp4.DecModeLazyMdat))
	if err != nil {
		return nil, fmt.Errorf("could not decode MP4: %w", err)
	}
	if parsedMp4.Moov == nil {
		return nil, fmt.Errorf("no moov box found")
	}

	// Find video trak
	var trak *mp4.TrakBox
	for _, t := range parsedMp4.Moov.Traks {
		if t.Mdia != nil && t.Mdia.Hdlr != nil &&
			t.Mdia.Hdlr.HandlerType == "vide" {
			trak = t
			break
		}
	}
	if trak == nil {
		return nil, fmt.Errorf("no video track found")
	}

	stbl := trak.Mdia.Minf.Stbl
	if stbl == nil || stbl.Stsd == nil {
		return nil, fmt.Errorf("incomplete sample table")
	}

	// Find HEVC visual sample entry
	var vse *mp4.VisualSampleEntryBox
	for _, child := range stbl.Stsd.Children {
		if v, ok := child.(*mp4.VisualSampleEntryBox); ok {
			if v.HvcC != nil {
				vse = v
				break
			}
		}
	}
	if vse == nil {
		return nil, fmt.Errorf("no HEVC sample entry found")
	}

	fmt.Fprintf(w, "Input MP4: %s (%dx%d)\n",
		vse.Type(), vse.Width, vse.Height)

	// Extract parameter sets from hvcC
	hdcr := vse.HvcC.DecConfRec
	baseVPS := hdcr.GetNalusForType(hevc.NALU_VPS)
	baseSPS := hdcr.GetNalusForType(hevc.NALU_SPS)
	basePPS := hdcr.GetNalusForType(hevc.NALU_PPS)
	baseSEI := hdcr.GetNalusForType(hevc.NALU_SEI_PREFIX)

	// Extract enhancement layer parameter sets from lhvC
	var enhSPS, enhPPS [][]byte
	if vse.LhvC != nil {
		enhSPS = vse.LhvC.DecConfRec.GetNalusForType(hevc.NALU_SPS)
		enhPPS = vse.LhvC.DecConfRec.GetNalusForType(hevc.NALU_PPS)
	}

	fmt.Fprintf(w, "Base VPS: %d, SPS: %d, PPS: %d, SEI: %d\n",
		len(baseVPS), len(baseSPS), len(basePPS), len(baseSEI))
	fmt.Fprintf(w, "Enhancement SPS: %d, PPS: %d\n",
		len(enhSPS), len(enhPPS))

	// Parse VPS
	if len(baseVPS) == 0 {
		return nil, fmt.Errorf("no VPS found in hvcC")
	}
	vps, err := hevc.ParseVPSNALUnit(baseVPS[0])
	if err != nil {
		return nil, fmt.Errorf("VPS parse error: %w", err)
	}
	fmt.Fprintf(w, "VPS: layers=%d views=%d multiLayer=%t\n",
		vps.GetNumLayers(), vps.GetNumViews(), vps.IsMultiLayer())

	// Get timing from mdhd and stts
	timeScale := trak.Mdia.Mdhd.Timescale
	var sampleDur uint32
	if stbl.Stts != nil && len(stbl.Stts.SampleTimeDelta) > 0 {
		sampleDur = stbl.Stts.SampleTimeDelta[0]
	}
	if sampleDur == 0 {
		return nil, fmt.Errorf("could not determine sample duration from stts")
	}
	fmt.Fprintf(w, "Timescale: %d, SampleDur: %d\n", timeScale, sampleDur)

	// Read sample data
	nrSamples := trak.GetNrSamples()
	fmt.Fprintf(w, "Reading %d samples...\n", nrSamples)

	// Get sync sample info
	syncMap := make(map[uint32]bool)
	if stbl.Stss != nil {
		for _, sn := range stbl.Stss.SampleNumber {
			syncMap[sn] = true
		}
	}

	// Read all sample data and build samples
	samples := make([]mvhevcSample, 0, nrSamples)
	mdat := parsedMp4.Mdat

	dataRanges, err := trak.GetRangesForSampleInterval(1, nrSamples)
	if err != nil {
		return nil, fmt.Errorf("get data ranges: %w", err)
	}

	// Build a per-sample view by iterating chunks and samples
	sampleNr := uint32(1)
	for _, dr := range dataRanges {
		chunkData, err := mdat.ReadData(int64(dr.Offset), int64(dr.Size), ifd)
		if err != nil {
			return nil, fmt.Errorf("read sample data at offset %d: %w",
				dr.Offset, err)
		}
		// Split chunk data into individual samples by size
		offset := uint32(0)
		for offset < uint32(len(chunkData)) && sampleNr <= nrSamples {
			sampleSize := stbl.Stsz.GetSampleSize(int(sampleNr))
			if offset+sampleSize > uint32(len(chunkData)) {
				break
			}
			sData := make([]byte, sampleSize)
			copy(sData, chunkData[offset:offset+sampleSize])
			samples = append(samples, mvhevcSample{
				data:   sData,
				size:   sampleSize,
				isSync: syncMap[sampleNr],
			})
			offset += sampleSize
			sampleNr++
		}
	}

	fmt.Fprintf(w, "Parsed %d samples\n", len(samples))

	inp := &mvhevcInput{
		baseVPS: baseVPS, baseSPS: baseSPS,
		basePPS: basePPS, baseSEI: baseSEI,
		enhSPS: enhSPS, enhPPS: enhPPS,
		vps:       vps,
		width:     vse.Width,
		height:    vse.Height,
		timeScale: timeScale,
		sampleDur: sampleDur,
		samples:   samples,
	}

	// Carry over existing spatial metadata from input
	if vse.Vexu != nil {
		inp.addSpatial = true
		if vse.Vexu.Eyes != nil {
			if vse.Vexu.Eyes.Stri != nil {
				inp.stereoFlags = vse.Vexu.Eyes.Stri.StereoFlags
			}
			if vse.Vexu.Eyes.Hero != nil {
				inp.heroEye = vse.Vexu.Eyes.Hero.HeroEye
			}
			if vse.Vexu.Eyes.Cams != nil &&
				vse.Vexu.Eyes.Cams.Blin != nil {
				inp.baseline = vse.Vexu.Eyes.Cams.Blin.Baseline
			}
		}
		if vse.Vexu.Proj != nil && vse.Vexu.Proj.Prji != nil {
			inp.projType = vse.Vexu.Proj.Prji.ProjectionType
		}
	}
	if vse.Hfov != nil {
		inp.hfov = vse.Hfov.FieldOfView
	}

	return inp, nil
}

// buildAndWriteMp4 builds a progressive MP4 from the parsed MV-HEVC input.
func buildAndWriteMp4(inp *mvhevcInput, outPath string, w io.Writer) error {
	// Create hvcC (base layer config)
	hvcC, err := mp4.CreateHvcC(inp.baseVPS, inp.baseSPS, inp.basePPS,
		true, true, true, true)
	if err != nil {
		return fmt.Errorf("CreateHvcC: %w", err)
	}

	// Add SEI to hvcC if present
	if len(inp.baseSEI) > 0 {
		hvcC.AddNaluArrays([]hevc.NaluArray{
			hevc.NewNaluArray(true, hevc.NALU_SEI_PREFIX, inp.baseSEI),
		})
	}

	// Create lhvC (enhancement layer config)
	var lhvC *mp4.LhvCBox
	if len(inp.enhSPS) > 0 || len(inp.enhPPS) > 0 {
		lhvC = mp4.CreateLhvCFromNalus(inp.enhSPS, inp.enhPPS)
	}

	// Create progressive MP4
	outFile := mp4.NewFile()
	outFile.AddChild(mp4.NewFtyp("iso4", 1, []string{"iso4"}), 0)

	moov := mp4.NewMoovBox()
	moov.AddChild(mp4.CreateMvhd())

	trak := &mp4.TrakBox{}
	tkhd := mp4.CreateTkhd()
	tkhd.TrackID = 1
	tkhd.Width = mp4.Fixed32(inp.width) << 16
	tkhd.Height = mp4.Fixed32(inp.height) << 16
	trak.AddChild(tkhd)

	mdia := &mp4.MdiaBox{}
	mdhd := &mp4.MdhdBox{Timescale: inp.timeScale}
	mdia.AddChild(mdhd)
	hdlr, _ := mp4.CreateHdlr("vide")
	mdia.AddChild(hdlr)

	minf := &mp4.MinfBox{}
	minf.AddChild(&mp4.VmhdBox{})
	dinf := &mp4.DinfBox{}
	dref := &mp4.DrefBox{}
	url := &mp4.URLBox{Flags: 1}
	dref.AddChild(url)
	dinf.AddChild(dref)
	minf.AddChild(dinf)

	stbl := &mp4.StblBox{}

	// stsd with hvc1 sample entry
	stsd := mp4.NewStsdBox()
	vse := mp4.CreateVisualSampleEntryBox("hvc1",
		inp.width, inp.height, hvcC)
	if lhvC != nil {
		vse.AddChild(lhvC)
	}
	if inp.addSpatial {
		vexu := mp4.CreateVexuBox(inp.stereoFlags, inp.heroEye,
			inp.baseline, inp.projType)
		vse.AddChild(vexu)
		vse.AddChild(&mp4.HfovBox{FieldOfView: inp.hfov})
	}
	stsd.AddChild(vse)
	stbl.AddChild(stsd)

	// stts - constant duration
	stts := &mp4.SttsBox{}
	stts.SampleCount = append(stts.SampleCount, uint32(len(inp.samples)))
	stts.SampleTimeDelta = append(stts.SampleTimeDelta, inp.sampleDur)
	stbl.AddChild(stts)

	// stss - sync samples
	stss := &mp4.StssBox{}
	for i, s := range inp.samples {
		if s.isSync {
			stss.SampleNumber = append(stss.SampleNumber, uint32(i+1))
		}
	}
	if len(stss.SampleNumber) > 0 {
		stbl.AddChild(stss)
	}

	// stsz - sample sizes
	stsz := &mp4.StszBox{SampleNumber: uint32(len(inp.samples))}
	for _, s := range inp.samples {
		stsz.SampleSize = append(stsz.SampleSize, s.size)
	}
	stbl.AddChild(stsz)

	// stsc + stco - one chunk containing all samples
	stsc := &mp4.StscBox{}
	stsc.Entries = append(stsc.Entries, mp4.StscEntry{
		FirstChunk:      1,
		SamplesPerChunk: uint32(len(inp.samples)),
	})
	stsc.SetSingleSampleDescriptionID(1)
	stbl.AddChild(stsc)

	stco := &mp4.StcoBox{}
	stco.ChunkOffset = append(stco.ChunkOffset, 0) // placeholder
	stbl.AddChild(stco)

	// sgpd + sbgp for oinf and linf
	if inp.vps.IsMultiLayer() {
		oinf := mp4.BuildOinfFromVPS(inp.vps)
		sgpdOinf := &mp4.SgpdBox{
			Version:       2,
			GroupingType:  "oinf",
			DefaultLength: uint32(oinf.Size()),
			SampleGroupEntries: []mp4.SampleGroupEntry{oinf},
		}
		stbl.AddChild(sgpdOinf)

		sbgpOinf := &mp4.SbgpBox{GroupingType: "oinf"}
		sbgpOinf.SampleCounts = append(sbgpOinf.SampleCounts,
			uint32(len(inp.samples)))
		sbgpOinf.GroupDescriptionIndices = append(
			sbgpOinf.GroupDescriptionIndices, 1)
		stbl.AddChild(sbgpOinf)

		maxTids := make([]byte, inp.vps.GetNumLayers())
		linf := mp4.BuildLinfFromVPS(inp.vps, maxTids)
		sgpdLinf := &mp4.SgpdBox{
			Version:       2,
			GroupingType:  "linf",
			DefaultLength: uint32(linf.Size()),
			SampleGroupEntries: []mp4.SampleGroupEntry{linf},
		}
		stbl.AddChild(sgpdLinf)

		sbgpLinf := &mp4.SbgpBox{GroupingType: "linf"}
		sbgpLinf.SampleCounts = append(sbgpLinf.SampleCounts,
			uint32(len(inp.samples)))
		sbgpLinf.GroupDescriptionIndices = append(
			sbgpLinf.GroupDescriptionIndices, 1)
		stbl.AddChild(sbgpLinf)
	}

	minf.AddChild(stbl)
	mdia.AddChild(minf)
	trak.AddChild(mdia)

	// trgr/cstg
	if inp.vps.IsMultiLayer() {
		trgr := &mp4.TrgrBox{}
		cstg := mp4.CreateTrackGroupTypeBox("cstg", 1001)
		trgr.AddChild(cstg)
		trak.AddChild(trgr)
	}

	moov.AddChild(trak)

	// Set durations
	totalDurMedia := uint64(len(inp.samples)) * uint64(inp.sampleDur)
	mdhd.Duration = totalDurMedia
	moov.Mvhd.Timescale = 600
	moov.Mvhd.Duration = totalDurMedia *
		uint64(moov.Mvhd.Timescale) / uint64(inp.timeScale)
	tkhd.Duration = moov.Mvhd.Duration

	outFile.AddChild(moov, 0)

	// Build mdat from sample data
	var mdatData []byte
	for _, s := range inp.samples {
		mdatData = append(mdatData, s.data...)
	}

	mdatBox := &mp4.MdatBox{}
	mdatBox.SetData(mdatData)
	outFile.AddChild(mdatBox, 0)

	// Write output
	ofd, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("could not create output file: %w", err)
	}
	defer ofd.Close()

	err = outFile.Encode(ofd)
	if err != nil {
		return fmt.Errorf("encode error: %w", err)
	}

	fmt.Fprintf(w, "Wrote %s (%dx%d, %d samples, %d layers)\n",
		outPath, inp.width, inp.height, len(inp.samples),
		inp.vps.GetNumLayers())
	return nil
}

// fpsToTimescale converts an FPS value to a timescale and sample duration.
func fpsToTimescale(fps float64) (timeScale uint32, sampleDur uint32) {
	switch {
	case fps > 23.975 && fps < 23.977: // 23.976
		return 24000, 1001
	case fps > 29.969 && fps < 29.971: // 29.97
		return 30000, 1001
	case fps > 59.939 && fps < 59.941: // 59.94
		return 60000, 1001
	default:
		ts := uint32(fps * 1000)
		return ts, 1000
	}
}
