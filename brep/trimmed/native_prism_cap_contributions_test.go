package trimmed_test

import (
	"context"
	"math/big"
	"plainsolid/brep"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"plainsolid/intersect"
	"testing"
)

type capOriginalOwner interface {
	OriginalOwner(int) (brep.Solid, error)
	OriginalCaps() [2][2]int
}

func checkCapSourceEndpoints(t *testing.T, owner capOriginalOwner, report *trimmed.PrismCapContributions) {
	t.Helper()
	for _, r := range report.Regions() {
		for _, loop := range r.Loops {
			for _, piece := range loop {
				for _, source := range piece.Sources {
					s, e := owner.OriginalOwner(source.Operand)
					if e != nil {
						t.Fatal(e)
					}
					edges, vertices := s.Edges(), s.Vertices()
					for layer, ei := range []int{source.LowerEdge, source.UpperEdge} {
						capID := owner.OriginalCaps()[source.Operand][layer]
						face := s.Faces()[capID]
						loops := append([][]brep.EdgeUse{face.Boundary}, face.Holes...)
						ring := source.LowerRing
						if layer == 1 {
							ring = source.UpperRing
						}
						if ring < 0 || ring >= len(loops) {
							t.Fatal("original ring index", source)
						}
						found := false
						for _, u := range loops[ring] {
							if u.Edge == ei {
								found = true
							}
						}
						if !found {
							t.Fatal("original ring/edge mismatch", source)
						}

						edge := edges[ei]
						a, b := vertices[edge.Start], vertices[edge.End]
						av, bv := [3]float64{a.X, a.Y, a.Z}, [3]float64{b.X, b.Y, b.Z}
						parameters := source.LowerParameter
						points := piece.Base
						if layer == 1 {
							parameters = source.UpperParameter
							points = piece.Top
						}
						for end := 0; end < 2; end++ {
							for axis := 0; axis < 3; axis++ {
								want := new(big.Rat).Add(new(big.Rat).SetFloat64(av[axis]), new(big.Rat).Mul(parameters[end], new(big.Rat).Sub(new(big.Rat).SetFloat64(bv[axis]), new(big.Rat).SetFloat64(av[axis]))))
								if want.Cmp(points[end][axis]) != 0 {
									t.Fatal("original cap edge mismatch", r.Component, source, layer, end, axis, want, points[end][axis])
								}
							}
						}
					}
				}
			}
		}
	}
}
func TestNativePrismCapContributions(t *testing.T) {
	a, _ := brep.Box(geom.Vec3{}, geom.Vec3{X: .75, Y: .75, Z: 2})
	b, _ := brep.Box(geom.Vec3{X: .25, Y: .25}, geom.Vec3{X: 1, Y: 1, Z: 2})
	for _, op := range []intersect.AffineRingOperation{intersect.AffineRingIntersection, intersect.AffineRingUnion, intersect.AffineRingDifference} {
		s, e := trimmed.CombineMatchingNativePrisms(context.Background(), a, b, op, trimmed.NativePrismBooleanOptions{})
		if e != nil {
			t.Fatal(e)
		}
		report, e := s.CapContributions(context.Background(), geom.SurfacePatchOptions{}, intersect.AffinePolygonContactOptions{})
		if e != nil {
			t.Fatal(e)
		}
		areas := [4]*big.Rat{new(big.Rat), new(big.Rat), new(big.Rat), new(big.Rat)}
		for _, r := range report.Regions() {
			areas[r.Owners].Add(areas[r.Owners], r.ProjectedArea)
			if r.Component != 0 || r.Faces != [2]int{0, 1} {
				t.Fatal(r)
			}
			for i := 0; i < 2; i++ {
				if r.Owners&(1<<i) == 0 {
					if r.OriginalCaps[i] != [2]int{-1, -1} {
						t.Fatal("tool boundary promoted to material owner")
					}
				} else if r.OriginalCaps[i] != s.OriginalCaps()[i] {
					t.Fatal(r)
				}
			}
		}
		wantA, wantB, wantShared := new(big.Rat), new(big.Rat), new(big.Rat)
		switch op {
		case intersect.AffineRingIntersection:
			wantShared.SetFrac64(1, 4)
		case intersect.AffineRingUnion:
			wantA.SetFrac64(5, 16)
			wantB.SetFrac64(5, 16)
			wantShared.SetFrac64(1, 4)
		case intersect.AffineRingDifference:
			wantA.SetFrac64(5, 16)
		}
		if areas[1].Cmp(wantA) != 0 || areas[2].Cmp(wantB) != 0 || areas[3].Cmp(wantShared) != 0 || report.AreaScaleSquared().Cmp(big.NewRat(1, 1)) != 0 {
			t.Fatal(areas)
		}
		checkCapSourceEndpoints(t, s, report)
		if op == intersect.AffineRingUnion {
			for _, cap := range []int{report.ExactOperations(), report.ExactOperations() - 1} {
				out, e := s.CapContributions(context.Background(), geom.SurfacePatchOptions{}, intersect.AffinePolygonContactOptions{MaxOperations: cap})
				if cap == report.ExactOperations() {
					if e != nil || out == nil {
						t.Fatal(e)
					}
				} else if e == nil || out != nil {
					t.Fatal("partial cap report")
				}
			}
			regions := report.Regions()
			regions[0].ProjectedArea.SetInt64(99)
			regions[0].Loops[0][0].Base[0][0].SetInt64(99)
			if report.Regions()[0].ProjectedArea.Cmp(big.NewRat(99, 1)) == 0 || report.Regions()[0].Loops[0][0].Base[0][0].Cmp(big.NewRat(99, 1)) == 0 {
				t.Fatal("mutable cap report")
			}
		}
	}
}
func TestNativePrismCapContributionsHolesAndOblique(t *testing.T) {
	polygon := func(lo, hi float64) []geom.Vec3 {
		return []geom.Vec3{{X: lo, Y: lo}, {X: hi, Y: lo}, {X: hi, Y: hi}, {X: lo, Y: hi}}
	}
	a, e := brep.PrismWithHoles(polygon(0, .75), [][]geom.Vec3{polygon(1./16, 1./8)}, geom.Vec3{Z: 2})
	if e != nil {
		t.Fatal(e)
	}
	b, e := brep.PrismWithHoles(polygon(.25, 1), [][]geom.Vec3{polygon(.5, .625)}, geom.Vec3{Z: 2})
	if e != nil {
		t.Fatal(e)
	}
	s, e := trimmed.CombineMatchingNativePrisms(context.Background(), a, b, intersect.AffineRingDifference, trimmed.NativePrismBooleanOptions{})
	if e != nil {
		t.Fatal(e)
	}
	report, e := s.CapContributions(context.Background(), geom.SurfacePatchOptions{}, intersect.AffinePolygonContactOptions{})
	if e != nil {
		t.Fatal(e)
	}
	regions := report.Regions()
	if len(regions) != 2 || regions[0].Component == regions[1].Component {
		t.Fatal(regions)
	}
	sum := new(big.Rat)
	toolBoundary := false
	for _, r := range regions {
		if r.Owners != 1 || r.OriginalCaps[1] != [2]int{-1, -1} {
			t.Fatal(r)
		}
		sum.Add(sum, r.ProjectedArea)
		for _, loop := range r.Loops {
			for _, piece := range loop {
				for _, use := range piece.Sources {
					if use.Operand == 1 {
						toolBoundary = true
					}
				}
			}
		}
	}
	if !toolBoundary || sum.Cmp(big.NewRat(83, 256)) != 0 {
		t.Fatal(toolBoundary, sum)
	}
	checkCapSourceEndpoints(t, s, report)
	// An oblique unit-determinant affine placement scales cap area by3/2.
	pose := geom.Affine{{1, 0, .5, 3}, {0, 1, .25, -2}, {1, .5, 1.625, 4}}
	a, e = a.Transformed(pose)
	if e != nil {
		t.Fatal(e)
	}
	b, e = b.Transformed(pose)
	if e != nil {
		t.Fatal(e)
	}
	oblique, e := trimmed.CombineAffineNativePrisms(context.Background(), a, b, intersect.AffineRingIntersection, trimmed.AffineNativePrismOptions{})
	if e != nil {
		t.Fatal(e)
	}
	r, e := oblique.CapContributions(context.Background(), geom.SurfacePatchOptions{}, intersect.AffinePolygonContactOptions{})
	if e != nil {
		t.Fatal(e)
	}
	area := new(big.Rat)
	for _, region := range r.Regions() {
		if region.Owners != 3 {
			t.Fatal(region)
		}
		area.Add(area, region.ProjectedArea)
	}
	area.Mul(area, area)
	area.Mul(area, r.AreaScaleSquared())
	if area.Cmp(big.NewRat(2025, 16384)) != 0 {
		t.Fatal("physical cap area squared", area)
	}
	checkCapSourceEndpoints(t, oblique, r)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if out, e := s.CapContributions(ctx, geom.SurfacePatchOptions{}, intersect.AffinePolygonContactOptions{}); e != context.Canceled || out != nil {
		t.Fatal(out, e)
	}
}

