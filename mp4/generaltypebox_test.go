package mp4_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"strings"
	"testing"

	"github.com/Eyevinn/mp4ff/mp4"
)

func TestStyp(t *testing.T) {
	styp := mp4.CreateStyp()
	boxDiffAfterEncodeAndDecode(t, styp)
}

// brandBox is the brand API that FtypBox and StypBox share.
type brandBox interface {
	mp4.Box
	MajorBrand() string
	MinorVersion() uint32
	CompatibleBrands() []string
	AddCompatibleBrands([]string)
	HasCompatibleBrand(string) bool
}

func TestGeneralTypeBoxBrands(t *testing.T) {
	ftyp := mp4.NewFtyp(mp4.BrandCmfc, 0, []string{mp4.BrandIso6})
	styp := mp4.NewStyp(mp4.BrandCmfs, 0, []string{mp4.BrandMsdh})
	boxes := []struct {
		box  brandBox
		copy func() brandBox
	}{
		{ftyp, func() brandBox { return ftyp.Copy() }},
		{styp, func() brandBox { return styp.Copy() }},
	}
	for _, b := range boxes {
		t.Run(b.box.Type(), func(t *testing.T) {
			box := b.box
			major := box.MajorBrand()
			if box.HasCompatibleBrand(major) {
				t.Errorf("major brand %s reported as compatible although not listed", major)
			}
			if box.HasCompatibleBrand(mp4.BrandLmsg) {
				t.Errorf("lmsg reported before it was added")
			}
			if box.HasCompatibleBrand("lms") {
				t.Errorf("3-character brand reported as present")
			}
			cp := b.copy()
			box.AddCompatibleBrands([]string{mp4.BrandLmsg})
			if !box.HasCompatibleBrand(mp4.BrandLmsg) {
				t.Errorf("lmsg not reported after it was added")
			}
			if cp.HasCompatibleBrand(mp4.BrandLmsg) {
				t.Errorf("copy changed when the original was modified")
			}
			if got := len(box.CompatibleBrands()); got != 2 {
				t.Errorf("got %d compatible brands, want 2", got)
			}
			if box.Size() != cp.Size()+4 {
				t.Errorf("size %d, want %d", box.Size(), cp.Size()+4)
			}
			boxDiffAfterEncodeAndDecode(t, box)
		})
	}
}

// TestBrandConstants checks that every constant in brands.go holds a
// four-character brand, that no brand is defined twice, and that the name
// follows the pattern Brand + brand with the first letter in upper case.
func TestBrandConstants(t *testing.T) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "brands.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]string{}
	nrConsts := 0
	for _, decl := range f.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.CONST {
			continue
		}
		for _, spec := range gd.Specs {
			vs := spec.(*ast.ValueSpec)
			for i, name := range vs.Names {
				lit, ok := vs.Values[i].(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					t.Errorf("%s: value is not a string literal", name.Name)
					continue
				}
				brand, err := strconv.Unquote(lit.Value)
				if err != nil {
					t.Fatal(err)
				}
				nrConsts++
				if len(brand) != 4 {
					t.Errorf("%s = %q: brand is not four characters", name.Name, brand)
					continue
				}
				wantName := "Brand" + strings.ToUpper(brand[:1]) + brand[1:]
				if name.Name != wantName {
					t.Errorf("%s = %q: want name %s", name.Name, brand, wantName)
				}
				if prev, ok := seen[brand]; ok {
					t.Errorf("%s and %s both define %q", prev, name.Name, brand)
				}
				seen[brand] = name.Name
			}
		}
	}
	if nrConsts == 0 {
		t.Fatal("no brand constants found")
	}
	// Spot-check that the parsed file is the one the package compiles.
	for _, b := range []string{mp4.BrandIsom, mp4.BrandIso6, mp4.BrandMp42, mp4.BrandCmfc, mp4.BrandLmsg} {
		if _, ok := seen[b]; !ok {
			t.Errorf("brand %q not found in brands.go", b)
		}
	}
}
