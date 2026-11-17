### Fixed

- `StscBox.AddEntry` returns an error instead of writing an invalid table when `firstChunk` is not larger
  than that of the previous entry, or the sample description ID is 0
