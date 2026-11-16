### Fixed

- `mp4ff-crop` writes a valid sample-to-chunk table when the crop ends inside the first chunk of a
  table entry, as for audio files with all samples in one chunk
