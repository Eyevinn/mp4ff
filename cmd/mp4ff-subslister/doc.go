/*
mp4ff-subslister lists and displays content of wvtt, wvtc, stpp, or stpc samples.
These corresponds to WebVTT or TTML subtitles in ISOBMFF files.
wvtc and stpc are the experimental paint-model variants, where a sample may be a
no-change box (vttn or ttmn) or, for stpc, a body-only box (ttmb).
Uses track with given non-zero track ID or first subtitle track found in an asset.

	Usage of mp4ff-subslister:

		mp4ff-subslister [options]

	options:

		-m int
				Max nr of samples to parse (default -1)
		-t int
				trackID to extract (0 is unspecified)
		-version
				Get mp4ff version
*/
package main
