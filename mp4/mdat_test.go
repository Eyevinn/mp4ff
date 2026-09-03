package mp4_test

import (
	"bytes"
	"math"
	"testing"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/Eyevinn/mp4ff/mp4"
	"github.com/go-test/deep"
)

func TestEncodeAndDecodeMdat(t *testing.T) {

	mdat := &mp4.MdatBox{
		StartPos: 4000,
	}

	sample := []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06}

	mdat.AddSampleData(sample)

	expectedMdatSize := 15
	mdatSize := mdat.Size()
	if mdatSize != uint64(expectedMdatSize) {
		t.Errorf("mdat size is %d instead of expected %d", mdatSize, expectedMdatSize)
	}

	var buf bytes.Buffer

	err := mdat.Encode(&buf)
	if err != nil {
		t.Error(err)
	}
	box, err := mp4.DecodeBox(0, &buf)
	if err != nil {
		t.Errorf("Could not decode written mdat box")
	}

	mdatDec := box.(*mp4.MdatBox)
	if mdatDec.Size() != uint64(expectedMdatSize) {
		t.Errorf("Decoded mdat size is %d instead of expected %d", mdatDec.Size(), expectedMdatSize)
	}
	if diff := deep.Equal(mdatDec.Data, sample); diff != nil {
		t.Error(diff)
	}
}

func TestEncodeAndDecodeMdatLargeSize(t *testing.T) {

	mdat := &mp4.MdatBox{
		StartPos: 4000,
	}

	sample := []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06}

	mdat.AddSampleData(sample)
	mdat.LargeSize = true

	expectedMdatSize := 15 + 8 // 8  is large size length
	mdatSize := mdat.Size()
	if mdatSize != uint64(expectedMdatSize) {
		t.Errorf("mdat size is %d instead of expected %d", mdatSize, expectedMdatSize)
	}

	var buf bytes.Buffer

	err := mdat.Encode(&buf)
	if err != nil {
		t.Error(err)
	}
	box, err := mp4.DecodeBox(0, &buf)
	if err != nil {
		t.Errorf("Could not decode written mdat box")
	}

	mdatDec := box.(*mp4.MdatBox)
	if mdatDec.Size() != uint64(expectedMdatSize) {
		t.Errorf("Decoded mdat size is %d instead of expected %d", mdatDec.Size(), expectedMdatSize)
	}
	if diff := deep.Equal(mdatDec.Data, sample); diff != nil {
		t.Error(diff)
	}
}

func TestReadData_NormalMode(t *testing.T) {

	mdat := &mp4.MdatBox{
		StartPos: 0,
		Data:     []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06},
	}

	data, err := mdat.ReadData(9, 5, nil)
	if err != nil {
		t.Error(err)
	}

	expected := mdat.Data[1:6]

	if !bytes.Equal(data, expected) {
		t.Errorf("expected %v, got %v", expected, data)
	}

}

func TestLazyMdatMode(t *testing.T) {

	mdatPayload := []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05}
	lazyMdat, testSeeker := createLazyMdat(t, mdatPayload)

	if !lazyMdat.IsLazy() {
		t.Error("expected lazy")
	}
	data, err := lazyMdat.ReadData(8, 6, testSeeker)
	if err != nil {
		t.Error(err)
	}

	expected := mdatPayload

	if !bytes.Equal(data, expected) {
		t.Errorf("expected %v, got %v", expected, data)
	}

	// Encode lazily does not write the mdat data, but just the header
	var buf bytes.Buffer
	err = lazyMdat.Encode(&buf)
	if err != nil {
		t.Error(err)
	}

	header := []byte{0x00, 0x00, 0x00, 0x0e, 'm', 'd', 'a', 't'}
	if !bytes.Equal(buf.Bytes(), header) {
		t.Errorf("expected %v, got %v", header, buf.Bytes())
	}
}

func TestCopyData_NormalMode(t *testing.T) {

	mdat := &mp4.MdatBox{
		StartPos: 0,
		Data:     []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06},
	}

	var outBuffer bytes.Buffer
	n, err := mdat.CopyData(9, 5, nil, &outBuffer)
	if n != 5 {
		t.Errorf("did get %d bytes instead of 5", n)
	}
	if err != nil {
		t.Error(err)
	}

	expected := mdat.Data[1:6]

	if !bytes.Equal(outBuffer.Bytes(), expected) {
		t.Errorf("expected %v, got %v", expected, outBuffer.Bytes())
	}
}

