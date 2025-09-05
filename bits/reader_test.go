package bits_test

import (
	"bytes"
	"io"
	"testing"

	"github.com/Eyevinn/mp4ff/bits"
)

func TestAccErrReader(t *testing.T) {
	t.Run("Read bits", func(t *testing.T) {
		input := []byte{0xff, 0x0f} // 1111 1111 0000 1111
		rd := bytes.NewReader(input)
		reader := bits.NewReader(rd)

		cases := []struct {
			readNrBits  int
			want        uint
			nrBytesRead int
			nrBitsRead  int
		}{
			{2, 3, 1, 2},   // 11
			{3, 7, 1, 5},   // 111
			{5, 28, 2, 10}, // 11100
			{3, 1, 2, 13},  // 001
			{3, 7, 2, 16},  // 111
		}

		for _, tc := range cases {
			got := reader.Read(tc.readNrBits)

			if got != tc.want {
				t.Errorf("Read(%d)=%b, want=%b", tc.readNrBits, got, tc.want)
			}
			if reader.NrBytesRead() != tc.nrBytesRead {
				t.Errorf("NrBytesRead()=%d, want=%d", reader.NrBytesRead(), tc.nrBytesRead)
			}
			if reader.NrBitsRead() != tc.nrBitsRead {
				t.Errorf("NrBitsRead()=%d, want=%d", reader.NrBitsRead(), tc.nrBitsRead)
			}
		}
		err := reader.AccError()
		if err != nil {
			t.Errorf("Got accumulated error: %s", err.Error())
		}
	})
	t.Run("Read signed bits", func(t *testing.T) {
		input := []byte{0xff, 0x0f} // 1111 1111 0000 1111
		rd := bytes.NewReader(input)
		reader := bits.NewReader(rd)

		cases := []struct {
			n    int
			want uint
		}{
			{2, 3},  // 11
			{3, 7},  // 111
			{5, 28}, // 11100
			{3, 1},  // 001
			{3, 7},  // 111
		}

		for _, tc := range cases {
			got := reader.Read(tc.n)

			if got != tc.want {
				t.Errorf("Read(%d)=%b, want=%b", tc.n, got, tc.want)
			}
		}
		err := reader.AccError()
		if err != nil {
			t.Errorf("Got accumulated error: %s", err.Error())
		}
	})
	t.Run("Read remaining bytes", func(t *testing.T) {
		input := []byte{0xef, 0x0f} // 1110 1111 0000 1111
		rd := bytes.NewReader(input)
		r := bits.NewReader(rd)
		gotRemaining := r.ReadRemainingBytes()
		if !bytes.Equal(gotRemaining, input) {
			t.Errorf("ReadRemainingBytes()=%b, want=%b", gotRemaining, input)
		}
		// Next check with non-bytealigned status
		rd = bytes.NewReader(input)
		r = bits.NewReader(rd)
		_ = r.ReadFlag()
		_ = r.ReadRemainingBytes()
		err := r.AccError()
		if err == nil {
			t.Errorf("Expected error due to not byte-aligned, but got nil")
		}
	})

	t.Run("Read flags", func(t *testing.T) {
		input := []byte{0xe5} // 1110 0101
		rd := bytes.NewReader(input)
		r := bits.NewReader(rd)
		for i := 0; i < 8; i++ {
			expectedValue := (input[0] >> (7 - i) & 1) > 0
			gotFlag := r.ReadFlag()
			if gotFlag != expectedValue {
				t.Errorf("Read flag %t, but wanted %t, case %d", gotFlag, expectedValue, i)
			}
		}
		_ = r.ReadFlag()
		wantedError := io.EOF
		if err := r.AccError(); err != wantedError {
			t.Errorf("wanted error %s, got %s", wantedError, err.Error())
		}
	})
}

func TestAccErrReaderSigned(t *testing.T) {
	input := []byte{0xff, 0x0c} // 1111 1111 0000 1100
	rd := bytes.NewReader(input)
	reader := bits.NewReader(rd)

	cases := []struct {
		readNrBits int
		want       int
	}{
		{2, -1}, // 11
		{3, -1}, // 111
		{5, -4}, // 11100
		{3, 1},  // 001
		{3, -4}, // 100
	}

	for _, tc := range cases {
		got := reader.ReadSigned(tc.readNrBits)

		if got != tc.want {
			t.Errorf("Read(%d)=%b, want=%b", tc.readNrBits, got, tc.want)
		}
	}
	err := reader.AccError()
	if err != nil {
		t.Errorf("Got accumulated error: %s", err.Error())
	}
}

