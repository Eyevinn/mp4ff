/*
image-items is an example on how to read the items of a HEIF, HEIC or AVIF image file, and the movie of an
iPhone Live Photo.

It lists the images of the file with their size and role, and the metadata items. With -o, it writes the items
to files: coded images as HEVC (.hevc) or AV1 (.obu) elementary streams, where a grid is written as a stream of
its tiles in row order, Exif as TIFF (.tif) and XMP as .xmp.

Given the movie of a Live Photo as well, it tells if the image and the movie belong together, and where in the
movie the photo was taken.

	Usage of image-items:
	image-items [options] image [movie]

	options:
	-o string
			Directory to write the items to
	-version
			Get mp4ff version

The tiles of a grid can be put together with ffmpeg, here for a grid of 6 rows and 8 columns:

	ffmpeg -i item-49.hevc -vf tile=8x6 -frames:v 1 image.png

The result has the size of the tiles together, and is not cropped to the size of the grid or rotated.
*/
package main
