package av1

import (
	"encoding/binary"
	"fmt"

	"github.com/Eyevinn/mp4ff/sei"
)

// CreateCTA608MetadataOBU returns a complete metadata OBU carrying CTA-608 closed
// captions: obu_header, obu_size, metadata_type = ITU-T T.35, the CTA-608 T.35/GA94
// header, ccData, and trailing_bits.
//
// ccData is a cc_data() structure as specified in ATSC A/53 Part 4 Section 6.2.2 and
// CTA-708-E Section 4.3 — the same payload the AVC/HEVC SEI path carries, so this is the
// AV1 counterpart of sei.CreateCTA608SEIMessage. Unlike a SEI NAL unit, an OBU has no
// emulation prevention, so the bytes are written as-is.
//
// In an MP4 av01 sample, the returned OBU is placed in the temporal unit that holds the
// frame the captions belong to. ExtractCTA608 is the inverse.
func CreateCTA608MetadataOBU(ccData []byte) []byte {
	// The country code is the first byte of the T.35 payload, but ITUTT35 models it
	// separately, so the itu_t_t35_payload_bytes start at the provider code.
	payload := sei.CreateCTA608Payload(ccData)
	t := ITUTT35{CountryCode: payload[0], Payload: payload[1:]}
	return t.MetadataOBU().Encode()
}

// ITUData returns the registered-user-data identification at the start of the
// itu_t_t35 payload, so that the registered format can be recognized. ok is false if the
// payload is too short to hold the identification.
//
// The AV1 country code and its optional extension byte are modelled by ITUTT35 itself,
// while sei.ITUData assumes a country code other than 0xff; for such a country code the
// returned value is the same 8-byte identification as in an AVC/HEVC SEI message of
// type 4, which is what makes sei.ITUData.IsCTA608 usable for AV1.
func (t ITUTT35) ITUData() (data sei.ITUData, ok bool) {
	if t.CountryCode == countryCodeExtensionEscape || len(t.Payload) < sei.ITUDataSize-1 {
		return sei.ITUData{}, false
	}
	return sei.ITUData{
		CountryCode:      t.CountryCode,
		ProviderCode:     binary.BigEndian.Uint16(t.Payload[0:2]),
		UserIdentifier:   binary.BigEndian.Uint32(t.Payload[2:6]),
		UserDataTypeCode: t.Payload[6],
	}, true
}

// CTA608CCData returns the cc_data() bytes of a CTA-608 itu_t_t35 payload, or nil if t
// does not carry CTA-608. Parse them with sei.ParseCTA608.
func (t ITUTT35) CTA608CCData() []byte {
	data, ok := t.ITUData()
	if !ok || !data.IsCTA608() {
		return nil
	}
	ccData := t.Payload[sei.ITUDataSize-1:]
	if len(ccData) == 0 {
		return nil
	}
	return ccData
}

// ExtractCTA608 returns the CTA-608 field 1 and field 2 byte pairs carried by the OBUs of
// one temporal unit, e.g. an MP4 av01 sample split with SplitOBUs. Parity bits are kept.
// Both fields are nil when the temporal unit carries no CTA-608 captions.
//
// OBUs that are not CTA-608 metadata OBUs are skipped, so a whole temporal unit can be
// passed. An error is only returned for a metadata OBU that is structurally broken or
// whose cc_data() is malformed. This is the inverse of CreateCTA608MetadataOBU, and the
// AV1 counterpart of scanning a sample's NAL units for a CTA-608 SEI message.
func ExtractCTA608(obus []OBU) (field1, field2 []byte, err error) {
	for i, obu := range obus {
		if obu.Header.Type != OBUMetadata {
			continue
		}
		m, err := ParseMetadataOBU(obu.Payload)
		if err != nil {
			return nil, nil, fmt.Errorf("OBU %d: %w", i, err)
		}
		if m.Type != MetadataTypeITUTT35 {
			continue
		}
		t, err := ParseITUTT35(m.Payload)
		if err != nil {
			return nil, nil, fmt.Errorf("OBU %d: %w", i, err)
		}
		ccData := t.CTA608CCData()
		if ccData == nil {
			continue
		}
		f1, f2, err := sei.ParseCTA608(ccData)
		if err != nil {
			return nil, nil, fmt.Errorf("OBU %d: cc_data: %w", i, err)
		}
		field1 = append(field1, f1...)
		field2 = append(field2, f2...)
	}
	return field1, field2, nil
}
