package mp4

// ffmpeg boxes according to https://kdenlive.org/en/project/adding-meta-data-to-mp4-video
import (
	"io"
	"io/ioutil"
)

// CTooBox - ©too box defines the ffmpeg encoding tool information
type CTooBox struct {
	Children []Box
}

// DecodeCToo - box-specific decode
func DecodeCToo(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	children, err := DecodeContainerChildren(hdr, startPos+8, startPos+hdr.size, r)
	if err != nil {
		return nil, err
	}
	b := &CTooBox{}
	for _, c := range children {
		b.AddChild(c)
	}
	return b, nil
}

// AddChild - Add a child box and update SampleCount
func (b *CTooBox) AddChild(child Box) {
	b.Children = append(b.Children, child)
}

// Type - box type
func (b *CTooBox) Type() string {
	return "\xa9too"
}

// Size - calculated size of box
func (b *CTooBox) Size() uint64 {
	return containerSize(b.Children)
}

// GetChildren - list of child boxes
func (b *CTooBox) GetChildren() []Box {
	return b.Children
}

// Encode - box-specific encode of stsd - not a usual container
func (b *CTooBox) Encode(w io.Writer) error {
	err := EncodeHeader(b, w)
	if err != nil {
		return err
	}
	for _, c := range b.Children {
		err = c.Encode(w)
		if err != nil {
			return err
		}
	}
	return nil
}

// Info - box-specific Info
func (b *CTooBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	return ContainerInfo(b, w, specificBoxLevels, indent, indentStep)
}

// DataBox - data box used by ffmpeg for providing information.
type DataBox struct {
	Data []byte
}

// DecodeData - decode Data (from mov_write_string_data_tag in movenc.c in ffmpegß)
func DecodeData(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	b := DataBox{data[8:]}
	return &b, nil
}

// Type - box type
func (b *DataBox) Type() string {
	return "data"
}

// Size - calculated size of box
func (b *DataBox) Size() uint64 {
	return uint64(boxHeaderSize + 8 + len(b.Data))
}

// Encode - write box to w
func (b *DataBox) Encode(w io.Writer) error {
	err := EncodeHeader(b, w)
	if err != nil {
		return err
	}
	buf := makebuf(b)
	sw := NewSliceWriter(buf)
	sw.WriteUint32(0x00000001)
	sw.WriteUint32(0x00000000)
	sw.WriteBytes(b.Data)
	_, err = w.Write(buf)
	return err
}

// Info - box-specific Info
func (b *DataBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, -1, 0)
	bd.write(" - data: %s", string(b.Data))
	return bd.err
}
