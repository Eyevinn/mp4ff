package mp4

import (
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
)

const (
	UUID_PlayReady = "9a04f079-9840-4286-ab92-e65be0885f95"
	UUID_Widevine  = "edef8ba9-79d6-4ace-a3c8-27dcd51d21ed"
	UUID_FairPlay  = "94CE86FB-07FF-4F43-ADB8-93D2FA968CA2"
	UUID_VCAS      = "9a27dd82-fde2-4725-8cbc-4234aa06ec09"
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
	case UUID_PlayReady:
		return "PlayReady"
	case UUID_Widevine:
		return "Widevine"
	case UUID_FairPlay:
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
func DecodePssh(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	s := NewSliceReader(data)
	versionAndFlags := s.ReadUint32()
	version := byte(versionAndFlags >> 24)

	b := &PsshBox{
		Version: version,
		Flags:   versionAndFlags & flagsMask,
	}
	b.SystemID = UUID(s.ReadFixedLengthString(16))
	if b.Version > 0 {
		kidCount := s.ReadUint32()
		for i := uint32(0); i < kidCount; i++ {
			b.KIDs = append(b.KIDs, UUID(s.ReadFixedLengthString(16)))

		}
	}
	dataLength := int(s.ReadUint32())
	b.Data = s.ReadBytes(dataLength)
	return b, nil
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
	err := EncodeHeader(b, w)
	if err != nil {
		return err
	}
	buf := makebuf(b)
	sw := NewSliceWriter(buf)
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
	_, err = w.Write(buf)
	return err
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
