![Logo](images/logo.png)

![Test](https://github.com/edgeware/mp4ff/workflows/Test/badge.svg)

MP4 media file parser and writer. Focused on fragmented files as used for streaming in DASH, MSS and HLS fMP4.

## Library

The library has functions for parsing (called Decode) and writing (Encode).

Traditional multiplexed non-fragmented mp4 files can be parsed and decoded, see `examples/segment`.

The focus is, however, on non-multiplexed single-track fragmented mp4 files as used in DASH, HLS, and CMAF.

The top level structure for both non-fragmented and fragmented mp4 files is `mp4.File`.

In the non-fragmented files, the members Ftyp, Moov, and Mdat are used.
A fragmented `mp4.File` file can be a single init segment, one or more media segments, or a a combination of both like a CMAF track which renders into a playable one-track asset.

The following high-level structures are used:

* `InitSegment` contains an `ftyp` and `moov` box and provides the metadata for a fragmented files. It corresponds to a CMAF header
* `MediaSegment` starts with an optional `styp` box and contains on or more `Fragment`s
* `Fragment` is an mp4 fragment with exactly one `moof` box followed by a `mdat` box where the latter contains the media data. It is limited to have exactly one `trun` box.

The typical child boxes are exported so that one can write paths such as

    fragment.Moof.Traf.Trun

to access the (only) trun box in a fragment.

The codec currently supported are AVC/H.264 and AAC.

## Usage for creating new fragmented files

A typical use case is to produce an init segment and followed by a series of media segments.

The first step is to create an init segment. This is done in two steps and can be seen in
`examples/initcreator:

1. A call to `CreateEmptyInitSegment(timecale, mediatype, language)`
2. Fill in the SampleDescriptor based on video parameter sets, or audio codec information

The second step is to start producing media segments. They should use the timescale that
was set when creating the init segments. The timescales should be chosen so that the
sample durations have exact values.

A media segment contains one or more fragments, where each fragment has a moof and an mdat box.
If all samples are available before the segment is to be used, one can use use a single
fragment in each segment. Example code for this can be found in `examples/segmenter`.

The high-level code to do that is to first create a slice of `SampleComplete` with the data needed.
All times are in the track timescale set when creating the init segment and coded in the `mdhd` box.

	mp4.SampleComplete{
		Sample: mp4.Sample{
	        Flags uint32 // Flag sync sample etc
	        Dur   uint32 // Sample duration in mdhd timescale
	        Size  uint32 // Size of sample data
	        Cto   int32  // Signed composition time offset
		},
	    DecodeTime uint64 // Absolute decode time (offset + accumulated sample Dur)
	    Data       []byte // Sample data
	}

The `mp4.Sample` part is what will be written into the `trun` box.
`DecodeTime` is the media timeline accumulated time. The value of the first samples of a fragment, will
be set as the `BaseMediaDecodeTime` in the `tfdt` box when calling Encode on the fragment.
Once a number of such samples are available, then can be added to a media segment

	seg := mp4.NewMediaSegment()
	frag := mp4.CreateFragment(uint32(segNr), mp4.DefaultTrakID)
	seg.AddFragment(frag)
	for _, sample := range samples {
		frag.AddSample(sample)
	}

This segment can finally be output to a `w io.Writer` as

    err := seg.Encode(w)


## Command Line Tools

Some simple command line tools are available in `cmd`.

## Example code

Example code is available in the `examples` directory.
These are

1. `initcreator` which creates typical init segments (ftyp + moov) for video and audio
2. `resegmenter` which reads a segmented file (CMAF track) and resegments it with other
    segment durations.
3. `segmenter` which takes a progressive mp4 file and creates init and media segments from it.


## LICENSE

See [LICENSE.md](LICENSE.md).
