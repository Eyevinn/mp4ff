package sei

import (
	"bytes"
	"fmt"

	"github.com/edgeware/mp4ff/bits"
)

// TimeCodeSEI carries the data of an SEI 136 TimeCode message.
type TimeCodeSEI struct {
	Clocks []ClockTS
}

// ClockTS carries a clock time stamp.
type ClockTS struct {
	timeOffsetValue     uint32
	nFrames             uint16
	hours               int8
	minutes             int8
	seconds             int8
	clockTimeStampFlag  bool
	unitsFieldBasedFlag bool
	fullTimeStampFlag   bool
	discontinuityFlag   bool
	cntDroppedFlag      bool
	countingType        byte
	timeOffsetLength    byte
}

// String returns time stamp with -1 for parts not set.
func (c ClockTS) String() string {
	return fmt.Sprintf("%02d:%02d:%02d;%02d", c.hours, c.minutes, c.seconds, c.nFrames)
}

// CreateClockTS creates a clock timestamp with time parts set to -1.
func CreateClockTS() ClockTS {
	return ClockTS{hours: -1, minutes: -1, seconds: -1}
}

func DecodeClockTS(br *bits.AccErrReader) ClockTS {
	c := CreateClockTS()
	c.clockTimeStampFlag = br.ReadFlag()
	if c.clockTimeStampFlag {
		c.unitsFieldBasedFlag = br.ReadFlag()
		c.countingType = byte(br.Read(5))
		c.fullTimeStampFlag = br.ReadFlag()
		c.discontinuityFlag = br.ReadFlag()
		c.cntDroppedFlag = br.ReadFlag()
		c.nFrames = uint16(br.Read(9))
		if c.fullTimeStampFlag {
			c.seconds = int8(br.Read(6))
			c.minutes = int8(br.Read(6))
			c.hours = int8(br.Read(5))
		} else {
			if br.ReadFlag() {
				c.seconds = int8(br.Read(6))
				if br.ReadFlag() {
					c.minutes = int8(br.Read(6))
					if br.ReadFlag() {
						c.hours = int8(br.Read(5))
					}
				}
			}
		}
		c.timeOffsetLength = byte(br.Read(5))
		if c.timeOffsetLength > 0 {
			c.timeOffsetValue = uint32(br.Read(int(c.timeOffsetLength)))
		}
	}
	return c
}

// DecodeTimeCodeSEI decodes SEI message 136 TimeCode.
func DecodeTimeCodeSEI(sd *SEIData) (SEIMessage, error) {
	buf := bytes.NewBuffer(sd.Payload())
	br := bits.NewAccErrReader(buf)
	numClockTS := int(br.Read(2))
	tc := TimeCodeSEI{make([]ClockTS, 0, numClockTS)}
	for i := 0; i < numClockTS; i++ {
		c := DecodeClockTS(br)
		tc.Clocks = append(tc.Clocks, c)
	}
	return &tc, br.AccError()
}

// Type returns the SEI payload type.
func (s *TimeCodeSEI) Type() uint {
	return SEITimeCodeType
}

// Payload returns the SEI raw rbsp payload.
func (s *TimeCodeSEI) Payload() []byte {
	sw := bits.NewFixedSliceWriter(int(s.Size()))
	sw.WriteBits(uint(len(s.Clocks)), 2)
	for _, c := range s.Clocks {
		sw.WriteFlag(c.clockTimeStampFlag)
		if c.clockTimeStampFlag {
			sw.WriteFlag(c.unitsFieldBasedFlag)
			sw.WriteBits(uint(c.countingType), 5)
			sw.WriteFlag(c.fullTimeStampFlag)
			sw.WriteFlag(c.discontinuityFlag)
			sw.WriteFlag(c.cntDroppedFlag)
			sw.WriteBits(uint(c.nFrames), 9)
			if c.fullTimeStampFlag {
				sw.WriteBits(uint(c.seconds), 6)
				sw.WriteBits(uint(c.minutes), 6)
				sw.WriteBits(uint(c.hours), 5)
			} else {
				sw.WriteFlag(c.seconds >= 0)
				if c.seconds >= 0 {
					sw.WriteBits(uint(c.seconds), 6)
					sw.WriteFlag(c.minutes >= 0)
					if c.minutes >= 0 {
						sw.WriteBits(uint(c.minutes), 6)
						sw.WriteFlag(c.hours >= 0)
						if c.hours >= 0 {
							sw.WriteBits(uint(c.hours), 5)
						}
					}
				}
			}
			sw.WriteBits(uint(c.timeOffsetLength), 5)
			if c.timeOffsetLength > 0 {
				sw.WriteBits(uint(c.timeOffsetLength), int(c.timeOffsetLength))
			}
		}
	}
	sw.WriteFlag(true) // Final 1 and then byte align
	sw.FlushBits()
	return sw.Bytes()
}

// String returns string representation of TimeCodeSEI.
func (s *TimeCodeSEI) String() string {
	msgType := SEIType(s.Type())
	msg := fmt.Sprintf("%s, size=%d, time=%s", msgType, s.Size(), s.Clocks[0].String())
	if len(s.Clocks) > 1 {
		for i := 1; i < len(s.Clocks); i++ {
			msg += fmt.Sprintf(", time=%s", s.Clocks[i].String())
		}
	}
	return msg
}

// Size is size in bytes of raw SEI message rbsp payload.
func (s *TimeCodeSEI) Size() uint {
	nrBits := 2
	for _, c := range s.Clocks {
		nrBits++
		if c.clockTimeStampFlag {
			nrBits += 18
			if c.fullTimeStampFlag {
				nrBits += 17
			} else {
				nrBits++
				if c.seconds >= 0 {
					nrBits += 6 + 1
					if c.minutes >= 0 {
						nrBits += 6 + 1
						if c.hours >= 0 {
							nrBits += 5
						}
					}
				}
			}
			nrBits += 5
			nrBits += int(c.timeOffsetLength)
		}
	}
	return uint((nrBits + 7) / 8)
}