func TestCopyData_LazyMdatMode(t *testing.T) {

	mdatPayload := []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05}
	lazyMdat, testReadSeeker := createLazyMdat(t, mdatPayload)

	if !lazyMdat.IsLazy() {
		t.Error("expected lazy")
	}
	outBuf := bytes.Buffer{}
	n, err := lazyMdat.CopyData(9, 5, testReadSeeker, &outBuf)
	if n != 5 {
		t.Errorf("did get %d bytes instead of 5", n)
	}
	if err != nil {
		t.Error(err)
	}

	expected := mdatPayload[1:6]

	if !bytes.Equal(outBuf.Bytes(), expected) {
		t.Errorf("expected %v, got %v", expected, outBuf.Bytes())
	}
}

// TestAddParts - adding parts to mdat should give the same result as one big slice
func TestAddParts(t *testing.T) {
	mdat := &mp4.MdatBox{}
	part1 := []byte{0, 1, 2, 3, 4}
	part2 := []byte{5, 6, 7, 8}
	mdat.AddSampleData(part1)
	mdat.AddSampleData(part2)

	expMdat := &mp4.MdatBox{Data: []byte{0, 1, 2, 3, 4, 5, 6, 7, 8}}
	out := make([]byte, 17)
	outBuf := bytes.NewBuffer(out)
	err := mdat.Encode(outBuf)
	if err != nil {
		t.Error(err)
	}

	outExp := make([]byte, 17)
	outBufExp := bytes.NewBuffer(outExp)
	err = expMdat.Encode(outBufExp)
	if err != nil {
		t.Error(err)
	}
	if !bytes.Equal(outBuf.Bytes(), outBufExp.Bytes()) {
		t.Errorf("expected %v, got %v", outBufExp.Bytes(), outBuf.Bytes())
	}
}

func TestMdatHeaderSizeForLargeLazyPayload(t *testing.T) {
	cases := []struct {
		lazyDataSize   uint64
		wantHeaderSize uint64
	}{
		{1, 8},
		{math.MaxUint32 - 8, 8},  // largest payload where size fits 32 bits
		{math.MaxUint32 - 7, 16}, // one byte more needs the large header
		{1 << 32, 16},
	}
	for _, c := range cases {
		mdat := &mp4.MdatBox{}
		mdat.SetLazyDataSize(c.lazyDataSize)
		// HeaderSize must be right before Size() has flipped the LargeSize flag.
		if got := mdat.HeaderSize(); got != c.wantHeaderSize {
			t.Errorf("lazy payload %d: got header size %d, wanted %d", c.lazyDataSize, got, c.wantHeaderSize)
		}
		if size := mdat.Size(); size != c.wantHeaderSize+c.lazyDataSize {
			t.Errorf("lazy payload %d: size %d does not match header size %d", c.lazyDataSize, size, c.wantHeaderSize)
		}
	}
}

func TestMdatDataRangeChecks(t *testing.T) {
	// Payload of 7 bytes starting 8 bytes into the file.
	newMdat := func() *mp4.MdatBox {
		return &mp4.MdatBox{
			StartPos: 0,
			Data:     []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06},
		}
	}
	cases := []struct {
		desc    string
		start   int64
		size    int64
		wantErr bool
	}{
		{desc: "full payload", start: 8, size: 7, wantErr: false},
		{desc: "last byte", start: 14, size: 1, wantErr: false},
		{desc: "empty range at end", start: 15, size: 0, wantErr: false},
		{desc: "one byte too far", start: 8, size: 8, wantErr: true},
		{desc: "start at end with size", start: 15, size: 1, wantErr: true},
		{desc: "start before payload", start: 0, size: 4, wantErr: true},
		{desc: "start far beyond payload", start: 1 << 28, size: 4, wantErr: true},
		{desc: "negative size", start: 8, size: -1, wantErr: true},
		{desc: "negative start", start: -8, size: 4, wantErr: true},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			mdat := newMdat()
			data, err := mdat.ReadData(c.start, c.size, nil)
			switch {
			case c.wantErr && err == nil:
				t.Errorf("ReadData: expected error, got %v", data)
			case !c.wantErr && err != nil:
				t.Errorf("ReadData: unexpected error: %v", err)
			case !c.wantErr && int64(len(data)) != c.size:
				t.Errorf("ReadData: got %d bytes instead of %d", len(data), c.size)
			}

			var buf bytes.Buffer
			n, err := mdat.CopyData(c.start, c.size, nil, &buf)
			switch {
			case c.wantErr && err == nil:
				t.Errorf("CopyData: expected error, wrote %d bytes", n)
			case !c.wantErr && err != nil:
				t.Errorf("CopyData: unexpected error: %v", err)
			case !c.wantErr && n != c.size:
				t.Errorf("CopyData: wrote %d bytes instead of %d", n, c.size)
			}
		})
	}
}

