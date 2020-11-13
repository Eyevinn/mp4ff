package mp4

import (
	"encoding/binary"
	"io"
	"io/ioutil"

	log "github.com/sirupsen/logrus"
)

//AVCDecConfRec - AVCDecoderConfigurationRecord
type AVCDecConfRec struct {
	AVCProfileIndication byte
	ProfileCompatibility byte
	AVCLevelIndication   byte
	SPSnalus             [][]byte
	PPSnalus             [][]byte
	ChromaFormat         byte
	BitDepthLumaMinus1   byte
	BitDepthChromaMinus1 byte
	NumSPSExt            byte
	NoTrailingInfo       bool // To handle strange cases where trailing info is missing
}

// CreateAVCDecConfRec - Create an AVCDecConfRec based on SPS and PPS
func CreateAVCDecConfRec(spsNALU []byte, ppsNALUs [][]byte) AVCDecConfRec {
	sps, err := ParseSPSNALUnit(spsNALU, false) // false -> parse only start of VUI
	if err != nil {
		panic("Could not parse SPS")
	}

	return AVCDecConfRec{
		AVCProfileIndication: byte(sps.Profile),
		ProfileCompatibility: byte(sps.ProfileCompatibility),
		AVCLevelIndication:   byte(sps.Level),
		SPSnalus:             [][]byte{spsNALU},
		PPSnalus:             ppsNALUs,
		ChromaFormat:         1,
		BitDepthLumaMinus1:   0,
		BitDepthChromaMinus1: 0,
		NumSPSExt:            0,
		NoTrailingInfo:       false,
	}
}

// DecodeAVCDecConfRec - decode an AVCDecConfRec
func DecodeAVCDecConfRec(r io.Reader) (AVCDecConfRec, error) {

	data, err := ioutil.ReadAll(r)
	if err != nil {
		return AVCDecConfRec{}, err
	}
	//configurationVersion := data[0]   // Should be 1
	AVCProfileIndication := data[1]
	ProfileCompatibility := data[2]
	AVCLevelIndication := data[3]
	LengthSizeMinus1 := data[4] & 0x03 // The first 5 bits are 1
	if LengthSizeMinus1 != 0x3 {
		panic("Can only handle 4byte NAL length size")
	}
	numSPS := data[5] & 0x1f // 5 bits following 3 reserved bits
	pos := 6
	spsNALUs := make([][]byte, 0, 1)
	for i := 0; i < int(numSPS); i++ {
		nalLength := int(binary.BigEndian.Uint16(data[pos : pos+2]))
		pos += 2
		spsNALUs = append(spsNALUs, data[pos:pos+nalLength])
		pos += nalLength
	}
	ppsNALUs := make([][]byte, 0, 1)
	numPPS := data[pos]
	pos++
	for i := 0; i < int(numPPS); i++ {
		nalLength := int(binary.BigEndian.Uint16(data[pos : pos+2]))
		pos += 2
		ppsNALUs = append(ppsNALUs, data[pos:pos+nalLength])
		pos += nalLength
	}
	adcr := AVCDecConfRec{
		AVCProfileIndication: AVCProfileIndication,
		ProfileCompatibility: ProfileCompatibility,
		AVCLevelIndication:   AVCLevelIndication,
		SPSnalus:             spsNALUs,
		PPSnalus:             ppsNALUs,
	}
	switch AVCProfileIndication {
	case 66, 77, 88: // From ISO/IEC 14496-15 2019 Section 5.3.1.1.2
		// No more bytes
	default:
		if pos == len(data) { // Not according to standard, but have been seen
			log.Warningf("No ChromaFormat info for AVCProfileIndication=%d", AVCProfileIndication)
			adcr.NoTrailingInfo = true
			return adcr, nil
		}
		adcr.ChromaFormat = data[pos] & 0x03
		adcr.BitDepthLumaMinus1 = data[pos+1] & 0x07
		adcr.BitDepthChromaMinus1 = data[pos+2] & 0x07
		adcr.NumSPSExt = data[pos+3]
		if adcr.NumSPSExt != 0 {
			panic("Cannot handle SPS extensions")
		}
	}

	return adcr, nil
}

// Encode - write an AVCDecConfRec to w
func (a *AVCDecConfRec) Encode(w io.Writer) error {
	var errWrite error
	writeByte := func(b byte) {
		if errWrite != nil {
			return
		}
		errWrite = binary.Write(w, binary.BigEndian, b)
	}
	writeSlice := func(s []byte) {
		if errWrite != nil {
			return
		}
		_, errWrite = w.Write(s)
	}
	writeUint16 := func(u uint16) {
		if errWrite != nil {
			return
		}
		errWrite = binary.Write(w, binary.BigEndian, u)
	}

	var configurationVersion byte = 1
	writeByte(configurationVersion)
	writeByte(a.AVCProfileIndication)
	writeByte(a.ProfileCompatibility)
	writeByte(a.AVCLevelIndication)
	writeByte(0xff) // Set length to 4

	var nrSPS byte = byte(len(a.SPSnalus)) | 0xe0 // Added reserved 3 bits
	writeByte(nrSPS)
	for _, sps := range a.SPSnalus {
		var length uint16 = uint16(len(sps))
		writeUint16(length)
		writeSlice(sps)
	}
	var nrPPS byte = byte(len(a.PPSnalus))
	writeByte(nrPPS)
	for _, pps := range a.PPSnalus {
		var length uint16 = uint16(len(pps))
		writeUint16(length)
		writeSlice(pps)
	}
	switch a.AVCProfileIndication {
	case 66, 77, 88: // From ISO/IEC 14496-15 2019 Section 5.3.1.1.2
		// No extra data according to standard
	default:
		if a.NoTrailingInfo { // Strange content, but consistent with Size()
			return errWrite
		}
		writeByte(0xfc | a.ChromaFormat)
		writeByte(0xf8 | a.BitDepthLumaMinus1)
		writeByte(0xf8 | a.BitDepthChromaMinus1)
		writeByte(a.NumSPSExt)
	}

	return errWrite
}
