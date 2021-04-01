package avc

import "fmt"

//Codecs - MIME subtype like avc1.42E00C where avc1 is sampleEntry string.
func Codecs(sampleEntry string, sps *SPS) string {
	return fmt.Sprintf("%s.%02X%02X%02X", sampleEntry, sps.Profile, sps.ProfileCompatibility, sps.Level)
}
