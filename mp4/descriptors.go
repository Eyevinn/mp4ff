package mp4

import (
	"fmt"

	"github.com/edgeware/mp4ff/bits"
)

/* Elementary Stream Descriptors are defined in ISO/IEC 14496-1.
The full spec looks like, below, but we don't parse all that stuff.
*/

const (
	// Following Table 1 of Class Tags for descriptors in ISO/IEC 14496-1. There are more types
	ObjectDescrTag        = 1
	InitialObjectDescrTag = 2
	ES_DescrTag           = 3
	DecoderConfigDescrTag = 4
	DecSpecificInfoTag    = 5
	SLConfigDescrTag      = 6

	minimalEsDescrSize = 25
)

type Descriptor interface {
	// Tag - descriptor tag. Fixed for each descriptor type
	Tag() byte
	// Size - size of descriptor, excluding tag byte and size field
	Size() uint32
	// SizeSize - size of descriptor including tag byte and size field
	SizeSize() uint32
	// EncodeSW - Write descriptor to slice writer
	EncodeSW(sw bits.SliceWriter) error
	// Info - provide information about descriptor
}

/*
ESDescriptor is defined in ISO/IEC 14496-1 7.2.6.5

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
  } else  { // SL extension is possible.
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
type ESDescriptor struct {
	EsID                uint16
	DependsOnEsID       uint16
	OCResID             uint16
	FlagsAndPriority    byte
	sizeFieldSizeMinus1 byte
	URLString           string
	DecConfigDescriptor DecoderConfigDescriptor
	SLConfigDescriptor  SLConfigDescriptor
	OtherDescriptors    []RawDescriptor
}

func DecodeESDescriptor(sr bits.SliceReader, descSize uint32) (ESDescriptor, error) {
	ed := ESDescriptor{}
	srStart := sr.GetPos()
	tag := sr.ReadUint8()
	if tag != ES_DescrTag {
		return ed, fmt.Errorf("got tag %d instead of ESDescriptorTag %d", tag, ES_DescrTag)
	}

	sizeFieldSizeMinus1, size, err := readSizeSize(sr)
	if err != nil {
		return ed, err
	}
	ed.sizeFieldSizeMinus1 = sizeFieldSizeMinus1
	ed.EsID = sr.ReadUint16()
	ed.FlagsAndPriority = sr.ReadUint8()
	streamDependenceFlag := ed.FlagsAndPriority >> 7
	urlFlag := (ed.FlagsAndPriority >> 6) & 0x1
	ocrStreamFlag := (ed.FlagsAndPriority >> 5) & 0x1
	// streamPriority := ed.FlagsAndPriority & 0x1f

	if streamDependenceFlag == 1 {
		ed.DependsOnEsID = sr.ReadUint16()
	}
	if urlFlag == 1 {
		urlLen := sr.ReadUint8()
		ed.URLString = sr.ReadFixedLengthString(int(urlLen))
	}
	if ocrStreamFlag == 1 {
		ed.OCResID = sr.ReadUint16()
	}
	ed.DecConfigDescriptor, err = DecodeDecoderConfigDescriptor(sr)
	if err != nil {
		return ed, err
	}
	ed.SLConfigDescriptor, err = DecodeSLConfigDescriptor(sr)
	if err != nil {
		return ed, err
	}
	for {
		nrBytesLeft := int(descSize) - (sr.GetPos() - srStart)
		if nrBytesLeft == 0 {
			break
		}
		if nrBytesLeft < 0 {
			return ed, fmt.Errorf("read too far in ESDescriptor")
		}
		desc, err := DecodeRawDescriptor(sr)
		if err != nil {
			return ed, err
		}
		ed.OtherDescriptors = append(ed.OtherDescriptors, desc)
	}
	if size != ed.Size() {
		return ed, fmt.Errorf("read size %d differs from calculated size %d", size, ed.Size())
	}

	return ed, sr.AccError()
}

func (e *ESDescriptor) Tag() byte {
	return ES_DescrTag
}

func (e *ESDescriptor) Size() uint32 {
	var size uint32 = 2 + 1
	streamDependenceFlag := e.FlagsAndPriority >> 7
	urlFlag := (e.FlagsAndPriority >> 6) & 0x1
	ocrStreamFlag := (e.FlagsAndPriority >> 5) & 0x1
	if streamDependenceFlag == 1 {
		size += 2
	}
	if urlFlag == 1 {
		size += 1 + uint32(len(e.URLString))
	}
	if ocrStreamFlag == 1 {
		size += 2
	}
	size += e.DecConfigDescriptor.SizeSize()
	size += e.SLConfigDescriptor.SizeSize()
	for _, od := range e.OtherDescriptors {
		size += od.SizeSize()
	}
	return size
}

func (e *ESDescriptor) SizeSize() uint32 {
	return 1 + uint32(e.sizeFieldSizeMinus1) + 1 + e.Size()
}

func (e *ESDescriptor) EncodeSW(sw bits.SliceWriter) error {
	sw.WriteBits(uint(e.Tag()), 8)
	writeDescriptorSize(sw, e.Size(), e.sizeFieldSizeMinus1)
	sw.WriteUint16(e.EsID)
	sw.WriteUint8(e.FlagsAndPriority)
	streamDependenceFlag := e.FlagsAndPriority >> 7
	urlFlag := (e.FlagsAndPriority >> 6) & 0x1
	ocrStreamFlag := (e.FlagsAndPriority >> 5) & 0x1
	// streamPriority := ed.FlagsAndPriority & 0x1f
	if streamDependenceFlag == 1 {
		sw.WriteUint16(e.DependsOnEsID)
	}
	if urlFlag == 1 {
		sw.WriteUint8(byte(len(e.URLString)))
		sw.WriteString(e.URLString, false /* no zero-termination */)
	}
	if ocrStreamFlag == 1 {
		sw.WriteUint16(e.OCResID)
	}

	err := e.DecConfigDescriptor.EncodeSW(sw)
	if err != nil {
		return err
	}

	err = e.SLConfigDescriptor.EncodeSW(sw)
	if err != nil {
		return err
	}
	return sw.AccError()
}

