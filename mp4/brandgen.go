package mp4

import (
	"fmt"
	"slices"
)

// isoBrandChain lists the structural brands of ISO/IEC 14496-12:2026 Annex E
// in order. Each brand requires support for everything the earlier ones do.
var isoBrandChain = []string{
	BrandIsom, BrandAvc1, BrandIso2, BrandIso3, BrandIso4, BrandIso5, BrandIso6,
	BrandIso7, BrandIso8, BrandIso9, BrandIsoa, BrandIsob, BrandIsoc, BrandIsod,
}

// Positions in isoBrandChain.
const (
	levelIsom = iota
	levelAvc1
	levelIso2
	levelIso3
	levelIso4
	levelIso5
	levelIso6
	levelIso7
	levelIso8
	levelIso9
	levelIsoa
	levelIsob
	levelIsoc
	levelIsod
)

// isoBrandLevelByType gives the lowest brand in isoBrandChain that supports a
// box or sample entry type whose mere presence matters (Annex E). Types whose
// version, flags or position matter are handled in boxBrandLevel.
var isoBrandLevelByType = map[string]int{
	"sdtp": levelAvc1,
	"sbgp": levelAvc1,
	"pdin": levelIso2,
	"subs": levelIso2,
	"sinf": levelIso2,
	"idat": levelIso4,
	"iref": levelIso4,
	"trgr": levelIso4,
	"resv": levelIso5,
	"saiz": levelIso6,
	"saio": levelIso6,
	"tfdt": levelIso6,
	"styp": levelIso6,
	"sidx": levelIso6,
	"ssix": levelIso6,
	"prft": levelIso6,
	"trep": levelIso7,
	"assp": levelIso7,
	"sthd": levelIso8,
	"elng": levelIso9,
	"iprp": levelIsoa,
	"grpl": levelIsoa,
	"ttyp": levelIsob,
	"brnd": levelIsob,
	"!mov": levelIsoc,
	"!mof": levelIsoc,
	"!six": levelIsoc,
	"!ssx": levelIsoc,
	"otyp": levelIsoc,
	"hdlp": levelIsod,
	"spki": levelIsod,
	"prsl": levelIsod,
}

// boxBrandLevel returns the lowest brand in isoBrandChain that supports b,
// which has a parent box of type parentType ("" at the top level). The box
// itself is considered, not its children.
func boxBrandLevel(b Box, parentType string) int {
	level := isoBrandLevelByType[b.Type()]
	switch box := b.(type) {
	case *CttsBox:
		if box.Version == 1 {
			level = max(level, levelIso4)
		}
	case *CslgBox:
		if box.Version == 0 {
			level = max(level, levelIso4)
		} else {
			level = max(level, levelIso9)
		}
	case *SgpdBox:
		switch {
		case box.Version >= 3:
			level = max(level, levelIsod)
		case parentType == "traf":
			level = max(level, levelIso6)
		default:
			level = max(level, levelAvc1)
		}
	case *MetaBox:
		if parentType == "moof" || parentType == "traf" {
			level = max(level, levelIso8)
		} else {
			level = max(level, levelIso2)
		}
	case *TrunBox:
		if box.Version == 1 {
			level = max(level, levelIso6)
		}
	case *TfhdBox:
		if box.Flags&TfhdDefaultBaseIsMoofFlag != 0 {
			level = max(level, levelIso5)
		}
	case *ElstBox:
		for _, e := range box.Entries {
			if e.MediaRateFraction != 0 || (e.MediaRateInteger != 0 && e.MediaRateInteger != 1) {
				level = max(level, levelIsob)
			}
		}
	}
	return level
}

// childBoxes returns the children of b, including the sample entries of stsd
// and the boxes inside visual and audio sample entries.
func childBoxes(b Box) []Box {
	switch box := b.(type) {
	case *StsdBox:
		return box.Children
	case *VisualSampleEntryBox:
		return box.Children
	case *AudioSampleEntryBox:
		return box.Children
	case ContainerBox:
		return box.GetChildren()
	}
	return nil
}

