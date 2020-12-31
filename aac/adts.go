package aac

import (
	"bytes"
	"fmt"
	"io"

	"github.com/edgeware/mp4ff/bits"
)

// ADTSHeader - data for an unencrypted ADTS Header with one AAC frame.
// Not used in mp4 files, but in MPEG-2 TS.
// Defined in ISO/IEC 13818-7
type ADTSHeader struct {
	ObjectType             byte
	SamplingFrequencyIndex byte
	ChannelConfig          byte
	PayloadLength          uint16
	BufferFullness         uint16
}

func NewADTSHeader(samplingFrequency int, channelConfig byte, objectType byte, plLen uint16) (*ADTSHeader, error) {
	if objectType != AAClc {
		return nil, fmt.Errorf("Must use AAC-LC (type 2) not %d", objectType)
	}
	sfi, ok := reverseFrequencies[samplingFrequency]
	if !ok {
		return nil, fmt.Errorf("Sampling frequency %d not supported", samplingFrequency)
	}
	return &ADTSHeader{
		ObjectType:             objectType,
		SamplingFrequencyIndex: sfi,
		ChannelConfig:          channelConfig,
		PayloadLength:          plLen,
		BufferFullness:         0x7ff, // variable bitrate
	}, nil

}

func (a ADTSHeader) Encode() []byte {
	buf := bytes.Buffer{}
	bw := bits.NewWriter(&buf)
	bw.Write(0xfff, 12)                         // sync word
	bw.Write(0x01, 4)                           //ID=0 for MPEG-4 + layer + protection absent
	bw.Write(uint(a.ObjectType)-1, 2)           // profile
	bw.Write(uint(a.SamplingFrequencyIndex), 4) // sampling frequency index (3 = 48KHz)
	bw.Write(0, 1)                              // private
	bw.Write(uint(a.ChannelConfig), 3)          // Channel configuration
	bw.Write(0, 4)                              // Copyright etc
	bw.Write(uint(a.PayloadLength+7), 13)       // The length should include this 7-byte header
	bw.Write(uint(a.BufferFullness), 11)        // Buffer fullness value
	bw.Write(0, 2)                              // Nr AAC frames in ADTS frame minus 1
	return buf.Bytes()
}

func DecodedAdtsHeader(r io.Reader) (*ADTSHeader, error) {
	br := bits.NewAccErrReader(r)
	sync := br.Read(12)
	if sync != 0xfff {
		return nil, fmt.Errorf("Bad sync")
	}
	mpegID := br.Read(1)
	if mpegID != 0 {
		return nil, fmt.Errorf("ID not 0 (MPEG-4)")
	}
	layer := br.Read(2)
	if layer != 0 {
		return nil, fmt.Errorf("Non-permitted layer value %d", layer)
	}
	protectionAbsent := br.Read(1)
	if protectionAbsent != 1 {
		return nil, fmt.Errorf("protection_absent not set. Not supported")
	}
	ah := &ADTSHeader{}
	profile := br.Read(2)
	ah.ObjectType = byte(profile + 1)
	ah.SamplingFrequencyIndex = byte(br.Read(4))
	_ = br.Read(1) // ignore private
	ah.ChannelConfig = byte(br.Read(3))
	_ = br.Read(4) // ignore original/copy, home, copyright
	frame_length := br.Read(13)
	ah.PayloadLength = uint16(frame_length - 7)
	ah.BufferFullness = uint16(br.Read(11))
	nrRawBlocksMinus1 := br.Read(2)
	if nrRawBlocksMinus1 != 0 {
		return nil, fmt.Errorf("only 1 raw block supported")
	}
	if br.AccError() != nil {
		return nil, br.AccError()
	}

	return ah, nil
}
