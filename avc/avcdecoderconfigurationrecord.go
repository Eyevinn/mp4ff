package avc

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
)

var ErrCannotParseAVCExtension = errors.New("Cannot parse SPS extensions")
var ErrLengthSize = errors.New("Can only handle 4byte NAL length size")

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
func CreateAVCDecConfRec(spsNALUs [][]byte, ppsNALUs [][]byte) (*AVCDecConfRec, error) {

	sps, err := ParseSPSNALUnit(spsNALUs[0], false) // false -> parse only start of VUI
	if err != nil {
		return nil, err
	}

	return &AVCDecConfRec{
		AVCProfileIndication: byte(sps.Profile),
		ProfileCompatibility: byte(sps.ProfileCompatibility),
		AVCLevelIndication:   byte(sps.Level),
		SPSnalus:             spsNALUs,
		PPSnalus:             ppsNALUs,
		ChromaFormat:         1,
		BitDepthLumaMinus1:   0,
		BitDepthChromaMinus1: 0,
		NumSPSExt:            0,
		NoTrailingInfo:       false,
	}, nil
}

// DecodeAVCDecConfRec - decode an AVCDecConfRec
func DecodeAVCDecConfRec(r io.Reader) (AVCDecConfRec, error) {

	data, err := ioutil.ReadAll(r)
	if err != nil {
		return AVCDecConfRec{}, err
	}
	configurationVersion := data[0] // Should be 1
	if configurationVersion != 1 {
		return AVCDecConfRec{}, fmt.Errorf("AVC decoder configuration record version %d unknown",
			configurationVersion)
	}
	AVCProfileIndication := data[1]
	ProfileCompatibility := data[2]
	AVCLevelIndication := data[3]
	LengthSizeMinus1 := data[4] & 0x03 // The first 5 bits are 1
	if LengthSizeMinus1 != 0x3 {
		return AVCDecConfRec{}, ErrLengthSize
	}
	numSPS := data[5] & 0x1f // 5 bits following 3 reserved bits
	pos := 6
	spsNALUs := make([][]byte, 0, 1)
	for i := 0; i < int(numSPS); i++ {
		naluLength := int(binary.BigEndian.Uint16(data[pos : pos+2]))
		pos += 2
		spsNALUs = append(spsNALUs, data[pos:pos+naluLength])
		pos += naluLength
	}
	ppsNALUs := make([][]byte, 0, 1)
	numPPS := data[pos]
	pos++
	for i := 0; i < int(numPPS); i++ {
		naluLength := int(binary.BigEndian.Uint16(data[pos : pos+2]))
		pos += 2
		ppsNALUs = append(ppsNALUs, data[pos:pos+naluLength])
		pos += naluLength
	}
	adcr := AVCDecConfRec{
		AVCProfileIndication: AVCProfileIndication,
		ProfileCompatibility: ProfileCompatibility,
		AVCLevelIndication:   AVCLevelIndication,
		SPSnalus:             spsNALUs,
		PPSnalus:             ppsNALUs,
	}
	// The rest of this structure may vary
	// ISO/IEC 14496-15 2017 says that
	// Compatible extensions to this record will extend it and
	// will not change the configuration version code.
	//Readers should be prepared to ignore unrecognized
	// data beyond the definition of the data they understand
	//(e.g. after the parameter sets in this specification).

	switch AVCProfileIndication {
	case 100, 110, 122, 144: // From ISO/IEC 14496-15 2017 Section 5.3.3.1.2
		if pos == len(data) { // Not according to standard, but have been seen
			adcr.NoTrailingInfo = true
			return adcr, nil
		}
		adcr.ChromaFormat = data[pos] & 0x03
		adcr.BitDepthLumaMinus1 = data[pos+1] & 0x07
		adcr.BitDepthChromaMinus1 = data[pos+2] & 0x07
		adcr.NumSPSExt = data[pos+3]
		if adcr.NumSPSExt != 0 {
			return adcr, ErrCannotParseAVCExtension
		}
	default:
		// No more data to read
	}

	return adcr, nil
}

func (a *AVCDecConfRec) Size() uint64 {
	totalSize := 7
	for _, nalu := range a.SPSnalus {
		totalSize += 2 + len(nalu)
	}
	for _, nalu := range a.PPSnalus {
		totalSize += 2 + len(nalu)
	}
	switch a.AVCProfileIndication {
	case 66, 77, 88: // From ISO/IEC 14496-15 2019 Section 5.3.1.1.2
		// No extra bytes
	default:
		if !a.NoTrailingInfo {
			totalSize += 4
		}
	}
	return uint64(totalSize)
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
	case 100, 110, 122, 144: // From ISO/IEC 14496-15 2017 Section 5.3.3.1.2
		if a.NoTrailingInfo { // Strange content, but consistent with Size()
			return errWrite
		}
		writeByte(0xfc | a.ChromaFormat)
		writeByte(0xf8 | a.BitDepthLumaMinus1)
		writeByte(0xf8 | a.BitDepthChromaMinus1)
		writeByte(a.NumSPSExt)
	default:
		//Nothing more to write
	}

	return errWrite
}
