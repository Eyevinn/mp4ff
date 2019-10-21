package mp4

import (
	"encoding/binary"
	"io"
	"io/ioutil"
)

// AvcCBox - AVCConfigurationBox (ISO/IEC 14496-15 5.4.2.1.2 and 5.3.3.1.2)
type AvcCBox struct {
	AVCProfileIndication byte
	ProfileCompatibility byte
	AVCLevelIndication   byte
	SPSnalus             [][]byte
	PPSnalus             [][]byte
}

// CreateAvcC - Create an avcC box based on SPS and PPS
func CreateAvcC(spsNALU []byte, ppsNALUs [][]byte) *AvcCBox {
	sps, err := ParseSPSNALUnit(spsNALU)
	if err != nil {
		panic("Could not part SPS")
	}

	avcC := &AvcCBox{
		AVCProfileIndication: byte(sps.Profile),
		ProfileCompatibility: byte(sps.ProfileCompatibility),
		AVCLevelIndication:   byte(sps.Level),
		SPSnalus:             [][]byte{spsNALU},
		PPSnalus:             ppsNALUs,
	}
	return avcC
}

// DecodeAvcC - box-specific decode
func DecodeAvcC(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	//configurationVersion := data[0]
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

	b := &AvcCBox{
		AVCProfileIndication: AVCProfileIndication,
		ProfileCompatibility: ProfileCompatibility,
		AVCLevelIndication:   AVCLevelIndication,
		SPSnalus:             spsNALUs,
		PPSnalus:             ppsNALUs,
	}
	return b, nil
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
	var configurationVersion byte = 1
	var ffByte byte = 0xff
	binary.Write(w, binary.BigEndian, configurationVersion)
	binary.Write(w, binary.BigEndian, a.AVCProfileIndication)
	binary.Write(w, binary.BigEndian, a.ProfileCompatibility)
	binary.Write(w, binary.BigEndian, a.AVCLevelIndication)
	binary.Write(w, binary.BigEndian, ffByte)     // Set length to 4
	var nrSPS byte = byte(len(a.SPSnalus)) | 0xe0 // Added reserved 3 bits
	binary.Write(w, binary.BigEndian, nrSPS)
	for _, sps := range a.SPSnalus {
		var len uint16 = uint16(len(sps))
		binary.Write(w, binary.BigEndian, len)
		w.Write(sps)
	}
	var nrPPS byte = byte(len(a.PPSnalus))
	binary.Write(w, binary.BigEndian, nrPPS)
	for _, pps := range a.PPSnalus {
		var len uint16 = uint16(len(pps))
		binary.Write(w, binary.BigEndian, len)
		w.Write(pps)
	}
	return err
}
