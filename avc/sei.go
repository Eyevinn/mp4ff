package avc

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"

	"github.com/edgeware/mp4ff/bits"
)

const (
	SEIPicTimingType    = 1
	SEIRegisteredType   = 4
	SEIUnregisteredType = 5
)

// SEI - Supplementary Enhancement Information as defined in ISO/IEC 14496-10
// High level syntax in Section 7.3.2.3
// The actual types are listed in Annex D
type SEI struct {
	SEIMessages []SEIMessage
}

// SEIMessage is common part of any SEI message
type SEIMessage interface {
	Type() uint
	Size() uint
	String() string
	Payload() []byte
}

// DecodeSEIMessage decodes an SEIMessage
func DecodeSEIMessage(sd *SEIData) (SEIMessage, error) {
	switch sd.Type() {
	case 4:
		return DecodeUserDataRegisteredSEI(sd)
	case 5:
		return DecodeUserDataUnregisteredSEI(sd)
	default:
		return sd, nil
	}
}

// SEIData - raw parsed SEI message with rbsp data
type SEIData struct {
	payloadType uint
	payload     []byte
}

// Type - SEI payload type
func (s *SEIData) Type() uint {
	return s.payloadType
}

// Payload - SEI raw rbsp payload
func (s *SEIData) Payload() []byte {
	return s.payload
}

// String - print up to 100 bytes of payload
func (s *SEIData) String() string {
	return fmt.Sprintf("SEI type %d, size=%d, %q", s.Type(), s.Size(), hex.EncodeToString(s.payload))
}

// Size - size in bytes of raw SEI message rbsp payload
func (s *SEIData) Size() uint {
	return uint(len(s.payload))
}

// ExtractSEIData - parse ebsp and return SEIData in rbsp format
func ExtractSEIData(r io.ReadSeeker) (seiData []SEIData, err error) {
	ar := bits.NewAccErrEBSPReader(r)
	for {
		payloadType := uint(0)
		for {
			nextByte := ar.Read(8)
			payloadType += uint(nextByte)
			if nextByte != 0xff {
				break
			}
		}
		payloadSize := uint32(0)
		for {
			nextByte := ar.Read(8)
			payloadSize += uint32(nextByte)
			if nextByte != 0xff {
				break
			}
		}
		payload := ar.ReadBytes(int(payloadSize))
		if ar.AccError() != nil {
			return nil, ar.AccError()
		}

		seiData = append(seiData, SEIData{payloadType, payload})
		if ar.AccError() != nil {
			return nil, ar.AccError()
		}
		// Break loop if no more rbsp data (end of sei messages)
		more, err := ar.MoreRbspData()
		if err != nil {
			return nil, err
		}
		if ar.AccError() != nil {
			return nil, ar.AccError()
		}
		if !more {
			break
		}
	}
	return seiData, nil
}

// DecodeUserDataRegisteredSEI - decode a SEI message of byte 4
func DecodeUserDataRegisteredSEI(sd *SEIData) (SEIMessage, error) {
	itutData := ITUData{
		CountryCode:      sd.payload[0],
		ProviderCode:     binary.BigEndian.Uint16(sd.payload[1:3]),
		UserIdentifier:   binary.BigEndian.Uint32(sd.payload[3:7]),
		UserDataTypeCode: sd.payload[7],
	}
	if itutData.IsCEA608() {
		return NewCEA608sei(sd)
	}
	return NewRegisteredSEI(sd, itutData), nil
}

// ITUData - first 8 bytes of payload for CEA-608 in type 4 (User data registered by ITU-T Rec T 35)
type ITUData struct {
	CountryCode      byte
	UserDataTypeCode byte
	ProviderCode     uint16
	UserIdentifier   uint32
}

// IsCEA608 - check if ITU-T data corresponds to CEA-608
func (i ITUData) IsCEA608() bool {
	return (i.CountryCode == 0xb5 &&
		i.ProviderCode == 0x31 &&
		i.UserIdentifier == 0x47413934 &&
		i.UserDataTypeCode == 0x3)
}

// RegisteredSEI - user_data_registered_itu_t_t35 SEI message
type RegisteredSEI struct {
	payload  []byte
	ITUTData ITUData
}

