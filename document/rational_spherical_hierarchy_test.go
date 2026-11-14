package document

import (
	"bytes"
	"context"
	"errors"
	"math/big"
	"os"
	"reflect"
	"testing"

	"plainsolid/exchange/step"
	"plainsolid/geom"
)

func TestRationalSphericalHierarchyDocument(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		filename string
		vendor   bool
	}{{"rational-spherical-hierarchy.step", false}, {"occt-rational-spherical-hierarchy.step", true}} {
		t.Run(tc.filename, func(t *testing.T) {
			raw, err := os.ReadFile("../exchange/step/testdata/" + tc.filename)
			if err != nil {
				t.Fatal(err)
			}
			readSTEP, readNative, otherNative := ReadRationalSphericalHierarchyDocumentSTEP, ReadRationalSphericalHierarchyDocumentNative, ReadOCCTRationalSphericalHierarchyDocumentNative
			if tc.vendor {
				readSTEP, readNative, otherNative = ReadOCCTRationalSphericalHierarchyDocumentSTEP, ReadOCCTRationalSphericalHierarchyDocumentNative, ReadRationalSphericalHierarchyDocumentNative
			}
			d, err := readSTEP(ctx, bytes.NewReader(raw), STEPAssemblyOptions{}, 1e-9)
			if err != nil {
				t.Fatal(err)
			}
			uses := d.Occurrences()
			if len(uses) != 7 || d.LengthUnit() != .001 {
				t.Fatal("proved material uses and units", uses, d.LengthUnit())
			}
			distinct := map[int64]int{}
			fraction := false
			for _, u := range uses {
				distinct[u.DefinitionID]++
				a, ok := d.Asset(u.DefinitionID)
				if !ok || a.SourceBodyID != u.SourceBodyID || a.SourceProductID != u.SourceProductID || a.Family != u.Family || a.DefinitionID != u.DefinitionID || !reflect.DeepEqual(a.NativeFaceSources, u.SourceFaceIDs) {
					t.Fatal("source ancestry and native geometry must agree", u, a)
				}
				if a.Family == "planar" && (a.Planar == nil || a.Trimmed != nil) || a.Family != "planar" && (a.Trimmed == nil || a.Planar != nil) {
					t.Fatal("asset geometry in wrong local representation", a)
				}
				for i := range u.World {
					for j := range u.World[i] {
						r := u.World[i][j]
						if _, exact := r.Float64(); !exact {
							fraction = true
						}
					}
				}
			}
			if !fraction || len(distinct) != 4 {
				t.Fatal("oblique rational poses and genuine sharing are required", fraction, distinct)
			}
			changed := d.Occurrences()
			changed[0].Path[0]++
			changed[0].SourceFaceIDs[0]++
			changed[0].World[0][0].SetInt64(25)
			a, _ := d.Asset(uses[0].DefinitionID)
			a.NativeFaceSources[0]++
			if !reflect.DeepEqual(uses, d.Occurrences()) {
				t.Fatal("caller mutation reached the document proof")
			}
			var persisted, staged bytes.Buffer
			if err := d.Write(&persisted); err != nil {
				t.Fatal(err)
			}
			if err := d.WriteSTEP(&staged); err != nil {
				t.Fatal(err)
			}
			native, err := readNative(ctx, bytes.NewReader(persisted.Bytes()), step.IndependentMaterialJSONLimits{}, STEPAssemblyOptions{})
			if err != nil || !reflect.DeepEqual(uses, native.Occurrences()) {
				t.Fatal("exact native source reload", native, err)
			}
			reparsed, err := readSTEP(ctx, bytes.NewReader(staged.Bytes()), STEPAssemblyOptions{}, 1e-9)
			if err != nil || !reflect.DeepEqual(uses, reparsed.Occurrences()) {
				t.Fatal("exact original STEP reread", reparsed, err)
			}
			var stable bytes.Buffer
			if err := native.Write(&stable); err != nil || !bytes.Equal(stable.Bytes(), persisted.Bytes()) {
				t.Fatal("native source-bound persistence must be byte stable", err)
			}
			if _, err := otherNative(ctx, bytes.NewReader(persisted.Bytes()), step.IndependentMaterialJSONLimits{}, STEPAssemblyOptions{}); err == nil {
				t.Fatal("unrelated transfer convention accepted")
			}
			if d, err := readSTEP(ctx, bytes.NewReader(raw), STEPAssemblyOptions{MaxNodes: 2}, 1e-9); d != nil || !errors.Is(err, geom.ErrConvergence) {
				t.Fatal("expanded node budget must fail atomically", d, err)
			}
			b, err := d.Bounds(ctx, step.RationalHierarchyBoundsOptions{})
			if err != nil || len(b.Leaves) != len(uses) {
				t.Fatal("proved document support bounds", b, err)
			}
			if !tc.vendor && b.Aggregate.Max()[0].Cmp(big.NewRat(162, 5)) != 0 {
				t.Fatal("rational document placed extent changed", b.Aggregate.Max())
			}
			mass, err := d.Mass(ctx, step.RationalHierarchyMassOptions{})
			if err != nil || len(mass.Occurrences) != 7 || len(mass.Products) != 4 {
				t.Fatal("symbolic per-occurrence mass", mass, err)
			}
			volume := mass.Sum.Volume().Coefficients()
			for i, expect := range []*big.Rat{big.NewRat(24, 1), big.NewRat(136, 3), big.NewRat(12, 1)} {
				if volume[i].Cmp(expect) != 0 {
					t.Fatal("exact assembly volume coefficient", i, volume, expect)
				}
			}
			origin := [3]*big.Rat{new(big.Rat), new(big.Rat), new(big.Rat)}
			dir := [3]*big.Rat{big.NewRat(1, 1), new(big.Rat), new(big.Rat)}
			locations, err := d.Locate(ctx, origin, step.RationalHierarchyPointOptions{})
			if err != nil || len(locations.Leaves) != 7 {
				t.Fatal("document world point query", locations, err)
			}
			hits, err := d.ClipLine(ctx, origin, dir, big.NewRat(-50, 1), big.NewRat(50, 1), step.RationalHierarchyLineOptions{})
			if err != nil || len(hits.Leaves) != 7 {
				t.Fatal("document world line query", hits, err)
			}
		})
	}
}
