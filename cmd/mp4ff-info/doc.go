/*
mp4ff-info prints the box tree of input mp4 (ISOBMFF) file.

With -tree, or if the file cannot be decoded completely, it prints the box tree with
the position of each box, decodes as much as it can, and adds an error line for each
box that cannot be decoded. With -json, it prints that tree as JSON.

With -l, boxes show more details, such as the entries of sample tables, for all
boxes with all:1 or for some box types with trun:1,subs:1. Unless -json is set, the
output ends with a line that tells which levels show more details of the boxes in
the file.

With -brands, it instead checks the brands of the ftyp and styp boxes against
the content, prints the issues found, and fails if any of them is an error.

To browse the boxes, their fields and bytes interactively, in a terminal or a web
browser, use mp4lens: https://github.com/Eyevinn/mp4lens

	Usage of mp4ff-info:

		mp4ff-info [options] infile

	options:

		-brands
			check the ftyp and styp brands instead of printing the box tree
		-json
			print the box tree with box positions and errors as JSON
		-l string
			level of details, e.g. all:1 or trun:1,subs:1
		-tree
			print the box tree with box positions and errors, decoding as much as possible
		-version
			Get mp4ff version
*/
package main
