package mp4

import (
	"encoding/hex"
	"io"
	"strings"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/Eyevinn/mp4ff/iamf"
)

// IacbBox - IAMF Configuration Box (iacb)
// Defined in IAMF v1.0 Section 6.2.4
type IacbBox struct {
	ConfigurationVersion byte
	IASequenceData       []byte // Contains the IAMF descriptors (OBUs)
}

// DecodeIacb - box-specific decode
func DecodeIacb(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := readBoxBody(r, hdr)
	if err != nil {
		return nil, err
	}
	sr := bits.NewFixedSliceReader(data)
	return DecodeIacbSR(hdr, startPos, sr)
}

// DecodeIacbSR - box-specific decode
func DecodeIacbSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	b := &IacbBox{}

	// Read configurationVersion (1 byte)
	b.ConfigurationVersion = sr.ReadUint8()

	// Read descriptors_size as LEB128
	descriptorsSize, err := iamf.ReadLeb128(sr)
	if err != nil {
		return nil, err
	}

	// Read the IA Sequence descriptors data
	b.IASequenceData = sr.ReadBytes(int(descriptorsSize))

	return b, sr.AccError()
}

// Type - return box type
func (b *IacbBox) Type() string {
	return "iacb"
}

// Size - return calculated size
func (b *IacbBox) Size() uint64 {
	// 1 byte for version + LEB128 size + descriptor data
	len := len(b.IASequenceData)
	return uint64(boxHeaderSize + 1 + iamf.Leb128Size(uint64(len)) + len)
}

// Encode - write box to w
func (b *IacbBox) Encode(w io.Writer) error {
	sw := bits.NewFixedSliceWriter(int(b.Size()))
	err := b.EncodeSW(sw)
	if err != nil {
		return err
	}
	_, err = w.Write(sw.Bytes())
	return err
}

// EncodeSW - box-specific encode to slicewriter
func (b *IacbBox) EncodeSW(sw bits.SliceWriter) error {
	err := EncodeHeaderSW(b, sw)
	if err != nil {
		return err
	}
	sw.WriteUint8(b.ConfigurationVersion)
	iamf.WriteLeb128(sw, uint64(len(b.IASequenceData)))
	sw.WriteBytes(b.IASequenceData)
	return sw.AccError()
}

// Info - write box info to w
func (b *IacbBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, -1, 0)
	bd.write(" - configurationVersion: %d", b.ConfigurationVersion)
	bd.write(" - descriptorsSize: %d", len(b.IASequenceData))

	// Parse and display OBUs
	level := getInfoLevel(b, specificBoxLevels)
	if level > 0 {
		if level > 1 {
			or := iamf.NewObuReader(b.IASequenceData, len(b.IASequenceData))
			for {
				obu, err := or.ReadObu()
				if err != nil {
					bd.write(" - error parsing OBUs: %v", err)
					break
				}
				if obu == nil {
					break
				}
				// err = obu.Info(func(format string, p ...interface{}) {
				// 	bd.write("   "+format, p...)
				// })
				if err != nil {
					return err
				}
				if level > 2 {
					ctx, err := obu.ReadDescriptors(&or)
					if err != nil {
						bd.write(" - error: %v", err)
						break
					}
					if ctx == nil {
						continue
					}
					// err = ctx.Info(func(level int, format string, p ...interface{}) {
					// 	bd.write("   "+indentStep+strings.Repeat(indentStep, level)+format, p...)
					// })
					if err != nil {
						return err
					}
				} else {
					or.SkipPayload(obu)
				}
			}
		} else {
			bd.write(" - descriptors: %s", hex.EncodeToString(b.IASequenceData))
		}
	}

	return bd.err
}
