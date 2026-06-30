// mp4ff-mvhevc is a tool to inspect and create MV-HEVC (Multi-View HEVC) MP4 files.
package main

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/Eyevinn/mp4ff/avc"
	"github.com/Eyevinn/mp4ff/hevc"
	"github.com/Eyevinn/mp4ff/internal"
	"github.com/Eyevinn/mp4ff/mp4"
)

const appName = "mp4ff-mvhevc"

var usg = `%s inspects and creates MV-HEVC (Multi-View HEVC) MP4 files.

Subcommands:
  info [-idr] <input.mp4>                          Display MV-HEVC metadata
  add  [options] <input.hevc|mp4> <output.mp4>     Mux into an MV-HEVC MP4

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
		return fmt.Errorf("need a subcommand: info or add")
	}
	switch args[1] {
	case "info":
		return runInfo(args[1:], w)
	case "add":
		return runAdd(args[1:], w)
	case "version", "-version", "--version":
		fmt.Fprintf(w, "%s %s\n", appName, internal.GetVersion())
		return nil
	default:
		fmt.Fprintf(os.Stderr, usg, appName, appName)
		return fmt.Errorf("unknown subcommand: %s", args[1])
	}
}

type infoOptions struct {
	idr bool
}

func parseInfoOptions(fs *flag.FlagSet, args []string) (*infoOptions, error) {
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "%s info [-idr] <input.mp4>\n\noptions:\n", appName)
		fs.PrintDefaults()
	}
	opts := infoOptions{}
	fs.BoolVar(&opts.idr, "idr", false, "show IDR (sync) frame positions")
	err := fs.Parse(args[1:])
	return &opts, err
}

func runInfo(args []string, w io.Writer) error {
	fs := flag.NewFlagSet("info", flag.ContinueOnError)
	opts, err := parseInfoOptions(fs, args)
	if err != nil {
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
		return fmt.Errorf("could not decode mp4: %w", err)
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
		for _, child := range stbl.Stsd.Children {
			vse, ok := child.(*mp4.VisualSampleEntryBox)
			if !ok {
				continue
			}
			printSampleEntry(w, vse)
		}

		timeScale := trak.Mdia.Mdhd.Timescale
		if parsedMp4.IsFragmented() {
			printFragmentedInfo(parsedMp4, trak.Tkhd.TrackID, timeScale, opts.idr, w)
		} else {
			printProgressiveInfo(w, trak, stbl, timeScale, opts.idr)
		}

		for _, child := range stbl.Children {
			if sgpd, ok := child.(*mp4.SgpdBox); ok {
				printSampleGroup(w, sgpd)
			}
		}

		if trak.Trgr != nil {
			fmt.Fprintf(w, "  trgr (Track Group):\n")
			for _, child := range trak.Trgr.Children {
				if tgt, ok := child.(*mp4.TrackGroupTypeBox); ok {
					fmt.Fprintf(w, "    %s: trackGroupID=%d\n", tgt.Type(), tgt.TrackGroupID)
				}
			}
		}
	}
	return nil
}

func printSampleEntry(w io.Writer, vse *mp4.VisualSampleEntryBox) {
	fmt.Fprintf(w, "  Sample entry: %s (%dx%d)\n", vse.Type(), vse.Width, vse.Height)

	if vse.HvcC != nil {
		hdcr := vse.HvcC.DecConfRec
		fmt.Fprintf(w, "  hvcC (base layer config):\n")
		fmt.Fprintf(w, "    Profile: space=%d tier=%t idc=%d level=%d\n",
			hdcr.GeneralProfileSpace, hdcr.GeneralTierFlag, hdcr.GeneralProfileIDC, hdcr.GeneralLevelIDC)
		fmt.Fprintf(w, "    Chroma: %d  BitDepth: luma=%d chroma=%d\n",
			hdcr.ChromaFormatIDC, hdcr.BitDepthLumaMinus8+8, hdcr.BitDepthChromaMinus8+8)
		fmt.Fprintf(w, "    NumTemporalLayers: %d  LengthSize: %d\n",
			hdcr.NumTemporalLayers, hdcr.LengthSizeMinusOne+1)
		if vpsNalus := hdcr.GetNalusForType(hevc.NALU_VPS); len(vpsNalus) > 0 {
			if vps, err := hevc.ParseVPSNALUnit(vpsNalus[0]); err != nil {
				fmt.Fprintf(w, "    VPS parse error: %v\n", err)
			} else {
				fmt.Fprintf(w, "    VPS: layers=%d views=%d multiLayer=%t\n",
					vps.GetNumLayers(), vps.GetNumViews(), vps.IsMultiLayer())
			}
		}
		for _, array := range hdcr.NaluArrays {
			fmt.Fprintf(w, "    %s: %d nalus (complete=%d)\n", array.NaluType(), len(array.Nalus), array.Complete())
		}
	}

	if vse.LhvC != nil {
		hdcr := vse.LhvC.DecConfRec
		fmt.Fprintf(w, "  lhvC (enhancement layer config):\n")
		fmt.Fprintf(w, "    NumTemporalLayers: %d  LengthSize: %d\n",
			hdcr.NumTemporalLayers, hdcr.LengthSizeMinusOne+1)
		for _, array := range hdcr.NaluArrays {
			fmt.Fprintf(w, "    %s: %d nalus (complete=%d)\n", array.NaluType(), len(array.Nalus), array.Complete())
			for _, nalu := range array.Nalus {
				fmt.Fprintf(w, "      %s\n", hex.EncodeToString(nalu))
			}
		}
	}

	if vse.Vexu != nil {
		fmt.Fprintf(w, "  vexu (Spatial Video):\n")
		if eyes := vse.Vexu.Eyes; eyes != nil {
			if eyes.Stri != nil {
				fmt.Fprintf(w, "    stri: left=%t right=%t additional=%t reversed=%t\n",
					eyes.Stri.HasLeftEye(), eyes.Stri.HasRightEye(),
					eyes.Stri.HasAdditionalViews(), eyes.Stri.EyeViewsReversed())
			}
			if eyes.Hero != nil {
				fmt.Fprintf(w, "    hero: %s (%d)\n", eyes.Hero.HeroEyeName(), eyes.Hero.HeroEye)
			}
			if eyes.Cams != nil && eyes.Cams.Blin != nil {
				fmt.Fprintf(w, "    baseline: %d um (%.3f mm)\n",
					eyes.Cams.Blin.Baseline, float64(eyes.Cams.Blin.Baseline)/1000.0)
			}
		}
		if vse.Vexu.Proj != nil && vse.Vexu.Proj.Prji != nil {
			fmt.Fprintf(w, "    projection: %s\n", vse.Vexu.Proj.Prji.ProjectionType)
		}
	}

	if vse.Hfov != nil {
		fmt.Fprintf(w, "  hfov: %d/1000 degrees (%.3f)\n",
			vse.Hfov.FieldOfView, float64(vse.Hfov.FieldOfView)/1000.0)
	}
}

func printProgressiveInfo(w io.Writer, trak *mp4.TrakBox, stbl *mp4.StblBox, timeScale uint32, showIDR bool) {
	nrSamples := trak.GetNrSamples()
	var sampleDur uint32
	if stbl.Stts != nil && len(stbl.Stts.SampleTimeDelta) > 0 {
		sampleDur = stbl.Stts.SampleTimeDelta[0]
	}
	if sampleDur > 0 {
		fmt.Fprintf(w, "  Samples: %d, Timescale: %d, SampleDur: %d (%.3f fps)\n",
			nrSamples, timeScale, sampleDur, float64(timeScale)/float64(sampleDur))
	} else {
		fmt.Fprintf(w, "  Samples: %d, Timescale: %d\n", nrSamples, timeScale)
	}
	if stbl.Stss != nil && showIDR {
		printSyncFrames(w, stbl.Stss.SampleNumber)
	}
}

func printSyncFrames(w io.Writer, syncNrs []uint32) {
	fmt.Fprintf(w, "  Sync (IDR) frames (%d):\n", len(syncNrs))
	for i, sn := range syncNrs {
		if i == 0 {
			fmt.Fprintf(w, "    frame %d\n", sn)
		} else {
			fmt.Fprintf(w, "    frame %d (interval=%d)\n", sn, sn-syncNrs[i-1])
		}
	}
}

func printSampleGroup(w io.Writer, sgpd *mp4.SgpdBox) {
	switch sgpd.GroupingType {
	case "oinf":
		for _, entry := range sgpd.SampleGroupEntries {
			oinf, ok := entry.(*mp4.OinfSampleGroupEntry)
			if !ok {
				continue
			}
			fmt.Fprintf(w, "  oinf (Operating Points Information):\n")
			fmt.Fprintf(w, "    ScalabilityMask: 0x%04x\n", oinf.ScalabilityMask)
			fmt.Fprintf(w, "    ProfileTierLevels: %d\n", len(oinf.ProfileTierLevels))
			for j, ptl := range oinf.ProfileTierLevels {
				fmt.Fprintf(w, "      PTL[%d]: space=%d tier=%t profile=%d level=%d\n",
					j, ptl.GeneralProfileSpace, ptl.GeneralTierFlag, ptl.GeneralProfileIDC, ptl.GeneralLevelIDC)
			}
			fmt.Fprintf(w, "    OperatingPoints: %d\n", len(oinf.OperatingPoints))
			for j, op := range oinf.OperatingPoints {
				fmt.Fprintf(w, "      OP[%d]: olsIdx=%d maxTid=%d layers=%d dims=%dx%d-%dx%d\n",
					j, op.OutputLayerSetIdx, op.MaxTemporalID, len(op.Layers),
					op.MinPicWidth, op.MinPicHeight, op.MaxPicWidth, op.MaxPicHeight)
				for k, l := range op.Layers {
					fmt.Fprintf(w, "        layer[%d]: ptlIdx=%d layerId=%d output=%t\n",
						k, l.PtlIdx, l.LayerID, l.IsOutputLayer)
				}
			}
			fmt.Fprintf(w, "    DependencyLayers: %d\n", len(oinf.DependencyLayers))
			for j, dep := range oinf.DependencyLayers {
				fmt.Fprintf(w, "      Dep[%d]: layerId=%d dependsOn=%v dimIds=%v\n",
					j, dep.LayerID, dep.DependsOnLayers, dep.DimensionIds)
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
				fmt.Fprintf(w, "    Layer[%d]: layerId=%d minTid=%d maxTid=%d subFlags=0x%02x\n",
					j, l.LayerID, l.MinTemporalID, l.MaxTemporalID, l.SubLayerPresenceFlags)
			}
		}
	}
}

// printFragmentedInfo prints timing and sync sample info for a fragmented file.
func printFragmentedInfo(f *mp4.File, trackID uint32, timeScale uint32, showIDR bool, w io.Writer) {
	var totalSamples, sampleDur, sampleNr uint32
	var syncFrames []uint32
	for _, seg := range f.Segments {
		for _, frag := range seg.Fragments {
			if frag.Moof == nil {
				continue
			}
			for _, traf := range frag.Moof.Trafs {
				if traf.Tfhd.TrackID != trackID {
					continue
				}
				var defaultFlags uint32
				if traf.Tfhd.HasDefaultSampleFlags() {
					defaultFlags = traf.Tfhd.DefaultSampleFlags
				}
				if sampleDur == 0 && traf.Tfhd.HasDefaultSampleDuration() {
					sampleDur = traf.Tfhd.DefaultSampleDuration
				}
				for _, trun := range traf.Truns {
					for i, s := range trun.Samples {
						sampleNr++
						if sampleDur == 0 && s.Dur > 0 {
							sampleDur = s.Dur
						}
						flags := s.Flags
						if !trun.HasSampleFlags() {
							if i == 0 && trun.HasFirstSampleFlags() {
								flags, _ = trun.FirstSampleFlags()
							} else {
								flags = defaultFlags
							}
						}
						if mp4.IsSyncSampleFlags(flags) {
							syncFrames = append(syncFrames, sampleNr)
						}
					}
					totalSamples += trun.SampleCount()
				}
			}
		}
	}
	if sampleDur > 0 {
		fmt.Fprintf(w, "  Samples: %d, Timescale: %d, SampleDur: %d (%.3f fps)\n",
			totalSamples, timeScale, sampleDur, float64(timeScale)/float64(sampleDur))
	} else {
		fmt.Fprintf(w, "  Samples: %d, Timescale: %d\n", totalSamples, timeScale)
	}
	if showIDR && len(syncFrames) > 0 {
		printSyncFrames(w, syncFrames)
	}
}

type addOptions struct {
	fps      float64
	spatial  bool
	baseline uint
	hfov     uint
	hero     string
	reversed bool
}

func parseAddOptions(fs *flag.FlagSet, args []string) (*addOptions, error) {
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "%s add [options] <input.hevc|mp4> <output.mp4>\n\noptions:\n", appName)
		fs.PrintDefaults()
	}
	opts := addOptions{}
	fs.Float64Var(&opts.fps, "fps", 0, "frame rate, required for .hevc input (e.g. 23.976, 24, 30, 60)")
	fs.BoolVar(&opts.spatial, "spatial", false, "add Apple spatial video metadata (vexu/hfov)")
	fs.UintVar(&opts.baseline, "baseline", 63500, "camera baseline in micrometers")
	fs.UintVar(&opts.hfov, "hfov", 63500, "horizontal field of view in 1/1000 degrees")
	fs.StringVar(&opts.hero, "hero", "left", "hero eye: left, right, or none")
	fs.BoolVar(&opts.reversed, "reversed", false, "eye views are reversed (right eye is base layer)")
	err := fs.Parse(args[1:])
	return &opts, err
}

func runAdd(args []string, w io.Writer) error {
	fs := flag.NewFlagSet("add", flag.ContinueOnError)
	opts, err := parseAddOptions(fs, args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if fs.NArg() != 2 {
		fs.Usage()
		return fmt.Errorf("need input and output files")
	}
	inPath, outPath := fs.Arg(0), fs.Arg(1)

	var inp *mvhevcInput
	if isMp4Input(inPath) {
		inp, err = parseMp4Input(inPath, w)
	} else {
		if opts.fps <= 0 {
			return fmt.Errorf("-fps is required for Annex B input")
		}
		inp, err = parseAnnexBInput(inPath, opts.fps, w)
	}
	if err != nil {
		return err
	}

	if opts.spatial {
		inp.addSpatial = true
		inp.baseline = uint32(opts.baseline)
		inp.hfov = uint32(opts.hfov)
		inp.projType = "rect"
		inp.stereoFlags = mp4.StriHasLeftEyeView | mp4.StriHasRightEyeView
		if opts.reversed {
			inp.stereoFlags |= mp4.StriEyeViewsReversed
		}
		switch opts.hero {
		case "left":
			inp.heroEye = 1
		case "right":
			inp.heroEye = 2
		case "none":
			inp.heroEye = 0
		default:
			return fmt.Errorf("invalid -hero value: %s", opts.hero)
		}
	}

	return buildAndWriteMp4(inp, outPath, w)
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

	addSpatial  bool
	stereoFlags byte
	heroEye     byte
	baseline    uint32 // micrometers
	hfov        uint32 // thousandths of a degree
	projType    string
}

// mvhevcSample holds the length-prefixed NALU data of a single access unit.
type mvhevcSample struct {
	data   []byte
	size   uint32
	isSync bool
	cto    int32 // composition time offset (PTS-DTS), 0 for Annex B input
}

// isMp4Input returns true if the file extension suggests an MP4 container.
func isMp4Input(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".mp4", ".m4v", ".mov":
		return true
	}
	return false
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

	// Deduplicate parameter sets: encoders repeat VPS/SPS/PPS before every IDR
	// but the decoder configuration records should hold only unique entries.
	seen := make(map[string]bool)
	addUnique := func(dst *[][]byte, nalu []byte) {
		if key := string(nalu); !seen[key] {
			seen[key] = true
			*dst = append(*dst, nalu)
		}
	}

	for _, nalu := range allNalus {
		if len(nalu) < 2 {
			continue
		}
		info := hevc.ParseNaluHeader(nalu)
		switch {
		case info.Type == hevc.NALU_VPS:
			if info.LayerID == 0 {
				addUnique(&baseVPS, nalu)
			}
		case info.Type == hevc.NALU_SPS:
			if info.LayerID == 0 {
				addUnique(&baseSPS, nalu)
			} else {
				addUnique(&enhSPS, nalu)
			}
		case info.Type == hevc.NALU_PPS:
			if info.LayerID == 0 {
				addUnique(&basePPS, nalu)
			} else {
				addUnique(&enhPPS, nalu)
			}
		case info.Type == hevc.NALU_SEI_PREFIX:
			// Base-layer prefix SEI is hoisted into hvcC. Per-picture and suffix
			// SEI are not carried into the samples (a muxer simplification).
			if info.LayerID == 0 {
				addUnique(&baseSEI, nalu)
			}
		case info.Type <= 31: // VCL (slice) NALU types
			// A new access unit starts at the first slice of a base-layer picture.
			// first_slice_segment_in_pic_flag is the first bit of the slice header,
			// i.e. the top bit of the first byte after the 2-byte NAL unit header.
			firstSlice := len(nalu) > 2 && nalu[2]&0x80 != 0
			if info.LayerID == 0 && firstSlice && len(currentAU) > 0 {
				videoNalus = append(videoNalus, currentAU...)
				videoNalus = append(videoNalus, sampleNalu{nalu: nil}) // AU boundary sentinel
				currentAU = nil
			}
			currentAU = append(currentAU, sampleNalu{nalu: nalu, layerID: info.LayerID})
		}
	}
	if len(currentAU) > 0 {
		videoNalus = append(videoNalus, currentAU...)
	}

	// Group NALUs into samples (access units) with 4-byte length prefixes.
	var samples []mvhevcSample
	var curData []byte
	var curSize uint32
	var curSync bool
	flush := func() {
		if curSize > 0 {
			samples = append(samples, mvhevcSample{data: curData, size: curSize, isSync: curSync})
			curData, curSize, curSync = nil, 0, false
		}
	}
	for _, sn := range videoNalus {
		if sn.nalu == nil {
			flush()
			continue
		}
		naluType := hevc.GetNaluType(sn.nalu[0])
		lenBuf := make([]byte, 4)
		binary.BigEndian.PutUint32(lenBuf, uint32(len(sn.nalu)))
		curData = append(curData, lenBuf...)
		curData = append(curData, sn.nalu...)
		curSize += 4 + uint32(len(sn.nalu))
		if sn.layerID == 0 && (naluType == hevc.NALU_IDR_W_RADL ||
			naluType == hevc.NALU_IDR_N_LP || naluType == hevc.NALU_CRA) {
			curSync = true
		}
	}
	flush()

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
	fmt.Fprintf(w, "Timescale: %d, SampleDur: %d (%.3f fps)\n", timeScale, sampleDur, fps)

	return &mvhevcInput{
		baseVPS: baseVPS, baseSPS: baseSPS, basePPS: basePPS, baseSEI: baseSEI,
		enhSPS: enhSPS, enhPPS: enhPPS,
		vps:       vps,
		width:     uint16(imgW),
		height:    uint16(imgH),
		timeScale: timeScale,
		sampleDur: sampleDur,
		samples:   samples,
	}, nil
}

// parseMp4Input reads an HEVC/MV-HEVC MP4 file and extracts its data.
func parseMp4Input(inPath string, w io.Writer) (*mvhevcInput, error) {
	ifd, err := os.Open(inPath)
	if err != nil {
		return nil, fmt.Errorf("could not open input: %w", err)
	}
	defer ifd.Close()

	parsedMp4, err := mp4.DecodeFile(ifd, mp4.WithDecodeMode(mp4.DecModeLazyMdat))
	if err != nil {
		return nil, fmt.Errorf("could not decode mp4: %w", err)
	}
	if parsedMp4.Moov == nil {
		return nil, fmt.Errorf("no moov box found")
	}

	var trak *mp4.TrakBox
	for _, t := range parsedMp4.Moov.Traks {
		if t.Mdia != nil && t.Mdia.Hdlr != nil && t.Mdia.Hdlr.HandlerType == "vide" {
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

	var vse *mp4.VisualSampleEntryBox
	for _, child := range stbl.Stsd.Children {
		if v, ok := child.(*mp4.VisualSampleEntryBox); ok && v.HvcC != nil {
			vse = v
			break
		}
	}
	if vse == nil {
		return nil, fmt.Errorf("no HEVC sample entry found")
	}
	fmt.Fprintf(w, "Input MP4: %s (%dx%d)\n", vse.Type(), vse.Width, vse.Height)

	hdcr := vse.HvcC.DecConfRec
	baseVPS := dedupNalus(hdcr.GetNalusForType(hevc.NALU_VPS))
	baseSPS := dedupNalus(hdcr.GetNalusForType(hevc.NALU_SPS))
	basePPS := dedupNalus(hdcr.GetNalusForType(hevc.NALU_PPS))
	baseSEI := dedupNalus(hdcr.GetNalusForType(hevc.NALU_SEI_PREFIX))

	var enhSPS, enhPPS [][]byte
	if vse.LhvC != nil {
		enhSPS = dedupNalus(vse.LhvC.GetNalusForType(hevc.NALU_SPS))
		enhPPS = dedupNalus(vse.LhvC.GetNalusForType(hevc.NALU_PPS))
	}
	fmt.Fprintf(w, "Base VPS: %d, SPS: %d, PPS: %d, SEI: %d\n",
		len(baseVPS), len(baseSPS), len(basePPS), len(baseSEI))
	fmt.Fprintf(w, "Enhancement SPS: %d, PPS: %d\n", len(enhSPS), len(enhPPS))

	if len(baseVPS) == 0 {
		return nil, fmt.Errorf("no VPS found in hvcC")
	}
	vps, err := hevc.ParseVPSNALUnit(baseVPS[0])
	if err != nil {
		return nil, fmt.Errorf("VPS parse error: %w", err)
	}
	fmt.Fprintf(w, "VPS: layers=%d views=%d multiLayer=%t\n",
		vps.GetNumLayers(), vps.GetNumViews(), vps.IsMultiLayer())

	timeScale := trak.Mdia.Mdhd.Timescale
	var sampleDur uint32
	if stbl.Stts != nil && len(stbl.Stts.SampleTimeDelta) > 0 {
		sampleDur = stbl.Stts.SampleTimeDelta[0]
	}
	if sampleDur == 0 {
		return nil, fmt.Errorf("could not determine sample duration from stts")
	}
	fmt.Fprintf(w, "Timescale: %d, SampleDur: %d\n", timeScale, sampleDur)

	nrSamples := trak.GetNrSamples()
	fmt.Fprintf(w, "Reading %d samples...\n", nrSamples)

	syncMap := make(map[uint32]bool)
	if stbl.Stss != nil {
		for _, sn := range stbl.Stss.SampleNumber {
			syncMap[sn] = true
		}
	}

	samples := make([]mvhevcSample, 0, nrSamples)
	mdat := parsedMp4.Mdat
	dataRanges, err := trak.GetRangesForSampleInterval(1, nrSamples)
	if err != nil {
		return nil, fmt.Errorf("get data ranges: %w", err)
	}
	sampleNr := uint32(1)
	for _, dr := range dataRanges {
		chunkData, err := mdat.ReadData(int64(dr.Offset), int64(dr.Size), ifd)
		if err != nil {
			return nil, fmt.Errorf("read sample data at offset %d: %w", dr.Offset, err)
		}
		offset := uint32(0)
		for offset < uint32(len(chunkData)) && sampleNr <= nrSamples {
			sampleSize := stbl.Stsz.GetSampleSize(int(sampleNr))
			if offset+sampleSize > uint32(len(chunkData)) {
				break
			}
			sData := make([]byte, sampleSize)
			copy(sData, chunkData[offset:offset+sampleSize])
			var cto int32
			if stbl.Ctts != nil {
				cto = stbl.Ctts.GetCompositionTimeOffset(sampleNr)
			}
			samples = append(samples, mvhevcSample{data: sData, size: sampleSize, isSync: syncMap[sampleNr], cto: cto})
			offset += sampleSize
			sampleNr++
		}
	}
	fmt.Fprintf(w, "Parsed %d samples\n", len(samples))

	inp := &mvhevcInput{
		baseVPS: baseVPS, baseSPS: baseSPS, basePPS: basePPS, baseSEI: baseSEI,
		enhSPS: enhSPS, enhPPS: enhPPS,
		vps:       vps,
		width:     vse.Width,
		height:    vse.Height,
		timeScale: timeScale,
		sampleDur: sampleDur,
		samples:   samples,
	}

	// Carry over any existing spatial metadata from the input sample entry.
	if vse.Vexu != nil {
		inp.addSpatial = true
		if eyes := vse.Vexu.Eyes; eyes != nil {
			if eyes.Stri != nil {
				inp.stereoFlags = eyes.Stri.StereoFlags
			}
			if eyes.Hero != nil {
				inp.heroEye = eyes.Hero.HeroEye
			}
			if eyes.Cams != nil && eyes.Cams.Blin != nil {
				inp.baseline = eyes.Cams.Blin.Baseline
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
	hvcC, err := mp4.CreateHvcC(inp.baseVPS, inp.baseSPS, inp.basePPS, true, true, true, true)
	if err != nil {
		return fmt.Errorf("CreateHvcC: %w", err)
	}
	if len(inp.baseSEI) > 0 {
		hvcC.AddNaluArrays([]hevc.NaluArray{hevc.NewNaluArray(true, hevc.NALU_SEI_PREFIX, inp.baseSEI)})
	}

	var lhvC *mp4.LhvCBox
	if len(inp.enhSPS) > 0 || len(inp.enhPPS) > 0 {
		lhvC = mp4.CreateLhvCFromNalus(inp.enhSPS, inp.enhPPS)
	}

	outFile := mp4.NewFile()
	outFile.AddChild(mp4.NewFtyp("isom", 0, []string{"isom", "iso2", "mp41"}), 0)

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
	hdlr, err := mp4.CreateHdlr("vide")
	if err != nil {
		return fmt.Errorf("CreateHdlr: %w", err)
	}
	mdia.AddChild(hdlr)

	minf := &mp4.MinfBox{}
	minf.AddChild(mp4.CreateVmhd())
	dinf := &mp4.DinfBox{}
	dref := &mp4.DrefBox{}
	dref.AddChild(&mp4.URLBox{Flags: 1})
	dinf.AddChild(dref)
	minf.AddChild(dinf)

	stbl := &mp4.StblBox{}

	stsd := mp4.NewStsdBox()
	vse := mp4.CreateVisualSampleEntryBox("hvc1", inp.width, inp.height, hvcC)
	if lhvC != nil {
		vse.AddChild(lhvC)
	}
	if inp.addSpatial {
		vse.AddChild(mp4.CreateVexuBox(inp.stereoFlags, inp.heroEye, inp.baseline, inp.projType))
		vse.AddChild(&mp4.HfovBox{FieldOfView: inp.hfov})
	}
	stsd.AddChild(vse)
	stbl.AddChild(stsd)

	stts := &mp4.SttsBox{}
	stts.SampleCount = append(stts.SampleCount, uint32(len(inp.samples)))
	stts.SampleTimeDelta = append(stts.SampleTimeDelta, inp.sampleDur)
	stbl.AddChild(stts)

	// ctts - carry over composition time offsets (B-frame reordering) if present,
	// run-length encoded. Annex B input has none (PTS == DTS).
	if ctts := buildCtts(inp.samples); ctts != nil {
		stbl.AddChild(ctts)
	}

	stss := &mp4.StssBox{}
	for i, s := range inp.samples {
		if s.isSync {
			stss.SampleNumber = append(stss.SampleNumber, uint32(i+1))
		}
	}
	if len(stss.SampleNumber) > 0 {
		stbl.AddChild(stss)
	}

	stsz := &mp4.StszBox{SampleNumber: uint32(len(inp.samples))}
	for _, s := range inp.samples {
		stsz.SampleSize = append(stsz.SampleSize, s.size)
	}
	stbl.AddChild(stsz)

	stsc := &mp4.StscBox{}
	stsc.Entries = append(stsc.Entries, mp4.StscEntry{FirstChunk: 1, SamplesPerChunk: uint32(len(inp.samples))})
	stsc.SetSingleSampleDescriptionID(1)
	stbl.AddChild(stsc)

	stco := &mp4.StcoBox{}
	stco.ChunkOffset = append(stco.ChunkOffset, 0) // placeholder, fixed up after layout
	stbl.AddChild(stco)

	// oinf and linf sample groups for the layered stream.
	if inp.vps.IsMultiLayer() {
		oinf, err := mp4.BuildOinfFromVPS(inp.vps)
		if err != nil {
			return fmt.Errorf("build oinf: %w", err)
		}
		addSampleGroup(stbl, "oinf", oinf, len(inp.samples))

		maxTids := make([]byte, inp.vps.GetNumLayers())
		linf, err := mp4.BuildLinfFromVPS(inp.vps, maxTids)
		if err != nil {
			return fmt.Errorf("build linf: %w", err)
		}
		addSampleGroup(stbl, "linf", linf, len(inp.samples))
	}

	minf.AddChild(stbl)
	mdia.AddChild(minf)
	trak.AddChild(mdia)
	moov.AddChild(trak)

	totalDurMedia := uint64(len(inp.samples)) * uint64(inp.sampleDur)
	mdhd.Duration = totalDurMedia
	moov.Mvhd.Timescale = 600
	moov.Mvhd.Duration = totalDurMedia * uint64(moov.Mvhd.Timescale) / uint64(inp.timeScale)
	tkhd.Duration = moov.Mvhd.Duration

	outFile.AddChild(moov, 0)

	var mdatData []byte
	for _, s := range inp.samples {
		mdatData = append(mdatData, s.data...)
	}
	mdatBox := &mp4.MdatBox{}
	mdatBox.SetData(mdatData)
	outFile.AddChild(mdatBox, 0)

	// Fix up stco: the single chunk starts at the mdat payload.
	// Layout is ftyp | moov | mdat_header | mdat_payload.
	_ = mdatBox.Size() // evaluate so HeaderSize reflects a possible largesize header
	var sizeBeforeMdat uint64
	for _, box := range outFile.Children {
		if box.Type() != "mdat" {
			sizeBeforeMdat += box.Size()
		}
	}
	chunkOffset := sizeBeforeMdat + mdatBox.HeaderSize()
	if chunkOffset > math.MaxUint32 {
		return fmt.Errorf("output too large for 32-bit chunk offsets (%d bytes); co64 is not supported", chunkOffset)
	}
	stco.ChunkOffset[0] = uint32(chunkOffset)

	ofd, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("could not create output file: %w", err)
	}
	defer ofd.Close()
	if err := outFile.Encode(ofd); err != nil {
		return fmt.Errorf("encode error: %w", err)
	}

	fmt.Fprintf(w, "Wrote %s (%dx%d, %d samples, %d layers)\n",
		outPath, inp.width, inp.height, len(inp.samples), inp.vps.GetNumLayers())
	return nil
}

// buildCtts run-length encodes the per-sample composition offsets into a CttsBox,
// or returns nil if all offsets are zero (PTS == DTS).
func buildCtts(samples []mvhevcSample) *mp4.CttsBox {
	var counts []uint32
	var offsets []int32
	anyNonZero := false
	negative := false
	for _, s := range samples {
		if s.cto != 0 {
			anyNonZero = true
		}
		if s.cto < 0 {
			negative = true
		}
		if n := len(offsets); n > 0 && offsets[n-1] == s.cto {
			counts[n-1]++
		} else {
			counts = append(counts, 1)
			offsets = append(offsets, s.cto)
		}
	}
	if !anyNonZero {
		return nil
	}
	ctts := &mp4.CttsBox{}
	if negative {
		ctts.Version = 1 // version 1 allows signed offsets
	}
	_ = ctts.AddSampleCountsAndOffset(counts, offsets) // counts and offsets are equal length by construction
	return ctts
}

// addSampleGroup adds an sgpd + matching sbgp mapping all samples to group 1.
func addSampleGroup(stbl *mp4.StblBox, groupingType string, entry mp4.SampleGroupEntry, nrSamples int) {
	sgpd := &mp4.SgpdBox{
		Version:            2,
		GroupingType:       groupingType,
		DefaultLength:      uint32(entry.Size()),
		SampleGroupEntries: []mp4.SampleGroupEntry{entry},
	}
	stbl.AddChild(sgpd)

	sbgp := &mp4.SbgpBox{GroupingType: groupingType}
	sbgp.SampleCounts = append(sbgp.SampleCounts, uint32(nrSamples))
	sbgp.GroupDescriptionIndices = append(sbgp.GroupDescriptionIndices, 1)
	stbl.AddChild(sbgp)
}

// fpsToTimescale converts an FPS value to a timescale and per-sample duration.
func fpsToTimescale(fps float64) (timeScale uint32, sampleDur uint32) {
	switch {
	case fps > 23.975 && fps < 23.977: // 23.976
		return 24000, 1001
	case fps > 29.969 && fps < 29.971: // 29.97
		return 30000, 1001
	case fps > 59.939 && fps < 59.941: // 59.94
		return 60000, 1001
	default:
		return uint32(fps * 1000), 1000
	}
}

// dedupNalus removes duplicate NALUs, keeping the first occurrence of each.
func dedupNalus(nalus [][]byte) [][]byte {
	seen := make(map[string]bool, len(nalus))
	out := make([][]byte, 0, len(nalus))
	for _, n := range nalus {
		if key := string(n); !seen[key] {
			seen[key] = true
			out = append(out, n)
		}
	}
	return out
}
