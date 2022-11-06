# Versions

| Version | Highlight |
| ------  | --------- |
| 0.30.1 | Fix optimized sample copying introduced in v0.30.0 |
| 0.30.0 | Full AVC slice header parsing. Enhanced SEI Messages. Optimizations in ctts and sample copying. Bug fixes in HEVC nalu, tfdt, emsg |
| 0.29.0 | Improved uuid and esds box handling. Extended decryption example with cbcs and in-place cenc decryption |
| 0.28.0 | Full HEVC SPS parsing. Better video sample entry generation. More AC-3/EC-3 support. Extended EBSPWriter. Bug fixes |
| 0.27.0 | New more efficient SliceReader/SliceWriter based Box methods. Add AC-3 and Enhanced AC-3 support. Public trun flag bits and DecodeHeader. mp4ff-nallister now takes Annex byte stream. Bug fixes |
| 0.26.1 | fix: don't move trak boxes to be before mvex |
| 0.26.0 | New example code for decrypting segment. New tool for cropping mp4 file. SEI parsing for H.264. Interpret timestamps |
| 0.25.0 | Support sample intervals. Control first sample flags. Create subtitle init segments. Minor improvements and fixes |
| 0.24.0 | api-change: DecodeFile lazy mode. Enhanced segmenter example with lazy read/write |
| 0.23.1 | fix: segment encode mode without optimization
| 0.23.0 | api-change: encode mode and optimization options |
| 0.22.0 | feat: add codec string for AVC and HEVC |
| 0.21.1 | fix: allow MPEG-2 ADTS |
| 0.21.0 | feat: version number in apps and mp4 package |
| 0.20.0 | feat: mp4ff-pslister better for hex SPS input |
| 0.19.0 | fix: trun optimization, feat: mfra-related boxes |
| 0.18.0 | feat: new mp4ff-wvttlister tool and fuller HEVC support |
| 0.17.1 | fix: HEVC box decode and encode with test |
| 0.17.0 | HEVC support and new tool mp4ff-nallister |
| 0.16.2 | fix: Minor fixes to sampleNumber and tfhd Info |
| 0.16.1 | fix: isNonSync flag declaration and use sdtp values in segmenter example |
| 0.16.0 | New mp4ff-info tool. Many new boxes incl. encryption boxes and sdtp. ADTS support. Test improvements with golden files |
| 0.15.0 | Support for multiple SPS and more explicit NALU types for AVC |
| 0.14.0 | Added functions to use Annex B byte stream for AVC/H.264 |
| 0.13.0 | Removed third-party log library |
| 0.12.0 | Complete parsing of AVC/H.264 SPS and PPS |