// TestMdatMixedDataAndParts - data added with AddSampleData and with
// AddSampleDataPart can be combined in any order, and the payload is written
// in the order it was added. Before, adding a part on top of monolithic data
// panicked, and adding monolithic data on top of parts was silently dropped.
func TestMdatMixedDataAndParts(t *testing.T) {
	a := []byte{0, 1, 2}
	b := []byte{3, 4}
	c := []byte{5, 6, 7, 8}

	cases := []struct {
		name string
		add  func(m *mp4.MdatBox)
		want []byte
	}{
		{"dataOnly", func(m *mp4.MdatBox) {
			m.AddSampleData(a)
			m.AddSampleData(b)
		}, []byte{0, 1, 2, 3, 4}},
		{"partsOnly", func(m *mp4.MdatBox) {
			m.AddSampleDataPart(a)
			m.AddSampleDataPart(b)
		}, []byte{0, 1, 2, 3, 4}},
		{"dataThenPart", func(m *mp4.MdatBox) {
			m.AddSampleData(a)
			m.AddSampleDataPart(b)
		}, []byte{0, 1, 2, 3, 4}},
		{"partThenData", func(m *mp4.MdatBox) {
			m.AddSampleDataPart(a)
			m.AddSampleData(b)
		}, []byte{0, 1, 2, 3, 4}},
		{"interleaved", func(m *mp4.MdatBox) {
			m.AddSampleData(a)
			m.AddSampleDataPart(b)
			m.AddSampleData(c)
			m.AddSampleDataPart(a)
		}, []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 0, 1, 2}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			mdat := &mp4.MdatBox{}
			c.add(mdat)

			if got := mdat.DataLength(); got != uint64(len(c.want)) {
				t.Errorf("DataLength() is %d, expected %d", got, len(c.want))
			}
			if got, exp := mdat.Size(), mdat.HeaderSize()+uint64(len(c.want)); got != exp {
				t.Errorf("Size() is %d, expected %d", got, exp)
			}

			var buf bytes.Buffer
			if err := mdat.Encode(&buf); err != nil {
				t.Fatal(err)
			}
			if got := uint64(buf.Len()); got != mdat.Size() {
				t.Errorf("encoded %d bytes, but Size() is %d", got, mdat.Size())
			}
			payload := buf.Bytes()[mdat.HeaderSize():]
			if !bytes.Equal(payload, c.want) {
				t.Errorf("Encode payload is %v, expected %v", payload, c.want)
			}

			// The SliceWriter path must write the same bytes.
			sw := bits.NewFixedSliceWriter(int(mdat.Size()))
			if err := mdat.EncodeSW(sw); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(sw.Bytes(), buf.Bytes()) {
				t.Error("EncodeSW and Encode wrote different bytes")
			}
		})
	}
}

// TestMdatSetDataDropsParts - SetData replaces the whole payload, so earlier
// parts are dropped. Before, the parts won and the new data was ignored.
func TestMdatSetDataDropsParts(t *testing.T) {
	mdat := &mp4.MdatBox{}
	mdat.AddSampleDataPart([]byte{0, 1, 2})
	mdat.AddSampleData([]byte{3, 4})

	replacement := []byte{7, 7, 7, 7}
	mdat.SetData(replacement)

	if got := mdat.DataLength(); got != uint64(len(replacement)) {
		t.Errorf("DataLength() is %d after SetData, expected %d", got, len(replacement))
	}
	if len(mdat.DataParts) != 0 {
		t.Errorf("SetData left %d data parts", len(mdat.DataParts))
	}
	var buf bytes.Buffer
	if err := mdat.Encode(&buf); err != nil {
		t.Fatal(err)
	}
	if payload := buf.Bytes()[mdat.HeaderSize():]; !bytes.Equal(payload, replacement) {
		t.Errorf("payload after SetData is %v, expected %v", payload, replacement)
	}
}
