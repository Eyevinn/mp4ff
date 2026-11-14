### Fixed

- `hevc.ParseSliceHeader` infers index zero for sole SPS short- and long-term reference picture sets,
  keeping reference-list parsing and the CBCS clear-header boundary aligned
