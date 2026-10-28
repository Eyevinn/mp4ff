package mp4

import (
	"fmt"
	"io"
	"math"
	"strings"

	"github.com/Eyevinn/mp4ff/bits"
)

// ChanBox is the QuickTime audio channel layout atom chan, an optional extension of a sound sample description.
// It is a full atom followed by a big-endian AudioChannelLayout structure of Core Audio.
type ChanBox struct {
	Version byte
	Flags   uint32
	// LayoutTag is ChannelLayoutTagUseDescriptions, ChannelLayoutTagUseBitmap, or a tag whose upper 16 bits
	// name a layout, such as 101 for stereo, and whose lower 16 bits are the number of channels.
	LayoutTag uint32
	// Bitmap has bit i set for a channel with label i+1, for ChannelLayoutTagUseBitmap.
	Bitmap uint32
	// Descriptions describe each channel, for ChannelLayoutTagUseDescriptions.
	Descriptions []ChannelDescription
}

// ChannelDescription is the description of a channel in a ChanBox.
type ChannelDescription struct {
	// Label is the channel label, such as 1 for left, 2 for right and 3 for center.
	Label uint32
	// Flags tell how to read Coordinates: bit 0 for rectangular, bit 1 for spherical, and bit 2 for meters.
	Flags       uint32
	Coordinates [3]float32
}

// Channel layout tags with a special meaning.
const (
	// ChannelLayoutTagUseDescriptions means that the channels are given by ChanBox.Descriptions.
	ChannelLayoutTagUseDescriptions uint32 = 0
	// ChannelLayoutTagUseBitmap means that the channels are given by ChanBox.Bitmap.
	ChannelLayoutTagUseBitmap uint32 = 1 << 16
)

// channelLayoutNames are the names of the channel layouts of Core Audio, by the upper 16 bits of the layout tag.
var channelLayoutNames = map[uint32]string{
	100: "Mono", 101: "Stereo", 102: "StereoHeadphones", 103: "MatrixStereo", 104: "MidSide", 105: "XY",
	106: "Binaural", 107: "Ambisonic_B_Format", 108: "Quadraphonic", 109: "Pentagonal", 110: "Hexagonal",
	111: "Octagonal", 112: "Cube", 113: "MPEG_3_0_A", 114: "MPEG_3_0_B", 115: "MPEG_4_0_A", 116: "MPEG_4_0_B",
	117: "MPEG_5_0_A", 118: "MPEG_5_0_B", 119: "MPEG_5_0_C", 120: "MPEG_5_0_D", 121: "MPEG_5_1_A",
	122: "MPEG_5_1_B", 123: "MPEG_5_1_C", 124: "MPEG_5_1_D", 125: "MPEG_6_1_A", 126: "MPEG_7_1_A",
	127: "MPEG_7_1_B", 128: "MPEG_7_1_C", 129: "Emagic_Default_7_1", 130: "SMPTE_DTV", 131: "ITU_2_1",
	132: "ITU_2_2", 133: "DVD_4", 134: "DVD_5", 135: "DVD_6", 136: "DVD_10", 137: "DVD_11", 138: "DVD_18",
	139: "AudioUnit_6_0", 140: "AudioUnit_7_0", 141: "AAC_6_0", 142: "AAC_6_1", 143: "AAC_7_0",
	144: "AAC_Octagonal", 145: "TMH_10_2_std", 146: "TMH_10_2_full", 147: "DiscreteInOrder",
	148: "AudioUnit_7_0_Front", 149: "AC3_1_0_1", 150: "AC3_3_0", 151: "AC3_3_1", 152: "AC3_3_0_1",
	153: "AC3_2_1_1", 154: "AC3_3_1_1", 155: "EAC_6_0_A", 156: "EAC_7_0_A", 157: "EAC3_6_1_A", 158: "EAC3_6_1_B",
	159: "EAC3_6_1_C", 160: "EAC3_7_1_A", 161: "EAC3_7_1_B", 162: "EAC3_7_1_C", 163: "EAC3_7_1_D",
	164: "EAC3_7_1_E", 165: "EAC3_7_1_F", 166: "EAC3_7_1_G", 167: "EAC3_7_1_H",
}

// channelLabelNames are the names of the channel labels of Core Audio.
var channelLabelNames = map[uint32]string{
	0: "Unused", 1: "Left", 2: "Right", 3: "Center", 4: "LFEScreen", 5: "LeftSurround", 6: "RightSurround",
	7: "LeftCenter", 8: "RightCenter", 9: "CenterSurround", 10: "LeftSurroundDirect", 11: "RightSurroundDirect",
	12: "TopCenterSurround", 13: "VerticalHeightLeft", 14: "VerticalHeightCenter", 15: "VerticalHeightRight",
	16: "TopBackLeft", 17: "TopBackCenter", 18: "TopBackRight", 33: "RearSurroundLeft", 34: "RearSurroundRight",
	35: "LeftWide", 36: "RightWide", 37: "LFE2", 38: "LeftTotal", 39: "RightTotal", 40: "HearingImpaired",
	41: "Narration", 42: "Mono", 43: "DialogCentricMix", 44: "CenterSurroundDirect", 45: "Haptic",
	100: "UseCoordinates", 0xffffffff: "Unknown",
}

