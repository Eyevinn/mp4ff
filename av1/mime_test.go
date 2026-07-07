package av1

import "testing"

func TestCodecConfRecCodecString(t *testing.T) {
	cases := []struct {
		name string
		rec  CodecConfRec
		want string
	}{
		{
			name: "profile 0, level 9, 8-bit",
			rec:  CodecConfRec{SeqProfile: 0, SeqLevelIdx0: 9, SeqTier0: 0, HighBitdepth: 0},
			want: "av01.0.09M.08",
		},
		{
			name: "profile 0, level 1, 10-bit",
			rec:  CodecConfRec{SeqProfile: 0, SeqLevelIdx0: 1, SeqTier0: 0, HighBitdepth: 1},
			want: "av01.0.01M.10",
		},
		{
			name: "high tier",
			rec:  CodecConfRec{SeqProfile: 0, SeqLevelIdx0: 13, SeqTier0: 1, HighBitdepth: 0},
			want: "av01.0.13H.08",
		},
		{
			name: "profile 2, 12-bit",
			rec:  CodecConfRec{SeqProfile: 2, SeqLevelIdx0: 8, SeqTier0: 0, HighBitdepth: 1, TwelveBit: 1},
			want: "av01.2.08M.12",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.rec.CodecString("av01"); got != c.want {
				t.Errorf("got %s, want %s", got, c.want)
			}
		})
	}
}

func TestSequenceHeaderCodecString(t *testing.T) {
	cases := []struct {
		name string
		sh   SequenceHeader
		want string
	}{
		{
			name: "default color collapses to short form",
			sh: SequenceHeader{SeqProfile: 0, SeqLevelIdx0: 4, BitDepth: 10,
				SubsamplingX: 1, SubsamplingY: 1, ChromaSamplePosition: 0,
				ColorPrimaries: 1, TransferCharacteristics: 1, MatrixCoefficients: 1, ColorRange: false},
			want: "av01.0.04M.10",
		},
		{
			name: "unspecified color emits full form",
			sh: SequenceHeader{SeqProfile: 0, SeqLevelIdx0: 0, BitDepth: 8,
				SubsamplingX: 1, SubsamplingY: 1, ChromaSamplePosition: 0,
				ColorPrimaries: 2, TransferCharacteristics: 2, MatrixCoefficients: 2, ColorRange: false},
			want: "av01.0.00M.08.0.110.02.02.02.0",
		},
		{
			name: "monochrome full range",
			sh: SequenceHeader{SeqProfile: 0, SeqLevelIdx0: 8, BitDepth: 8, MonoChrome: true,
				SubsamplingX: 1, SubsamplingY: 1, ChromaSamplePosition: 0,
				ColorPrimaries: 2, TransferCharacteristics: 2, MatrixCoefficients: 2, ColorRange: true},
			want: "av01.0.08M.08.1.110.02.02.02.1",
		},
		{
			name: "profile 1 4:4:4 BT.709 high tier",
			sh: SequenceHeader{SeqProfile: 1, SeqLevelIdx0: 8, SeqTier0: 1, BitDepth: 10,
				SubsamplingX: 0, SubsamplingY: 0, ChromaSamplePosition: 0,
				ColorPrimaries: 1, TransferCharacteristics: 1, MatrixCoefficients: 1, ColorRange: false},
			want: "av01.1.08H.10.0.000.01.01.01.0",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.sh.CodecString("av01"); got != c.want {
				t.Errorf("got %s, want %s", got, c.want)
			}
		})
	}
}
