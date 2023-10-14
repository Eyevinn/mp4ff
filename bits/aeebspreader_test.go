package bits_test

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/Eyevinn/mp4ff/bits"
)

func TestAccErrEBSPReader(t *testing.T) {
	testCases := []struct {
		hexData                         string
		readBits                        int
		expectedValue                   uint
		expectedNrBytesRead             int
		expectedNrBitsRead              int
		expectedNrBitsReadInCurrentByte int
	}{
		{"00", 0, 0, 0, 0, 8},
		{"0001", 1, 0, 1, 1, 1},
		{"0001", 8, 0x00, 1, 8, 8},
		{"0010", 12, 0x001, 2, 12, 4},
		{"0102", 16, 0x0102, 2, 16, 8},
		{"00000304", 24, 0x000004, 4, 32, 8}, // Note the start-code emulation 0x03
	}
	for _, tc := range testCases {
		t.Run("", func(t *testing.T) {
			data, err := hex.DecodeString(tc.hexData)
			if err != nil {
				t.Error(err)
			}
			buf := bytes.NewBuffer(data)
			r := bits.NewAccErrEBSPReader(buf)
			if r.AccError() != nil {
				t.Errorf("expected no error, got %v", r.AccError())
			}
			v := r.Read(tc.readBits)
			if v != tc.expectedValue {
				t.Errorf("expected value %d, got %d", tc.expectedValue, v)
			}
			if r.NrBytesRead() != tc.expectedNrBytesRead {
				t.Errorf("expected %d bytes read, got %d", tc.expectedNrBytesRead, r.NrBytesRead())
			}
			if r.NrBitsRead() != tc.expectedNrBitsRead {
				t.Errorf("expected %d bits read, got %d", tc.expectedNrBitsRead, r.NrBitsRead())
			}
			if r.NrBitsReadInCurrentByte() != tc.expectedNrBitsReadInCurrentByte {
				t.Errorf("expected %d bits read in current byte, got %d", tc.expectedNrBitsReadInCurrentByte, r.NrBitsReadInCurrentByte())
			}
		})
	}
}
