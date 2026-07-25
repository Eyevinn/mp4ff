package sei

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
)

// DecodeUserDataRegisteredSEI decodes a SEI message of type 4.
func DecodeUserDataRegisteredSEI(sd *SEIData) (SEIMessage, error) {
	if len(sd.payload) < ITUDataSize {
		return nil, errShortITUTT35Payload(len(sd.payload))
	}
	itutData := ITUData{
		CountryCode:      sd.payload[0],
		ProviderCode:     binary.BigEndian.Uint16(sd.payload[1:3]),
		UserIdentifier:   binary.BigEndian.Uint32(sd.payload[3:7]),
		UserDataTypeCode: sd.payload[7],
	}
	if itutData.IsCTA608() {
		return ExtractCTA608sei(sd)
	}
	return NewRegisteredSEI(sd, itutData), nil
}

// ITUData identifies registered payload in SEI of type 4 (User data registered by ITU-T Rec T 35).
type ITUData struct {
	CountryCode      byte
	UserDataTypeCode byte
	ProviderCode     uint16
	UserIdentifier   uint32
}

// ITUDataSize is the size in bytes of the ITUData header that starts a
// user_data_registered_itu_t_t35 payload.
const ITUDataSize = 8

// errShortITUTT35Payload reports a type 4 payload too short to hold the ITUData header.
func errShortITUTT35Payload(n int) error {
	return fmt.Errorf("SEI type 4 (user_data_registered_itu_t_t35) payload too short: %d bytes", n)
}

// CTA608ITUData returns the ITUData that identifies ATSC A/53 CTA-608 closed captions:
// country code 0xb5 (USA), provider code 0x0031 (ATSC), user identifier "GA94" and
// user_data_type_code 0x03. The same identification is used in an AVC/HEVC SEI message
// of type 4 and in an AV1 metadata OBU of type ITU-T T.35.
func CTA608ITUData() ITUData {
	return ITUData{
		CountryCode:      0xb5,
		ProviderCode:     0x31,
		UserIdentifier:   0x47413934, // "GA94"
		UserDataTypeCode: 0x03,
	}
}

// IsCTA608 checks if ITU-T data corresponds to CTA-608.
func (i ITUData) IsCTA608() bool {
	return (i.CountryCode == 0xb5 &&
		i.ProviderCode == 0x31 &&
		i.UserIdentifier == 0x47413934 &&
		i.UserDataTypeCode == 0x3)
}

// Encode returns the ITUDataSize bytes that start a user_data_registered_itu_t_t35
// payload. Like the decoder, it assumes a country code other than 0xff, i.e. that no
// itu_t_t35_country_code_extension_byte is present.
func (i ITUData) Encode() []byte {
	out := make([]byte, ITUDataSize)
	out[0] = i.CountryCode
	binary.BigEndian.PutUint16(out[1:3], i.ProviderCode)
	binary.BigEndian.PutUint32(out[3:7], i.UserIdentifier)
	out[7] = i.UserDataTypeCode
	return out
}

// CreateCTA608Payload returns the payload of a CTA-608 user_data_registered_itu_t_t35
// message: the CTA608ITUData header followed by ccData.
//
// ccData is a cc_data() structure as specified in ATSC A/53 Part 4 Section 6.2.2 and
// CTA-708-E Section 4.3. It is the same payload in AVC/HEVC and AV1, so this is the
// common part of CreateCTA608SEIMessage and av1.CreateCTA608MetadataOBU. The bytes are
// not validated here; ParseCTA608 is the inverse.
func CreateCTA608Payload(ccData []byte) []byte {
	payload := make([]byte, 0, ITUDataSize+len(ccData))
	payload = append(payload, CTA608ITUData().Encode()...)
	return append(payload, ccData...)
}

// CreateCTA608SEIMessage returns a user_data_registered_itu_t_t35 (type 4) SEI message
// carrying CTA-608 closed captions, the encode-side counterpart of ExtractCTA608sei.
// Turn it into a NAL unit with avc.CreateSEINalu or hevc.CreateSEINalu; the AV1
// counterpart is av1.CreateCTA608MetadataOBU.
//
// ccData is a cc_data() structure (see CreateCTA608Payload).
func CreateCTA608SEIMessage(ccData []byte) SEIMessage {
	return NewSEIData(SEIUserDataRegisteredITUtT35Type, CreateCTA608Payload(ccData))
}