// walkBoxes calls visit for every box in the trees rooted at boxes, parents
// before their children. parent is nil for the boxes at the top.
func walkBoxes(boxes []Box, parent Box, visit func(b, parent Box)) {
	for _, b := range boxes {
		visit(b, parent)
		walkBoxes(childBoxes(b), b, visit)
	}
}

// isoBrandLevel returns the lowest brand in isoBrandChain that supports all
// boxes in the trees rooted at boxes.
func isoBrandLevel(boxes []Box) int {
	level := levelIsom
	nrSubs := map[Box]int{}
	walkBoxes(boxes, nil, func(b, parent Box) {
		parentType := ""
		if parent != nil {
			parentType = parent.Type()
		}
		level = max(level, boxBrandLevel(b, parentType))
		if b.Type() == "subs" {
			nrSubs[parent]++
			if nrSubs[parent] > 1 {
				level = max(level, levelIso8) // more than one subs per track
			}
		}
	})
	return level
}

// codecBrands returns the brands that codec specifications require or
// recommend in compatible_brands for the sample entries found in boxes:
// av01 (AV1-ISOBMFF), iamf (IAMF) and dby1 (Dolby Vision).
func codecBrands(boxes []Box) []string {
	var av1, iamf, dolbyVision bool
	walkBoxes(boxes, nil, func(b, _ Box) {
		switch b.Type() {
		case "av01":
			av1 = true
		case "iamf":
			iamf = true
		case "dvh1", "dvhe", "dav1", "dva1", "dvav", "dvcC", "dvvC", "dvwC":
			dolbyVision = true
		}
	})
	var brands []string
	if av1 {
		brands = append(brands, BrandAv01)
	}
	if iamf {
		brands = append(brands, BrandIamf)
	}
	if dolbyVision {
		brands = append(brands, BrandDby1)
	}
	return brands
}

// FragmentFeatures declares what the media segments of a track will contain.
// GenerateFtyp needs it since the ftyp of an init segment also covers media
// segments, which may not exist yet when the init segment is written.
type FragmentFeatures struct {
	DefaultBaseIsMoof bool // default-base-is-moof in tfhd (iso5)
	Tfdt              bool // tfdt (iso6)
	TrunV1            bool // trun version 1, with signed composition time offsets (iso6)
	Styp              bool // styp at the start of segments (iso6)
	Sidx              bool // sidx (iso6)
	Prft              bool // prft (iso6)
	SgpdInTraf        bool // sgpd in traf (iso6)
	SampleAuxInfo     bool // saiz and saio in traf, as for Common Encryption (iso6)
}

// DefaultFragmentFeatures returns the features of the media segments that
// mp4ff writes with NewMediaSegment and CreateFragment: styp, tfdt, trun
// version 1 and default-base-is-moof.
func DefaultFragmentFeatures() FragmentFeatures {
	return FragmentFeatures{
		DefaultBaseIsMoof: true,
		Tfdt:              true,
		TrunV1:            true,
		Styp:              true,
	}
}

// isoBrandLevel returns the lowest brand in isoBrandChain that supports f.
func (f FragmentFeatures) isoBrandLevel() int {
	switch {
	case f.Tfdt || f.TrunV1 || f.Styp || f.Sidx || f.Prft || f.SgpdInTraf || f.SampleAuxInfo:
		return levelIso6
	case f.DefaultBaseIsMoof:
		return levelIso5
	default:
		return levelIsom
	}
}

// FtypOptions declares what GenerateFtyp cannot find in the init segment.
type FtypOptions struct {
	// Fragments declares what the media segments will contain.
	// nil means DefaultFragmentFeatures().
	Fragments *FragmentFeatures
	// CMAF is BrandCmfc or BrandCmf2 to make the init segment a CMAF header,
	// or empty. The caller asserts that the track conforms to the brand.
	CMAF string
	// Extra lists more brands to add, such as CMAF media profile brands.
	Extra []string
}

