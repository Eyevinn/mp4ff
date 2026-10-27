package mp4

// ffmpeg boxes according to https://kdenlive.org/en/project/adding-meta-data-to-mp4-video

// CTooBox - ©too box defines the ffmpeg encoding tool information
type CTooBox struct {
	Children []Box
}
