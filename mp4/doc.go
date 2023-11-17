/*
Package mp4 - library for parsing and writing MP4/ISOBMFF files with a focus on fragmented files.

Most boxes have their own file named after the box four-letter name in the ISO/IEC 14996-12 standard,
but in some cases, there may be multiple boxes that have the same content, and the code is then having a
generic name like visualsampleentry.go.

The Box interface is specified in box.go. It decodes box size and type in the box header and
dispatched decode for each individual box depending on its type.

# Implement a new box

To implement a new box "fooo", the following is needed:

Create a file fooo.go and with struct type FoooBox.

FoooBox should then implement the Box interface methods:

	Type()
	Size()
	Encode()
	EncodeSW()
	Info()

but also its own decode method DecodeFooo and DecodeFoooSR, and register these
methods in the decoders map in box.go and decodersSR map in boxsr.go.
For a simple example, look at the `prft` box in `prft.go`.

# Container Boxes

Container boxes like moof, have a list of all their children called Children,
but also direct pointers to the children with appropriate names,
like Mfhd and Traf. This makes it easy to chain box paths to reach an
element like a TfhdBox as

	file.Moof.Traf.Tfhd

When there may be multiple children with the same name, there may be both a
pointer to a slice like Trafs with all boxes and Traf that points to the first.

# Media Sample Data Structures

To handle media sample data there are two structures:

1. `Sample` stores the sample information used in trun

2. `FullSample` also carries a slice with the samples binary data as well as decode time

# Fragmenting segments

A MediaSegment can be fragmented into multiple fragments by the method

	func (s *MediaSegment) Fragmentify(timescale uint64, trex *TrexBox, duration uint32) ([]*Fragment, error)
*/
package mp4
