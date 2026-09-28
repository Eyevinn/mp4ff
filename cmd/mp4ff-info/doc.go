/*
mp4ff-info prints the box tree of input mp4 (ISOBMFF) file.

With -brands, it instead checks the brands of the ftyp and styp boxes against
the content, prints the issues found, and fails if any of them is an error.

	Usage of mp4ff-info:

		mp4ff-info [options] infile

	options:

		-brands
			check the ftyp and styp brands instead of printing the box tree
		-l string
			level of details, e.g. all:1 or trun:1,subs:1
		-version
			Get mp4ff version
*/
package main
