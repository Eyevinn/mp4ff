/*
mp4ff-mvhevc inspects and creates MV-HEVC (Multi-View HEVC) MP4 files, such as stereo and Apple spatial video.

The info subcommand prints, for each video track, the base layer configuration (hvcC with its VPS),
the enhancement layer configuration (lhvC), Dolby Vision configuration, the Apple spatial video
metadata (vexu with stri, hero, blin and proj, and hfov), the number of samples and the frame rate.
It also prints the sample groups and track groups. With -idr, it lists the sync (IDR) frames.

The add subcommand muxes an HEVC bitstream, either as Annex B or in an mp4 file, into a progressive
mp4 file with hvcC for the base layer and, if the bitstream has parameter sets for a second layer,
lhvC for that layer. An Annex B input needs the frame rate as -fps. With -spatial, it adds the
Apple spatial video metadata vexu and hfov.

	Usage:

		mp4ff-mvhevc info [-idr] <input.mp4>                       Display MV-HEVC metadata
		mp4ff-mvhevc add [options] <input.hevc|mp4> <output.mp4>   Mux into an MV-HEVC MP4
		mp4ff-mvhevc -version                                      Print version and exit

	info options:

		-idr
			show IDR (sync) frame positions

	add options:

		-baseline uint
			camera baseline in micrometers (default 63500)
		-fps float
			frame rate, required for .hevc input (e.g. 23.976, 24, 30, 60)
		-hero string
			hero eye: left, right, or none (default "left")
		-hfov uint
			horizontal field of view in 1/1000 degrees (default 63500)
		-reversed
			eye views are reversed (right eye is base layer)
		-spatial
			add Apple spatial video metadata (vexu/hfov)
*/
package main
