# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- New function: `mp4.NewMediaSegmentWithStyp()`

- Associate emsg boxes with fragments

- New Fragment method: `AddEmsg()`

- `colr` box support

- CHANGELOG.md (this file) instead of Versions.md

### Changed

- Optimized translation from Annex B (start-code separated) video byte streams into length-field
  separated one
- Output of cenc example changed with styp boxes not included
- ADTS parsing somewhat more robust
- LastFragment() returns nil if no fragment in Segment

## [0.33.2] - 2023-01-26

### Changed

- Restored parsing of non-complete mdat box to v0.33.0 behavior, where a partial
  mdat box is not an error

## [0.33.1] - 2023-01-25

### Fixed

- Added missing parsing of sps_scaling_list_data_present_flag in HEVC SPS

## [0.33.0] - 2023-01-25

### Added

- Support QuickTime meta box as well as ISOBMFF meta box
- New possibility to disable parsing of specific boxes

## [0.32.0] - 2023-01-05

### Changed

- Moved repo to github.com/Eyevinn/mp4ff

### Deprecated

- The github.com/edgeware package path (but redirected by GitHub)

## [0.31.0] - 2022-12-28

### Changed

- Support multiple sidx
- Optimize stsc lookups
- New return type for NewNaluArray

### Fixed

- Fixed bugs in prft, adts and avc interlace

## [0.30.1] - 2022-11-06

### Fixed

- Fixed optimized sample copying bug introduced in v0.30.0

## [0.30.0] - 2022-11-04

### Added

