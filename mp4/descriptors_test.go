package mp4

import (
	"bytes"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/Eyevinn/mp4ff/bits"
)

const badSizeDescriptor = `031900010004134015000000000000000001f40005021190060102`
const missingSLConfig = `031600010004114015000000000000000001d40005021190`
const partOfEsdsProgIn = `03808080250002000480808017401500000000010d88000003f80580808005128856e500068080800102`

func TestDecodeDescriptor(t *testing.T) {
	cases := []struct {
		desc string
		data string
	}{
		{"badSizeDescriptor", badSizeDescriptor},
		{"missingSLConfig", missingSLConfig},
		{"partOfEsdsProgIn", partOfEsdsProgIn},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			data, err := hex.DecodeString(c.data)
			if err != nil {
				t.Error(err)
			}
			sr := bits.NewFixedSliceReader(data)
			desc, err := DecodeESDescriptor(sr, uint32(len(data)))
			if err != nil {
				t.Error(err)
			}
			if desc.Tag() != ES_DescrTag {
				t.Error("tag is not 3")
			}
			out := make([]byte, len(data))
			sw := bits.NewFixedSliceWriterFromSlice(out)
			err = desc.EncodeSW(sw)
			if err != nil {
				t.Error(err)
			}
			if !bytes.Equal(sw.Bytes(), data) {
				t.Errorf("written es descriptor differs from read\n%s\n%s",
					hex.EncodeToString(sw.Bytes()), hex.EncodeToString(data))
			}
		})
	}
}

func TestDescriptorInfo(t *testing.T) {
	cases := []struct {
		desc       string
		data       string
		wantedInfo string
	}{
		{"badSizeDescriptor", badSizeDescriptor,
			`Descriptor "tag=3 ES" size=2+25
			- EsID: 1
			- DependsOnEsID: 0
			- OCResID: 0
			- FlagsAndPriority: 0
			- URLString:
			 Descriptor "tag=4 DecoderConfig" size=2+19
			  - ObjectType: 64
			  - StreamType: 21
			  - BufferSizeDB: 0
			  - MaxBitrate: 0
			  - AvgBitrate: 128000
			   Descriptor "tag=5 DecoderSpecificInfo" size=2+2
				- DecConfig (2B): 1190
			  - UnknownData (2B): 0601
			- Missing SLConfigDescriptor
			- UnknownData (1B): 02
			`},
		{"missingSLConfig", missingSLConfig,
			`Descriptor "tag=3 ES" size=2+22
		- EsID: 1
		- DependsOnEsID: 0
		- OCResID: 0
		- FlagsAndPriority: 0
		- URLString:
		 Descriptor "tag=4 DecoderConfig" size=2+17
		  - ObjectType: 64
		  - StreamType: 21
		  - BufferSizeDB: 0
		  - MaxBitrate: 0
		  - AvgBitrate: 119808
		   Descriptor "tag=5 DecoderSpecificInfo" size=2+2
			- DecConfig (2B): 1190
		- Missing SLConfigDescriptor
		`},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			data, err := hex.DecodeString(c.data)
			if err != nil {
				t.Error(err)
			}
			sr := bits.NewFixedSliceReader(data)
			desc, err := DecodeESDescriptor(sr, uint32(len(data)))
			if err != nil {
				t.Error(err)
			}
			buf := bytes.Buffer{}
			err = desc.Info(&buf, "esds:1", "", "  ")
			if err != nil {
				t.Error(err)
			}
			gotLines := strings.Split(buf.String(), "\n")
			wantedLines := strings.Split(c.wantedInfo, "\n")
			if len(gotLines) != len(wantedLines) {
				t.Errorf("got %d lines, wanted %d", len(gotLines), len(wantedLines))
			}
			for i, line := range gotLines {
				gotTrimmed := strings.TrimSpace(line)
				wantedTrimmed := strings.TrimSpace(wantedLines[i])
				if gotTrimmed != wantedTrimmed {
					t.Errorf("line %d differs\n%s\n%s", i, gotTrimmed, wantedTrimmed)
				}
			}
		})
	}
}
