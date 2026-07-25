package av1

import (
	"errors"
	"fmt"
)

// Metadata OBU errors
var (
	ErrTruncatedITUTT35 = errors.New("truncated metadata_itu_t_t35 payload")
	ErrNotMetadataOBU   = errors.New("OBU is not a metadata OBU")
	ErrNotITUTT35       = errors.New("metadata OBU is not of type ITU-T T.35")
)

// MetadataType is metadata_type in a metadata OBU (AV1 spec 6.7.1).
type MetadataType uint64

const (
	MetadataTypeHDRCLL      MetadataType = 1 // HDR content light level
	MetadataTypeHDRMDCV     MetadataType = 2 // HDR mastering display colour volume
	MetadataTypeScalability MetadataType = 3
	MetadataTypeITUTT35     MetadataType = 4 // user data registered by ITU-T Rec. T.35
	MetadataTypeTimecode    MetadataType = 5
)

func (t MetadataType) String() string {
	switch t {
	case MetadataTypeHDRCLL:
		return "HDRCLL"
	case MetadataTypeHDRMDCV:
		return "HDRMDCV"
	case MetadataTypeScalability:
		return "Scalability"
	case MetadataTypeITUTT35:
		return "ITUTT35"
	case MetadataTypeTimecode:
		return "Timecode"
	default:
		return fmt.Sprintf("Reserved(%d)", uint64(t))
	}
}

// metadataTrailingByte is trailing_bits() for a byte-aligned metadata payload: the
// trailing_one_bit followed by zero padding to the byte boundary (AV1 spec 5.3.4).
const metadataTrailingByte = 0x80

// countryCodeExtensionEscape signals that itu_t_t35_country_code is followed by an
// extension byte (ITU-T T.35 Annex A).
const countryCodeExtensionEscape = 0xff

// MetadataOBU is a parsed metadata_obu() (AV1 spec 5.8.1): a metadata_type and the
// metadata payload that follows it. It is codec metadata rather than coded video, and
// carries HDR light levels, mastering display colour volume, scalability structure,
// timecodes or ITU-T T.35 registered user data such as closed captions or HDR10+.
type MetadataOBU struct {
	Type MetadataType
	// Payload is the metadata payload after metadata_type, with trailing_bits removed.
	Payload []byte
	// PayloadHasTrailingBits tells OBU and Encode that the last byte of Payload already
	// holds trailing_bits, so that no trailing_bits byte of their own is added.
	// ParseMetadataOBU sets it when the payload is not byte-aligned and therefore shares
	// its last byte with trailing_bits. It stays false for a payload built from scratch,
	// which is byte-aligned and needs its own trailing_bits byte.
	PayloadHasTrailingBits bool
}

// ParseMetadataOBU parses the payload of an OBU_METADATA, i.e. the Payload of an OBU
// with Header.Type == OBUMetadata as returned by SplitOBUs.
//
// The metadata payload has no explicit length — it runs to the end of the OBU, which
// also holds trailing_bits — so its end is found by scanning back from the end of the
// OBU. AV1 spec 6.7.1 requires that the last byte of valid content is the last byte
// that is not equal to zero, so trailing zero bytes are dropped and a final
// trailing_one_bit byte (0x80) is dropped with them.
//
// A payload that is not byte-aligned (possible for metadata_timecode) shares its last
// byte with trailing_bits; such a byte is not 0x80 and is therefore kept, so the caller
// can parse it bit-wise, and PayloadHasTrailingBits is set so that Encode does not add a
// second trailing_one_bit. A non-conformant OBU that omits trailing_bits altogether keeps
// all of its bytes, unless its payload happens to end with 0x80; it is indistinguishable
// from the unaligned case and is therefore re-encoded without trailing_bits as well.
func ParseMetadataOBU(payload []byte) (MetadataOBU, error) {
	mt, n, err := ReadLEB128(payload)
	if err != nil {
		return MetadataOBU{}, fmt.Errorf("metadata_type: %w", err)
	}
	body, hasTrailingBits := trimMetadataTrailingBits(payload[n:])
	return MetadataOBU{
		Type:                   MetadataType(mt),
		Payload:                body,
		PayloadHasTrailingBits: hasTrailingBits,
	}, nil
}

// ParseMetadataOBUFromOBU is ParseMetadataOBU for a full OBU. It returns
// ErrNotMetadataOBU unless obu is a metadata OBU.
func ParseMetadataOBUFromOBU(obu OBU) (MetadataOBU, error) {
	if obu.Header.Type != OBUMetadata {
		return MetadataOBU{}, ErrNotMetadataOBU
	}
	return ParseMetadataOBU(obu.Payload)
}

