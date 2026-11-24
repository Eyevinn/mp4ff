### Fixed

- `Defragment` and `mp4ff-defragment` keep the last frames of B-frame video: an edit that runs to the end of the media
  now ends where the last frame is presented, not where the last frame is decoded
