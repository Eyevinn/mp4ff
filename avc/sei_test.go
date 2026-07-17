package avc_test

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/Eyevinn/mp4ff/avc"
	"github.com/Eyevinn/mp4ff/sei"
)

func TestCreateSEINaluRoundTrip(t *testing.T) {
	testCases := []struct {
		desc string
		msgs []sei.SEIMessage
	}{
		{
			desc: "single registered ITU-T T.35 message",
			msgs: []sei.SEIMessage{
				sei.NewSEIData(sei.SEIUserDataRegisteredITUtT35Type, []byte{0xb5, 0x00, 0x31, 0x11, 0x22, 0x33, 0x44, 0x55}),
			},
		},
		{
			desc: "single unregistered message",
			msgs: []sei.SEIMessage{
				sei.NewSEIData(sei.SEIUserDataUnregisteredType, []byte("0123456789abcdef")),
			},
		},
		{
			desc: "multiple messages in one NAL unit",
			msgs: []sei.SEIMessage{
				sei.NewSEIData(sei.SEIUserDataRegisteredITUtT35Type, []byte{0xb5, 0x00, 0x31, 0x11, 0x22, 0x33, 0x44, 0x55}),
				sei.NewSEIData(sei.SEIUserDataUnregisteredType, []byte("0123456789abcdef")),
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			nalu, err := avc.CreateSEINalu(tc.msgs)
			if err != nil {
				t.Fatalf("CreateSEINalu failed: %v", err)
			}
			if avc.GetNaluType(nalu[0]) != avc.NALU_SEI {
				t.Errorf("expected NAL unit type %d, got %d", avc.NALU_SEI, avc.GetNaluType(nalu[0]))
			}
			msgs, err := avc.ParseSEINalu(nalu, nil)
			if err != nil {
				t.Fatalf("ParseSEINalu failed: %v", err)
			}
			if len(msgs) != len(tc.msgs) {
				t.Fatalf("expected %d messages, got %d", len(tc.msgs), len(msgs))
			}
			for i, msg := range msgs {
				if msg.Type() != tc.msgs[i].Type() {
					t.Errorf("message %d: expected type %d, got %d", i, tc.msgs[i].Type(), msg.Type())
				}
				if !bytes.Equal(msg.Payload(), tc.msgs[i].Payload()) {
					t.Errorf("message %d: expected payload %x, got %x", i, tc.msgs[i].Payload(), msg.Payload())
				}
			}
		})
	}
}

func TestSEIParsing(t *testing.T) {
	testCases := []struct {
		desc              string
		spsNALUHex        string
		seiNALUHex        string
		expectedMsgs      []sei.SEIMessage
		expectedTimeStamp string
		expectedErr       error
	}{
		{
			desc:              "Test SEI pic_timing with SPS HRD params",
			spsNALUHex:        "6764002aac2cac0780227e5c04f000003e90001d4c0e6a000337ec001bcef5ef80f8442370",
			seiNALUHex:        "06010e0000030000030000030002120806ff0b80",
			expectedMsgs:      []sei.SEIMessage{&sei.PicTimingAvcSEI{}},
			expectedTimeStamp: "11:56:31:03 offset=0",
			expectedErr:       sei.ErrRbspTrailingBitsMissing,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			spsBytes, err := hex.DecodeString(tc.spsNALUHex)
			if err != nil {
				t.Error(err)
			}
			sps, err := avc.ParseSPSNALUnit(spsBytes, true)
			if err != nil {
				t.Fatalf("ParseSPSNALU failed: %v", err)
			}
			seiBytes, err := hex.DecodeString(tc.seiNALUHex)
			if err != nil {
				t.Error(err)
			}
			msgs, err := avc.ParseSEINalu(seiBytes, sps)
			if err != tc.expectedErr {
				t.Fatalf("expected err %q got : %v", tc.expectedErr, err)
			}
			if len(msgs) != len(tc.expectedMsgs) {
				t.Fatalf("Expected %d messages, got %d", len(tc.expectedMsgs), len(msgs))
			}
			for i, msg := range msgs {
				msgType := msg.Type()
				if msgType != tc.expectedMsgs[i].Type() {
					t.Errorf("Expected message type %d, got %d", tc.expectedMsgs[i].Type(), msg.Type())
				}
				if (msg.Type() == sei.SEIPicTimingType) && tc.expectedTimeStamp != "" {
					picTimeSEI := msg.(*sei.PicTimingAvcSEI)
					gotTimestamp := picTimeSEI.Clocks[0].String()
					if gotTimestamp != tc.expectedTimeStamp {
						t.Errorf("Expected timestamp %s, got %s", tc.expectedTimeStamp, gotTimestamp)
					}
				}
			}
		})
	}
}

func TestParseSEINaluShortInput(t *testing.T) {
	// An empty NAL unit must not panic on the nalu[0] access.
	msgs, err := avc.ParseSEINalu(nil, nil)
	if err != avc.ErrNotSEINalu {
		t.Errorf("expected ErrNotSEINalu, got %v", err)
	}
	if msgs != nil {
		t.Errorf("expected nil messages, got %v", msgs)
	}
}