func TestNativeCapContributionIndependentRingOrder(t *testing.T) {
	polygon := func(lo, hi float64) []geom.Vec3 {
		return []geom.Vec3{{X: lo, Y: lo}, {X: hi, Y: lo}, {X: hi, Y: hi}, {X: lo, Y: hi}}
	}
	a, e := brep.PrismWithHoles(polygon(0, 1), [][]geom.Vec3{polygon(.125, .25), polygon(.625, .75)}, geom.Vec3{Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	faces := a.Faces()
	faces[0].Holes[0], faces[0].Holes[1] = faces[0].Holes[1], faces[0].Holes[0]
	a, e = brep.NewSolid(a.Vertices(), a.Edges(), faces)
	if e != nil {
		t.Fatal(e)
	}
	b, e := brep.Prism(polygon(.375, .5), geom.Vec3{Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	s, e := trimmed.CombineMatchingNativePrisms(context.Background(), a, b, intersect.AffineRingUnion, trimmed.NativePrismBooleanOptions{})
	if e != nil {
		t.Fatal(e)
	}
	r, e := s.CapContributions(context.Background(), geom.SurfacePatchOptions{}, intersect.AffinePolygonContactOptions{})
	if e != nil {
		t.Fatal(e)
	}
	checkCapSourceEndpoints(t, s, r)
	different := false
	for _, region := range r.Regions() {
		for _, loop := range region.Loops {
			for _, piece := range loop {
				for _, use := range piece.Sources {
					if use.Operand == 0 && use.UpperRing > 0 && use.LowerRing != use.UpperRing {
						different = true
					}
				}
			}
		}
	}
	if !different {
		t.Fatal("lower cap hole ordering was inferred from upper cap")
	}
}
