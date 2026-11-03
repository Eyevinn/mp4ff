/*
Package mp4 is a library for parsing and writing MP4/ISOBMFF files with a focus on fragmented files.

Most boxes have their own file named after the box four-letter name in the ISO/IEC 14496-12 standard,
but in some cases, there may be multiple boxes that have the same content, and the code then has a
generic name like visualsampleentry.go.

# Structure and usage

The top level structure for both non-fragmented and fragmented mp4 files is [File].
A whole file is read with [ReadMP4File] or [DecodeFile], and written with [WriteToFile] or [File.Encode].

In a progressive (non-fragmented) [File], the top-level attributes "Ftyp", "Moov", and "Mdat"
point to the corresponding top level boxes.

A fragmented [File] can be more or less complete, like a single init segment,
one or more media segments, or a combination of both, like a CMAF track which renders
into a playable one-track asset. It can also have multiple tracks.
For fragmented files, the following high-level attributes are used:

  - Init is an [*InitSegment] and contains a ftyp and a moov box and provides the
    general metadata for a fragmented file, track definitions including time scale and sample descriptors.
    It corresponds to a CMAF header. It can also contain one or more sidx boxes, and keeps any boxes
    between ftyp and moov, such as a C2PA manifest box.
  - Segments is a slice of [*MediaSegment] which start with an optional [StypBox],
    possibly one or more [SidxBox] and then one or more [Fragment].
  - [Fragment] is a mp4 fragment with exactly one [MoofBox] followed by a [MdatBox] where the latter
    contains the media data. It has one or more [TrunBox] containing the metadata for the samples.
    Other top-level boxes before the moof, such as emsg, prft, free or a C2PA uuid box, belong to the
    fragment and are listed with it in Children. The emsg and prft boxes are also available as
    Emsgs and Prfts. Boxes after the last mdat of a segment follow the mdat in the Children of its last
    fragment. Boxes that this structure has no place for at their position, such as a box after the mfra,
    are listed in the MisplacedBoxes attribute of [File].

It should be noted that it is sometimes hard to decide what should belong to a Segment or Fragment.

All child boxes of container boxes such as [MoofBox] are listed in the Children attribute, but the
most prominent child boxes have direct links with names which makes it possible to write a path such
as

	fragment.Moof.Traf.Trun

to access the (single or first) [TrunBox] in a fragment inside the (single or first) [TrafBox]
of a fragment.

There are corresponding structures with a plural form for accessing later boxes of the same type, e.g.

	fragment.Moof.Trafs[1].Truns[1]

to get the second [TrunBox] of the second [TrafBox] (provided that they exist). Care must be
taken to assert that none of the intermediate pointers are nil to avoid panic.

# Creating new fragmented files

A typical use case is to generate a fragmented file consisting of an init segment
followed by a series of media segments.

The first step is to create the init segment. This is done in three steps as can be seen in
[examples/initcreator]:

	init := mp4.CreateEmptyInit()
	trak := init.AddEmptyTrack(timescale, "video", "und")
	err := trak.SetHEVCDescriptor("hvc1", vpsNALUs, spsNALUs, ppsNALUs, nil, true)

Here the third step fills in codec-specific parameters into the sample descriptor of the track.
There are similar methods of [TrakBox] for other codecs, such as [TrakBox.SetAVCDescriptor] and
[TrakBox.SetAACDescriptor]. For multiple tracks, call AddEmptyTrack once per track. All tracks are
then available in the slice attribute init.Moov.Traks.

The second step is to start producing media segments. They should use the timescale that
was set when creating the init segment. Generally, that timescale should be chosen so that the
sample durations have exact values without rounding errors, e.g. 48000 for 48kHz audio.

A media segment contains one or more fragments.
If all samples are available before the segment is created, one can use a single
fragment in each segment. Example code for this can be found in [examples/segmenter].
For low-latency MPEG-DASH generation, short-duration fragments are added to the segment as the
corresponding media samples become available.

A simple, but not optimal, way of creating a media segment is to first create a slice of
[FullSample] with the data needed. A [FullSample] combines a [Sample], which is what is written
into the [TrunBox], with the sample data and the absolute decode time of the sample.
The decode time of the first sample of a fragment is written as BaseMediaDecodeTime in the [TfdtBox].
Once a number of such full samples are available, they can be added to a media segment like

	seg := mp4.NewMediaSegment()
	frag, err := mp4.CreateFragment(uint32(segNr), mp4.DefaultTrakID)
	if err != nil {
		return err
	}
	seg.AddFragment(frag)
	for _, sample := range samples {
		frag.AddFullSample(sample)
	}

This segment can finally be output to a [io.Writer] as

	err := seg.Encode(w)

or to a [bits.SliceWriter] as

	err := seg.EncodeSW(sw)

For multi-track segments, the code is a bit more involved. Please have a look at [examples/segmenter]
to see how it is done. A more optimal way of handling media sample is
to handle them lazily, or using intervals, as explained next.

# Lazy decoding and writing of mdat data

For video and audio, the dominating part of a mp4 file is the media data which is stored
in one or more [MdatBox]. In some cases, for example when segmenting large progressive
files, it is much more memory efficient to just read the movie or fragment metadata
from the [MoovBox] or [MoofBox] and defer the reading of the media data from
the [MdatBox] to later.

For decoding, this is supported by running
[DecodeFile] in lazy mode as

	parsedMp4, err = mp4.DecodeFile(ifd, mp4.WithDecodeMode(mp4.DecModeLazyMdat))

In this case, the media data of the [MdatBox] box will not be read, but only its size is being saved.
To read or copy the actual data corresponding to a sample, one must calculate the
corresponding byte range and either call

	func (m *MdatBox) ReadData(start, size int64, rs io.ReadSeeker) ([]byte, error)

or

	func (m *MdatBox) CopyData(start, size int64, rs io.ReadSeeker, w io.Writer) (nrWritten int64, err error)

Example code for this, including lazy writing of [MdatBox], can be found in [examples/segmenter]
with the lazy mode set.

# Sample data in other files

The dref box of a track can place its sample data in other files, as in Unified Streaming dref MP4 files
and files made with MP4Box -dref. [TrakBox.CheckDataIsSelfContained] reports such tracks.
[File.ReadSampleData] reads sample data from the file itself or, through a [DataResolver]
such as [LocalFileResolver], from the other files:

	resolver := mp4.NewLocalFileResolver(filepath.Dir(path))
	defer resolver.Close()
	data, err := parsedMp4.ReadSampleData(trak, startSampleNr, endSampleNr, nil, resolver)

[examples/segmenter] segments such files this way.

# More efficient I/O using SliceReader and SliceWriter

The use of the interfaces [io.Reader] and [io.Writer] for reading and writing boxes gives a lot of
flexibility, but is not optimal when it comes to memory allocation. In particular, the
Read(p []byte) method needs a slice "p" of the proper size to read data, which leads to a
lot of allocations and copying of data.
In order to achieve better performance, it is advantageous to read the full top level boxes into
one, or a few, slices and decode these. This is the reason that [bits.SliceReader] and [bits.SliceWriter]
were introduced and that there are double methods for decoding and encoding all boxes using
either of the interfaces. For example, a [TrunBox] is decoded by both [DecodeTrunSR] and [DecodeTrun].
[bits.FixedSliceReader] implements [bits.SliceReader] on a fixed-size slice.
For benchmarks, see [BENCHMARKS.md].

For further reduction of memory allocation, use a buffered top-level reader, especially
when reading the [MdatBox] box of a progressive file.

# Writing files and segments efficiently

Most Encode(w [io.Writer]) methods make roughly one Write call per box, nested boxes
included, since each box encodes itself into a small slice and writes it in one go.
Handed a bare [os.File], that is one write syscall per box, and a moov box consists of
many small boxes. Wrapping the file in a [bufio.Writer] and flushing it is therefore
several times faster.

[Fragment.Encode] is the exception. It encodes all boxes up to the [MdatBox] payload into
one pooled buffer, writes that with one call, and then writes the payload directly, one
call per payload part, without allocating. A decoded fragment therefore takes two Write
calls, and [MediaSegment.Encode] adds one for the styp box and one for each sidx box.
A buffer still saves those few small writes.

A large [MdatBox] payload is not slowed down by a buffer, since [bufio.Writer] passes a
write larger than its buffer directly to the file once the buffer is empty.
[WriteToFile] buffers internally. Code that opens its own file should do the same, and
must not forget to Flush before closing.

The EncodeSW(sw [bits.SliceWriter]) methods are an alternative: encode into a
[bits.FixedSliceWriter] of Size() bytes and write it out with a single call. Note that
the cost of this route is dominated by the per-call allocation and the zeroing of memory
that is then overwritten in full, not by the encoding, so allocating a new slice writer
for a single structure is not faster than a buffered Encode. It pays off when the same
buffer is reused across many structures, for example when writing a sequence of segments,
using [bits.NewFixedSliceWriterFromSlice] to wrap it. That call takes the size from the
length of the slice, not its capacity, so it must be sliced to at least Size() bytes.
For fragments it brings little, since [Fragment.Encode] already reuses a buffer.

# More about mp4 boxes

The mp4 package contains a lot of box implementations.

The [Box] interface is specified in box.go. [DecodeBox] and [DecodeBoxSR] decode the box size and
type in the box header and dispatch decode for each individual box depending on its type.

There is also a [ContainerBox] interface which is used for boxes that contain other boxes.

Most boxes have their own file named after the box, but in some cases, there may be multiple boxes
that have the same content, and the box structure and the source code file then has a generic name like
[VisualSampleEntryBox].

The interfaces define common Box methods including encode (writing),
but not the decode (parsing) methods which have distinct names for each box type and are
dispatched from the parsed box name.

That dispatch based on box name is defined by the tables "decodersSR" and "decoders"
for the functions [DecodeBoxSR] and [DecodeBox], respectively.
The SR variant that uses [bits.SliceReader] should normally be used for better performance.
If a box name is unknown, it will result in an [UnknownBox] being created.

How to add a new box to this package is described in [CONTRIBUTING.md].

# Direct changes of attributes

Many attributes are public and can therefore be changed freely.
The advantage of this is that it is possible to write code that can manipulate boxes
in many different ways, but one must be cautious to avoid breaking links to sub boxes or
create inconsistent states in the boxes.

As an example, container boxes such as [TrafBox] have a method "AddChild" which
adds a box to "Children", its slice of children boxes, but also sets a specific
member reference such as "Tfdt" to point to that box. If "Children" is manipulated
directly, that link may no longer be valid.

# Encoding modes and optimizations

For fragmented files, one can choose to either encode all boxes in a [File], or only code
the ones which are included in the init and media segments. The attribute of [File] that controls that
is called FragEncMode and has the type [EncFragFileMode].
Another attribute EncOptimize of type [EncOptimize] controls possible optimizations of the file
encoding process. Currently, there is only one possible optimization called [OptimizeTrun].
It can reduce the size of the [TrunBox] by finding and writing default
values in the [TfhdBox] and omitting the corresponding values from the [TrunBox].
Note that this may change the size of all ancestor boxes of the [TrunBox].

# Sample Number Offset

Following the ISOBMFF standard, sample numbers and other numbers start at 1 (one-based).
This applies to arguments of functions and methods.
The actual storage in slices is zero-based, so sample nr 1 has index 0 in the corresponding slice.

[examples/initcreator]: https://pkg.go.dev/github.com/Eyevinn/mp4ff/examples/initcreator
[examples/segmenter]: https://pkg.go.dev/github.com/Eyevinn/mp4ff/examples/segmenter
[BENCHMARKS.md]: https://github.com/Eyevinn/mp4ff/blob/master/BENCHMARKS.md
[CONTRIBUTING.md]: https://github.com/Eyevinn/mp4ff/blob/master/CONTRIBUTING.md
[bits.SliceReader]: https://pkg.go.dev/github.com/Eyevinn/mp4ff/bits#SliceReader
[bits.SliceWriter]: https://pkg.go.dev/github.com/Eyevinn/mp4ff/bits#SliceWriter
[bits.FixedSliceReader]: https://pkg.go.dev/github.com/Eyevinn/mp4ff/bits#FixedSliceReader
[bits.FixedSliceWriter]: https://pkg.go.dev/github.com/Eyevinn/mp4ff/bits#FixedSliceWriter
[bits.NewFixedSliceWriterFromSlice]: https://pkg.go.dev/github.com/Eyevinn/mp4ff/bits#NewFixedSliceWriterFromSlice
*/
package mp4
