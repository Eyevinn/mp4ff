// ivf-to-mp4 muxes an AV1 IVF file into a fragmented MP4 (one fragment per closed GOP).
//
// It reads the raw AV1 bitstream produced by encoders like aomenc, SVT-AV1 or ffmpeg's IVF
// muxer, builds an av1C configuration record from the sequence header, and writes a fragmented
// MP4 with an av01 track. The result can be segmented further with the segmenter example.
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
	if rd.Header.FourCC != ivf.CodecAV1 {
		return fmt.Errorf("only AV1 (%s) is supported, got %q", ivf.CodecAV1, rd.Header.FourCC)
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

	// The sequence header lives in the first temporal unit; use it for av1C and RAP detection.
	seqOBU, err := findOBU(frames[0].Data, av1.OBUSequenceHeader)
	if err != nil {
		return fmt.Errorf("locate sequence header: %w", err)
	}
	sh, err := av1.ParseSequenceHeader(seqOBU.Payload)
	if err != nil {
		return fmt.Errorf("parse sequence header: %w", err)
	}
	av1C := &mp4.Av1CBox{
		CodecConfRec: av1.CodecConfRecFromSequenceHeader(sh, seqOBU.Encode()),
	}

	width := rd.Header.Width
	height := rd.Header.Height
	if width == 0 || height == 0 {
		width, height = uint16(sh.Width()), uint16(sh.Height())
	}

	init := mp4.CreateEmptyInit()
	trak := init.AddEmptyTrack(rd.Header.Rate, "video", "und")
	if err := trak.SetAV1Descriptor("av01", av1C, width, height); err != nil {
		return err
	}
	trackID := trak.Tkhd.TrackID

	out, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer out.Close()
	if err := init.Encode(out); err != nil {
		return fmt.Errorf("encode init: %w", err)
	}

	// Per-frame sync flag, decode time and duration (mdhd timescale == Rate, so a tick is Scale).
	n := len(frames)
	isKey := make([]bool, n)
	for i := range frames {
		isKey[i], err = av1.IsRAPSample(frames[i].Data, sh)
		if err != nil {
			return fmt.Errorf("frame %d RAP detection: %w", i, err)
		}
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

	// One fragment per closed GOP (a run of samples starting at a random-access point).
	var seqNr uint32
	for i := 0; i < n; {
		seqNr++
		frag, err := mp4.CreateFragment(seqNr, trackID)
		if err != nil {
			return fmt.Errorf("create fragment: %w", err)
		}
		for ; i < n; i++ {
			if i > 0 && isKey[i] && frag.Moof.Traf.Trun.SampleCount() > 0 {
				break // next GOP
			}
			flags := mp4.SetNonSyncSampleFlags(0)
			if isKey[i] {
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
