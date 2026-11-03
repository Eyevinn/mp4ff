/*
Module mp4ff is a Go library and a set of command line tools for parsing and writing MP4 (ISOBMFF) files.
It is focused on fragmented files as used for streaming in MPEG-DASH, HLS fMP4, CMAF and MSS, but can also
decode and encode progressive MP4 and QuickTime files.

The boxes and the file structure are in package [mp4], whose documentation is the guide to the library.
Other packages parse the bitstreams of video and audio codecs, such as [avc], [hevc], [av1] and [aac],
and package [bits] has the bit and byte readers and writers that they all use.

The [README] lists the supported codecs, the command line tools and the example programs.

[mp4]: https://pkg.go.dev/github.com/Eyevinn/mp4ff/mp4
[avc]: https://pkg.go.dev/github.com/Eyevinn/mp4ff/avc
[hevc]: https://pkg.go.dev/github.com/Eyevinn/mp4ff/hevc
[av1]: https://pkg.go.dev/github.com/Eyevinn/mp4ff/av1
[aac]: https://pkg.go.dev/github.com/Eyevinn/mp4ff/aac
[bits]: https://pkg.go.dev/github.com/Eyevinn/mp4ff/bits
[README]: https://github.com/Eyevinn/mp4ff#readme
*/
package mp4ff
