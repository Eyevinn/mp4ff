package mp4

import (
	"io"

	"github.com/edgeware/mp4ff/avc"
)

// AvcCBox - AVCConfigurationBox (ISO/IEC 14496-15 5.4.2.1.2 and 5.3.3.1.2)
// Contains one AVCDecoderConfigurationRecord
type AvcCBox struct {
	avc.AVCDecConfRec
}

// CreateAvcC - Create an avcC box based on SPS and PPS
func CreateAvcC(spsNALUs [][]byte, ppsNALUs [][]byte) *AvcCBox {
	avcDecConfRec := avc.CreateAVCDecConfRec(spsNALUs, ppsNALUs)

	return &AvcCBox{avcDecConfRec}
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

func (a *AvcCBox) Dump(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newBoxDumper(w, indent, a, -1)
	return bd.err
}