// trimMetadataTrailingBits drops trailing zero bytes and a final trailing_one_bit byte.
// hasTrailingBits reports that the last remaining byte still holds trailing_bits, which is
// the case when the payload does not end on a byte boundary so that its last byte is not
// 0x80. An empty result never has trailing bits, so that it is re-encoded with them.
func trimMetadataTrailingBits(payload []byte) (trimmed []byte, hasTrailingBits bool) {
	end := len(payload)
	for end > 0 && payload[end-1] == 0 {
		end--
	}
	if end > 0 && payload[end-1] == metadataTrailingByte {
		return payload[:end-1], false
	}
	return payload[:end], end > 0
}

// OBU returns m as an OBU ready for Encode: metadata_type, the payload, and the
// mandatory trailing_bits byte. Metadata OBUs always carry trailing_bits (AV1 spec 5.2:
// every OBU type except tile group, tile list and frame does), but the byte is left out
// when PayloadHasTrailingBits says the payload already ends with them, since a second
// trailing_one_bit would make the OBU non-conformant.
func (m MetadataOBU) OBU() OBU {
	size := leb128Len(uint64(m.Type)) + len(m.Payload)
	if !m.PayloadHasTrailingBits {
		size++
	}
	payload := make([]byte, 0, size)
	payload = appendLEB128(payload, uint64(m.Type))
	payload = append(payload, m.Payload...)
	if !m.PayloadHasTrailingBits {
		payload = append(payload, metadataTrailingByte)
	}
	return OBU{
		Header:  OBUHeader{Type: OBUMetadata, HasSizeField: true, HeaderSize: 1},
		Payload: payload,
	}
}

// Encode returns the complete metadata OBU: obu_header, obu_size, metadata_type, the
// payload and trailing_bits. AV1 OBUs carry no emulation-prevention bytes, so the
// payload is written as-is.
func (m MetadataOBU) Encode() []byte {
	return m.OBU().Encode()
}

// Size returns the number of bytes Encode would write.
func (m MetadataOBU) Size() int {
	return m.OBU().Size()
}

// ITUTT35 is a parsed metadata_itu_t_t35() (AV1 spec 5.8.2): user data registered by
// ITU-T Rec. T.35, identified by a country code. Country code 0xb5 (USA) covers both
// ATSC A/53 closed captions (see CreateCTA608MetadataOBU) and SMPTE ST 2094-40 (HDR10+),
// which differ by the provider code at the start of the payload.
type ITUTT35 struct {
	CountryCode byte
	// CountryCodeExtension is itu_t_t35_country_code_extension_byte. It is only present,
	// and only valid, when CountryCode is 0xff.
	CountryCodeExtension byte
	// Payload is itu_t_t35_payload_bytes: everything after the country code, starting
	// with the terminal provider code.
	Payload []byte
}

// ParseITUTT35 parses a metadata_itu_t_t35() from the payload of a metadata OBU of type
// MetadataTypeITUTT35, i.e. from MetadataOBU.Payload.
func ParseITUTT35(payload []byte) (ITUTT35, error) {
	if len(payload) < 1 {
		return ITUTT35{}, ErrTruncatedITUTT35
	}
	t := ITUTT35{CountryCode: payload[0]}
	pos := 1
	if t.CountryCode == countryCodeExtensionEscape {
		if len(payload) < 2 {
			return ITUTT35{}, ErrTruncatedITUTT35
		}
		t.CountryCodeExtension = payload[1]
		pos = 2
	}
	t.Payload = payload[pos:]
	return t, nil
}

// Encode returns the metadata_itu_t_t35() body: the country code, its extension byte
// when the country code is 0xff, and the payload.
func (t ITUTT35) Encode() []byte {
	out := make([]byte, 0, t.Size())
	out = append(out, t.CountryCode)
	if t.CountryCode == countryCodeExtensionEscape {
		out = append(out, t.CountryCodeExtension)
	}
	return append(out, t.Payload...)
}

// Size returns the number of bytes Encode would write.
func (t ITUTT35) Size() int {
	n := 1 + len(t.Payload)
	if t.CountryCode == countryCodeExtensionEscape {
		n++
	}
	return n
}

// MetadataOBU wraps t as a metadata OBU of type ITU-T T.35.
func (t ITUTT35) MetadataOBU() MetadataOBU {
	return MetadataOBU{Type: MetadataTypeITUTT35, Payload: t.Encode()}
}