- Full AVC slice header parsing.
- Complete set of AVC and HEVC SEI message detection
- Parsing of some SEI messages (136, 137, 144)
- Write of SEI message (#182)
- Optimizations in ctts and sample copying
- More byte stream methods
- mp4ff-pslister can now work on mp4 segments
- New functions for extracting NALUs for AVC and HEVC
- Optimized ctts lookup
- Optimized file.CopySampleData output allocations

### Fixed

- mp4ff-nallister NALU output
- emsg parsing
- HEVC nalu array bug
- Overflow in tfdt time (#177)

## [0.29.0] - 2022-06-21

### Changed

- Improved uuid box handling
- Improved esds box and underlying descriptor handling
- Extended decryption example with cbcs encryption
- Improved the decryption example with in-place cenc decryption

## [0.28.0] - 2022-05-12

### Added

- Full HEVC SPS parsing

### Changed

- Better video sample entry generation
- More AC-3/EC-3 support.
- Extended EBSPWriter
- Optimize: struct field alignments in bits package

### Fixed

- sdtp reference in StblBox
- decrypt-cenc example
- mp4ff-crop bad command line parameters

## [0.27.0] - 2022-03-06

### Added

- New more efficient SliceReader/SliceWriter based Box methods
- AC-3 and Enhanced AC-3 support

### Changed

- Public trun flag bits
- Public DecodeHeader method and BoxHeader structure
- mp4ff-nallister now takes Annex B byte stream
- mp4ff-pslister now takes Annex B byte stream and prints codec string

### Fixed

- mp4ff-crop stss bug
- ffmpeg data box decode
- stsz uniform size decode

## [0.26.1] - 2022-01-14

### Fixed

- Don't move trak boxes to be before mvex

## [0.26.0] - 2022-01-13

### Added

- New tool mp4ff-crop for cropping mp4 file
- New example decrypt-cenc for decrypting segment
- SEI parsing for H.264
- Interpret timestamps in mvhd, tkhd, and mdhd

## [0.25.0] - 2021-10-04

### Added

- Size() methods added to InitSegment, Fragment and MediaSegment
- SampleIntervals for more efficient transformation of segments
- Slices of samples and fullsamples
- Create init segments for wvtt and stpp

### Changed

- Improvements to ctts and stts boxes
- Changed sample slices to remove pointers
- Spell out compositionTimeOffset instead of cto
- More efficient code to check for AVC and HEVC parameter sets

## [0.24.0] - 2021-06-26

### Added

- Support for cslg and ©too boxes

### Changed

- DecodeFile API change to allow for lazy mdat decode
- segmenter example extension with lazy mode for decode and encode
- StssBox.IsSyncSample thread safe

## [0.23.1] - 2021-05-20

### Fixed

- Fixed segment encode mode without optimization

## [0.23.0] - 2021-04-30

### Changed

- API change: Verbatim encode more flexible with FragEncMode and EncOptimize

## [0.22.0] - 2021-04-23

### Added

- Construct AVC and HEVC codec strings from SPS

### Changed

- More robust parsing of ADTS

## [0.21.1] - 2021-03-24

### Fixed

- Allow ADTS ID corresponding to MPEG-2

## [0.21.0] - 2021-03-18

### Added

- Version number can be retrieved from cmd apps and from mp4 library.

### Changed

- Updated version is inserted as cmd apps are built using top-level Makefile.

## [0.20.0] - 2021-03-09

### Added

- New tool `mp4ff-pslister` for SPS and other parameter sets

## [0.19.0] - 2021-03-08

### Added

- Added mfra, mfro, tfra boxes
- Added kind, trep, skip, ilst boxes

### Fixed

- Fix for double trun-optimization

## [0.18.0] - 2021-01-26

### Added

- New tool `mp4ff-wvttlister` lists details of wvtt (WebVTT in ISOBMFF) samples
- More functions and methods for HEVC video handling

### Fixed

## [0.17.1] - 2021-01-19

### Fixed

- Fixed bugs in encoding/decoding hvcC box and HEVCDecoderConfigurationRecord

## [0.17.0] - 2021-01-15

### Added

- Support for HEVC/H.265 video parsing and encoding of boxes.
- New tool `mp4ff-nallister` to list nal units inside video samples in MP4 files.
- Support for multi-track fragmented files including new example
- Support for some new boxes: meta, udta, pasp, clap

### Changed

- mp4ff-pslister updated to support HEVC and reading parameter sets from byte streams and hex strings
- Improvements to Info output from mp4ff-info

## [0.16.2] - 2021-01-05

### Fixed

- Fixed and clarified that sample number starts at one

## [0.16.1] - 2021-01-04

### Fixed

- Fixed isNonSync flag definition
- Use sdtp entries and transfer to sample flags in segmenter example

## [0.16.0] - 2020-12-31

### Added

- New tool `mp4ff-info` prints details of a hierarchy of boxes. Has configurable levels of details
- Many new boxes: saio, saiz, enca, encv, frma, schm, schi, sinf, tenc, sbgp, sgpd, sdtp
- Support for writing largesize mdat box, even for smaller sizes
- Support for adts encode and decode

### Changed

- Much more detailed box info via recursive Info method
- Better tests vs golden files that can be updated with -update flag
- More error robustness in aac reading with new AccErrReader
- Replaced some panic calls with return of errors

## [0.15.0] - 2020-12-09

### Changed

- API for AVC video to to allow multiple SPS.
- Some more NALU types identified.

### Fixed

- Bug in handling of start-code emulation prevention bytes

## [0.14.0] - 2020-12-09

### Added

- AVC Byte stream (Annex B) support

## [0.13.0] - 2020-11-17

### Fixed

- Removed non-standard log package dependency

## [0.12.0] - 2020-11-17

### Changed

- Complete parsing of AVC (H.264) SPS and PP

## [0.11.1] - 2020-11-13

### Fixed

- Made SPS parser more robust

## [0.11.0] - 2020-11-13

### Added

- Lots of stuff

## [0.10.1] - 2020-09-23

### Fixed

- First sample-flag in trun and traf boxes

## [0.10.0] - 2020-09-03

### Added

- First release tag on GitHub
- Lots of stuff

### Changed

- New unique repo name: `mp4ff`

[unreleased]: https://github.com/Eyevinn/mp4ff/compare/v0.33.2...HEAD
[0.33.2]: https://github.com/Eyevinn/mp4ff/compare/v0.33.1...v0.33.2
[0.33.1]: https://github.com/Eyevinn/mp4ff/compare/v0.33.0...v0.33.1
[0.33.0]: https://github.com/Eyevinn/mp4ff/compare/v0.32.0...v0.33.0
[0.32.0]: https://github.com/Eyevinn/mp4ff/compare/v0.31.0...v0.32.0
[0.31.0]: https://github.com/Eyevinn/mp4ff/compare/v0.30.0...v0.31.0
[0.30.1]: https://github.com/Eyevinn/mp4ff/compare/v0.30.0...v0.30.1
[0.30.0]: https://github.com/Eyevinn/mp4ff/compare/v0.29.0...v0.30.0
[0.29.0]: https://github.com/Eyevinn/mp4ff/compare/v0.28.0...v0.29.0
[0.28.0]: https://github.com/Eyevinn/mp4ff/compare/v0.27.0...v0.28.0
[0.27.0]: https://github.com/Eyevinn/mp4ff/compare/v0.26.0...v0.27.0
[0.26.1]: https://github.com/Eyevinn/mp4ff/compare/v0.26.0...v0.26.1
[0.26.0]: https://github.com/Eyevinn/mp4ff/compare/v0.25.0...v0.26.0
[0.25.0]: https://github.com/Eyevinn/mp4ff/compare/v0.24.0...v0.25.0
[0.24.0]: https://github.com/Eyevinn/mp4ff/compare/v0.23.0...v0.24.0
[0.23.1]: https://github.com/Eyevinn/mp4ff/compare/v0.23.0...v0.23.1
[0.23.0]: https://github.com/Eyevinn/mp4ff/compare/v0.22.0...v0.23.0
[0.22.0]: https://github.com/Eyevinn/mp4ff/compare/v0.21.0...v0.22.0
[0.21.1]: https://github.com/Eyevinn/mp4ff/compare/v0.21.0...v0.21.1
[0.21.0]: https://github.com/Eyevinn/mp4ff/compare/v0.20.0...v0.21.0
[0.20.0]: https://github.com/Eyevinn/mp4ff/compare/v0.19.0...v0.20.0
[0.19.0]: https://github.com/Eyevinn/mp4ff/compare/v0.18.0...v0.19.0
[0.18.0]: https://github.com/Eyevinn/mp4ff/compare/v0.17.0...v0.18.0
[0.17.1]: https://github.com/Eyevinn/mp4ff/compare/v0.17.0...v0.17.1
[0.17.0]: https://github.com/Eyevinn/mp4ff/compare/v0.16.0...v0.17.0
[0.16.2]: https://github.com/Eyevinn/mp4ff/compare/v0.16.1...v0.16.2
[0.16.1]: https://github.com/Eyevinn/mp4ff/compare/v0.16.0...v0.16.1
[0.16.0]: https://github.com/Eyevinn/mp4ff/compare/v0.15.0...v0.16.0
[0.15.0]: https://github.com/Eyevinn/mp4ff/compare/v0.14.0...v0.15.0
[0.14.0]: https://github.com/Eyevinn/mp4ff/compare/v0.13.0...v0.14.0
[0.13.0]: https://github.com/Eyevinn/mp4ff/compare/v0.12.0...v0.13.0
[0.12.0]: https://github.com/Eyevinn/mp4ff/compare/v0.11.0...v0.12.0
[0.11.1]: https://github.com/Eyevinn/mp4ff/compare/v0.11.0...v0.11.1
[0.11.0]: https://github.com/Eyevinn/mp4ff/compare/v0.10.0...v0.11.0
[0.10.1]: https://github.com/Eyevinn/mp4ff/compare/v0.10.0...v0.10.1
[0.10.0]: https://github.com/Eyevinn/mp4ff/compare/v0.9.0...v0.10.0