type DecoderConfigDescriptor struct {
	ObjectType          byte
	StreamType          byte
	sizeFieldSizeMinus1 byte
	BufferSizeDB        uint32
	MaxBitrate          uint32
	AvgBitrate          uint32
	DecSpecificInfo     DecSpecificInfoDescriptor
}

func DecodeDecoderConfigDescriptor(sr bits.SliceReader) (DecoderConfigDescriptor, error) {
	dd := DecoderConfigDescriptor{}
	tag := sr.ReadUint8()
	if tag != DecoderConfigDescrTag {
		return dd, fmt.Errorf("got tag %d instead of DecoderConfigDescrTag %d", tag, DecoderConfigDescrTag)
	}

	sizeFieldSizeMinus1, size, err := readSizeSize(sr)
	if err != nil {
		return dd, err
	}
	dd.sizeFieldSizeMinus1 = sizeFieldSizeMinus1
	dd.ObjectType = sr.ReadUint8()

	streamTypeAndBufferSizeDB := sr.ReadUint32()
	dd.StreamType = byte(streamTypeAndBufferSizeDB >> 24)
	dd.BufferSizeDB = streamTypeAndBufferSizeDB & 0xffffff
	dd.MaxBitrate = sr.ReadUint32()
	dd.AvgBitrate = sr.ReadUint32()
	dd.DecSpecificInfo, err = DecodeDecSpecificInfoDescriptor(sr)
	if err != nil {
		return dd, err
	}
	if size != dd.Size() {
		return dd, fmt.Errorf("read size %d differs from calculated size %d", size, dd.Size())
	}
	return dd, nil
}

