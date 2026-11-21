package mp4

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
)

// ItemProperty is a property of an item, as an IpmaBox associates it with the item.
type ItemProperty struct {
	// Essential tells that a reader must understand the property to use the item.
	Essential bool
	// Index is the one-based index of the property in the IpcoBox.
	Index uint16
	// Property is the property, or nil if the IpcoBox has no property with the index.
	Property Box
}

// GetItemProperties returns the properties of the item with itemID in the order they are associated with it,
// which is the order they apply in. Transformations such as IrotBox and ImirBox are applied in that order.
func (b *IprpBox) GetItemProperties(itemID uint32) []ItemProperty {
	var props []ItemProperty
	for _, ipma := range b.Ipmas {
		for _, e := range ipma.Entries {
			if e.ItemID != itemID {
				continue
			}
			for _, a := range e.Associations {
				if a.PropertyIndex == 0 { // no property
					continue
				}
				prop := ItemProperty{Essential: a.Essential, Index: a.PropertyIndex}
				if b.Ipco != nil {
					prop.Property = b.Ipco.GetProperty(a.PropertyIndex)
				}
				props = append(props, prop)
			}
		}
	}
	return props
}

// GetItem returns the location of the item with itemID, or nil if there is none.
func (b *IlocBox) GetItem(itemID uint32) *IlocItem {
	for i := range b.Items {
		if b.Items[i].ItemID == itemID {
			return &b.Items[i]
		}
	}
	return nil
}

// GetToItemIDs returns the items that the item fromItemID refers to with references of type name, in the
// order of the references. For a grid item and dimg, these are its tiles in row order.
func (b *IrefBox) GetToItemIDs(name string, fromItemID uint32) []uint32 {
	var ids []uint32
	for _, ref := range b.Refs {
		if ref.Name == name && ref.FromItemID == fromItemID {
			ids = append(ids, ref.ToItemIDs...)
		}
	}
	return ids
}

// GetFromItemIDs returns the items that refer to the item toItemID with references of type name. For an
// image item, these are its thumbnails for thmb, its auxiliary images such as an alpha plane for auxl, and
// its metadata such as Exif for cdsc.
func (b *IrefBox) GetFromItemIDs(name string, toItemID uint32) []uint32 {
	var ids []uint32
	for _, ref := range b.Refs {
		if ref.Name != name {
			continue
		}
		for _, id := range ref.ToItemIDs {
			if id == toItemID {
				ids = append(ids, ref.FromItemID)
				break
			}
		}
	}
	return ids
}

// GetItemData returns the data of the item with itemID, which the IlocBox of the meta box locates in mdat or
// in the IdatBox of the meta box. The extents of an item with several are joined.
//
// mdat is the mdat box of the file. rs is the file, and is only used for an mdat box that was decoded with
// DecModeLazyMdat. The data of an item in one extent that is in memory is returned without copying, so it
// shares memory with mdat or the IdatBox.
//
// Items in another file (DataReferenceIndex not 0), and items that are made from other items
// (IlocConstructionItem), give an error, as does an item without location. An extent with length 0, which
// means the rest of its source, is only supported in the IdatBox.
func (b *MetaBox) GetItemData(itemID uint32, mdat *MdatBox, rs io.ReadSeeker) ([]byte, error) {
	if b == nil || b.Iloc == nil {
		return nil, errors.New("no iloc box")
	}
	item := b.Iloc.GetItem(itemID)
	if item == nil {
		return nil, fmt.Errorf("item %d has no location in iloc", itemID)
	}
	var source itemDataSource
	switch item.ConstructionMethod {
	case IlocConstructionFile:
		if item.DataReferenceIndex != 0 {
			return nil, fmt.Errorf("item %d is in another file (data reference %d)", itemID, item.DataReferenceIndex)
		}
		if mdat == nil {
			return nil, fmt.Errorf("item %d is in the file, but there is no mdat box", itemID)
		}
		source = mdatItemSource{mdat: mdat, rs: rs}
	case IlocConstructionIdat:
		if b.Idat == nil {
			return nil, fmt.Errorf("item %d is in the idat box, but there is none", itemID)
		}
		source = idatItemSource(b.Idat.Data)
	default:
		return nil, fmt.Errorf("item %d has construction method %d, which is not supported", itemID,
			item.ConstructionMethod)
	}
	// The extents must fit in the source one by one, and together, so that the size of the data is limited.
	start, size := source.span()
	ranges := make([]DataRange, 0, len(item.Extents))
	var total uint64
	for i, e := range item.Extents {
		off := item.BaseOffset + e.Offset
		if off < item.BaseOffset || off < start || off-start > size {
			return nil, fmt.Errorf("item %d: extent %d at %d is outside the item data source", itemID, i+1, off)
		}
		left := size - (off - start)
		length := e.Length
		if length == 0 { // the rest of the source
			if item.ConstructionMethod != IlocConstructionIdat {
				return nil, fmt.Errorf("item %d: extent %d has no length, which is only supported in an idat box",
					itemID, i+1)
			}
			length = left
		}
		total += length
		if length > left || total > size || total > math.MaxInt {
			return nil, fmt.Errorf("item %d: extent %d at %d of %d bytes does not fit in the item data source",
				itemID, i+1, off, length)
		}
		ranges = append(ranges, DataRange{Offset: off, Size: length})
	}
	var data []byte
	for i, r := range ranges {
		part, err := source.read(r.Offset, r.Size)
		if err != nil {
			return nil, fmt.Errorf("item %d: extent %d: %w", itemID, i+1, err)
		}
		if len(ranges) == 1 {
			return part, nil
		}
		if data == nil {
			data = make([]byte, 0, total)
		}
		data = append(data, part...)
	}
	return data, nil
}

