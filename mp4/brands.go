package mp4

// Brands for the ftyp and styp boxes, grouped by the specification that
// defines them.
//
// Listing a brand in compatible_brands claims that the file or segment
// conforms to the specification the brand identifies (ISO/IEC 14496-12:2026
// E.1). The major brand should also be repeated among the compatible brands
// (14496-12:2026 Section 5.2).

// Structural brands of ISO/IEC 14496-12:2026 Annex E.
//
// isom, avc1, iso2 ... iso9, isoa ... isod form a chain in which each brand
// requires support for everything the previous one does. Each comment names
// what the brand adds to the previous brand in the chain.
const (
	// BrandIsom - basic ISOBMFF: moov, moof, trun v0, ctts v0, edit lists with rate 0 or 1 (E.2)
	BrandIsom = "isom"
	// BrandAvc1 - sdtp, sbgp, sgpd and the 'roll' sample group (E.3). Not the AVC sample entry
	BrandAvc1 = "avc1"
	// BrandIso2 - subs, pdin, meta, and content protection with sinf (E.4)
	BrandIso2 = "iso2"
	// BrandIso3 - file delivery boxes, iinf v1, 'rash' sample group (E.6)
	BrandIso3 = "iso3"
	// BrandIso4 - ctts v1, cslg v0, trgr, iloc v1, idat, iref, 'alst' sample group (E.7)
	BrandIso4 = "iso4"
	// BrandIso5 - default-base-is-moof in tfhd, restricted sample entries 'resv' (E.8)
	BrandIso5 = "iso5"
	// BrandIso6 - tfdt, trun v1, saiz v0, saio, styp, sidx, ssix, prft, sgpd in traf (E.9)
	BrandIso6 = "iso6"
	// BrandIso7 - trep, assp, 32-bit item IDs, incomplete tracks (E.10)
	BrandIso7 = "iso7"
	// BrandIso8 - sthd, meta in movie fragments, more than one subs per track (E.11)
	BrandIso8 = "iso8"
	// BrandIso9 - elng, cslg v1 (E.12)
	BrandIso9 = "iso9"
	// BrandIsoa - iprp, grpl, 'altr' entity group, 'stmi', 'drap', 'prol' and 'sap ' sample groups (E.13)
	BrandIsoa = "isoa"
	// BrandIsob - ttyp, brnd, any edit list media_rate, trgr with flags&1 (E.14)
	BrandIsob = "isob"
	// BrandIsoc - compressed boxes !mov, !mof, !six, !ssx, and otyp (E.16)
	BrandIsoc = "isoc"
	// BrandIsod - hdlp, spki, prsl, sgpd v3, sample-packed tracks, meta without hdlr (E.19)
	BrandIsod = "isod"
	// BrandSaie - saiz v1 and v2 on top of iso6 (E.20). Not part of the chain above
	BrandSaie = "saie"
	// BrandMp71 - file-level MetaBox with MPEG-7 handler (E.5)
	BrandMp71 = "mp71"
	// BrandRelo - relocatable media data (imda) with a BoxFileIndexBox, used in tyco (E.15)
	BrandRelo = "relo"
	// BrandComp - compressed boxes of Section 8.16 (E.17)
	BrandComp = "comp"
	// BrandUnif - unified ID space for tracks, track groups, items and entity groups (E.18)
	BrandUnif = "unif"
)

// MP4 file format brands of ISO/IEC 14496-14. An MP4 file shall have at
// least one of them as a compatible brand.
const (
	// BrandMp41 - MP4 version 1
	BrandMp41 = "mp41"
	// BrandMp42 - MP4 version 2
	BrandMp42 = "mp42"
)

// Brands of ISO/IEC 14496-15 for layered and tiled HEVC. There are no brands
// for the avc1/avc3, hvc1/hev1 or vvc1/vvi1 sample entries.
const (
	// BrandHvce - L-HEVC with explicit reconstruction using extractors (D.4.1)
	BrandHvce = "hvce"
	// BrandHvci - L-HEVC with implicit reconstruction, one layer per track (D.4.2)
	BrandHvci = "hvci"
	// BrandHvti - HEVC tile tracks (D.4.3)
	BrandHvti = "hvti"
	// BrandLhti - L-HEVC tile tracks with implicit reconstruction (D.4.4)
	BrandLhti = "lhti"
	// BrandLhte - L-HEVC tile tracks with explicit reconstruction (D.4.5)
	BrandLhte = "lhte"
	// BrandHvcx - L-HEVC extended explicit reconstruction with hvc3/hev3 (D.4.6)
	BrandHvcx = "hvcx"
	// BrandNras - no RASL pictures associated with CRA or BLA pictures in sync samples (D.5)
	BrandNras = "nras"
)

