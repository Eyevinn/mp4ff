package mp4

import "testing"

func TestUdta(t *testing.T) {
	udta := &UdtaBox{}
	unknown := &UnknownBox{
		name:       "\xa9enc",
		size:       12,
		notDecoded: []byte{0, 0, 0, 0},
	}

	udta.AddChild(unknown) // Any arbitrary box
	boxDiffAfterEncodeAndDecode(t, udta)
}
