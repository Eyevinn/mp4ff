package mp4

import (
	"encoding/hex"
	"io"

	"github.com/edgeware/mp4ff/bits"
)

const (
	uuidTfxd = "\x6d\x1d\x9b\x05\x42\xd5\x44\xe6\x80\xe2\x14\x1d\xaf\xf7\x57\xb2"
	uuidTfrf = "\xd4\x80\x7e\xf2\xca\x39\x46\x95\x8e\x54\x26\xcb\x9e\x46\xa7\x9f"
	uuidPIFF = "\xa2\x39\x4f\x52\x5a\x9b\x4f\x14\xa2\x44\x6c\x42\x7c\x64\x8d\xf4"
)

// UUIDBox - Used as container for MSS boxes tfxd and tfrf
type UUIDBox struct {
	UUID    string // 16 bytes
	SubType string
	Tfxd    *TfxdData
	Tfrf    *TfrfData
	PIFF	PiffData
}

// TfxdData - MSS TfxdBox data after UUID part
// Defined in MSS-SSTR v20180912 section 2.2.4.4
type TfxdData struct {
	Version                  byte
	Flags                    uint32
	FragmentAbsoluteTime     uint64
	FragmentAbsoluteDuration uint64
}

// TfrfData - MSS TfrfBox data after UUID part
// Defined in MSS-SSTR v20180912 section 2.2.4.5
type TfrfData struct {
	Version                   byte
	Flags                     uint32
	FragmentCount             byte
	FragmentAbsoluteTimes     []uint64
	FragmentAbsoluteDurations []uint64
}

type PiffData []byte

// DecodeUUIDBox - decode a UUID box including tfxd or tfrf
func DecodeUUIDBox(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := readBoxBody(r, hdr)
	if err != nil {
		return nil, err
	}
	sr := bits.NewFixedSliceReader(data)
	return DecodeUUIDBoxSR(hdr, startPos, sr)
}

// DecodeUUIDBoxSR - decode a UUID box including tfxd or tfrf
func DecodeUUIDBoxSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	b := &UUIDBox{}
	b.UUID = string(sr.ReadBytes(16))
	switch b.UUID {
	case uuidTfxd:
		b.SubType = "tfxd"
		tfxd, err := decodeTfxd(sr)
		if err != nil {
			return nil, err
		}
		b.Tfxd = tfxd
	case uuidTfrf:
		b.SubType = "tfrf"
		tfrf, err := decodeTfrf(sr)
		if err != nil {
			return nil, err
		}
		b.Tfrf = tfrf
	case uuidPIFF:
		b.SubType = "piff"
		b.PIFF = sr.ReadBytes(int(hdr.Size) - 16 - 8)
	default:
		// err := fmt.Errorf("Unknown uuid=%s", b.UUID)
		// return nil, err
	}

	return b, sr.AccError()
}

// Type - return box type
func (b *UUIDBox) Type() string {
	return "uuid"
}

// Size - return calculated size including tfxd/tfrf
func (b *UUIDBox) Size() uint64 {
	var size uint64 = 8 + 16
	switch b.SubType {
	case "tfxd":
		size += b.Tfxd.size()
	case "tfrf":
		size += b.Tfrf.size()
	case "piff":
		size += b.PIFF.size()
	}
	return size
}

// Encode - write box to w
func (b *UUIDBox) Encode(w io.Writer) error {
	sw := bits.NewFixedSliceWriter(int(b.Size()))
	err := b.EncodeSW(sw)
	if err != nil {
		return err
	}
	_, err = w.Write(sw.Bytes())
	return err
}

// EncodeSW - box-specific encode to slicewriter
func (b *UUIDBox) EncodeSW(sw bits.SliceWriter) error {
	err := EncodeHeaderSW(b, sw)
	if err != nil {
		return err
	}
	sw.WriteString(b.UUID, false)
	if b.SubType == "tfxd" {
		err = b.Tfxd.encode(sw)
	} else if b.SubType == "tfrf" {
		err = b.Tfrf.encode(sw)
	}
	return err
}

