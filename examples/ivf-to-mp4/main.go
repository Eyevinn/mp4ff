// ivf-to-mp4 muxes an AV1 or VP9 IVF file into a fragmented MP4 (one fragment per closed GOP).
//
// It reads the raw bitstream produced by encoders like aomenc, SVT-AV1, vpxenc or ffmpeg's IVF
// muxer, builds the codec configuration record (av1C or vpcC) from the bitstream, and writes a
// fragmented MP4. The result can be segmented further with the segmenter example.
//
//	ivf-to-mp4 input.ivf output.mp4
package main

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/Eyevinn/mp4ff/av1"
	"github.com/Eyevinn/mp4ff/ivf"
	"github.com/Eyevinn/mp4ff/mp4"
	"github.com/Eyevinn/mp4ff/vp8"
	"github.com/Eyevinn/mp4ff/vp9"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintf(os.Stderr, "usage: %s input.ivf output.mp4\n", os.Args[0])
		os.Exit(1)
	}
	if err := run(os.Args[1], os.Args[2]); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// keyFrameFunc reports whether a coded frame is a random-access point (sync sample).
type keyFrameFunc func(sample []byte) (bool, error)

func run(inPath, outPath string) error {
	in, err := os.Open(inPath)
	if err != nil {
		return err
	}
	defer in.Close()

	rd, err := ivf.NewReader(in)
	if err != nil {
		return err
	}
	if rd.Header.Rate == 0 {
		return fmt.Errorf("ivf header has zero frame-rate (Rate)")
	}
	scale := uint64(rd.Header.Scale)
	if scale == 0 {
		scale = 1
	}

	var frames []ivf.Frame
	for {
		f, err := rd.ReadFrame()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		frames = append(frames, f)
	}
	if len(frames) == 0 {
		return fmt.Errorf("no frames in %s", inPath)
	}

	var (
		init  *mp4.InitSegment
		isKey keyFrameFunc
	)
	switch rd.Header.FourCC {
	case ivf.CodecAV1:
		init, isKey, err = setupAV1(rd.Header, frames)
	case ivf.CodecVP9:
		init, isKey, err = setupVP9(rd.Header, frames)
	case ivf.CodecVP8:
		init, isKey, err = setupVP8(rd.Header, frames)
	default:
		return fmt.Errorf("unsupported IVF codec %q (supported: %s, %s, %s)",
			rd.Header.FourCC, ivf.CodecAV1, ivf.CodecVP9, ivf.CodecVP8)
	}
	if err != nil {
		return err
	}

	out, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer out.Close()
	return writeFragmentedMP4(out, init, frames, scale, isKey)
}

// setupAV1 builds the init segment for an AV1 track from the sequence header in the first frame.
func setupAV1(hdr ivf.FileHeader, frames []ivf.Frame) (*mp4.InitSegment, keyFrameFunc, error) {
	seqOBU, err := findOBU(frames[0].Data, av1.OBUSequenceHeader)
	if err != nil {
		return nil, nil, fmt.Errorf("locate sequence header: %w", err)
	}
	sh, err := av1.ParseSequenceHeader(seqOBU.Payload)
	if err != nil {
		return nil, nil, fmt.Errorf("parse sequence header: %w", err)
	}
	av1C := &mp4.Av1CBox{CodecConfRec: av1.CodecConfRecFromSequenceHeader(sh, seqOBU.Encode())}
	width, height := frameSize(hdr, uint16(sh.Width()), uint16(sh.Height()))

	init := mp4.CreateEmptyInit()
	trak := init.AddEmptyTrack(hdr.Rate, "video", "und")
	if err := trak.SetAV1Descriptor("av01", av1C, width, height); err != nil {
		return nil, nil, err
	}
	isKey := func(sample []byte) (bool, error) { return av1.IsRAPSample(sample, sh) }
	return init, isKey, nil
}

// setupVP9 builds the init segment for a VP9 track from the first key frame's header.
func setupVP9(hdr ivf.FileHeader, frames []ivf.Frame) (*mp4.InitSegment, keyFrameFunc, error) {
	h, err := vp9.ParseFrameHeader(frames[0].Data)
	if err != nil {
		return nil, nil, fmt.Errorf("parse first VP9 frame header: %w", err)
	}
	if !h.KeyFrame {
		return nil, nil, fmt.Errorf("first VP9 frame is not a key frame")
	}
	prim, trc, mtx := h.CICP()
	scale := hdr.Scale
	if scale == 0 {
		scale = 1
	}
	vpcC := &mp4.VppCBox{
		Version:                 1,
		Profile:                 h.Profile,
		Level:                   vp9.Level(h.Width, h.Height, float64(hdr.Rate)/float64(scale)),
		BitDepth:                h.BitDepth,
		ChromaSubsampling:       h.VpcCChromaSubsampling(),
		VideoFullRangeFlag:      boolToByte(h.ColorRange),
		ColourPrimaries:         prim,
		TransferCharacteristics: trc,
		MatrixCoefficients:      mtx,
	}
	width, height := frameSize(hdr, uint16(h.Width), uint16(h.Height))

	init := mp4.CreateEmptyInit()
	trak := init.AddEmptyTrack(hdr.Rate, "video", "und")
	if err := trak.SetVPxDescriptor("vp09", vpcC, width, height); err != nil {
		return nil, nil, err
	}
	return init, vp9.IsKeyFrame, nil
}