// GenerateFtyp returns an ftyp box for the init segment.
//
// The compatible brands are the lowest brand of the ISO/IEC 14496-12:2026
// Annex E chain that supports the boxes of the init segment and the declared
// fragment features, the codec brands av01, iamf and dby1 when their sample
// entries are present, and opts.Extra. The 'avc1' brand of the chain is
// replaced by 'iso2', which includes it and is not mistaken for the AVC sample
// entry. A brand of the chain below the lowest one would be a false claim, so
// such brands are rejected in opts.Extra. This also keeps isom to iso4 out of
// files with default-base-is-moof, as 14496-12:2026 Section 8.8.7.1 requires.
//
// With opts.CMAF set, that brand is the major brand, with minor version 0,
// and cmf2 is accompanied by cmfc. Otherwise the major brand is mp42, since
// Annex E brands should not be the major brand. The major brand is repeated
// among the compatible brands, as 14496-12:2026 Section 5.2 recommends.
func (s *InitSegment) GenerateFtyp(opts FtypOptions) (*FtypBox, error) {
	if s.Moov == nil {
		return nil, fmt.Errorf("init segment has no moov box")
	}
	fragments := DefaultFragmentFeatures()
	if opts.Fragments != nil {
		fragments = *opts.Fragments
	}
	level := max(isoBrandLevel([]Box{s.Moov}), fragments.isoBrandLevel())
	isoBrand := isoBrandChain[level]
	if isoBrand == BrandAvc1 {
		isoBrand = BrandIso2
	}

	var brands []string
	switch opts.CMAF {
	case "":
		brands = append(brands, BrandMp42)
	case BrandCmfc, BrandCmf2:
		// ISO/IEC 23000-19:2024 Sections 7.3.1 and 7.5
		if len(s.Moov.Traks) != 1 {
			return nil, fmt.Errorf("a CMAF header has one track, not %d", len(s.Moov.Traks))
		}
		if level > levelIso9 {
			return nil, fmt.Errorf("CMAF allows the box versions and flags of %s, but %s is needed",
				BrandIso9, isoBrand)
		}
		if !fragments.DefaultBaseIsMoof || !fragments.Tfdt {
			return nil, fmt.Errorf("CMAF fragments need default-base-is-moof and tfdt")
		}
		brands = append(brands, opts.CMAF)
		if opts.CMAF == BrandCmf2 {
			brands = append(brands, BrandCmfc)
		}
	default:
		return nil, fmt.Errorf("unknown CMAF structural brand %q", opts.CMAF)
	}
	brands = append(brands, isoBrand)
	brands = append(brands, codecBrands([]Box{s.Moov})...)
	for _, b := range opts.Extra {
		if len(b) != 4 {
			return nil, fmt.Errorf("brand %q is not four characters", b)
		}
		if i := slices.Index(isoBrandChain, b); i >= 0 && i < level {
			return nil, fmt.Errorf("brand %s is too low for the boxes used, which need %s", b, isoBrand)
		}
		if !slices.Contains(brands, b) {
			brands = append(brands, b)
		}
	}
	return NewFtyp(brands[0], 0, brands), nil
}

// SetFtyp replaces the ftyp box of the init segment, or inserts it first if
// there is none.
func (s *InitSegment) SetFtyp(ftyp *FtypBox) {
	s.Ftyp = ftyp
	for i, c := range s.Children {
		if c.Type() == "ftyp" {
			s.Children[i] = ftyp
			return
		}
	}
	s.Children = append([]Box{ftyp}, s.Children...)
}

// StypOptions declares what GenerateStyp cannot find in the media segment.
type StypOptions struct {
	// CMAF adds the CMAF brands cmfs, cmff and cmfl, which a CMAF segment
	// starting with a CMAF fragment and chunk carries, and cmfr if the segment
	// starts with a sync sample. The caller asserts that the segment is a CMAF
	// segment.
	CMAF bool
	// Last adds lmsg, which DASH allows only in the last Media Segment of a
	// Representation.
	Last bool
}

