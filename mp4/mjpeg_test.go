package mp4_test

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"testing"

	"github.com/Eyevinn/mp4ff/mp4"
)

// TestMJpegMuxDemux muxes generated JPEG frames into a fragmented MP4 with an mjpg track
// (one sample per fragment), decodes it again, and checks that the extracted frames decode
// as JPEG. The jpgC case stores the samples without the leading SOI marker and carries it
// as jpgC JPEG prefix instead, so that prefix + sample data forms a complete JPEG image.
func TestMJpegMuxDemux(t *testing.T) {
	const (
		frameWidth  = 32
		frameHeight = 24
		nrFrames    = 3
	)
	frames := make([][]byte, 0, nrFrames)
	for i := 0; i < nrFrames; i++ {
		img := image.NewRGBA(image.Rect(0, 0, frameWidth, frameHeight))
		c := color.RGBA{R: byte(80 * i), G: byte(255 - 80*i), B: 128, A: 255}
		for y := 0; y < frameHeight; y++ {
			for x := 0; x < frameWidth; x++ {
				img.Set(x, y, c)
			}
		}
		buf := bytes.Buffer{}
		if err := jpeg.Encode(&buf, img, nil); err != nil {
			t.Fatal(err)
		}
		frames = append(frames, buf.Bytes())
	}

	cases := []struct {
		name       string
		jpegPrefix []byte
	}{
		{"selfContainedSamples", nil},
		{"jpgCPrefix", []byte{0xff, 0xd8}}, // SOI marker shared via jpgC
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Mux one JPEG per fragment
			init := mp4.CreateEmptyInit()
			init.AddEmptyTrack(1000, "video", "und")
			trak := init.Moov.Trak
			err := trak.SetMJpegDescriptor(frameWidth, frameHeight, c.jpegPrefix)
			assertNoError(t, err)
			buf := bytes.Buffer{}
			err = init.Encode(&buf)
			assertNoError(t, err)
			for i, frame := range frames {
				sample := frame[len(c.jpegPrefix):]
				frag, err := mp4.CreateFragment(uint32(i+1), trak.Tkhd.TrackID)
				assertNoError(t, err)
				frag.AddFullSample(mp4.FullSample{
					Sample: mp4.Sample{
						Flags: mp4.SetSyncSampleFlags(0),
						Dur:   1000,
						Size:  uint32(len(sample)),
					},
					DecodeTime: uint64(i) * 1000,
					Data:       sample,
				})
				err = frag.Encode(&buf)
				assertNoError(t, err)
			}

			// Demux and check that jpgC prefix + sample data are decodable JPEG images
			decFile, err := mp4.DecodeFile(&buf)
			assertNoError(t, err)
			mjpg := decFile.Init.Moov.Trak.Mdia.Minf.Stbl.Stsd.Mjpg
			if mjpg == nil {
				t.Fatal("no mjpg sample entry")
			}
			var jpegPrefix []byte
			if mjpg.JpgC != nil {
				jpegPrefix = mjpg.JpgC.JpegPrefix
			}
			if !bytes.Equal(jpegPrefix, c.jpegPrefix) {
				t.Errorf("got jpegPrefix %v, expected %v", jpegPrefix, c.jpegPrefix)
			}
			nrDecFrames := 0
			for _, seg := range decFile.Segments {
				for _, frag := range seg.Fragments {
					fullSamples, err := frag.GetFullSamples(decFile.Init.Moov.Mvex.Trex)
					assertNoError(t, err)
					for _, fs := range fullSamples {
						frame := append(append([]byte{}, jpegPrefix...), fs.Data...)
						if !bytes.Equal(frame, frames[nrDecFrames]) {
							t.Errorf("frame %d does not match muxed frame", nrDecFrames+1)
						}
						cfg, err := jpeg.DecodeConfig(bytes.NewReader(frame))
						if err != nil {
							t.Fatalf("frame %d does not decode as JPEG: %v", nrDecFrames+1, err)
						}
						if cfg.Width != frameWidth || cfg.Height != frameHeight {
							t.Errorf("frame %d is %dx%d, want %dx%d", nrDecFrames+1, cfg.Width, cfg.Height, frameWidth, frameHeight)
						}
						nrDecFrames++
					}
				}
			}
			if nrDecFrames != nrFrames {
				t.Errorf("got %d frames, want %d", nrDecFrames, nrFrames)
			}
		})
	}
}
