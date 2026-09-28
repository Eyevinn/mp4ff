package mp4

import (
	"fmt"
	"slices"
	"strings"
)

// BrandSeverity tells how serious a BrandIssue is.
type BrandSeverity int

const (
	// BrandWarning means that a recommendation of a specification is not
	// followed, or that the content uses boxes that readers of a claimed
	// brand need not support.
	BrandWarning BrandSeverity = iota
	// BrandError means that the content contradicts a claimed brand, so
	// that readers of that brand would misinterpret it, or that a
	// requirement of a specification is not met.
	BrandError
)

// String returns "warning" or "error".
func (s BrandSeverity) String() string {
	if s == BrandError {
		return "error"
	}
	return "warning"
}

// BrandIssue is a problem with the brands of an ftyp or styp box.
type BrandIssue struct {
	Severity BrandSeverity
	Box      string // "ftyp" or "styp"
	Segment  int    // for styp, the number of the media segment, starting at 1
	Msg      string
}

// String returns the issue on one line.
func (i BrandIssue) String() string {
	where := i.Box
	if i.Box == "styp" {
		where = fmt.Sprintf("styp of segment %d", i.Segment)
	}
	return fmt.Sprintf("%s: %s: %s", i.Severity, where, i.Msg)
}

// CheckBrands checks the brands of the ftyp and styp boxes of f against the
// boxes that f contains. It returns nil if there is no issue.
//
// Both boxes are checked for:
//   - the major brand not being repeated among the compatible brands
//     (ISO/IEC 14496-12:2026 Section 5.2), or being an Annex E brand, which
//     should not be the major brand (E.1)
//   - claimed brands of the Annex E chain (isom, avc1, iso2 ... isod) that are
//     lower than what the boxes need. This is an error when readers of the
//     claimed brand would misinterpret a box (ctts or trun version 1,
//     default-base-is-moof, edit list media rates), and a warning when they
//     need not support it (such as sgpd, tfdt or sthd)
//
// The ftyp is also checked for saie, which saiz version 1 or 2 needs, for
// the codec brands av01 and iamf, which are required, and dby1, which is
// recommended, for the CMAF header rules when cmfc or cmf2 is claimed
// (ISO/IEC 23000-19:2024), and for a dash brand without any sidx. The boxes
// of the whole file count, so that the ftyp of an init segment followed by
// media segments covers the segments too.
//
// The styp of each media segment is also checked for msdh and msix, which
// need the Simple and Indexed formats of ISO/IEC 23009-1, lmsg, which only
// the last segment of f may have, and cmfr, which needs a sync sample first.
func (f *File) CheckBrands() []BrandIssue {
	var issues []BrandIssue
	if f.Ftyp != nil {
		issues = append(issues, f.checkFtypBrands()...)
	}
	for i, seg := range f.Segments {
		if seg.Styp != nil {
			issues = append(issues, seg.checkStypBrands(i+1, i == len(f.Segments)-1)...)
		}
	}
	return issues
}

// brandIssues collects the issues of one ftyp or styp box.
type brandIssues struct {
	box     string
	segment int
	list    []BrandIssue
}

func (b *brandIssues) add(severity BrandSeverity, format string, args ...any) {
	b.list = append(b.list, BrandIssue{severity, b.box, b.segment, fmt.Sprintf(format, args...)})
}

// claimedBrands returns the major brand and the compatible brands, without duplicates.
func claimedBrands(major string, compatible []string) []string {
	brands := []string{major}
	for _, b := range compatible {
		if !slices.Contains(brands, b) {
			brands = append(brands, b)
		}
	}
	return brands
}

// checkTypeBoxBrands does the checks that ftyp and styp share. needs are
// what the boxes covered by the type box need.
func (b *brandIssues) checkTypeBoxBrands(major string, compatible []string, needs []brandNeed) {
	if !slices.Contains(compatible, major) {
		b.add(BrandWarning, "major brand %s is not repeated among the compatible brands (ISO/IEC 14496-12 5.2)", major)
	}
	if slices.Contains(isoBrandChain, major) || major == BrandSaie {
		b.add(BrandWarning, "major brand %s is an ISO/IEC 14496-12 Annex E brand, which should not be the major brand (E.1)", major)
	}
	for _, brand := range claimedBrands(major, compatible) {
		claimed := slices.Index(isoBrandChain, brand)
		if claimed < 0 {
			continue
		}
		var misread, unsupported []string
		for _, n := range needs {
			if n.level <= claimed {
				continue
			}
			reason := fmt.Sprintf("%s (%s)", n.reason, isoBrandChain[n.level])
			if n.strict {
				misread = append(misread, reason)
			} else {
				unsupported = append(unsupported, reason)
			}
		}
		if len(misread) > 0 {
			b.add(BrandError, "%s is claimed, but %s readers would misinterpret %s (ISO/IEC 14496-12 Annex E)",
				brand, brand, strings.Join(misread, ", "))
		}
		if len(unsupported) > 0 {
			b.add(BrandWarning, "%s is claimed, but %s readers need not support %s (ISO/IEC 14496-12 Annex E)",
				brand, brand, strings.Join(unsupported, ", "))
		}
	}
}