// CMAF structural and segment brands of ISO/IEC 23000-19:2024 Section 7.2.
// When a CMAF structural brand is the major brand, minor_version shall be 0.
const (
	// BrandCmfc - CMAF structural brand for the constraints of Section 7.6 (ftyp and styp)
	BrandCmfc = "cmfc"
	// BrandCmf2 - cmfc plus the constraints of Section 7.7 (ftyp and styp)
	BrandCmf2 = "cmf2"
	// BrandCmfs - CMAF segment (styp)
	BrandCmfs = "cmfs"
	// BrandCmff - CMAF fragment (styp)
	BrandCmff = "cmff"
	// BrandCmfl - CMAF chunk (styp)
	BrandCmfl = "cmfl"
	// BrandCmfr - CMAF random access chunk, starting with SAP type 1, 2 or 3 (styp)
	BrandCmfr = "cmfr"
)

// CMAF media profile brands of ISO/IEC 23000-19:2024 with Amd 1:2024. They
// are placed among the compatible brands of the CMAF header, and should only
// be listed when the track meets the constraints of the media profile.
const (
	// BrandCfsd - AVC SD: High@3.1, up to 864x576, 60 fps (A.2)
	BrandCfsd = "cfsd"
	// BrandCfhd - AVC HD: High@4.0, up to 1920x1080, 60 fps (A.2)
	BrandCfhd = "cfhd"
	// BrandChdf - AVC HDHF: High@4.2, up to 1920x1080, 60 fps (A.2)
	BrandChdf = "chdf"
	// BrandChhd - HEVC HHD8: Main@4.1, up to 1920x1080, 60 fps (B.5)
	BrandChhd = "chhd"
	// BrandChh1 - HEVC HHD10: Main10@4.1, up to 1920x1080, 60 fps (B.5)
	BrandChh1 = "chh1"
	// BrandCud8 - HEVC UHD8: Main@5.0, up to 3840x2160, 60 fps (B.5)
	BrandCud8 = "cud8"
	// BrandCud1 - HEVC UHD10: Main10@5.1, up to 3840x2160, 60 fps, BT.709 or BT.2020 (B.5)
	BrandCud1 = "cud1"
	// BrandChd1 - HEVC HDR10: Main10@5.1, up to 3840x2160, 60 fps, BT.2100 PQ (B.5)
	BrandChd1 = "chd1"
	// BrandClg1 - HEVC HLG10: Main10@5.1, up to 3840x2160, 60 fps, BT.2100 HLG (B.5)
	BrandClg1 = "clg1"
	// BrandCud9 - HEVC UHD8 at up to 120 fps (B.6)
	BrandCud9 = "cud9"
	// BrandCud2 - HEVC UHD10 at up to 120 fps (B.6)
	BrandCud2 = "cud2"
	// BrandChd2 - HEVC HDR10 at up to 120 fps (B.6)
	BrandChd2 = "chd2"
	// BrandClg2 - HEVC HLG10 at up to 120 fps (B.6)
	BrandClg2 = "clg2"
	// BrandCint - HEVC INT10: interlaced Main10, level up to 4.1 (B.7)
	BrandCint = "cint"
	// BrandCsh1 - Scalable HEVC SHV10: Scalable Main 10@5.2, up to 3840x2160, 120 fps (H.6)
	BrandCsh1 = "csh1"
	// BrandCvvc - VVC Main 10, single layer (M.2)
	BrandCvvc = "cvvc"
	// BrandCvvm - multilayer VVC (M.3)
	BrandCvvm = "cvvm"
	// BrandCevb - EVC Baseline (Annex N)
	BrandCevb = "cevb"
	// BrandCevm - EVC Main (Annex N)
	BrandCevm = "cevm"
	// BrandClv1 - LCEVC Main@4.1, up to 7680x4320, 120 fps (Amd 1, Annex O)
	BrandClv1 = "clv1"
	// BrandCaac - AAC core: AAC-LC, HE-AAC or HE-AACv2, up to 2 channels, 48 kHz (10.4)
	BrandCaac = "caac"
	// BrandCaaa - AAC adaptive: caac with constraints for seamless switching (10.5)
	BrandCaaa = "caaa"
	// BrandCamc - AAC multichannel: AAC-LC or HE-AAC level 6, up to 8 channels (Annex I)
	BrandCamc = "camc"
	// BrandCama - AAC multichannel adaptive (Annex I)
	BrandCama = "cama"
	// BrandCmhs - MPEG-H 3D Audio LC profile, single stream mhm1 (Annex J)
	BrandCmhs = "cmhs"
	// BrandCmhm - MPEG-H 3D Audio LC profile, multi-stream mhm2 (Annex J)
	BrandCmhm = "cmhm"
	// BrandCmh1 - MPEG-H 3D Audio Baseline profile, single stream mhm1 (Annex J)
	BrandCmh1 = "cmh1"
	// BrandCmh2 - MPEG-H 3D Audio Baseline profile, multi-stream mhm2 (Annex J)
	BrandCmh2 = "cmh2"
	// BrandCasu - USAC stereo with MPEG-D DRC (Annex K)
	BrandCasu = "casu"
	// BrandCwvt - WebVTT subtitles (11.2)
	BrandCwvt = "cwvt"
	// BrandIm1t - TTML IMSC1 Text profile (11.3.3)
	BrandIm1t = "im1t"
	// BrandIm1i - TTML IMSC1 Image profile (11.3.4)
	BrandIm1i = "im1i"
	// BrandIm2t - TTML IMSC1.1 Text profile (L.2)
	BrandIm2t = "im2t"
	// BrandIm2i - TTML IMSC1.1 Image profile (L.3)
	BrandIm2i = "im2i"
	// BrandCcea - CTA-608/708 captions in video SEI, a supplemental data brand (11.4)
	BrandCcea = "ccea"
)

