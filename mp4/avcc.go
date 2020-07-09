package mp4

import (
	"io"
)

// AvcCBox - AVCConfigurationBox (ISO/IEC 14496-15 5.4.2.1.2 and 5.3.3.1.2)
// Contains one AVCDecoderConfigurationRecord
type AvcCBox struct {
	AVCDecConfRec
}

// CreateAvcC - Create an avcC box based on SPS and PPS
func CreateAvcC(spsNALU []byte, ppsNALUs [][]byte) *AvcCBox {
	avcDecConfRec := CreateAVCDecConfRec(spsNALU, ppsNALUs)

	return &AvcCBox{avcDecConfRec}
}

// DecodeAvcC - box-specific decode
func DecodeAvcC(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	avcDecConfRec, err := DecodeAVCDecConfRec(r)
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
	totalNalLen := 0
	for _, nal := range a.SPSnalus {
		totalNalLen += 2 + len(nal)
	}
	for _, nal := range a.PPSnalus {
		totalNalLen += 2 + len(nal)
	}
	return uint64(boxHeaderSize + 7 + totalNalLen)
}

// Encode - write box to w
func (a *AvcCBox) Encode(w io.Writer) error {
	err := EncodeHeader(a, w)
	if err != nil {
		return err
	}
	a.AVCDecConfRec.Encode(w)
	return nil
}
