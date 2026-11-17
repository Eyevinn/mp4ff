### Fixed

- Streaming `SampleAccessor.GetSamples` and `GetSampleRange` read contiguous sample runs together,
  while keeping each sample's data in its own buffer.
