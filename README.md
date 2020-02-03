# gomp4

MP4 media file parser and writer. Focused on fragmented files.

## Library
The library has functions for parsing (called Decode) and writing (Encocde).
mp4.File is a representation of a "File" which can be more or less complete

It can include

* InitSegment (ftyp + moov boxes)

* Segments (Optional styp box followed by fragments)

* Fragments must always consist of a moof box followed by an mdat box.

The typical child boxes are exported so that one can write paths such as

    fragment.Moof.Traf.Trun

to access the (only) trun box in a fragment.

## CLI Tools

There is a main tool mp4tool, that can be build in the cli directory.

    go build mp4tool

It's current functionality is to resegment a segmented file.
It can also show info about a file.

## LICENSE

See LICENSE
