package mp4

import (
	"encoding/hex"
	"errors"
	"io"
	"io/ioutil"
)

/*
Elementary Stream Descriptors are defined in ISO/IEC 14496-1.
The full spec looks like, below, but we don't parse all that stuff.

class ES_Descriptor extends BaseDescriptor : bit(8) tag=ES_DescrTag {
bit(16) ES_ID;
bit(1) streamDependenceFlag;
bit(1) URL_Flag;
bit(1) OCRstreamFlag;
bit(5) streamPriority;
if (streamDependenceFlag)
bit(16) dependsOn_ES_ID;
if (URL_Flag) {
bit(8) URLlength;
bit(8) URLstring[URLlength];
}
if (OCRstreamFlag)
bit(16) OCR_ES_Id;
DecoderConfigDescriptor decConfigDescr;
if (ODProfileLevelIndication==0x01) //no SL extension.
{
SLConfigDescriptor slConfigDescr;
}
else // SL extension is possible.
{
SLConfigDescriptor slConfigDescr;
}
IPI_DescrPointer ipiPtr[0 .. 1];
IP_IdentificationDataSet ipIDS[0 .. 255];
IPMP_DescriptorPointer ipmpDescrPtr[0 .. 255];
LanguageDescriptor langDescr[0 .. 255];
QoS_Descriptor qosDescr[0 .. 1];
RegistrationDescriptor regDescr[0 .. 1];
ExtensionDescriptor extDescr[0 .. 255];
}
*/

// EsdsBox as used for MPEG-audio, see ISO 14496-1 Section 7.2.6.6  for DecoderConfigDescriptor
type EsdsBox struct {
	Version               byte
	Flags                 uint32
	EsDescrTag            byte
	EsID                  uint16
	FlagsAndPriority      byte
	DecoderConfigDescrTag byte
	ObjectType            byte
	StreamType            byte
	BufferSizeDB          uint32
	MaxBitrate            uint32
	AvgBitrate            uint32
	DecSpecificInfoTag    byte
	DecConfig             []byte
	SLConfigDescrTag      byte
	SLConfigValue         byte
	nrExtraSizeBytes      int // Calculates extra bytes in the variable length size fields
}

// CreateEsdsBox - Create an EsdsBox geiven decConfig
func CreateEsdsBox(decConfig []byte) *EsdsBox {
	e := &EsdsBox{
		EsDescrTag:            0x03, // 14496-1 table 1
		EsID:                  0x01,
		DecoderConfigDescrTag: 0x04,
		ObjectType:            0x40, // Audio ISO/IEC 14496-3
		StreamType:            0x15, // 0x5 << 2 + 0x01 (audioType + upstreamFlag + reserved)
		DecSpecificInfoTag:    0x05,
		DecConfig:             decConfig,
		SLConfigDescrTag:      0x06, //Synclayer description
		SLConfigValue:         0x02,
	}
	return e
}

const fixedPartLen = 37

// DecodeEsds - box-specific decode
func DecodeEsds(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}

	s := NewSliceReader(data)
	versionAndFlags := s.ReadUint32()
	version := byte(versionAndFlags >> 24)

	e := &EsdsBox{
		Version:    version,
		Flags:      versionAndFlags & flagsMask,
		EsDescrTag: s.ReadUint8(),
	}

	_, nrBytesRead := readSizeOfInstance(s)
	e.nrExtraSizeBytes += nrBytesRead - 1

	e.EsID = s.ReadUint16()

	e.FlagsAndPriority = s.ReadUint8()
	e.DecoderConfigDescrTag = s.ReadUint8()

	_, nrBytesRead = readSizeOfInstance(s)
	e.nrExtraSizeBytes += nrBytesRead - 1
	e.ObjectType = s.ReadUint8()

	streamTypeAndBufferSizeDB := s.ReadUint32()
	e.StreamType = byte(streamTypeAndBufferSizeDB >> 24)
	e.BufferSizeDB = streamTypeAndBufferSizeDB & 0xffffff
	e.MaxBitrate = s.ReadUint32()
	e.AvgBitrate = s.ReadUint32()
	e.DecSpecificInfoTag = s.ReadUint8()
	size, nrBytesRead := readSizeOfInstance(s)
	e.nrExtraSizeBytes += nrBytesRead - 1
	e.DecConfig = s.ReadBytes(size)
	e.SLConfigDescrTag = s.ReadUint8()
	size, nrBytesRead = readSizeOfInstance(s)
	e.nrExtraSizeBytes += nrBytesRead - 1
	if size != 1 {
		return e, errors.New("Cannot handle SLConfigDescr not equal to 1 byte")
	}
	e.SLConfigValue = s.ReadUint8()
	return e, nil
}

// readSizeOfInstance - accumulate size by 7 bits from each byte. MSB = 1 indicates more bytes.
// Defined in ISO 14496-1 Section 8.3.3

func readSizeOfInstance(s *SliceReader) (int, int) {
	tmp := s.ReadUint8()
	nrBytesRead := 1
	var sizeOfInstance int = int(tmp & 0x7f)
	for {
		if (tmp >> 7) == 0 {
			break // Last byte of size field
		}
		tmp = s.ReadUint8()
		nrBytesRead++
		sizeOfInstance = sizeOfInstance<<7 | int(tmp&0x7f)
	}
	return sizeOfInstance, nrBytesRead
}

// Type - box type
func (e *EsdsBox) Type() string {
	return "esds"
}

// Size - calculated size of box
func (e *EsdsBox) Size() uint64 {
	return uint64(fixedPartLen + len(e.DecConfig) + e.nrExtraSizeBytes)
}

// Encode - write box to w
func (e *EsdsBox) Encode(w io.Writer) error {
	err := EncodeHeader(e, w)
	if err != nil {
		return err
	}

	decCfgLen := len(e.DecConfig)

	buf := makebuf(e)
	sw := NewSliceWriter(buf)
	versionAndFlags := (uint32(e.Version) << 24) + e.Flags
	sw.WriteUint32(versionAndFlags)
	sw.WriteUint8(e.EsDescrTag)
	sw.WriteUint8(23 + byte(decCfgLen)) // Length
	sw.WriteUint16(e.EsID)
	sw.WriteUint8(e.FlagsAndPriority)

	sw.WriteUint8(e.DecoderConfigDescrTag)
	sw.WriteUint8(15 + byte(decCfgLen)) // length
	sw.WriteUint8(e.ObjectType)
	streamTypeAndBufferSizeDB := (uint32(e.StreamType) << 24) | e.BufferSizeDB
	sw.WriteUint32(streamTypeAndBufferSizeDB)
	sw.WriteUint32(e.MaxBitrate)
	sw.WriteUint32(e.AvgBitrate)
	sw.WriteUint8(e.DecSpecificInfoTag)
	sw.WriteUint8(byte(decCfgLen)) // length
	sw.WriteBytes(e.DecConfig)

	// 3 bytes slConfigDescr
	sw.WriteUint8(e.SLConfigDescrTag)
	for i := 0; i < e.nrExtraSizeBytes; i++ {
		sw.WriteUint8(0x80) // To create the same size as read.
	}
	sw.WriteUint8(1)               // final length byte
	sw.WriteUint8(e.SLConfigValue) // Constant

	_, err = w.Write(buf)
	return err
}

func (e *EsdsBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, e, int(e.Version), e.Flags)
	bd.write(" - maxBitrate: %d", e.MaxBitrate)
	bd.write(" - avgBitrate: %d", e.AvgBitrate)
	bd.write(" - decConfig: %s", hex.EncodeToString(e.DecConfig))

	return bd.err
}
