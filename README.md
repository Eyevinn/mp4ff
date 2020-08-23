![Logo](images/logo.png)

![Test](https://github.com/edgeware/mp4ff/workflows/Test/badge.svg)

MP4 media file parser and writer. Focused on fragmented files as used for streaming in DASH, MSS and HLS fMP4.

## Library

The library has functions for parsing (called Decode) and writing (Encode).
mp4.File is a representation of a "File" which can be more or less complete, but should have some top layer boxes.

It can include

* InitSegment (ftyp + moov boxes)

* One or more segments

* Each segment has an optional styp box followed by one or more fragments

* Fragment must always consist of one moof box followed by one mdat box.

The typical child boxes are exported so that one can write paths such as

    fragment.Moof.Traf.Trun

to access the (only) trun box in a fragment.

The codec currently supported are AVC (=H.264) and AAC.

When generating new content, the focus is on generating content following CMAF and DASH-IF guidelines.
In particular, it means that the content is not multiplexed, and there is only one track in each
moof box, and correspondingly a single `Traf` child box. The trakID is set to mp4.DefaultTrakID = 1.


## Usage for creating new fragmented files

A typical use case is to produce a series of segments start with an init segment and followed by media segments.

The first step is to create an init segment. This is done in two steps and can be seen in
`examples/initcreator:

1. A call to `CreateEmptyInitSegment(timecale, mediatype, language)`
2. Fill in the SampleDescriptor based on video parameter sets, or audio codec information.

The second step is to start producing media segments. They should use the timescale that
was set when creating the init segments. The timescales should be chosen so that the
sample durations have exact values.

A media segment contains one or more fragments, where each fragment has a moof and an mdat box.
If all samples are availble before the segment is to be used, the easiest is to use a single
fragment in each segment. Example code for this can be found in `examples/segmenter`.

	seg := mp4.NewMediaSegment()
	frag := mp4.CreateFragment(uint32(segNr), mp4.DefaultTrakID)
	seg.AddFragment(frag)
	for _, sample := range samples {
		frag.AddSample(sample)
	}

Here the samples are instances of `mp4.SampleComplete` which are binary data together with
timing information in the timescale of the trak and trun flags that should be set.
All times have a timescale of the track which was set in the `mdhd` box when the init segment
was created or read.


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
