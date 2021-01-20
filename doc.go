/*
Package mp4ff - MP4 media file parser and writer for AVC and HEVC video, AAC audio and stpp/wvtt subtitles.
Focused on fragmented files as used for streaming in DASH, MSS and HLS fMP4.

MP4 library

The mp4 library has functions for parsing (called Decode) and writing (called Encode).
It is focused on fragmented files as used for streaming in DASH, MSS and HLS fMP4.
mp4.File is a representation of a "File" which can be more or less complete,
but should have some top layer boxes. It can include

   * InitSegment (ftyp + moov boxes)
   * One or more segments
       * Each segment has an optional styp box followed by one or more fragments
       * A fragment must always consist of one moof box followed by one mdat box.

The typical child boxes are exported so that one can write paths such as

    moof.Traf.Trun

to access the (only) trun box of a moof box.

Command Line Tools

Some simple command line tools are available in cmd directory.

Example code

Example code is available in the examples directory.
*/
package mp4ff
