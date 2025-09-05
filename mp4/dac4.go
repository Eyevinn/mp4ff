package mp4

import (
	"bytes"
	"fmt"
	"io"

	"github.com/Eyevinn/mp4ff/bits"
)

// Dac4Box - AC4SpecificBox according to ETSI TS 103 190-2 V1.2.1 (2018-02) Annex E
// Contains ac4_dsi_v1 structure as defined in E.6.1
type Dac4Box struct {
	AC4DSIVersion    uint8             // 3 bits - version of the DSI
	BitstreamVersion uint8             // 7 bits - version of the bitstream
	FSIndex          uint8             // 1 bit - sampling frequency index
	FrameRateIndex   uint8             // 4 bits - frame rate index
	NPresentations   uint16            // 9 bits - number of presentations
	BProgramID       uint8             // 1 bit - program ID flag
	ShortProgramID   uint16            // 16 bits - short program ID (if BProgramID is true)
	BUUID            uint8             // 1 bit - UUID flag (if BProgramID is true)
	ProgramUUID      []byte            // 128 bits - program UUID (if BUUID is true)
	BitRateMode      uint8             // 2 bits - bit rate control algorithm
	BitRate          uint32            // 32 bits - bit rate in bits/second
	BitRatePrecision uint32            // 32 bits - precision of bit rate
	Presentations    []AC4Presentation // Presentation information
	RawData          []byte            // Raw DSI data for complex parsing
}

// AC4Presentation represents a presentation in the DSI
type AC4Presentation struct {
	PresentationVersion uint8  // 8 bits - presentation version
	PresBytes           uint8  // 8 bits - presentation data length
	AddPresBytes        uint16 // 16 bits - additional length (if PresBytes == 255)
	PresentationData    []byte // Raw presentation data
}

// DecodeDac4 - box-specific decode
func DecodeDac4(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := readBoxBody(r, hdr)
	if err != nil {
		return nil, err
	}
	return decodeDac4FromData(data)
}

// DecodeDac4SR - box-specific decode
func DecodeDac4SR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	data := sr.ReadBytes(hdr.payloadLen())
	if sr.AccError() != nil {
		return nil, sr.AccError()
	}
	return decodeDac4FromData(data)
}

func decodeDac4FromData(data []byte) (Box, error) {
	// According to ETSI TS 103 190-2 V1.2.1 Annex E, ac4_dsi_v1 requires minimum fields:
	// - ac4_dsi_version (3 bits) + bitstream_version (7 bits) + fs_index (1 bit) +
	//   frame_rate_index (4 bits) + n_presentations (9 bits) = 24 bits = 3 bytes
	// - Plus mandatory ac4_bitrate_dsi: bit_rate_mode (2 bits) + bit_rate (32 bits) +
	//   bit_rate_precision (32 bits) = 66 bits = 8.25 bytes
	// - Plus byte_align padding = minimum 11 bytes total
	if len(data) < 11 {
		return nil, fmt.Errorf("dac4 box: data too short (%d bytes), minimum 11 bytes required", len(data))
	}

	b := &Dac4Box{
		RawData: make([]byte, len(data)),
	}
	copy(b.RawData, data)

	buf := bytes.NewReader(data)
	br := bits.NewReader(buf)

	// Parse ac4_dsi_v1 according to E.6.1
	// Need at least 1 byte for version
	if len(data) < 1 {
		return b, nil
	}

	b.AC4DSIVersion = uint8(br.Read(3))
	if b.AC4DSIVersion > 1 {
		// According to spec, decoders should skip if version > 1
		return b, nil
	}

	// Need more data for full parsing
	if len(data) < 8 {
		// Not enough data for full DSI, return what we have
		return b, nil
	}

	b.BitstreamVersion = uint8(br.Read(7))
	b.FSIndex = uint8(br.Read(1))
	b.FrameRateIndex = uint8(br.Read(4))
	b.NPresentations = uint16(br.Read(9))

	// Check for program ID info (bitstream_version > 1)
	if b.BitstreamVersion > 1 {
		b.BProgramID = uint8(br.Read(1))
		if b.BProgramID == 1 {
			b.ShortProgramID = uint16(br.Read(16))
			b.BUUID = uint8(br.Read(1))
			if b.BUUID == 1 {
				b.ProgramUUID = make([]byte, 16)
				for i := 0; i < 16; i++ {
					b.ProgramUUID[i] = uint8(br.Read(8))
				}
			}
		}
	}

	// Parse ac4_bitrate_dsi
	b.BitRateMode = uint8(br.Read(2))
	b.BitRate = uint32(br.Read(32))
	b.BitRatePrecision = uint32(br.Read(32))

	// Byte align according to AC4 specification
	br.ByteAlign()

	// Parse presentations
	b.Presentations = make([]AC4Presentation, b.NPresentations)
	for i := 0; i < int(b.NPresentations); i++ {
		pr := &b.Presentations[i]
		pr.PresentationVersion = uint8(br.Read(8))
		pr.PresBytes = uint8(br.Read(8))

		presDataLen := int(pr.PresBytes)
		if pr.PresBytes == 255 {
			pr.AddPresBytes = uint16(br.Read(16))
			presDataLen += int(pr.AddPresBytes)
		}

		// Store raw presentation data for now
		pr.PresentationData = make([]byte, presDataLen)
		for j := 0; j < presDataLen; j++ {
			pr.PresentationData[j] = uint8(br.Read(8))
		}
	}

	return b, nil
}