// RegisteredSEI is user_data_registered_itu_t_t35 (type 4) SEI message.
type RegisteredSEI struct {
	payload  []byte
	ITUTData ITUData
}

// NewRegisteredSEI creates an ITU-T registered SEI message (type 4).
func NewRegisteredSEI(sd *SEIData, ituData ITUData) *RegisteredSEI {
	return &RegisteredSEI{
		payload:  sd.payload,
		ITUTData: ituData,
	}
}

// Type returns the SEI payload type.
func (s *RegisteredSEI) Type() uint {
	return SEIUserDataRegisteredITUtT35Type
}

// Size returns size in bytes of raw SEI message rbsp payload.
func (s *RegisteredSEI) Size() uint {
	return uint(len(s.payload))
}

// String provides a short description of the SEI message.
func (s *RegisteredSEI) String() string {
	return fmt.Sprintf("SEI type %d, size=%d, %v", s.Type(), s.Size(), s.ITUTData)
}

// Payload returns the SEI raw rbsp payload.
func (s *RegisteredSEI) Payload() []byte {
	return s.payload
}

// ExtractCTA608sei returns payload and parsed field for a CTA-608 SEI message.
// CTA-608 encapsulation in SEI nal unit is defined in ATSC-120 and further
// in CTA-708 specification. CTA-608 and CTA-708 were previously named CEA-608
// and CEA-708. CreateCTA608SEIMessage is the inverse.
//
// An error is returned for a payload too short to hold the ITUData header, so that the
// function is safe to call on an arbitrary type 4 SEI message.
func ExtractCTA608sei(sd *SEIData) (*CTA608sei, error) {
	if len(sd.payload) < ITUDataSize {
		return nil, errShortITUTT35Payload(len(sd.payload))
	}
	field1, field2, err := ParseCTA608(sd.payload[ITUDataSize:])
	if err != nil {
		return nil, err
	}
	return &CTA608sei{
		payload: sd.payload,
		Field1:  field1,
		Field2:  field2,
	}, nil
}

// CTA608sei data structure.
type CTA608sei struct {
	payload []byte // full raw payload
	Field1  []byte
	Field2  []byte
}

// Type returns the SEI payload type.
func (s *CTA608sei) Type() uint {
	return SEIUserDataRegisteredITUtT35Type
}

// Size is size in bytes of raw SEI message rbsp payload.
func (s *CTA608sei) Size() uint {
	return uint(len(s.payload))
}

// String provides a simple representation of the CTA608 data.
func (s *CTA608sei) String() string {
	return fmt.Sprintf("SEI type %d CTA-608, size=%d, field1: %q, field2: %q", s.Type(), s.Size(),
		hex.EncodeToString(s.Field1), hex.EncodeToString(s.Field2))
}

// Payload returns the SEI raw rbsp payload.
func (s *CTA608sei) Payload() []byte {
	return s.payload
}

// ParseCTA608 parses the fields of data from CTA-708 encapsulation.
// This is specified in Section 4.3 of ANSI/CTA-708-E R-2018.
func ParseCTA608(payload []byte) ([]byte, []byte, error) {
	if len(payload) == 0 {
		return nil, nil, fmt.Errorf("empty cc_data payload")
	}
	pos := 0
	ccCount := payload[pos] & 0x1f
	pos += 2 // Advance 1 and skip reserved byte
	var field1 []byte
	var field2 []byte

	for i := byte(0); i < ccCount; i++ {
		if len(payload) < pos+3 {
			return nil, nil, fmt.Errorf("not enough data for CTA-708 parsing")
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
			switch ccType {
			case 0:
				field1 = append(field1, ccData1)
				field1 = append(field1, ccData2)
			case 1:
				field2 = append(field2, ccData1)
				field2 = append(field2, ccData2)
			}
		}
	}
	// There should also be a 0xff marker bits byte before the end of the NALU
	return field1, field2, nil
}
