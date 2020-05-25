![Logo](images/logo.png)

![Test](https://github.com/edgeware/mp4ff/workflows/Test/badge.svg)

MP4 media file parser and writer. Focused on fragmented files as used for streaming in DASH, MSS and HLS fMP4.

## Library

The library has functions for parsing (called Decode) and writing (Encocde).
mp4.File is a representation of a "File" which can be more or less complete, but should have some top layer boxes.

It can include

* InitSegment (ftyp + moov boxes)

* One or more segments

* Each segment has an optional styp box followed by one or more fragments

* Fragment must always consist of one moof box followed by one mdat box.

The typical child boxes are exported so that one can write paths such as

    fragment.Moof.Traf.Trun

to access the (only) trun box in a fragment.

## Command Line Tools

Some simple command line tools are available in `cmd`.

## Example code

Example code is available in the `examples` directory.

## LICENSE

See [LICENSE.md](LICENSE.md).