func (d *DecoderConfigDescriptor) Tag() byte {
	return DecoderConfigDescrTag
}

func (d *DecoderConfigDescriptor) Size() uint32 {
	return 13 + d.DecSpecificInfo.SizeSize()
}

func (d *DecoderConfigDescriptor) SizeSize() uint32 {
	return 1 + uint32(d.sizeFieldSizeMinus1) + 1 + d.Size()
}

func (d *DecoderConfigDescriptor) EncodeSW(sw bits.SliceWriter) error {
	sw.WriteBits(uint(d.Tag()), 8)
	writeDescriptorSize(sw, d.Size(), d.sizeFieldSizeMinus1)
	sw.WriteUint8(d.ObjectType)
	streamTypeAndBufferSizeDB := (uint32(d.StreamType) << 24) | d.BufferSizeDB
	sw.WriteUint32(streamTypeAndBufferSizeDB)
	sw.WriteUint32(d.MaxBitrate)
	sw.WriteUint32(d.AvgBitrate)
	err := d.DecSpecificInfo.EncodeSW(sw)
	if err != nil {
		return err
	}
	return sw.AccError()
}

type DecSpecificInfoDescriptor struct {
	sizeFieldSizeMinus1 byte
	DecConfig           []byte
}

func DecodeDecSpecificInfoDescriptor(sr bits.SliceReader) (DecSpecificInfoDescriptor, error) {
	dd := DecSpecificInfoDescriptor{}
	tag := sr.ReadUint8()
	if tag != DecSpecificInfoTag {
		return dd, fmt.Errorf("got tag %d instead of DecSpecificInfoTag %d", tag, DecSpecificInfoTag)
	}

	sizeFieldSizeMinus1, size, err := readSizeSize(sr)
	if err != nil {
		return dd, err
	}
	dd.sizeFieldSizeMinus1 = sizeFieldSizeMinus1
	dd.DecConfig = sr.ReadBytes(int(size))
	return dd, sr.AccError()
}

func (d *DecSpecificInfoDescriptor) Tag() byte {
	return DecSpecificInfoTag
}

func (d *DecSpecificInfoDescriptor) Size() uint32 {
	return uint32(len(d.DecConfig))
}

func (d *DecSpecificInfoDescriptor) SizeSize() uint32 {
	return 1 + uint32(d.sizeFieldSizeMinus1) + 1 + d.Size()
}

func (d *DecSpecificInfoDescriptor) EncodeSW(sw bits.SliceWriter) error {
	sw.WriteBits(uint(d.Tag()), 8)
	writeDescriptorSize(sw, d.Size(), d.sizeFieldSizeMinus1)
	sw.WriteBytes(d.DecConfig)
	return sw.AccError()
}

type SLConfigDescriptor struct {
	sizeFieldSizeMinus1 byte
	ConfigValue         byte
	MoreData            []byte
}

func DecodeSLConfigDescriptor(sr bits.SliceReader) (SLConfigDescriptor, error) {
	d := SLConfigDescriptor{}
	tag := sr.ReadUint8()
	if tag != SLConfigDescrTag {
		return d, fmt.Errorf("got tag %d instead of SLConfigDescrTag %d", tag, SLConfigDescrTag)
	}
	sizeFieldSizeMinus1, size, err := readSizeSize(sr)
	if err != nil {
		return d, err
	}
	d.sizeFieldSizeMinus1 = sizeFieldSizeMinus1
	d.ConfigValue = sr.ReadUint8()
	if size > 1 {
		d.MoreData = sr.ReadBytes(int(size - 1))
	}
	return d, sr.AccError()
}

func (d *SLConfigDescriptor) Tag() byte {
	return SLConfigDescrTag
}

func (d *SLConfigDescriptor) Size() uint32 {
	return uint32(1 + len(d.MoreData))
}

func (d *SLConfigDescriptor) SizeSize() uint32 {
	return 1 + uint32(d.sizeFieldSizeMinus1) + 1 + d.Size()
}

