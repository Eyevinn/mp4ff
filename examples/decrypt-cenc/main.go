package main

import (
	"bytes"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/edgeware/mp4ff/mp4"
)

func main() {

	inFilePath := flag.String("i", "", "Required: Path to input file")
	outFilePath := flag.String("o", "", "Required: Output file")
	hexKey := flag.String("k", "", "Required: key (hex)")

	err := start(*inFilePath, *outFilePath, *hexKey)
	if err != nil {
		log.Fatalln(err)
	}

}

func start(inPath, outPath, hexKey string) error {

	ifh, err := os.Open(inPath)
	if err != nil {
		return err
	}
	defer ifh.Close()

	if len(hexKey) != 32 {
		return fmt.Errorf("Hex key must have length 32 chars")
	}
	key, err := hex.DecodeString(hexKey)
	if err != nil {
		return err
	}

	err = decryptMP4withCenc(ifh, key, outPath)
	if err != nil {
		return err
	}
	return nil
}

type trackInfo struct {
	trackID uint32
	sinf    *mp4.SinfBox
	trex    *mp4.TrexBox
}

func findTrackInfo(tracks []trackInfo, trackID uint32) trackInfo {
	for _, ti := range tracks {
		if ti.trackID == trackID {
			return ti
		}
	}
	return trackInfo{}
}

// decryptMP4withCenc - decrypt segmented mp4 file with CENC encryption
func decryptMP4withCenc(r io.Reader, key []byte, outPath string) error {
	inMp4, err := mp4.DecodeFile(r)
	if err != nil {
		return err
	}
	if !inMp4.IsFragmented() {
		return fmt.Errorf("file not fragmented. Not supported")
	}

	ofh, err := os.Create(outPath)
	if err != nil {
		return err
	}

	tracks := make([]trackInfo, 0, len(inMp4.Init.Moov.Traks))

	moov := inMp4.Init.Moov

	for _, trak := range moov.Traks {
		trackID := trak.Tkhd.TrackID
		stsd := trak.Mdia.Minf.Stbl.Stsd
		var encv *mp4.VisualSampleEntryBox
		var enca *mp4.AudioSampleEntryBox

		for _, child := range stsd.Children {
			switch child.Type() {
			case "encv":
				encv = child.(*mp4.VisualSampleEntryBox)
				sinf, err := encv.RemoveEncryption()
				if err != nil {
					return err
				}
				tracks = append(tracks, trackInfo{
					trackID: trackID,
					sinf:    sinf,
				})
			case "enca":
				enca = child.(*mp4.AudioSampleEntryBox)
				sinf, err := enca.RemoveEncryption()
				if err != nil {
					return err
				}
				tracks = append(tracks, trackInfo{
					trackID: trackID,
					sinf:    sinf,
				})
			default:
				continue
			}
		}
	}

	for _, trex := range moov.Mvex.Trexs {
		for i := range tracks {
			if tracks[i].trackID == trex.TrackID {
				tracks[i].trex = trex
				break
			}
		}
	}
	psshs := moov.RemovePsshs()
	for _, pssh := range psshs {
		psshInfo := bytes.Buffer{}
		err = pssh.Info(&psshInfo, "", "", "  ")
		if err != nil {
			return err
		}
		fmt.Printf("pssh: %s\n", psshInfo.String())
	}

	// Copy ftyp from input
	err = inMp4.Ftyp.Encode(ofh)
	if err != nil {
		return err
	}
	// Write the modified init segment
	err = inMp4.Init.Encode(ofh)
	if err != nil {
		return err
	}

	err = decryptAndWriteSegments(inMp4.Segments, tracks, key, ofh)
	if err != nil {
		return err
	}
	ofh.Close()
	return nil
}

func decryptAndWriteSegments(segs []*mp4.MediaSegment, tracks []trackInfo, key []byte, ofh io.Writer) error {
	var outNr uint32 = 1
	for i, seg := range segs {
		for j, frag := range seg.Fragments {
			fmt.Printf("Segment %d, fragment %d\n", i+1, j+1)
			err := decryptFragment(frag, tracks, key)
			if err != nil {
				return err
			}
			outNr++
		}
		err := seg.Encode(ofh)
		if err != nil {
			return err
		}
	}

	return nil
}

// decryptFragment - decrypt fragment in place
func decryptFragment(frag *mp4.Fragment, tracks []trackInfo, key []byte) error {
	moof := frag.Moof
	var nrBytesRemoved uint64 = 0
	for _, traf := range moof.Trafs {
		senc := traf.Senc
		ti := findTrackInfo(tracks, traf.Tfhd.TrackID)
		samples, err := frag.GetFullSamples(ti.trex)
		if err != nil {
			return err
		}
		err = decryptSamplesInPlace(samples, key, senc)
		if err != nil {
			return err
		}
		nrBytesRemoved += traf.RemoveEncryptionBoxes()
	}
	for _, traf := range moof.Trafs {
		for _, trun := range traf.Truns {
			trun.DataOffset -= int32(nrBytesRemoved)
		}
	}

	_ = moof.RemovePsshs()
	return nil
}

// decryptSample - decrypt sample inplace
func decryptSamplesInPlace(samples []mp4.FullSample, key []byte, senc *mp4.SencBox) error {

	// TODO. Interpret saio and saiz to get to the right place
	// Saio tells where the IV starts relative to moof start
	// It typically ends up inside senc (16 bytes after start)
	for i := range samples {
		encSample := samples[i].Data
		var iv []byte
		if len(senc.IVs[i]) == 8 {
			iv = make([]byte, 0, 16)
			iv = append(iv, senc.IVs[i]...)
			iv = append(iv, []byte{0, 0, 0, 0, 0, 0, 0, 0}...)
		} else {
			iv = senc.IVs[i]
		}

		var subSamplePatterns []mp4.SubSamplePattern
		if len(senc.SubSamples) != 0 {
			subSamplePatterns = senc.SubSamples[i]
		}
		decryptedSample, err := mp4.DecryptSampleCenc(encSample, key, iv, subSamplePatterns)
		if err != nil {
			return err
		}
		_ = copy(samples[i].Data, decryptedSample)
	}
	return nil
}