func TestBadAccErrReader(t *testing.T) {
	// Check that reading beyond EOF provides value = 0 after acc error
	input := []byte{0xff, 0x0f} // 1111 1111 0000 1111
	rd := bytes.NewReader(input)
	reader := bits.NewReader(rd)

	cases := []struct {
		err  error
		n    int
		want uint
	}{
		{nil, 2, 3},     // 11
		{nil, 3, 7},     // 111
		{io.EOF, 12, 0}, // 0 because of error
		{io.EOF, 3, 0},  // 0 because of acc error
		{io.EOF, 3, 0},  // 0 because of acc error
	}

	for _, tc := range cases {
		got := reader.Read(tc.n)

		if got != tc.want {
			t.Errorf("Read(%d)=%b, want=%b", tc.n, got, tc.want)
		}
	}
	err := reader.AccError()
	if err != io.EOF {
		t.Errorf("Wanted io.EOF but got %v", err)
	}
}

func TestByteAlign(t *testing.T) {
	t.Run("align from middle of byte", func(t *testing.T) {
		input := []byte{0xAB, 0xCD, 0xEF} // 10101011 11001101 11101111
		rd := bytes.NewReader(input)
		r := bits.NewReader(rd)

		// Read 3 bits: 101 (5 bits remaining in first byte)
		got := r.Read(3)
		if got != 5 { // 101 binary = 5 decimal
			t.Errorf("Expected 5, got %d", got)
		}

		// Now we should have 5 bits left in current byte
		nrBitsInCurrentByte := r.NrBitsReadInCurrentByte()
		if nrBitsInCurrentByte != 3 {
			t.Errorf("Expected 3 bits read in current byte, got %d", nrBitsInCurrentByte)
		}

		// Align to byte boundary
		r.ByteAlign()

		// Now reading should start from next byte (0xCD)
		got = r.Read(8)
		if got != 0xCD {
			t.Errorf("Expected 0xCD (%d), got %d", 0xCD, got)
		}

		// Verify no error occurred
		if err := r.AccError(); err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
	})

	t.Run("align when already byte-aligned", func(t *testing.T) {
		input := []byte{0xAB, 0xCD}
		rd := bytes.NewReader(input)
		r := bits.NewReader(rd)

		// Read exactly 8 bits (already byte-aligned)
		got := r.Read(8)
		if got != 0xAB {
			t.Errorf("Expected 0xAB (%d), got %d", 0xAB, got)
		}

		// Align (should be no-op)
		r.ByteAlign()

		// Next read should get the next byte
		got = r.Read(8)
		if got != 0xCD {
			t.Errorf("Expected 0xCD (%d), got %d", 0xCD, got)
		}

		// Verify no error occurred
		if err := r.AccError(); err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
	})

	t.Run("align with accumulated error", func(t *testing.T) {
		input := []byte{0xAB}
		rd := bytes.NewReader(input)
		r := bits.NewReader(rd)

		// Read more bits than available to cause error
		_ = r.Read(16) // Only 8 bits available

		// Ensure error is accumulated
		if err := r.AccError(); err == nil {
			t.Error("Expected error but got none")
		}

		// ByteAlign should not crash and should do nothing with error present
		r.ByteAlign()

		// Error should still be there
		if err := r.AccError(); err == nil {
			t.Error("Expected error to persist after ByteAlign")
		}
	})

	t.Run("align after reading various bit counts", func(t *testing.T) {
		testCases := []struct {
			name           string
			bitsToRead     int
			expectedResult uint
		}{
			{"1 bit", 1, 1},   // First bit of 0xAB (1010 1011) = 1
			{"2 bits", 2, 2},  // First 2 bits = 10 = 2
			{"3 bits", 3, 5},  // First 3 bits = 101 = 5
			{"4 bits", 4, 10}, // First 4 bits = 1010 = 10
			{"5 bits", 5, 21}, // First 5 bits = 10101 = 21
			{"6 bits", 6, 42}, // First 6 bits = 101010 = 42
			{"7 bits", 7, 85}, // First 7 bits = 1010101 = 85
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				input := []byte{0xAB, 0xCD} // 10101011 11001101
				rd := bytes.NewReader(input)
				r := bits.NewReader(rd)

				// Read specified number of bits
				got := r.Read(tc.bitsToRead)
				if got != tc.expectedResult {
					t.Errorf("Read(%d) = %d, expected %d", tc.bitsToRead, got, tc.expectedResult)
				}

				// Align to byte boundary
				r.ByteAlign()

				// Next read should get the second byte (0xCD)
				nextByte := r.Read(8)
				if nextByte != 0xCD {
					t.Errorf("After ByteAlign, expected 0xCD (%d), got %d", 0xCD, nextByte)
				}

				// Verify no error occurred
				if err := r.AccError(); err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			})
		}
	})
}
