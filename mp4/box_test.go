package mp4_test

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/Eyevinn/mp4ff/mp4"
)

// TestBadBoxAndRemoveBoxDecoder checks that we can avoid decoder error by removing a BoxDecode.
//
// The box is then interpreted as an UnknownBox and its data is not further processed with decoded.
func TestBadBoxAndRemoveBoxDecoder(t *testing.T) {
	badMetaBox := (`000000416d6574610000002168646c7300000000000000006d64746100000000` +
		`000000000000000000000000106b657973000000000000000000000008696c7374`)
	data, err := hex.DecodeString(badMetaBox)
	if err != nil {
		t.Error(err)
	}
	sr := bits.NewFixedSliceReader(data)
	_, err = mp4.DecodeBoxSR(0, sr)
	if err == nil {
		t.Errorf("reading bad meta box should have failed")
	}
	sr = bits.NewFixedSliceReader(data)
	mp4.RemoveBoxDecoder("meta")
	defer mp4.SetBoxDecoder("meta", mp4.DecodeMeta, mp4.DecodeMetaSR)
	box, err := mp4.DecodeBoxSR(0, sr)
	if err != nil {
		t.Error(err)
	}
	_, ok := box.(*mp4.MetaBox)
	if ok {
		t.Errorf("box should not be MetaBox")
	}
	unknown, ok := box.(*mp4.UnknownBox)
	if !ok {
		t.Errorf("box should be unknown")
	}
	if unknown.Type() != "meta" {
		t.Errorf("unknown type %q instead of meta", unknown.Type())
	}
	b := bytes.Buffer{}
	err = unknown.Encode(&b)
	if err != nil {
		t.Error(err)
	}
	if !bytes.Equal(data, b.Bytes()) {
		t.Errorf("written unknown differs")
	}
}

// TestDecodeHeader tests DecodeHeader with sufficient and insufficient bytes
func TestDecodeHeader(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		wantErr bool
	}{
		{
			name:    "7 bytes (one less than boxHeaderSize)",
			data:    make([]byte, 7),
			wantErr: true,
		},
		{
			name:    "8 bytes (exactly boxHeaderSize)",
			data:    []byte{0x00, 0x00, 0x00, 0x10, 't', 'e', 's', 't'}, // size=16, type="test"
			wantErr: false,
		},
		{
			name:    "extended size with insufficient bytes",
			data:    []byte{0x00, 0x00, 0x00, 0x01, 't', 'e', 's', 't', 0x00, 0x00, 0x00},
			wantErr: true,
		},
		{
			name:    "extended size with sufficient bytes",
			data:    []byte{0x00, 0x00, 0x00, 0x01, 't', 'e', 's', 't', 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x20},
			wantErr: false,
		},
		{
			name:    "zero size not supported",
			data:    []byte{0x00, 0x00, 0x00, 0x00, 't', 'e', 's', 't'},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := bytes.NewReader(tt.data)
			_, err := mp4.DecodeHeader(r)
			if (err != nil) != tt.wantErr {
				t.Errorf("DecodeHeader() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestFixed16and32(t *testing.T) {
	f16 := mp4.Fixed16(256)
	if f16.String() != "1.0" {
		t.Errorf("Fixed16(256) should be 1.0, not %s", f16.String())
	}
	f32 := mp4.Fixed32(65536)
	if f16.String() != "1.0" {
		t.Errorf("Fixed32(65536) should be 1.0, not %s", f32.String())
	}
}