// Type - box type
func (b *Dac4Box) Type() string {
	return "dac4"
}

// Size - calculated size of box
func (b *Dac4Box) Size() uint64 {
	return uint64(boxHeaderSize + len(b.RawData))
}

// Encode - write box to w
func (b *Dac4Box) Encode(w io.Writer) error {
	sw := bits.NewFixedSliceWriter(int(b.Size()))
	err := b.EncodeSW(sw)
	if err != nil {
		return err
	}
	_, err = w.Write(sw.Bytes())
	return err
}

// EncodeSW - write box to sw
func (b *Dac4Box) EncodeSW(sw bits.SliceWriter) error {
	err := EncodeHeaderSW(b, sw)
	if err != nil {
		return err
	}

	// Write raw data for now - this preserves exact binary structure
	sw.WriteBytes(b.RawData)
	return sw.AccError()
}

// Info - write box info to w
func (b *Dac4Box) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, -1, 0)
	bd.write(" - ac4DSIVersion=%d", b.AC4DSIVersion)
	bd.write(" - bitstreamVersion=%d (%s)", b.BitstreamVersion, b.GetBitstreamVersionString())
	bd.write(" - fsIndex=%d (%d Hz)", b.FSIndex, b.GetSamplingFrequency())
	bd.write(" - frameRateIndex=%d (%s)", b.FrameRateIndex, b.GetFrameRateString())
	bd.write(" - nPresentations=%d", b.NPresentations)

	if b.BitstreamVersion > 1 && b.BProgramID == 1 {
		bd.write(" - shortProgramID=%d", b.ShortProgramID)
		if b.BUUID == 1 {
			bd.write(" - programUUID=%x", b.ProgramUUID)
		}
	}

	bd.write(" - bitRateMode=%d (%s)", b.BitRateMode, b.GetBitRateModeString())
	bd.write(" - bitRate=%d bps", b.BitRate)
	if b.BitRatePrecision == 0xFFFFFFFF {
		bd.write(" - bitRatePrecision=4294967295 (unknown)")
	} else {
		bd.write(" - bitRatePrecision=%d", b.BitRatePrecision)
	}

	for i, pr := range b.Presentations {
		bd.write(" - presentation[%d]: version=%d, dataLen=%d", i, pr.PresentationVersion, len(pr.PresentationData))
	}

	return bd.err
}

// GetSamplingFrequency returns the sampling frequency based on fsIndex
// According to ETSI TS 103 190-1 Table 77
func (b *Dac4Box) GetSamplingFrequency() int {
	if b.FSIndex == 0 {
		return 44100 // 44.1 kHz
	}
	return 48000 // 48 kHz
}

// GetFrameRate returns frame rate based on frameRateIndex
// According to ETSI TS 103 190-2 Table E.1
func (b *Dac4Box) GetFrameRate() float64 {
	frameRates := []float64{
		23.976, 24, 25, 29.97, 30, 47.95, 48, 50, 59.94, 60, 100, 119.88, 120, 23.44,
	}

	if int(b.FrameRateIndex) < len(frameRates) {
		return frameRates[b.FrameRateIndex]
	}
	return 0 // Reserved or invalid
}

// GetBitstreamVersionString returns human-readable bitstream version
// According to ETSI TS 103 190-2 V1.2.1 Annex E
func (b *Dac4Box) GetBitstreamVersionString() string {
	switch b.BitstreamVersion {
	case 0:
		return "Reserved"
	case 1:
		return "ETSI TS 103 190 V1.1.1"
	case 2:
		return "ETSI TS 103 190 V1.2.1"
	case 3:
		return "ETSI TS 103 190 V1.3.1"
	case 4:
		return "ETSI TS 103 190 V1.4.1"
	default:
		if b.BitstreamVersion <= 31 {
			return fmt.Sprintf("ETSI TS 103 190 V1.%d.1", b.BitstreamVersion)
		}
		return "Reserved"
	}
}

// GetFrameRateString returns human-readable frame rate
// According to ETSI TS 103 190-2 Table E.1
func (b *Dac4Box) GetFrameRateString() string {
	frameRate := b.GetFrameRate()
	if frameRate == 0 {
		return "Reserved"
	}
	// Use %g format to automatically handle precision and remove trailing zeros
	return fmt.Sprintf("%g fps", frameRate)
}

// GetBitRateModeString returns human-readable bit rate mode
func (b *Dac4Box) GetBitRateModeString() string {
	switch b.BitRateMode {
	case 0:
		return "Not specified"
	case 1:
		return "Constant bit rate"
	case 2:
		return "Average bit rate"
	case 3:
		return "Variable bit rate"
	default:
		return "Unknown"
	}
}
