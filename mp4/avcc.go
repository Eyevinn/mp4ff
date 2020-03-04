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
	hasExt               bool
	ChromaFormat         byte
	BitDepthLumaMinus8   byte
	BitDepthChromaMinus8 byte
	SPSExt               [][]byte
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
		ChromaFormat:         byte(sps.ChromaFormatIDC),
		BitDepthLumaMinus8:   byte(sps.BitDepthLumaMinus8),
		BitDepthChromaMinus8: byte(sps.BitDepthChromaMinus8),
		SPSExt:               nil,
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
	var chromaFormat byte
	var bitDepthLumaMinus8 byte
	var bitDepthChromaMinus8 byte
	var spsExtNals [][]byte
	switch AVCProfileIndication {
	case 100:
	case 110:
	case 122:
	case 144:
		chromaFormat = data[pos] & 0x3
		bitDepthLumaMinus8 = data[pos+1] & 0x7
		bitDepthChromaMinus8 = data[pos+2] & 0x7
		numSPSExt := data[pos] + 4
		pos += 4
		spsExtNals := make([][]byte, 0)
		for i := 0; i < int(numSPSExt); i++ {
			nalLength := int(binary.BigEndian.Uint16(data[pos : pos+2]))
			pos += 2
			spsExtNals = append(spsExtNals, data[pos:pos+nalLength])
			pos += nalLength
		}
	}

	b := &AvcCBox{
		AVCProfileIndication: AVCProfileIndication,
		ProfileCompatibility: ProfileCompatibility,
		AVCLevelIndication:   AVCLevelIndication,
		SPSnalus:             spsNALUs,
		PPSnalus:             ppsNALUs,
		ChromaFormat:         chromaFormat,
		BitDepthLumaMinus8:   bitDepthLumaMinus8,
		BitDepthChromaMinus8: bitDepthChromaMinus8,
		SPSExt:               spsExtNals,
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
	totalSize := uint64(boxHeaderSize + 7 + totalNalLen)
	switch a.AVCProfileIndication {
	case 100:
	case 110:
	case 122:
	case 144:
		totalSize += 4
		if len(a.SPSExt) > 0 {
			for _, s := range a.SPSExt {
				totalSize += 2 + uint64(len(s))
			}
		}
	}
	return totalSize
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
	switch a.AVCProfileIndication {
	case 100:
	case 110:
	case 122:
	case 144:
		binary.Write(w, binary.BigEndian, a.ChromaFormat|0xfc)
		binary.Write(w, binary.BigEndian, a.BitDepthLumaMinus8|0xe0)
		binary.Write(w, binary.BigEndian, a.BitDepthChromaMinus8|0xe0)
		var nrSPSExt byte = byte(len(a.SPSExt))
		binary.Write(w, binary.BigEndian, nrSPSExt)
		for _, spse := range a.SPSExt {
			var len uint16 = uint16(len(spse))
			binary.Write(w, binary.BigEndian, len)
			w.Write(spse)
		}
	}
	return err
}