func decodeTfxd(s bits.SliceReader) (*TfxdData, error) {
	versionAndFlags := s.ReadUint32()
	version := byte(versionAndFlags >> 24)
	var fragmentAbsoluteTime uint64
	var fragmentAbsoluteDuration uint64
	if version == 0 {
		fragmentAbsoluteTime = uint64(s.ReadUint32())
		fragmentAbsoluteDuration = uint64(s.ReadUint32())
	} else {
		fragmentAbsoluteTime = s.ReadUint64()
		fragmentAbsoluteDuration = s.ReadUint64()
	}

	t := &TfxdData{
		Version:                  version,
		Flags:                    versionAndFlags & flagsMask,
		FragmentAbsoluteTime:     fragmentAbsoluteTime,
		FragmentAbsoluteDuration: fragmentAbsoluteDuration,
	}
	return t, nil
}

func (t *TfxdData) size() uint64 {
	return 4 + 8 + 8*uint64(t.Version)
}

func (t *TfxdData) encode(sw bits.SliceWriter) error {
	versionAndFlags := (uint32(t.Version) << 24) + t.Flags
	sw.WriteUint32(versionAndFlags)
	if t.Version == 0 {
		sw.WriteUint32(uint32(t.FragmentAbsoluteTime))
		sw.WriteUint32(uint32(t.FragmentAbsoluteDuration))
	} else {
		sw.WriteUint64(t.FragmentAbsoluteTime)
		sw.WriteUint64(t.FragmentAbsoluteDuration)
	}
	return sw.AccError()
}

func decodeTfrf(s bits.SliceReader) (*TfrfData, error) {
	versionAndFlags := s.ReadUint32()
	version := byte(versionAndFlags >> 24)
	t := &TfrfData{
		Version:       version,
		Flags:         versionAndFlags & flagsMask,
		FragmentCount: s.ReadUint8(),
	}
	if t.Version == 0 {
		for i := byte(0); i < t.FragmentCount; i++ {
			t.FragmentAbsoluteTimes = append(t.FragmentAbsoluteTimes, uint64(s.ReadUint32()))
			t.FragmentAbsoluteDurations = append(t.FragmentAbsoluteDurations, uint64(s.ReadUint32()))
		}
	} else {
		for i := byte(0); i < t.FragmentCount; i++ {
			t.FragmentAbsoluteTimes = append(t.FragmentAbsoluteTimes, s.ReadUint64())
			t.FragmentAbsoluteDurations = append(t.FragmentAbsoluteDurations, s.ReadUint64())
		}
	}
	return t, nil
}

func (t *TfrfData) size() uint64 {
	return 4 + 1 + (8+8*uint64(t.Version))*uint64(t.FragmentCount)
}

func (t *TfrfData) encode(sw bits.SliceWriter) error {
	versionAndFlags := (uint32(t.Version) << 24) + t.Flags
	sw.WriteUint32(versionAndFlags)
	sw.WriteUint8(t.FragmentCount)
	if t.Version == 0 {
		for i := byte(0); i < t.FragmentCount; i++ {
			sw.WriteUint32(uint32(t.FragmentAbsoluteTimes[i]))
			sw.WriteUint32(uint32(t.FragmentAbsoluteDurations[i]))
		}
	} else {
		for i := byte(0); i < t.FragmentCount; i++ {
			sw.WriteUint64(t.FragmentAbsoluteTimes[i])
			sw.WriteUint64(t.FragmentAbsoluteDurations[i])
		}
	}
	return sw.AccError()
}

func (p PiffData) size() uint64 {
	return uint64(len(p))
}

// Info - box-specific info
func (b *UUIDBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, -1, 0)
	bd.write(" - uuid: %s", hex.EncodeToString([]byte(b.UUID)))
	bd.write(" - subType: %s", b.SubType)
	level := getInfoLevel(b, specificBoxLevels)
	if level > 0 {
		switch b.SubType {
		case "tfxd":
			bd.write(" - absTime=%d absDur=%d", b.Tfxd.FragmentAbsoluteTime, b.Tfxd.FragmentAbsoluteDuration)
		case "tfrf":
			for i := 0; i < int(b.Tfrf.FragmentCount); i++ {
				bd.write(" - [%d]: absTime=%d absDur=%d", i+1, b.Tfrf.FragmentAbsoluteTimes[i], b.Tfrf.FragmentAbsoluteDurations[i])
			}
		}
	}
	return bd.err
}
