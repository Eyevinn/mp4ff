### Fixed

- `BoxSeekReader.Seek` reads only up to the target position on a forward seek after a backward seek, so
  `StreamFile.ProcessFragments` keeps its place when the fragment callback reads samples out of order
