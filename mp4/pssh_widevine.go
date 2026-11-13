package mp4

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"unicode/utf8"
)

// WidevinePsshData is the data of a Widevine pssh box: a WidevinePsshData protobuf message, as in
// https://github.com/shaka-project/shaka-packager/blob/main/packager/media/base/widevine_pssh_data.proto.
// The fields are in the order of their field numbers. A pointer field is nil if the message does not have it.
type WidevinePsshData struct {
	// Algorithm is 0 for UNENCRYPTED and 1 for AESCTR. Deprecated, but still found in content.
	Algorithm *uint32
	// KeyIDs are the content or entitlement key IDs.
	KeyIDs []UUID
	// Provider is the name of the content provider. Deprecated.
	Provider string
	// ContentID identifies the content, and may map to several keys.
	ContentID []byte
	// TrackType is SD, HD or AUDIO. Deprecated.
	TrackType string
	// Policy is the name of a registered policy. Deprecated.
	Policy string
	// CryptoPeriodIndex is the crypto period of content with key rotation.
	CryptoPeriodIndex *uint32
	// GroupedLicense is a serialized SignedMessage. Deprecated.
	GroupedLicense []byte
	// ProtectionScheme is the four-character code of the scheme, such as cenc or cbcs, as an integer.
	ProtectionScheme *uint32
	// CryptoPeriodSeconds is the duration of each crypto period of content with key rotation.
	CryptoPeriodSeconds *uint32
	// Type is 0 for SINGLE, 1 for ENTITLEMENT and 2 for ENTITLED_KEY. SINGLE is the default.
	Type *uint32
	// KeySequence is the key sequence of keys managed by Widevine.
	KeySequence *uint32
	// GroupIDs are the groups that the content belongs to.
	GroupIDs [][]byte
	// EntitledKeys are the wrapped content keys of an ENTITLED_KEY pssh.
	EntitledKeys []WidevineEntitledKey
	// VideoFeature selects keys together with ContentID, such as HDR.
	VideoFeature string
}

// WidevineEntitledKey is a content key wrapped with an entitlement key, an EntitledKey message in a
// WidevinePsshData message.
type WidevineEntitledKey struct {
	EntitlementKeyID UUID
	KeyID            UUID
	Key              []byte
	IV               []byte
	// EntitlementKeySizeBytes is the size of the entitlement key. Its default is 32.
	EntitlementKeySizeBytes *uint32
}

// Bit masks of the numbers of the fields of WidevinePsshData and EntitledKey with the varint and length-delimited
// wire types.
const (
	wvVarintFields         = 1<<1 | 1<<7 | 1<<9 | 1<<10 | 1<<11 | 1<<12
	wvLenFields            = 1<<2 | 1<<3 | 1<<4 | 1<<5 | 1<<6 | 1<<8 | 1<<13 | 1<<14 | 1<<15
	wvEntitledVarintFields = 1 << 5
	wvEntitledLenFields    = 1<<1 | 1<<2 | 1<<3 | 1<<4
)

// Protobuf wire types.
const (
	protobufVarint = 0
	protobufI64    = 1
	protobufLen    = 2
	protobufI32    = 5
)

