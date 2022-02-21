package mp4

import (
	"encoding/hex"
	"fmt"
	"io"

	"github.com/edgeware/mp4ff/bits"
)

// UUIDs for different DRM systems
const (
	UUIDPlayReady = "9a04f079-9840-4286-ab92-e65be0885f95"
	UUIDWidevine  = "edef8ba9-79d6-4ace-a3c8-27dcd51d21ed"
	UUIDFairPlay  = "94CE86FB-07FF-4F43-ADB8-93D2FA968CA2"
	UUID_VCAS     = "9a27dd82-fde2-4725-8cbc-4234aa06ec09"
)

// UUID - 16-byte KeyID or SystemID
type UUID []byte

func (u UUID) String() string {
	h := hex.EncodeToString(u)
	if len(u) != 16 {
		return h
	}
	return fmt.Sprintf("%s-%s-%s-%s-%s", h[0:8], h[8:12], h[12:16], h[16:20], h[20:32])
}

func systemName(systemID UUID) string {
	uStr := systemID.String()
	switch uStr {
	case UUIDPlayReady:
		return "PlayReady"
	case UUIDWidevine:
		return "Widevine"
	case UUIDFairPlay:
		return "FairPlay"
	case UUID_VCAS:
		return "Verimatrix VCAS"
	default:
		return "Unknown"
	}
}

// PsshBox - Protection System Specific Header Box
// Defined in ISO/IEC 23001-7 Secion 8.1
type PsshBox struct {
	Version  byte
	Flags    uint32
	SystemID UUID
	KIDs     []UUID
	Data     []byte
}

// DecodePssh - box-specific decode
func DecodePssh(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := readBoxBody(r, hdr)
	if err != nil {
		return nil, err
	}
	sr := bits.NewFixedSliceReader(data)
	return DecodePsshSR(hdr, startPos, sr)
}

// DecodePsshSR - box-specific decode
func DecodePsshSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	versionAndFlags := sr.ReadUint32()
	version := byte(versionAndFlags >> 24)

	b := PsshBox{
		Version: version,
		Flags:   versionAndFlags & flagsMask,
	}
	b.SystemID = UUID(sr.ReadFixedLengthString(16))
	if b.Version > 0 {
		kidCount := sr.ReadUint32()
		for i := uint32(0); i < kidCount; i++ {
			b.KIDs = append(b.KIDs, UUID(sr.ReadFixedLengthString(16)))

		}
	}
	dataLength := int(sr.ReadUint32())
	b.Data = sr.ReadBytes(dataLength)
	return &b, sr.AccError()
}

// Type - return box type
func (b *PsshBox) Type() string {
	return "pssh"
}

// Size - return calculated size
func (b *PsshBox) Size() uint64 {
	size := uint64(12 + 16 + 4 + len(b.Data))
	if b.Version > 0 {
		size += uint64(4 + 16*len(b.KIDs))
	}
	return size
}

// Encode - write box to w
func (b *PsshBox) Encode(w io.Writer) error {
	sw := bits.NewFixedSliceWriter(int(b.Size()))
	err := b.EncodeSW(sw)
	if err != nil {
		return err
	}
	_, err = w.Write(sw.Bytes())
	return err
}

// EncodeSW - box-specific encode to slicewriter
func (b *PsshBox) EncodeSW(sw bits.SliceWriter) error {
	err := EncodeHeaderSW(b, sw)
	if err != nil {
		return err
	}
	versionAndFlags := (uint32(b.Version) << 24) + b.Flags
	sw.WriteUint32(versionAndFlags)
	sw.WriteBytes(b.SystemID)
	if b.Version > 0 {
		sw.WriteUint32(uint32(len(b.KIDs)))
		for _, kid := range b.KIDs {
			sw.WriteBytes(kid)
		}
	}
	sw.WriteUint32(uint32(len(b.Data)))
	sw.WriteBytes(b.Data)
	return sw.AccError()
}

// Info - write box info to w
func (b *PsshBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) (err error) {
	bd := newInfoDumper(w, indent, b, int(b.Version), b.Flags)
	bd.write(" - systemID: %s (%s)", b.SystemID, systemName(b.SystemID))
	if b.Version > 0 {
		for i, kid := range b.KIDs {
			bd.write(" - KID[%d]=%s", i+1, kid)
		}
	}
	level := getInfoLevel(b, specificBoxLevels)
	if level > 0 {
		bd.write(" - data: %s", hex.EncodeToString(b.Data))
	}
	return bd.err
}
