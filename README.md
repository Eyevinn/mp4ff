![Logo](images/logo.png)

![Test](https://github.com/Eyevinn/mp4ff/workflows/Go/badge.svg)
![golangci-lint](https://github.com/Eyevinn/mp4ff/workflows/golangci-lint/badge.svg?branch=master)
[![GoDoc](https://godoc.org/github.com/Eyevinn/mp4ff?status.svg)](http://godoc.org/github.com/Eyevinn/mp4ff)
[![Go Report Card](https://goreportcard.com/badge/github.com/Eyevinn/mp4ff)](https://goreportcard.com/report/github.com/Eyevinn/mp4ff)
[![license](https://img.shields.io/github/license/Eyevinn/mp4ff.svg)](https://github.com/Eyevinn/mp4ff/blob/master/LICENSE)

Package mp4ff implements MP4 media file parsing and writing for AVC and HEVC video, AAC and AC-3 audio,  and stpp and wvtt subtitles.
It is focused on fragmented files as used for streaming in DASH, MSS and HLS fMP4, but can also decode and encode all boxes needed for
progressive MP4 files. In particular, the tool `mp4ff-crop` can be
used to crop a progressive file.


## Command Line Tools

Some useful command line tools are available in `cmd`.

1. `mp4ff-info` prints a tree of the box hierarchy of a mp4 file with information
    about the boxes. The level of detail can be increased with the option `-l`, like `-l all:1` for all boxes 
	or `-l trun:1,stss:1` for specific boxes.
2. `mp4ff-pslister` extracts and displays SPS and PPS for AVC or HEVC in a mp4 or a bytestream (Annex B) file.
    Partial information is printed for HEVC.
3. `mp4ff-nallister` lists NALUs and picture types for video in progressive or fragmented file
4. `mp4ff-wvttlister` lists details of wvtt (WebVTT in ISOBMFF) samples
5. `mp4ff-crop` shortens a progressive mp4 file to a specified duration

You can install these tools by going to their respective directory and run `go install .` or directly from the repo with

    go install github.com/Eyevinn/mp4ff/cmd/mp4ff-info@latest

## Example code

Example code is available in the `examples` directory.
The examples and their functions are:

1. `initcreator` creates typical init segments (ftyp + moov) for video and audio
2. `resegmenter` reads a segmented file (CMAF track) and resegments it with other
    segment durations using `fullSample`
3. `segmenter` takes a progressive mp4 file and creates init and media segments from it.
    This tool has been extended to support generation of segments with multiple tracks as well
	as reading and writing `mdat` in lazy mode
4. `multitrack` parses a fragmented file with multiple tracks
5. `decrypt-cenc` decrypts a segmented mp4 file encrypted in `cenc` mode

## Library

The library has functions for parsing (called Decode) and writing (Encode) in the package `mp4ff/mp4`.
It also contains codec specific parsing of AVC/H.264 including complete parsing of
SPS and PPS in the package `mp4ff.avc`. HEVC/H.265 parsing is less complete, and available as `mp4ff.hevc`.
Supplementary Enhancement Information can be parsed and written using the package `mp4ff.sei`.

Traditional multiplexed non-fragmented mp4 files can be parsed and decoded, but the focus is on fragmented mp4 files
as used in DASH, HLS, and CMAF.

Beyond single-track fragmented files, support has been added to parse and generate multi-track
fragmented files as can be seen in `examples/segment` and `examples/multitrack`.

The top level structure for both non-fragmented and fragmented mp4 files is `mp4.File`.

In a progressive (non-fragmented) `mp4.File`, the top level attributes Ftyp, Moov, and Mdat points to the corresponding boxes.

A fragmented `mp4.File` can be more or less complete, like a single init segment,
one or more media segments, or a combination of both like a CMAF track which renders
into a playable one-track asset. It can also have multiple tracks.
For fragmented files, the following high-level attributes are used:

* `Init` contains a `ftyp` and a `moov` box and provides the general metadata for a fragmented file.
   It corresponds to a CMAF header. It can also contain one or more `sidx` boxes.
* `Segments` is a slice of `MediaSegment` which start with an optional `styp` box, possibly one or more `sidx boxes
   and then one or more `Fragment`s.
* `Fragment` is a mp4 fragment with exactly one `moof` box followed by a `mdat` box where the latter
   contains the media data. It can have one or more `trun` boxes containing the metadata
   for the samples.

All child boxes of container box such as `MoovBox` are listed in the `Children` attribute, but the
most prominent child boxes have direct links with names which makes it possible to write a path such
as

```go
fragment.Moof.Traf.Trun
```

to access the (only) `trun` box in a fragment with only one `traf` box, or

```go
fragment.Moof.Trafs[1].Trun[1]
```

to get the second `trun` of the second `traf` box (provided that they exist). Care must be
taken to assert that none of the intermediate pointers are nil to avoid `panic`.


## Creating new fragmented files

A typical use case is to a fragment consisting of an init segment followed by a series of media segments.

The first step is to create the init segment. This is done in three steps as can be seen in
`examples/initcreator`:

```go
init := mp4.CreateEmptyInit()
init.AddEmptyTrack(timescale, mediatype, language)
init.Moov.Trak.SetHEVCDescriptor("hvc1", vpsNALUs, spsNALUs, ppsNALUs)
```

Here the third step fills in codec-specific parameters into the sample descriptor of the single track.
Multiple tracks are also available via the slice attribute `Traks` instead of `Trak`.

The second step is to start producing media segments. They should use the timescale that
was set when creating the init segment. Generally, that timescale should be chosen so that the
sample durations have exact values without rounding errors.

A media segment contains one or more fragments, where each fragment has a `moof` and a `mdat` box.
If all samples are available before the segment is created, one can use a single
fragment in each segment. Example code for this can be found in `examples/segmenter`.

A simple, but not optimal, way of creating a media segment is to first create a slice of `FullSample` with the data needed.
The definition of `mp4.FullSample` is

```go
mp4.FullSample{
	Sample: mp4.Sample{
		Flags uint32 // Flag sync sample etc
		Dur   uint32 // Sample duration in mdhd timescale
		Size  uint32 // Size of sample data
		Cto   int32  // Signed composition time offset
	},
	DecodeTime uint64 // Absolute decode time (offset + accumulated sample Dur)
	Data       []byte // Sample data
}
```

The `mp4.Sample` part is what will be written into the `trun` box.
`DecodeTime` is the media timeline accumulated time.
The `DecodeTime` value of the first sample of a fragment, will
be set as the `BaseMediaDecodeTime` in the `tfdt` box.

Once a number of such full samples are available, they can be added to a media segment like

```go
seg := mp4.NewMediaSegment()
frag := mp4.CreateFragment(uint32(segNr), mp4.DefaultTrakID)
seg.AddFragment(frag)
for _, sample := range samples {
	frag.AddFullSample(sample)
}
```

This segment can finally be output to a `w io.Writer` as

```go
err := seg.Encode(w)
```

For multi-track segments, the code is a bit more involved. Please have a look at `examples/segmenter`
to see how it is done. A more optimal way of handling media sample is
to handle them lazily, as explained next.

### Lazy decoding and writing of mdat data

For video and audio, the dominating part of a mp4 file is the media data which is stored
in one or more `mdat` boxes. In some cases, for example when segmenting large progressive
files, it is much more memory efficient to just read the movie or fragment data
from the `moov` or `moof` box and defer the reading of the media data from the `mdat` box
to later.

For decoding, this is supported by running `mp4.DecodeFile()` in lazy mode as

```go
parsedMp4, err = mp4.DecodeFile(ifd, mp4.WithDecodeMode(mp4.DecModeLazyMdat))
```

In this case, the media data of the `mdat` box will not be read, but only its size is being set.
To read or copy the actual data corresponding to a sample, one must calculate the
corresponding byte range and either call

```go
func (m *MdatBox) ReadData(start, size int64, rs io.ReadSeeker) ([]byte, error)
```

or

```go
func (m *MdatBox) CopyData(start, size int64, rs io.ReadSeeker, w io.Writer) (nrWritten int64, err error)
```

Example code for this, including lazy writing of `mdat`, can be found in `examples/segmenter`
with the `lazy` mode set.

## More efficient I/O using SliceReader and SliceWriter

The use of the interfaces `io.Reader` and `io.Writer` for reading and writing boxes gives a lot of
flexibility, but is not optimal when it comes to memory allocation. In particular, the
`Read(p []byte)` method needs a slice `p` of the proper size to read data, which leads to a
lot of allocations and copying of data.
In order to achieve better performance, it is advantageous to read the full top level boxes into
one, or a few, slices and decode these.

To enable that mode, version 0.27 of the code introduced `DecodeX(sr bits.SliceReader)`
methods to every box X where `mp4ff.bits.SliceReader` is an interface.
For example, the `TrunBox` gets the method `DecodeTrunSR(sr bits.SliceReader)` in addition to its old
`DecodeTrun(r io.Reader)` method. The `bits.SliceReader` interface provides methods to read all kinds
of data structures from an underlying slice of bytes. It has an implementation `bits.FixedSliceReader`
which uses a fixed-size slice as underlying slice, but one could consider implementing a growing version
which would get its data from some external source.

The memory allocation and speed improvements achieved by this may vary, but should be substantial,
especially compared to versions before 0.27 which used an extra `io.LimitReader` layer.

Fur further reduction of memory allocation when reading the ´mdat` data of a progressive file, some
sort of buffered reader should be used.

### Benchmarks

To investigate the efficiency of the new SliceReader and SliceWriter methods, benchmarks have been done.
The benchmarks are defined in
the file `mp4/benchmarks_test.go` and `mp4/benchmarks_srw_test.go`.
For `DecodeFile`, one can see a big improvement by going from version
0.26 to version 0.27 which both use the `io.Reader` interface
but another big increase by using the `SliceReader` source.
The latter benchmarks are called `BenchmarkDecodeFileSR` but have
here been given the same name, for easy comparison.
Note that the allocations here refers to the heap allocations
that are done inside the benchmark loop. Outside that loop,
a slice is allocated to keep the input data.

For `EncodeFile`, one can see that v0.27 is actually worse
than v0.26 when used with the `io.Writer` interface. That is
because the code was restructured so that all writes go
via the `SliceWriter` layer in order to reduce code duplication.
However, if instead using the `SliceWriter` methods directly,
there is a big relative gain in allocations as can be seen in
the last column.

| name \ time/op           |  v0.26 |  v0.27 | v0.27-srw |
| ------------------------ | ------ | ------ | --------- |
|DecodeFile/1.m4s-16       | 21.9µs |  6.7µs |    2.6µs  |
|DecodeFile/prog_8s.mp4-16 |  143µs |   48µs |     16µs  |
|EncodeFile/1.m4s-16       | 1.70µs | 2.14µs |   1.50µs  |
|EncodeFile/prog_8s.mp4-16 | 15.7µs | 18.4µs |   12.9µs  |

| name \ alloc/op          | v0.26 |  v0.27 | v0.27-srw |
| ------------------------ |------ | ------ | --------- |
DecodeFile/1.m4s-16     |    120kB |   28kB |       2kB |
DecodeFile/prog_8s.mp4-16 |  906kB |  207kB |      12kB |
EncodeFile/1.m4s-16       | 1.16kB | 1.39kB |    0.08kB |
EncodeFile/prog_8s.mp4-16 | 6.84kB | 8.30kB |    0.05kB |

| name \ allocs/op         | v0.26 | v0.27 | v0.27-srw |
| ------------------------ |------ | ----- | --------- |
|DecodeFile/1.m4s-16       |  98.0 |  42.0 |      34.0 |
|DecodeFile/prog_8s.mp4-16 |   454 |   180 |       169 |
|EncodeFile/1.m4s-16       |  15.0 |  15.0 |       3.0 |
|EncodeFile/prog_8s.mp4-16 |   101 |    86 |         1 |

## Box structure and interface

Most boxes have their own file named after the box, but in some cases, there may be multiple boxes
that have the same content, and the code file then has a generic name like
`mp4/visualsampleentry.go`.

The Box interface is specified in `mp4/box.go`. It does not contain decode (parsing) methods which have
distinct names for each box type and are dispatched,

The mapping for decoding dispatch is given in the table `mp4.decoders` for the
`io.Reader` methods and in `mp4.decodersSR` for the `mp4ff.bits.SliceReader` methods.

## How to implement a new box
To implement a new box `fooo`, the following is needed.

Create a file `fooo.go` and create a struct type `FoooBox`.

`FoooBox` must implement the Box interface methods:

```go
Type()
Size()
Encode(w io.Writer)
EncodeSW(sw bits.SliceWriter)  // new in v0.27.0
Info()
```

It also needs its own decode method `DecodeFooo`, which must be added in the `decoders` map in `box.go`,
and the new in v0.27.0 `DecodeFoooSR` method in `decodersSR`.
For a simple example, look at the `PrftBox` in `prft.go`.

A test file `fooo_test.go` should also have a test using the method `boxDiffAfterEncodeAndDecode` to check that
the box information is equal after encoding and decoding.


## Direct changes of attributes

Many attributes are public and can therefore be changed in freely.
The advantage of this is that it is possible to write code that can manipulate boxes
in many different ways, but one must be cautious to avoid breaking links to sub boxes or
create inconsistent states in the boxes.

As an example, container boxes such as `TrafBox` have a method `AddChild` which
adds a box to `Children`, its slice of children boxes, but also sets a specific
member reference such as `Tfdt` to point to that box. If `Children` is manipulated
directly, that link may not be valid.

## Encoding modes and optimizations
For fragmented files, one can choose to either encode all boxes in a `mp4.File`, or only code
the ones which are included in the init and media segments. The attribute that controls that
is called `FragEncMode`.
Another attribute `EncOptimize` controls possible optimizations of the file encoding process.
Currently, there is only one possible optimization called `OptimizeTrun`.
It can reduce the size of the `TrunBox` by finding and writing default
values in the `TfhdBox` and omitting the corresponding values from the `TrunBox`.
Note that this may change the size of all ancestor boxes of `trun`.

## Sample Number Offset
Following the ISOBMFF standard, sample numbers and other numbers start at 1 (one-based).
This applies to arguments of functions and methods.
The actual storage in slices is zero-based, so
sample nr 1 has index 0 in the corresponding slice.

## Stability
The APIs should be fairly stable, but minor non-backwards-compatible changes may happen until version 1.

## Specifications
The main specification for the MP4 file format is the ISO Base Media File Format (ISOBMFF) standard
ISO/IEC 14496-12 6th edition 2020. Some boxes are specified in other standards, as should be commented
in the code.

## LICENSE

MIT, see [LICENSE](LICENSE).

Some code in pkg/mp4, comes from or is based on https://github.com/jfbus/mp4 which has
`Copyright (c) 2015 Jean-François Bustarret`.

Some code in pkg/bits comes from or is based on https://github.com/tcnksm/go-casper/tree/master/internal/bits
`Copyright (c) 2017 Taichi Nakashima`.

## ChangeLog and Versions

See [CHANGELOG.md](CHANGELOG.md).

## Support

Join our [community on Slack](http://slack.streamingtech.se) where you can post any questions regarding any of our open source projects. Eyevinn's consulting business can also offer you:

- Further development of this component
- Customization and integration of this component into your platform
- Support and maintenance agreement

Contact [sales@eyevinn.se](mailto:sales@eyevinn.se) if you are interested.

## About Eyevinn Technology

[Eyevinn Technology](https://www.eyevinntechnology.se) is an independent consultant firm specialized in video and streaming. Independent in a way that we are not commercially tied to any platform or technology vendor. As our way to innovate and push the industry forward we develop proof-of-concepts and tools. The things we learn and the code we write we share with the industry in [blogs](https://dev.to/video) and by open sourcing the code we have written.

Want to know more about Eyevinn and how it is to work here. Contact us at work@eyevinn.se!