// setupVP8 builds the init segment for a VP8 track from the first key frame's header. VP8 is
// always profile 0, 8-bit, 4:2:0, and carries no CICP colour info in-band, so the colour fields
// are left unspecified (2) and the level undefined (0), matching ffmpeg's VP8 vpcC output.
func setupVP8(hdr ivf.FileHeader, frames []ivf.Frame) (*mp4.InitSegment, keyFrameFunc, error) {
	h, err := vp8.ParseFrameHeader(frames[0].Data)
	if err != nil {
		return nil, nil, fmt.Errorf("parse first VP8 frame header: %w", err)
	}
	if !h.KeyFrame {
		return nil, nil, fmt.Errorf("first VP8 frame is not a key frame")
	}
	vpcC := &mp4.VppCBox{
		Version:                 1,
		Profile:                 0,
		Level:                   0,
		BitDepth:                8,
		ChromaSubsampling:       1, // 4:2:0 colocated
		VideoFullRangeFlag:      0,
		ColourPrimaries:         2, // unspecified
		TransferCharacteristics: 2,
		MatrixCoefficients:      2,
	}
	width, height := frameSize(hdr, h.Width, h.Height)

	init := mp4.CreateEmptyInit()
	trak := init.AddEmptyTrack(hdr.Rate, "video", "und")
	if err := trak.SetVPxDescriptor("vp08", vpcC, width, height); err != nil {
		return nil, nil, err
	}
	return init, vp8.IsKeyFrame, nil
}

// writeFragmentedMP4 writes the init segment followed by one fragment per closed GOP (a run of
// samples starting at a random-access point). The mdhd timescale is the IVF Rate, so one IVF
// timestamp tick equals Scale mdhd ticks.
func writeFragmentedMP4(out io.Writer, init *mp4.InitSegment, frames []ivf.Frame, scale uint64, isKey keyFrameFunc) error {
	if err := init.Encode(out); err != nil {
		return fmt.Errorf("encode init: %w", err)
	}
	trackID := init.Moov.Trak.Tkhd.TrackID

	n := len(frames)
	key := make([]bool, n)
	for i := range frames {
		k, err := isKey(frames[i].Data)
		if err != nil {
			return fmt.Errorf("frame %d key-frame detection: %w", i, err)
		}
		key[i] = k
	}
	dur := func(i int) uint32 {
		if i+1 < n {
			return uint32((frames[i+1].Timestamp - frames[i].Timestamp) * scale)
		}
		if n >= 2 {
			return uint32((frames[n-1].Timestamp - frames[n-2].Timestamp) * scale)
		}
		return uint32(scale)
	}

	var seqNr uint32
	for i := 0; i < n; {
		seqNr++
		frag, err := mp4.CreateFragment(seqNr, trackID)
		if err != nil {
			return fmt.Errorf("create fragment: %w", err)
		}
		for ; i < n; i++ {
			if key[i] && frag.Moof.Traf.Trun.SampleCount() > 0 {
				break // start of the next GOP
			}
			flags := mp4.SetNonSyncSampleFlags(0)
			if key[i] {
				flags = mp4.SetSyncSampleFlags(0)
			}
			frag.AddFullSample(mp4.FullSample{
				Sample:     mp4.Sample{Flags: flags, Dur: dur(i), Size: uint32(len(frames[i].Data))},
				DecodeTime: frames[i].Timestamp * scale,
				Data:       frames[i].Data,
			})
		}
		if err := frag.Encode(out); err != nil {
			return fmt.Errorf("encode fragment %d: %w", seqNr, err)
		}
	}
	return nil
}

// frameSize returns the container size, falling back to the bitstream size when the IVF header
// does not carry it.
func frameSize(hdr ivf.FileHeader, bsWidth, bsHeight uint16) (uint16, uint16) {
	w, h := hdr.Width, hdr.Height
	if w == 0 || h == 0 {
		return bsWidth, bsHeight
	}
	return w, h
}

func boolToByte(b bool) byte {
	if b {
		return 1
	}
	return 0
}

// findOBU returns the first OBU of the given type in a temporal unit.
func findOBU(tu []byte, t av1.OBUType) (av1.OBU, error) {
	obus, err := av1.SplitOBUs(tu)
	if err != nil {
		return av1.OBU{}, err
	}
	for _, o := range obus {
		if o.Header.Type == t {
			return o, nil
		}
	}
	return av1.OBU{}, fmt.Errorf("no OBU of type %s", t)
}
