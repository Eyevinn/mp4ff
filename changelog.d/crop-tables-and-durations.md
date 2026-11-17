### Changed

- `mp4ff-crop` ends just before the first sync sample presented at or after the duration, following
  the edit list, and keeps all samples instead of failing when there is no such sample

### Fixed

- `mp4ff-crop` crops `sbgp`, `subs` and `cslg`, and sets the `mdhd` duration, to the kept samples
- `mp4ff-crop` ends the edit lists and `tkhd` durations of all tracks where the movie ends, and keeps the
  samples of the other tracks up to that presentation time, without edits past the end of the media
- `mp4ff-crop` no longer fails when the kept samples include the last sample of a track
- `mp4ff-crop` rejects tracks with sample auxiliary information (`saiz`/`saio`), which it cannot crop