// DASH brands of ISO/IEC 23009-1 (6th edition).
const (
	// BrandDash - Indexed Self-Initializing Media Segment: an Init Segment and
	// one Indexed Media Segment in one file (ftyp). Used for the on-demand profile
	BrandDash = "dash"
	// BrandDsms - Self-Initializing Media Segment (ftyp)
	BrandDsms = "dsms"
	// BrandMsdh - Media Segment in the Simple format, with tfdt in every traf (styp)
	BrandMsdh = "msdh"
	// BrandMsix - Indexed Media Segment: a sidx before the first moof indexes the whole segment (styp)
	BrandMsix = "msix"
	// BrandSims - Sub-Indexed Media Segment: msix plus an ssix after each sidx (styp)
	BrandSims = "sims"
	// BrandLmsg - last Media Segment of a Representation. Shall not be present on any other segment (styp)
	BrandLmsg = "lmsg"
	// BrandMiss - major brand of a Media Segment that is not decodable, a missing segment (styp)
	BrandMiss = "miss"
	// BrandDums - Delivery Unit Media Segment, used as major brand by the broadcast profile (styp)
	BrandDums = "dums"
)

// Brands defined by codec-specific specifications.
const (
	// BrandAv01 - AV1 in ISOBMFF. Shall be a compatible brand when AV1 is present (AV1-ISOBMFF 2.1)
	BrandAv01 = "av01"
	// BrandIamf - Immersive Audio Model and Formats. Shall be a compatible brand when IAMF is present (IAMF 6.1)
	BrandIamf = "iamf"
	// BrandDby1 - Dolby Vision. Should be a compatible brand, never the major brand
	BrandDby1 = "dby1"
	// BrandCeac - CMAF media profile for AC-3 and E-AC-3 (ETSI TS 102 366 Annex J)
	BrandCeac = "ceac"
	// BrandCa4m - CMAF media profile for AC-4 main (ETSI TS 103 190-2 Annex H)
	BrandCa4m = "ca4m"
	// BrandCa4s - CMAF media profile for AC-4 single stream (ETSI TS 103 190-2 Annex H)
	BrandCa4s = "ca4s"
)