func (f *File) checkFtypBrands() []BrandIssue {
	b := &brandIssues{box: "ftyp"}
	major, compatible := f.Ftyp.MajorBrand(), f.Ftyp.CompatibleBrands()
	claimed := claimedBrands(major, compatible)
	needs := isoBrandNeeds(f.Children)
	b.checkTypeBoxBrands(major, compatible, needs)

	if f.Moov != nil {
		for _, cb := range codecBrands([]Box{f.Moov}) {
			if slices.Contains(claimed, cb) {
				continue
			}
			switch cb {
			case BrandAv01:
				b.add(BrandError, "AV1 content needs the av01 brand (AV1-ISOBMFF 2.1)")
			case BrandIamf:
				b.add(BrandError, "IAMF content needs the iamf brand (IAMF 6.1)")
			case BrandDby1:
				b.add(BrandWarning, "Dolby Vision content should have the dby1 brand")
			}
		}
	}

	largeSaiz := usesLargeSaiz(f.Children)
	if largeSaiz && !slices.Contains(claimed, BrandSaie) {
		b.add(BrandError, "saiz version 1 or 2 needs the saie brand (ISO/IEC 14496-12 E.20)")
	}

	if slices.Contains(claimed, BrandCmfc) || slices.Contains(claimed, BrandCmf2) {
		f.checkCMAFHeaderBrands(b, major, needs, largeSaiz)
	}

	if slices.Contains(claimed, BrandDash) && !f.hasSidx() {
		b.add(BrandWarning, "dash declares an Indexed Self-Initializing Media Segment, "+
			"but there is no sidx (ISO/IEC 23009-1 6.3.6.2)")
	}
	return b.list
}

// checkCMAFHeaderBrands checks the rules of a CMAF header that follow from
// its boxes, for a file that claims cmfc or cmf2.
func (f *File) checkCMAFHeaderBrands(b *brandIssues, major string, needs []brandNeed, largeSaiz bool) {
	if (major == BrandCmfc || major == BrandCmf2) && f.Ftyp.MinorVersion() != 0 {
		b.add(BrandError, "minor version is %d, but shall be 0 with CMAF major brand %s (ISO/IEC 23000-19 7.2)",
			f.Ftyp.MinorVersion(), major)
	}
	if f.Moov != nil && len(f.Moov.Traks) != 1 {
		b.add(BrandError, "a CMAF header has exactly one track, not %d (ISO/IEC 23000-19 7.3.2.1)", len(f.Moov.Traks))
	}
	if largeSaiz {
		b.add(BrandError, "CMAF allows the box versions and flags of iso9, but saiz version 1 or 2 needs saie "+
			"(ISO/IEC 23000-19 7.3.1)")
	}
	for _, n := range needs {
		if n.level > levelIso9 {
			b.add(BrandError, "CMAF allows the box versions and flags of iso9, but %s needs %s (ISO/IEC 23000-19 7.3.1)",
				n.reason, isoBrandChain[n.level])
		}
	}
	for _, seg := range f.Segments {
		for _, frag := range seg.Fragments {
			if frag.Moof == nil {
				continue
			}
			for _, traf := range frag.Moof.Trafs {
				if traf.Tfdt == nil || traf.Tfhd == nil || traf.Tfhd.Flags&TfhdDefaultBaseIsMoofFlag == 0 {
					b.add(BrandError, "CMAF needs tfdt and default-base-is-moof in every traf (ISO/IEC 23000-19 7.5)")
					return
				}
			}
		}
	}
}

// hasSidx reports whether f has any sidx box.
func (f *File) hasSidx() bool {
	if len(f.Sidxs) > 0 {
		return true
	}
	for _, seg := range f.Segments {
		if len(seg.Sidxs) > 0 {
			return true
		}
	}
	return false
}

// checkStypBrands checks the styp of the media segment with number segNr,
// which is the last segment of the file if isLast.
func (s *MediaSegment) checkStypBrands(segNr int, isLast bool) []BrandIssue {
	b := &brandIssues{box: "styp", segment: segNr}
	major, compatible := s.Styp.MajorBrand(), s.Styp.CompatibleBrands()
	claimed := claimedBrands(major, compatible)
	boxes := []Box{s.Styp}
	for _, sidx := range s.Sidxs {
		boxes = append(boxes, sidx)
	}
	for _, frag := range s.Fragments {
		boxes = append(boxes, frag.Children...)
	}
	b.checkTypeBoxBrands(major, compatible, isoBrandNeeds(boxes))

	if slices.Contains(claimed, BrandMsdh) {
		if p := s.simpleFormatProblem(); p != "" {
			b.add(BrandError, "msdh is claimed, but %s (ISO/IEC 23009-1 6.3.5.3)", p)
		}
	}
	if slices.Contains(claimed, BrandMsix) {
		if p := s.indexedFormatProblem(); p != "" {
			b.add(BrandError, "msix is claimed, but %s (ISO/IEC 23009-1 6.3.5.4)", p)
		}
	}
	if slices.Contains(claimed, BrandLmsg) && !isLast {
		b.add(BrandError, "lmsg is claimed, but this is not the last segment (ISO/IEC 23009-1 7.3.1)")
	}
	if slices.Contains(claimed, BrandCmfr) {
		if sync, known := s.firstSampleIsSync(); known && !sync {
			b.add(BrandError, "cmfr is claimed, but the first sample is not a sync sample (ISO/IEC 23000-19 7.3.2.5)")
		}
	}
	return b.list
}