// GenerateStyp returns an styp box with the brands that the media segment
// conforms to, or nil if there are none, since styp is optional.
//
// Besides the brands of opts, msdh is added if the segment has the Simple
// format of ISO/IEC 23009-1 Section 6.3.5.3, and msix if it has the Indexed
// format of Section 6.3.5.4. The first brand is the major brand and is
// repeated among the compatible brands.
func (s *MediaSegment) GenerateStyp(opts StypOptions) *StypBox {
	var brands []string
	if opts.CMAF {
		brands = append(brands, BrandCmfs, BrandCmff, BrandCmfl)
		if s.startsWithSyncSample() {
			brands = append(brands, BrandCmfr)
		}
	}
	du := s.isDeliveryUnitFormat()
	if du && s.allTrafsHaveTfdt() && (len(s.Sidxs) == 0 || s.firstSidxCoversSegment()) {
		brands = append(brands, BrandMsdh)
	}
	if du && s.moofsFollowedByMdat() && len(s.Sidxs) > 0 && s.firstSidxCoversSegment() {
		brands = append(brands, BrandMsix)
	}
	if opts.Last {
		brands = append(brands, BrandLmsg)
	}
	if len(brands) == 0 {
		return nil
	}
	return NewStyp(brands[0], 0, brands)
}

// isDeliveryUnitFormat reports whether the segment meets the checkable rules
// of the Delivery Unit Media Segment format (ISO/IEC 23009-1 Section 6.3.5.2):
// every fragment has a moof with at least one traf and an mdat, and uses
// movie-fragment relative addressing.
func (s *MediaSegment) isDeliveryUnitFormat() bool {
	if len(s.Fragments) == 0 {
		return false
	}
	for _, f := range s.Fragments {
		if f.Moof == nil || f.Mdat == nil || len(f.Moof.Trafs) == 0 {
			return false
		}
		for _, traf := range f.Moof.Trafs {
			tfhd := traf.Tfhd
			if tfhd == nil || tfhd.Flags&TfhdDefaultBaseIsMoofFlag == 0 || tfhd.HasBaseDataOffset() {
				return false
			}
			for _, trun := range traf.Truns {
				if !trun.HasDataOffset() {
					return false
				}
			}
		}
	}
	return true
}

// allTrafsHaveTfdt reports whether every traf of the segment has a tfdt.
func (s *MediaSegment) allTrafsHaveTfdt() bool {
	for _, f := range s.Fragments {
		if f.Moof == nil {
			return false
		}
		for _, traf := range f.Moof.Trafs {
			if traf.Tfdt == nil {
				return false
			}
		}
	}
	return true
}

// moofsFollowedByMdat reports whether each moof is immediately followed by an mdat.
func (s *MediaSegment) moofsFollowedByMdat() bool {
	for _, f := range s.Fragments {
		i := slices.IndexFunc(f.Children, func(b Box) bool { return b.Type() == "moof" })
		if i < 0 || i+1 >= len(f.Children) || f.Children[i+1].Type() != "mdat" {
			return false
		}
	}
	return true
}

// firstSidxCoversSegment reports whether the first sidx references all bytes
// that follow it in the segment. MediaSegment encodes its sidx boxes before
// its fragments.
func (s *MediaSegment) firstSidxCoversSegment() bool {
	sidx := s.Sidxs[0]
	referenced := sidx.FirstOffset
	for _, ref := range sidx.SidxRefs {
		referenced += uint64(ref.ReferencedSize)
	}
	var following uint64
	for _, other := range s.Sidxs[1:] {
		following += other.Size()
	}
	for _, f := range s.Fragments {
		following += f.Size()
	}
	return referenced == following
}

// startsWithSyncSample reports whether the first sample of the segment is
// known to be a sync sample. Flags that are only given by trex are unknown.
func (s *MediaSegment) startsWithSyncSample() bool {
	if len(s.Fragments) == 0 || s.Fragments[0].Moof == nil {
		return false
	}
	traf := s.Fragments[0].Moof.Traf
	if traf == nil || traf.Trun == nil || traf.Trun.SampleCount() == 0 {
		return false
	}
	trun := traf.Trun
	var flags uint32
	switch {
	case trun.HasFirstSampleFlags():
		flags, _ = trun.FirstSampleFlags()
	case trun.HasSampleFlags():
		flags = trun.Samples[0].Flags
	case traf.Tfhd != nil && traf.Tfhd.HasDefaultSampleFlags():
		flags = traf.Tfhd.DefaultSampleFlags
	default:
		return false
	}
	return !DecodeSampleFlags(flags).SampleIsNonSync
}