func (d *SLConfigDescriptor) EncodeSW(sw bits.SliceWriter) error {
	sw.WriteBits(uint(d.Tag()), 8)
	writeDescriptorSize(sw, d.Size(), d.sizeFieldSizeMinus1)
	sw.WriteUint8(d.ConfigValue)
	if len(d.MoreData) > 0 {
		sw.WriteBytes(d.MoreData)
	}
	return sw.AccError()
}

// RawDescriptor - raw representation of any descriptor
type RawDescriptor struct {
	tag                 byte
	sizeFieldSizeMinus1 byte
	data                []byte
}

func DecodeRawDescriptor(sr bits.SliceReader) (RawDescriptor, error) {
	tag := sr.ReadUint8()
	sizeFieldSizeMinus1, size, err := readSizeSize(sr)
	d := RawDescriptor{
		tag:                 tag,
		sizeFieldSizeMinus1: sizeFieldSizeMinus1,
	}
	if err != nil {
		return d, err
	}

	d.data = sr.ReadBytes(int(size))
	return d, sr.AccError()
}

func CreateRawDescriptor(tag, sizeFieldSizeMinus1 byte, data []byte) (RawDescriptor, error) {
	return RawDescriptor{
		tag:                 tag,
		sizeFieldSizeMinus1: sizeFieldSizeMinus1,
		data:                data}, nil
}

func (s *RawDescriptor) Tag() byte {
	return s.tag
}

func (d *RawDescriptor) Size() uint32 {
	return uint32(len(d.data))
}

func (d *RawDescriptor) SizeSize() uint32 {
	return 1 + uint32(d.sizeFieldSizeMinus1) + 1 + uint32(len(d.data))
}

func (d *RawDescriptor) EncodeSW(sw bits.SliceWriter) error {
	sw.WriteBits(uint(d.tag), 8)
	writeDescriptorSize(sw, d.Size(), d.sizeFieldSizeMinus1)
	sw.WriteBytes(d.data)
	return sw.AccError()
}

func CreateESDescriptor(decConfig []byte) ESDescriptor {
	e := ESDescriptor{
		EsID: 0x01,
		DecConfigDescriptor: DecoderConfigDescriptor{
			ObjectType: 0x40, // Audio ISO/IEC 14496-3,
			StreamType: 0x15, // 0x5 << 2 + 0x01 (audioType + upstreamFlag + reserved)
			DecSpecificInfo: DecSpecificInfoDescriptor{
				DecConfig: decConfig,
			},
		},
		SLConfigDescriptor: SLConfigDescriptor{
			ConfigValue: 0x02,
		},
	}
	return e
}

// readTagAndSize - get size by accumulate 7 bits from each byte. MSB = 1 indicates more bytes.
// Defined in ISO 14496-1 Section 8.3.3
func readSizeSize(sr bits.SliceReader) (sizeFieldSizeMinus1 byte, size uint32, err error) {
	tmp := sr.ReadUint8()
	sizeOfInstance := uint32(tmp & 0x7f)
	for {
		if (tmp >> 7) == 0 {
			break // Last byte of size field
		}
		tmp = sr.ReadUint8()
		sizeFieldSizeMinus1++
		sizeOfInstance = sizeOfInstance<<7 | uint32(tmp&0x7f)
	}
	return sizeFieldSizeMinus1, sizeOfInstance, sr.AccError()
}

// writeDescriptorSize - write descriptor size 7-bit at a time in as many bytes as prescribed
func writeDescriptorSize(sw bits.SliceWriter, size uint32, sizeFieldSizeMinus1 byte) {
	for pos := int(sizeFieldSizeMinus1); pos >= 0; pos-- {
		value := byte(size>>uint32(7*pos)) & 0x7f
		if pos > 0 {
			value |= 0x80
		}
		sw.WriteBits(uint(value), 8)
	}
}
