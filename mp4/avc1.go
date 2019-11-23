package mp4

import (
	"bytes"
	"io"
	"io/ioutil"
)

// Avc1Box - AVC Sample Description Type X box (avc1/avc3)
type Avc1Box struct {
	DataReferenceIndex uint16
	Width              uint16
	Height             uint16
	Horizresolution    uint32
	Vertresolution     uint32
	FrameCount         uint16
	CompressorName     string
	visualSampleBytes  []byte
	AvcC               *AvcCBox
}

// DecodeAvc1 - box-specific decode
func DecodeAvc1(size uint64, startPos uint64, r io.Reader) (Box, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	a := &Avc1Box{}

	// 14496-12 8.5.2.2 Sample entry (8 bytes)
	// 14496-12 12.1.3.2 Visual Sample entry (70 bytes)
	// 14496-15  5.4.2.1.2 avcC should be inside avc1, avc3 box

	a.visualSampleBytes = make([]byte, 78, 78)
	copy(a.visualSampleBytes, data[:78])

	bR := bytes.NewReader(data[78:])
	box, err := DecodeBox(startPos+86, bR)
	if err != nil {
		return nil, err
	}

	a.AvcC = box.(*AvcCBox)
	return a, nil
}

// Type - return box type
func (a *Avc1Box) Type() string {
	return "avc1"
}

// Size - return calculated size
func (a *Avc1Box) Size() uint64 {
	return boxHeaderSize + 78 + a.AvcC.Size()
}

// Encode - write box to w
func (a *Avc1Box) Encode(w io.Writer) error {
	err := EncodeHeader(a, w)
	if err != nil {
		return err
	}
	w.Write(a.visualSampleBytes)
	a.AvcC.Encode(w)
	return err
}