// DecodeWidevinePsshData decodes the data of a Widevine pssh box. Fields with unknown numbers are skipped.
func DecodeWidevinePsshData(data []byte) (*WidevinePsshData, error) {
	d := &WidevinePsshData{}
	err := decodeProtobuf(data, wvVarintFields, wvLenFields, func(num int, v uint64, b []byte) error {
		switch num {
		case 1:
			d.Algorithm = uint32Ptr(v)
		case 2:
			d.KeyIDs = append(d.KeyIDs, UUID(b))
		case 3:
			d.Provider = string(b)
		case 4:
			d.ContentID = b
		case 5:
			d.TrackType = string(b)
		case 6:
			d.Policy = string(b)
		case 7:
			d.CryptoPeriodIndex = uint32Ptr(v)
		case 8:
			d.GroupedLicense = b
		case 9:
			d.ProtectionScheme = uint32Ptr(v)
		case 10:
			d.CryptoPeriodSeconds = uint32Ptr(v)
		case 11:
			d.Type = uint32Ptr(v)
		case 12:
			d.KeySequence = uint32Ptr(v)
		case 13:
			d.GroupIDs = append(d.GroupIDs, b)
		case 14:
			k, err := decodeWidevineEntitledKey(b)
			if err != nil {
				return fmt.Errorf("entitled key: %w", err)
			}
			d.EntitledKeys = append(d.EntitledKeys, k)
		case 15:
			d.VideoFeature = string(b)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("widevine pssh data: %w", err)
	}
	return d, nil
}

func decodeWidevineEntitledKey(data []byte) (WidevineEntitledKey, error) {
	var k WidevineEntitledKey
	err := decodeProtobuf(data, wvEntitledVarintFields, wvEntitledLenFields, func(num int, v uint64, b []byte) error {
		switch num {
		case 1:
			k.EntitlementKeyID = UUID(b)
		case 2:
			k.KeyID = UUID(b)
		case 3:
			k.Key = b
		case 4:
			k.IV = b
		case 5:
			k.EntitlementKeySizeBytes = uint32Ptr(v)
		}
		return nil
	})
	return k, err
}

// decodeProtobuf calls fn for each field of the protobuf message in data whose number is in the bit mask
// varintFields or lenFields, with the value v of a varint field or the bytes b of a length-delimited field.
// It skips fields with other numbers.
func decodeProtobuf(data []byte, varintFields, lenFields uint64, fn func(num int, v uint64, b []byte) error) error {
	for len(data) > 0 {
		key, n := binary.Uvarint(data)
		if n <= 0 {
			return errors.New("bad field key")
		}
		data = data[n:]
		num, wireType := key>>3, key&7
		var v uint64
		var b []byte
		switch wireType {
		case protobufVarint:
			if v, n = binary.Uvarint(data); n <= 0 {
				return fmt.Errorf("field %d: bad varint", num)
			}
		case protobufLen:
			length, m := binary.Uvarint(data)
			if m <= 0 || length > uint64(len(data)-m) {
				return fmt.Errorf("field %d: bad length", num)
			}
			n = m + int(length)
			b = data[m:n]
		case protobufI64:
			n = 8
		case protobufI32:
			n = 4
		default:
			return fmt.Errorf("field %d: unsupported wire type %d", num, wireType)
		}
		if n > len(data) {
			return fmt.Errorf("field %d: %d bytes are too few", num, len(data))
		}
		data = data[n:]
		var bit uint64
		if num < 64 {
			bit = 1 << num
		}
		switch {
		case varintFields&bit != 0 && wireType == protobufVarint, lenFields&bit != 0 && wireType == protobufLen:
			if err := fn(int(num), v, b); err != nil {
				return err
			}
		case (varintFields|lenFields)&bit != 0:
			return fmt.Errorf("field %d: wrong wire type %d", num, wireType)
		}
	}
	return nil
}

func uint32Ptr(v uint64) *uint32 {
	u := uint32(v)
	return &u
}

// widevineTypes are the names of the values of WidevinePsshData.Type.
var widevineTypes = []string{"SINGLE", "ENTITLEMENT", "ENTITLED_KEY"}

// widevineAlgorithms are the names of the values of WidevinePsshData.Algorithm.
var widevineAlgorithms = []string{"UNENCRYPTED", "AESCTR"}

// Info writes the fields of d to w, one " - name: value" line each, every line starting with indent.
func (d *WidevinePsshData) Info(w io.Writer, indent string) error {
	bd := &infoDumper{w: w, indent: indent}
	if d.Algorithm != nil {
		bd.write(" - algorithm: %s", enumText(*d.Algorithm, widevineAlgorithms))
	}
	for i, kid := range d.KeyIDs {
		bd.write(" - keyID[%d]: %s", i+1, kid)
	}
	writeText(bd, "provider", d.Provider)
	if len(d.ContentID) > 0 {
		bd.write(" - contentID: %s", bytesText(d.ContentID))
	}
	writeText(bd, "trackType", d.TrackType)
	writeText(bd, "policy", d.Policy)
	writeUint32(bd, "cryptoPeriodIndex", d.CryptoPeriodIndex)
	if len(d.GroupedLicense) > 0 {
		bd.write(" - groupedLicense: %s", hex.EncodeToString(d.GroupedLicense))
	}
	if d.ProtectionScheme != nil {
		scheme := binary.BigEndian.AppendUint32(nil, *d.ProtectionScheme)
		if isPrintable(scheme) {
			bd.write(" - protectionScheme: %s", scheme)
		} else {
			bd.write(" - protectionScheme: 0x%08x", *d.ProtectionScheme)
		}
	}
	writeUint32(bd, "cryptoPeriodSeconds", d.CryptoPeriodSeconds)
	if d.Type != nil {
		bd.write(" - type: %s", enumText(*d.Type, widevineTypes))
	}
	writeUint32(bd, "keySequence", d.KeySequence)
	for i, id := range d.GroupIDs {
		bd.write(" - groupID[%d]: %s", i+1, bytesText(id))
	}
	for i, k := range d.EntitledKeys {
		bd.write(" - entitledKey[%d]:", i+1)
		if bd.err == nil {
			bd.err = k.Info(w, indent+"  ")
		}
	}
	writeText(bd, "videoFeature", d.VideoFeature)
	return bd.err
}

// Info writes the fields of k to w, one " - name: value" line each, every line starting with indent.
func (k *WidevineEntitledKey) Info(w io.Writer, indent string) error {
	bd := &infoDumper{w: w, indent: indent}
	bd.write(" - entitlementKeyID: %s", k.EntitlementKeyID)
	bd.write(" - keyID: %s", k.KeyID)
	bd.write(" - key: %s", hex.EncodeToString(k.Key))
	bd.write(" - iv: %s", hex.EncodeToString(k.IV))
	writeUint32(bd, "entitlementKeySizeBytes", k.EntitlementKeySizeBytes)
	return bd.err
}

func enumText(v uint32, names []string) string {
	if int(v) < len(names) {
		return fmt.Sprintf("%d (%s)", v, names[v])
	}
	return fmt.Sprint(v)
}

func writeText(bd *infoDumper, name, s string) {
	if s != "" {
		bd.write(" - %s: %q", name, s)
	}
}

func writeUint32(bd *infoDumper, name string, v *uint32) {
	if v != nil {
		bd.write(" - %s: %d", name, *v)
	}
}

// bytesText writes b as a quoted string if it is printable text, and in hex otherwise.
func bytesText(b []byte) string {
	if isPrintable(b) {
		return fmt.Sprintf("%q", b)
	}
	return hex.EncodeToString(b)
}

// isPrintable tells if b is non-empty UTF-8 text without control characters.
func isPrintable(b []byte) bool {
	if len(b) == 0 || !utf8.Valid(b) {
		return false
	}
	for _, r := range string(b) {
		if r < 0x20 || r == 0x7f {
			return false
		}
	}
	return true
}
