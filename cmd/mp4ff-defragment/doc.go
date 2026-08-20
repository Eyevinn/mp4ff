/*
mp4ff-defragment converts a fragmented mp4 file into a progressive (moov-before-mdat) mp4 file.
Sample data is copied unchanged while progressive sample tables are synthesized
from the fragment metadata.

Usage of mp4ff-defragment:

mp4ff-defragment [options] <inFile> <outFile>

options:

	-t string
		Comma-separated track IDs to keep (default all tracks)
	-version
		Get mp4ff version
*/
package main