// NewRegisteredSEI - create an ITU-T registered SEI message (type 4)
func NewRegisteredSEI(sd *SEIData, ituData ITUData) *RegisteredSEI {
	return &RegisteredSEI{
		payload:  sd.payload,
		ITUTData: ituData,
	}
}

// Type - SEI payload type
func (s *RegisteredSEI) Type() uint {
	return SEIRegisteredType
}

// Size - size in bytes of raw SEI message rbsp payload
func (s *RegisteredSEI) Size() uint {
	return uint(len(s.payload))
}

func (s *RegisteredSEI) String() string {
	return fmt.Sprintf("SEI type %d, size=%d, %v", s.Type(), s.Size(), s.ITUTData)
}

// Payload - SEI raw rbsp payload
func (s *RegisteredSEI) Payload() []byte {
	return s.payload
}

// NewCEA608sei - new CEA 608 SEI message including parsing of CEA-608 fields
func NewCEA608sei(sd *SEIData) (*CEA608sei, error) {
	field1, field2, err := parseCEA608(sd.payload[8:])
	if err != nil {
		return nil, err
	}
	return &CEA608sei{
		payload: sd.payload,
		Field1:  field1,
		Field2:  field2,
	}, nil
}

// CEA608sei message according to
type CEA608sei struct {
	payload []byte // full raw payload
	Field1  []byte
	Field2  []byte
}

// Type - SEI payload type
func (s *CEA608sei) Type() uint {
	return SEIRegisteredType
}

// Size - size in bytes of raw SEI message rbsp payload
func (s *CEA608sei) Size() uint {
	return uint(len(s.payload))
}

func (s *CEA608sei) String() string {
	return fmt.Sprintf("SEI type %d CEA-608, size=%d, field1: %q, field2: %q", s.Type(), s.Size(),
		hex.EncodeToString(s.Field1), hex.EncodeToString(s.Field2))
}

// Payload - SEI raw rbsp payload
func (s *CEA608sei) Payload() []byte {
	return s.payload
}

func parseCEA608(payload []byte) ([]byte, []byte, error) {
	pos := 0
	ccCount := payload[pos] & 0x1f
	pos += 2 // Advance 1 and skip reserved byte
	var field1 []byte
	var field2 []byte

	for i := byte(0); i < ccCount; i++ {
		if len(payload) < pos+3 {
			return nil, nil, fmt.Errorf("Not enough data for CEA-708 parsing")
		}
		b := payload[pos]
		ccValid := b & 0x4
		ccType := b & 0x3
		pos++
		ccData1 := payload[pos] // Keep parity bit
		pos++
		ccData2 := payload[pos] // Keep parity bit
		pos++
		if ccValid != 0 && ((ccData1&0x7f)+(ccData2&0x7f) != 0) { //Check validity and non-empty data
			if ccType == 0 {
				field1 = append(field1, ccData1)
				field1 = append(field1, ccData2)
			} else if ccType == 1 {
				field2 = append(field2, ccData1)
				field2 = append(field2, ccData2)
			}
		}
	}
	return field1, field2, nil
}

// UnregisteredSEI - SEI message of type 5
type UnregisteredSEI struct {
	UUID    []byte
	payload []byte
}

// Type - SEI payload type
func (s *UnregisteredSEI) Type() uint {
	return SEIUnregisteredType
}

// Size - size in bytes of raw SEI message rbsp payload
func (s *UnregisteredSEI) Size() uint {
	return uint(len(s.payload))
}

func (s *UnregisteredSEI) String() string {
	payloadAfterUUID := string(s.payload[16:])
	return fmt.Sprintf("SEI type %d, size=%d, uuid=%q, payload=%q",
		s.Type(), s.Size(), hex.EncodeToString(s.UUID), payloadAfterUUID)
}

// Payload - SEI raw rbsp payload
func (s *UnregisteredSEI) Payload() []byte {
	return s.payload
}

// DecodeUserDataUnregisteredSEI - Decode an unregistered SEI message (type 5)
func DecodeUserDataUnregisteredSEI(sd *SEIData) (SEIMessage, error) {
	uuid := sd.payload[:16]
	return NewUnregisteredSEI(sd, uuid), nil
}

// NewUnregisteredSEI - Create an unregistered SEI message (type 5)
func NewUnregisteredSEI(sd *SEIData, uuid []byte) *UnregisteredSEI {
	return &UnregisteredSEI{
		UUID:    uuid,
		payload: sd.payload,
	}
}
