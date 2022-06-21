// decrypt-cenc  - decrypt a segmented mp4 file encrypted using Common Encryption (CENC).
//
// Currently supported modes are cenc and cbcs.
// The output is in the same format as the input but with samples decrypted
// and encryption information boxes such as pssh and schm removed.
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

	flag.Parse()

	ifh, err := os.Open(*inFilePath)
	if err != nil {
		log.Fatal(err)
	}
	defer ifh.Close()
	ofh, err := os.Create(*outFilePath)
	if err != nil {
		log.Fatal(err)
	}
	err = start(ifh, ofh, *hexKey)
	if err != nil {
		log.Fatalln(err)
	}
}

func start(r io.Reader, w io.Writer, hexKey string) error {

	if len(hexKey) != 32 {
		return fmt.Errorf("Hex key must have length 32 chars")
	}
	key, err := hex.DecodeString(hexKey)
	if err != nil {
		return err
	}

	err = decryptMP4withCenc(r, key, w)
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
func decryptMP4withCenc(r io.Reader, key []byte, w io.Writer) error {
	inMp4, err := mp4.DecodeFile(r)
	if err != nil {
		return err
	}
	if !inMp4.IsFragmented() {
		return fmt.Errorf("file not fragmented. Not supported")
	}

	tracks := make([]trackInfo, 0, len(inMp4.Init.Moov.Traks))

	moov := inMp4.Init.Moov

	for _, trak := range moov.Traks {
		trackID := trak.Tkhd.TrackID
		stsd := trak.Mdia.Minf.Stbl.Stsd
		var encv *mp4.VisualSampleEntryBox
		var enca *mp4.AudioSampleEntryBox
		var schemeType string

		for _, child := range stsd.Children {
			switch child.Type() {
			case "encv":
				encv = child.(*mp4.VisualSampleEntryBox)
				sinf, err := encv.RemoveEncryption()
				if err != nil {
					return err
				}
				schemeType = sinf.Schm.SchemeType
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
				schemeType = sinf.Schm.SchemeType
				tracks = append(tracks, trackInfo{
					trackID: trackID,
					sinf:    sinf,
				})
			default:
				continue
			}
		}
		if schemeType != "" && schemeType != "cenc" && schemeType != "cbcs" {
			return fmt.Errorf("scheme type %s not supported", schemeType)
		}
		if schemeType == "" {
			// Should be track in the clear
			tracks = append(tracks, trackInfo{
				trackID: trackID,
				sinf:    nil,
			})
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
		//fmt.Printf("pssh: %s\n", psshInfo.String())
	}

	// Write the modified init segment
	err = inMp4.Init.Encode(w)
	if err != nil {
		return err
	}

	err = decryptAndWriteSegments(inMp4.Segments, tracks, key, w)
	if err != nil {
		return err
	}
	return nil
}

func decryptAndWriteSegments(segs []*mp4.MediaSegment, tracks []trackInfo, key []byte, ofh io.Writer) error {
	var outNr uint32 = 1
	for _, seg := range segs {
		for _, frag := range seg.Fragments {
			//fmt.Printf("Segment %d, fragment %d\n", i+1, j+1)
			err := decryptFragment(frag, tracks, key)
			if err != nil {
				return err
			}
			outNr++
		}
		if seg.Sidx != nil {
			seg.Sidx = nil // drop sidx inside segment, since not modified properly
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
		ti := findTrackInfo(tracks, traf.Tfhd.TrackID)
		if ti.sinf != nil {
			schemeType := ti.sinf.Schm.SchemeType
			if schemeType != "cenc" && schemeType != "cbcs" {
				return fmt.Errorf("scheme type %s not supported", schemeType)
			}
			hasSenc, isParsed := traf.ContainsSencBox()
			if !hasSenc {
				return fmt.Errorf("no senc box in traf")
			}
			if !isParsed {
				defaultPerSampleIVSize := ti.sinf.Schi.Tenc.DefaultPerSampleIVSize
				err := traf.ParseReadSenc(defaultPerSampleIVSize, moof.StartPos)
				if err != nil {
					return fmt.Errorf("parseReadSenc: %w", err)
				}
			}

			tenc := ti.sinf.Schi.Tenc
			samples, err := frag.GetFullSamples(ti.trex)
			if err != nil {
				return err
			}

			err = decryptSamplesInPlace(schemeType, samples, key, tenc, traf.Senc)
			if err != nil {
				return err
			}
			nrBytesRemoved += traf.RemoveEncryptionBoxes()
		}
	}
	_, psshBytesRemoved := moof.RemovePsshs()
	nrBytesRemoved += psshBytesRemoved
	for _, traf := range moof.Trafs {
		for _, trun := range traf.Truns {
			trun.DataOffset -= int32(nrBytesRemoved)
		}
	}

	return nil
}

// decryptSample - decrypt samples inplace
func decryptSamplesInPlace(schemeType string, samples []mp4.FullSample, key []byte, tenc *mp4.TencBox, senc *mp4.SencBox) error {

	// TODO. Interpret saio and saiz to get to the right place
	// Saio tells where the IV starts relative to moof start
	// It typically ends up inside senc (16 bytes after start)

	for i := range samples {
		encSample := samples[i].Data
		var iv []byte
		if len(senc.IVs) == len(samples) {
			if len(senc.IVs[i]) == 8 {
				iv = make([]byte, 0, 16)
				iv = append(iv, senc.IVs[i]...)
				iv = append(iv, []byte{0, 0, 0, 0, 0, 0, 0, 0}...)
			} else if len(senc.IVs) == len(samples) {
				iv = senc.IVs[i]
			}
		} else if tenc.DefaultConstantIV != nil {
			iv = tenc.DefaultConstantIV
		}
		if len(iv) == 0 {
			return fmt.Errorf("iv has length 0")
		}

		var subSamplePatterns []mp4.SubSamplePattern
		if len(senc.SubSamples) != 0 {
			subSamplePatterns = senc.SubSamples[i]
		}
		switch schemeType {
		case "cenc":
			err := mp4.DecryptSampleCenc(encSample, key, iv, subSamplePatterns)
			if err != nil {
				return err
			}
		case "cbcs":
			err := mp4.DecryptSampleCbcs(encSample, key, iv, subSamplePatterns, tenc)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
