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
			distances, err := d.SphereDistance(ctx, origin, step.RationalHierarchySphereDistanceOptions{})
			if err != nil || len(distances.Leaves) != len(uses) {
				t.Fatal("retained occurrence sphere distances", distances, err)
			}
			for i, u := range uses {
				if distances.Leaves[i].Supported != (u.Family == "sphere") || !reflect.DeepEqual(distances.Leaves[i].Path, u.Path) {
					t.Fatal("sphere distance changed existing original occurrence ownership", distances.Leaves[i], u)
				}
			}
		})
	}
}

func TestRationalSphericalHierarchyDocumentReentrantQueries(t *testing.T) {
	ctx := context.Background()
	original, err := os.Open("../exchange/step/testdata/rational-spherical-hierarchy-reentrant.step")
	if err != nil {
		t.Fatal(err)
	}
	defer original.Close()
	d, err := ReadRationalSphericalHierarchyDocumentSTEP(ctx, original, STEPAssemblyOptions{}, 1e-9)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		p            [3]*big.Rat
		want         step.AnalyticPointLocation
		faces, edges int
	}{{[3]*big.Rat{big.NewRat(-8, 1), big.NewRat(-2, 1), new(big.Rat)}, step.AnalyticPointOutside, 0, 0},
		{[3]*big.Rat{big.NewRat(-9, 1), big.NewRat(-3, 1), new(big.Rat)}, step.AnalyticPointBoundary, 2, 1}} {
		out, err := d.Locate(ctx, tc.p, step.RationalHierarchyPointOptions{})
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, p := range out.Leaves {
			if p.Family == "planar" {
				found = true
				if p.Location != tc.want || len(p.ContactFaceIDs) != tc.faces || len(p.ContactEdgeIDs) != tc.edges {
					t.Fatal("reentrant source ownership or exact point classification", p)
				}
			}
		}
		if !found {
			t.Fatal("missing reentrant planar occurrence")
		}
	}
	origin := [3]*big.Rat{big.NewRat(-11, 1), new(big.Rat), new(big.Rat)}
	direction := [3]*big.Rat{big.NewRat(2, 1), big.NewRat(-2, 1), new(big.Rat)}
	line, err := d.ClipGeneralLine(ctx, origin, direction, new(big.Rat), big.NewRat(6, 1), step.RationalHierarchyLineOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, leaf := range line.Leaves {
		if leaf.Owner.Family != "planar" {
			continue
		}
		if !leaf.Supported || len(leaf.Spans) != 2 || len(leaf.BoundarySpans) != 0 {
			t.Fatal("reentrant material intervals", leaf)
		}
		for i, endpoints := range [][2]*big.Rat{{big.NewRat(1, 2), big.NewRat(1, 1)}, {big.NewRat(3, 2), big.NewRat(2, 1)}} {
			for j, endpoint := range []*geom.ExactAlgebraicRoot{leaf.Spans[i].Lower, leaf.Spans[i].Upper} {
				want, _, err := geom.NewExactRationalRoot(ctx, endpoints[j], geom.ExactCurvePlaneEventOptions{})
				if err != nil {
					t.Fatal(err)
				}
				order, err := endpoint.Compare(ctx, want, geom.ExactCurvePlaneEventOptions{})
				if err != nil || !order.Proven || order.Order != 0 {
					t.Fatal("original line parameter", i, j, order, err)
				}
			}
		}
		origin[2].SetInt64(-2)
		cap, err := d.ClipCoplanarGeneralLine(ctx, origin, direction, new(big.Rat), big.NewRat(6, 1), step.RationalHierarchyLineOptions{})
		if err != nil {
			t.Fatal(err)
		}
		for _, capLeaf := range cap.Leaves {
			if capLeaf.Owner.SourceBodyID != leaf.Owner.SourceBodyID {
				continue
			}
			if len(capLeaf.BoundarySpans) != 2 || len(capLeaf.Spans) != 0 || len(capLeaf.BoundarySpans[0].SourceFaceIDs) != 1 || len(capLeaf.BoundarySpans[1].SourceFaceIDs) != 1 || capLeaf.BoundarySpans[0].SourceFaceIDs[0] != capLeaf.BoundarySpans[1].SourceFaceIDs[0] {
				t.Fatal("original reentrant cap ownership", capLeaf)
			}
			return
		}
		t.Fatal("missing reentrant coplanar line occurrence")
	}
	t.Fatal("missing reentrant planar line occurrence")
}
