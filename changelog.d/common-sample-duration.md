### Fixed

- `Fragment.CommonSampleDuration` uses effective sample durations independently of header defaults and ignores empty runs.
  Fragments without samples return an error, also through `MediaSegment.CommonSampleDuration`.
