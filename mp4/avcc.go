package mp4

import (
	"encoding/hex"
	"fmt"
	"io"

	"github.com/edgeware/mp4ff/avc"
)

// AvcCBox - AVCConfigurationBox (ISO/IEC 14496-15 5.4.2.1.2 and 5.3.3.1.2)
// Contains one AVCDecoderConfigurationRecord
type AvcCBox struct {
	avc.AVCDecConfRec
}

// CreateAvcC - Create an avcC box based on SPS and PPS
func CreateAvcC(spsNALUs [][]byte, ppsNALUs [][]byte) (*AvcCBox, error) {
	avcDecConfRec, err := avc.CreateAVCDecConfRec(spsNALUs, ppsNALUs)
	if err != nil {
		return nil, fmt.Errorf("CreateAvcDecDecConfRec: %w", err)
	}

	return &AvcCBox{*avcDecConfRec}, nil
}

// DecodeAvcC - box-specific decode
func DecodeAvcC(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	avcDecConfRec, err := avc.DecodeAVCDecConfRec(r)
	if err != nil {
		return nil, err
	}
	return &AvcCBox{avcDecConfRec}, nil
}

// Type - return box type
func (a *AvcCBox) Type() string {
	return "avcC"
}

// Size - return calculated size
func (a *AvcCBox) Size() uint64 {
	totalNalLen := 7
	for _, nal := range a.SPSnalus {
		totalNalLen += 2 + len(nal)
	}
	for _, nal := range a.PPSnalus {
		totalNalLen += 2 + len(nal)
	}
	switch a.AVCProfileIndication {
	case 66, 77, 88: // From ISO/IEC 14496-15 2019 Section 5.3.1.1.2
		// No extra bytes
	default:
		if !a.NoTrailingInfo {
			totalNalLen += 4
		}
	}
	return uint64(boxHeaderSize + totalNalLen)
}

// Encode - write box to w
func (a *AvcCBox) Encode(w io.Writer) error {
	err := EncodeHeader(a, w)
	if err != nil {
		return err
	}
	return a.AVCDecConfRec.Encode(w)
}

func (a *AvcCBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, a, -1)
	bd.write(" - AVCProfileIndication: %d", a.AVCProfileIndication)
	bd.write(" - profileCompatibility: %02x", a.ProfileCompatibility)
	bd.write(" - AVCLevelIndication: %d", a.AVCLevelIndication)
	for _, sps := range a.SPSnalus {
		bd.write(" - SPS: %s", hex.EncodeToString(sps))
	}
	for _, pps := range a.PPSnalus {
		bd.write(" - PPS: %s", hex.EncodeToString(pps))
	}
	return bd.err
}