// ChannelLabelName returns the Core Audio name of a channel label, such as "Left", or the label as a number.
func ChannelLabelName(label uint32) string {
	if name, ok := channelLabelNames[label]; ok {
		return name
	}
	if label>>16 == 1 {
		return fmt.Sprintf("Discrete_%d", label&0xffff)
	}
	return fmt.Sprintf("%d", label)
}

// DecodeChan - box-specific decode
func DecodeChan(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := readBoxBody(r, hdr)
	if err != nil {
		return nil, err
	}
	return decodeOrUnknown(hdr, startPos, data, decodeChanSR)
}

// DecodeChanSR - box-specific decode
func DecodeChanSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	return decodeOrUnknownSR(hdr, startPos, sr, decodeChanSR)
}

func decodeChanSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	if hdr.payloadLen() < 16 {
		return nil, fmt.Errorf("chan: payload of %d bytes is too short", hdr.payloadLen())
	}
	versionAndFlags := sr.ReadUint32()
	b := ChanBox{Version: byte(versionAndFlags >> 24), Flags: versionAndFlags & flagsMask}
	b.LayoutTag = sr.ReadUint32()
	b.Bitmap = sr.ReadUint32()
	n := int(sr.ReadUint32())
	if n != (hdr.payloadLen()-16)/20 || (hdr.payloadLen()-16)%20 != 0 {
		return nil, fmt.Errorf("chan: %d channel descriptions do not fill the payload of %d bytes", n, hdr.payloadLen())
	}
	if n > 0 {
		b.Descriptions = make([]ChannelDescription, n)
	}
	for i := range b.Descriptions {
		d := &b.Descriptions[i]
		d.Label = sr.ReadUint32()
		d.Flags = sr.ReadUint32()
		for j := range d.Coordinates {
			d.Coordinates[j] = math.Float32frombits(sr.ReadUint32())
		}
	}
	return &b, sr.AccError()
}

// Type - box type
func (b *ChanBox) Type() string {
	return "chan"
}

// Size - calculated size of box
func (b *ChanBox) Size() uint64 {
	return uint64(boxHeaderSize + 16 + 20*len(b.Descriptions))
}

// NrChannels returns the number of channels of the layout.
func (b *ChanBox) NrChannels() int {
	switch b.LayoutTag {
	case ChannelLayoutTagUseDescriptions:
		return len(b.Descriptions)
	case ChannelLayoutTagUseBitmap:
		n := 0
		for m := b.Bitmap; m != 0; m &= m - 1 {
			n++
		}
		return n
	}
	return int(b.LayoutTag & 0xffff)
}

// LayoutName returns the name of the layout, such as "Stereo", "UseChannelBitmap" or "UseChannelDescriptions",
// or the upper 16 bits of the tag as a number if the name is not known.
func (b *ChanBox) LayoutName() string {
	switch b.LayoutTag {
	case ChannelLayoutTagUseDescriptions:
		return "UseChannelDescriptions"
	case ChannelLayoutTagUseBitmap:
		return "UseChannelBitmap"
	}
	if name, ok := channelLayoutNames[b.LayoutTag>>16]; ok {
		return name
	}
	return fmt.Sprintf("%d", b.LayoutTag>>16)
}

// Encode - write box to w
func (b *ChanBox) Encode(w io.Writer) error {
	sw := bits.NewFixedSliceWriter(int(b.Size()))
	err := b.EncodeSW(sw)
	if err != nil {
		return err
	}
	_, err = w.Write(sw.Bytes())
	return err
}

// EncodeSW - box-specific encode to slicewriter
func (b *ChanBox) EncodeSW(sw bits.SliceWriter) error {
	err := EncodeHeaderSW(b, sw)
	if err != nil {
		return err
	}
	sw.WriteUint32(uint32(b.Version)<<24 | b.Flags)
	sw.WriteUint32(b.LayoutTag)
	sw.WriteUint32(b.Bitmap)
	sw.WriteUint32(uint32(len(b.Descriptions)))
	for _, d := range b.Descriptions {
		sw.WriteUint32(d.Label)
		sw.WriteUint32(d.Flags)
		for _, c := range d.Coordinates {
			sw.WriteUint32(math.Float32bits(c))
		}
	}
	return sw.AccError()
}

// Info - write box-specific information
func (b *ChanBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, int(b.Version), b.Flags)
	bd.write(" - layoutTag: %#08x (%s)", b.LayoutTag, b.LayoutName())
	bd.write(" - channels: %d", b.NrChannels())
	if b.LayoutTag == ChannelLayoutTagUseBitmap || b.Bitmap != 0 {
		var names []string
		for i := 0; i < 32; i++ {
			if b.Bitmap&(1<<i) != 0 {
				names = append(names, ChannelLabelName(uint32(i+1)))
			}
		}
		bd.write(" - bitmap: %#08x %s", b.Bitmap, strings.Join(names, " "))
	}
	for i, d := range b.Descriptions {
		bd.write(" - channel[%d]: label=%s flags=%d coordinates=%v", i+1, ChannelLabelName(d.Label), d.Flags,
			d.Coordinates)
	}
	return bd.err
}