// itemDataSource is where the extents of an item are: the file, or an idat box.
type itemDataSource interface {
	// span returns the first offset and the number of bytes that can hold item data.
	span() (start, size uint64)
	// read returns size bytes at offset off, which span has.
	read(off, size uint64) ([]byte, error)
}

// mdatItemSource is the payload of an mdat box, with offsets from the start of the file.
type mdatItemSource struct {
	mdat *MdatBox
	rs   io.ReadSeeker
}

func (s mdatItemSource) span() (start, size uint64) {
	if s.mdat.IsLazy() {
		return s.mdat.PayloadAbsoluteOffset(), s.mdat.GetLazyDataSize()
	}
	return s.mdat.PayloadAbsoluteOffset(), s.mdat.DataLength()
}

func (s mdatItemSource) read(off, size uint64) ([]byte, error) {
	if off > math.MaxInt64 || size > math.MaxInt64 {
		return nil, fmt.Errorf("offset %d or size %d is too large", off, size)
	}
	return s.mdat.ReadData(int64(off), int64(size), s.rs)
}

// idatItemSource is the data of an idat box, with offsets from its start.
type idatItemSource []byte

func (s idatItemSource) span() (start, size uint64) {
	return 0, uint64(len(s))
}

func (s idatItemSource) read(off, size uint64) ([]byte, error) {
	return s[off : off+size], nil
}

// ImageGrid is the data of an item of type grid: an image made of the images of other items, which are put
// side by side in rows, as defined in ISO/IEC 23008-12:2025 Sec. 6.6.2.3. The items are the ones that the
// grid item refers to with dimg references, row by row, which IrefBox.GetToItemIDs returns.
type ImageGrid struct {
	// Rows and Columns are the number of rows and columns of the grid, 1 to 256.
	Rows, Columns uint16
	// OutputWidth and OutputHeight are the size of the image. Tiles that go beyond it are cropped.
	OutputWidth, OutputHeight uint32
}

// DecodeImageGrid decodes the data of an item of type grid, which MetaBox.GetItemData returns.
func DecodeImageGrid(data []byte) (*ImageGrid, error) {
	if len(data) < 8 {
		return nil, fmt.Errorf("image grid: %d bytes are too few", len(data))
	}
	if data[0] != 0 {
		return nil, fmt.Errorf("image grid: version %d is not supported", data[0])
	}
	large := data[1]&1 != 0 // 32-bit instead of 16-bit output size
	g := ImageGrid{Rows: uint16(data[2]) + 1, Columns: uint16(data[3]) + 1}
	switch {
	case !large && len(data) == 8:
		g.OutputWidth = uint32(binary.BigEndian.Uint16(data[4:]))
		g.OutputHeight = uint32(binary.BigEndian.Uint16(data[6:]))
	case large && len(data) == 12:
		g.OutputWidth = binary.BigEndian.Uint32(data[4:])
		g.OutputHeight = binary.BigEndian.Uint32(data[8:])
	default:
		return nil, fmt.Errorf("image grid: %d bytes do not match the field size of its flags %#02x", len(data), data[1])
	}
	return &g, nil
}
