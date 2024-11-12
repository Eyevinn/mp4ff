/*
mp4ff-decrypt decrypts a fragmented mp4 file encrypted with Common Encryption scheme cenc or cbcs.
For a media segment, it needs an init segment with encryption information.

	    Usage of mp4ff-decrypt:

	        mp4ff-decrypt [options] infile outfile

	    options:

		    -init string
		  	    Path to init file with encryption info (scheme, kid, pssh)
		    -k string
		     	Required: key (hex)
		    -version
		  	    Get mp4ff version
*/
package main
