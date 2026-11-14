package trimmed_test

import (
	"context"
	"math"
	"math/big"
	"testing"

	"plainsolid/brep"
	"plainsolid/brep/trimmed"
	"plainsolid/csg/poly"
	"plainsolid/geom"
	"plainsolid/intersect"
)

func nativeArrangementPatch(t *testing.T) intersect.Patch {
	t.Helper()
	s, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{}, {Y: 1}, {X: 1}, {X: 1, Y: 1}}, []float64{0, 0, 1, 1}, []float64{0, 0, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	return intersect.Patch{Surface: s, Max: geom.Vec2{X: 1, Y: 1}}
}
func nativeArrangementBox(lo, hi *big.Rat, hole bool) [][2]*big.Rat {
	r := [][2]*big.Rat{{lo, lo}, {hi, lo}, {hi, hi}, {lo, hi}}
	if hole {
		r[1], r[3] = r[3], r[1]
	}
	return r
}
func nativeArrangement(t *testing.T, op intersect.AffineRingOperation, holes bool) *intersect.AffineGeneralPolygonArrangement {
	t.Helper()
	a := nativeArrangementPatch(t)
	trims := [2]intersect.AffinePolygonTrim{{Outer: nativeArrangementBox(big.NewRat(0, 1), big.NewRat(3, 4), false)}, {Outer: nativeArrangementBox(big.NewRat(1, 4), big.NewRat(1, 1), false)}}
	if holes {
		trims[0].Holes = [][][2]*big.Rat{nativeArrangementBox(big.NewRat(1, 16), big.NewRat(1, 8), true)}
		trims[1].Holes = [][][2]*big.Rat{nativeArrangementBox(big.NewRat(1, 2), big.NewRat(5, 8), true)}
	}
	q, e := intersect.ArrangeGeneralAffinePolygons(context.Background(), a, a, trims, op, geom.SurfacePatchOptions{}, intersect.AffinePolygonContactOptions{})
	if e != nil {
		t.Fatal(e)
	}
	return q
}
func TestNativeAffineArrangementExtrusionOperations(t *testing.T) {
	a, _ := brep.Box(geom.Vec3{}, geom.Vec3{X: .75, Y: .75, Z: 2})
	b, _ := brep.Box(geom.Vec3{X: .25, Y: .25}, geom.Vec3{X: 1, Y: 1, Z: 2})
	for _, tc := range []struct {
		op     intersect.AffineRingOperation
		poly   poly.Operation
		volume *big.Rat
	}{{intersect.AffineRingIntersection, poly.Intersection, big.NewRat(1, 2)}, {intersect.AffineRingUnion, poly.Union, big.NewRat(7, 4)}, {intersect.AffineRingDifference, poly.Difference, big.NewRat(5, 8)}} {
		reference, err := poly.Combine(context.Background(), a, b, tc.poly, poly.Options{RequireExactConversion: true})
		if err != nil {
			t.Fatal(err)
		}
		referenceMass, err := reference.MassProperties()
		if err != nil {
			t.Fatal(err)
		}
		arr := nativeArrangement(t, tc.op, false)
		q, e := trimmed.ExtrudeAffineArrangement(context.Background(), arr, geom.Vec3{Z: 2}, trimmed.AffineArrangementPrismOptions{})
		if e != nil {
			t.Fatal(e)
		}
		if q.ExactVolume().Cmp(tc.volume) != 0 || len(q.Components()) != 1 {
			t.Fatal(q.ExactVolume(), tc.volume)
		}
		s := q.Components()[0]
		if e := s.Validate(); e != nil {
			t.Fatal(e)
		}
		mass, e := s.MassProperties()
		want, _ := tc.volume.Float64()
		if e != nil || math.Abs(mass.Volume-want) > 1e-12 || math.Abs(mass.Volume-referenceMass.Volume) > 1e-12 {
			t.Fatal(mass, referenceMass, e)
		}
		restored, e := brep.NewSolid(s.Vertices(), s.Edges(), s.Faces())
		if e != nil {
			t.Fatal(e)
		}
		again, _ := restored.MassProperties()
		if again.Volume != mass.Volume {
			t.Fatal("native reconstruction changed mass")
		}
		seen := make([]bool, len(arr.Area().Edges()))
		for _, wall := range q.Walls() {
			if wall.Component != 0 || wall.Face < 2 || len(wall.Sources) == 0 {
				t.Fatal(wall)
			}
			for _, ei := range wall.ArrangementEdges {
				if seen[ei] {
					t.Fatal("duplicate original piece")
				}
				seen[ei] = true
			}
		}
		for _, present := range seen {
			if !present {
				t.Fatal("lost original piece")
			}
		}
	}
}
func TestNativeAffineArrangementHoledDisconnectedAndSigned(t *testing.T) {
	arr := nativeArrangement(t, intersect.AffineRingDifference, true)
	for _, z := range []float64{2, -2} {
		q, e := trimmed.ExtrudeAffineArrangement(context.Background(), arr, geom.Vec3{Z: z}, trimmed.AffineArrangementPrismOptions{})
		if e != nil {
			t.Fatal(e)
		}
		if len(q.Components()) != 2 || q.ExactVolume().Cmp(big.NewRat(83, 128)) != 0 {
			t.Fatal(q.Components(), q.ExactVolume())
		}
		volume := 0.
		holeCount := 0
		for _, s := range q.Components() {
			if e := s.Validate(); e != nil {
				t.Fatal(e)
			}
			m, e := s.MassProperties()
			if e != nil {
				t.Fatal(e)
			}
			volume += m.Volume
			holeCount += len(s.Faces()[0].Holes)
		}
		if holeCount != 1 || math.Abs(volume-83./128) > 1e-12 {
			t.Fatal(holeCount, volume)
		}
		for _, w := range q.Walls() {
			if w.Reversed != (z < 0) {
				t.Fatal(w)
			}
		}
		for _, cap := range []int{q.ExactOperations(), q.ExactOperations() - 1} {
			out, e := trimmed.ExtrudeAffineArrangement(context.Background(), arr, geom.Vec3{Z: z}, trimmed.AffineArrangementPrismOptions{MaxOperations: cap})
			if cap == q.ExactOperations() {
				if e != nil || out == nil {
					t.Fatal(e)
				}
			} else if e == nil || out != nil {
				t.Fatal("partial native output")
			}
		}
		walls := q.Walls()
		walls[0].Sources[0].Parameter[0].SetInt64(99)
		if q.Walls()[0].Sources[0].Parameter[0].Cmp(big.NewRat(99, 1)) == 0 {
			t.Fatal("mutable ancestry")
		}
	}
	for _, o := range []trimmed.AffineArrangementPrismOptions{{MaxComponents: 1}, {MaxVertices: 6}, {MaxCoefficientBits: 63}} {
		if q, e := trimmed.ExtrudeAffineArrangement(context.Background(), arr, geom.Vec3{Z: 2}, o); e == nil || q != nil {
			t.Fatal(o)
		}
	}
	if q, e := trimmed.ExtrudeAffineArrangement(context.Background(), arr, geom.Vec3{X: 1}, trimmed.AffineArrangementPrismOptions{}); e == nil || q != nil {
		t.Fatal("planar extrusion accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if q, e := trimmed.ExtrudeAffineArrangement(ctx, arr, geom.Vec3{Z: 2}, trimmed.AffineArrangementPrismOptions{}); e != context.Canceled || q != nil {
		t.Fatal(q, e)
	}
}

func TestNativeAffineArrangementCollinearAndRepresentation(t *testing.T) {
	a := nativeArrangementPatch(t)
	r := func(x0, x1 *big.Rat) [][2]*big.Rat {
		return [][2]*big.Rat{{x0, big.NewRat(0, 1)}, {x1, big.NewRat(0, 1)}, {x1, big.NewRat(1, 1)}, {x0, big.NewRat(1, 1)}}
	}
	trims := [2]intersect.AffinePolygonTrim{{Outer: r(big.NewRat(0, 1), big.NewRat(1, 2))}, {Outer: r(big.NewRat(1, 2), big.NewRat(1, 1))}}
	arr, e := intersect.ArrangeGeneralAffinePolygons(context.Background(), a, a, trims, intersect.AffineRingUnion, geom.SurfacePatchOptions{}, intersect.AffinePolygonContactOptions{})
	if e != nil {
		t.Fatal(e)
	}
	q, e := trimmed.ExtrudeAffineArrangement(context.Background(), arr, geom.Vec3{Z: 1}, trimmed.AffineArrangementPrismOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if len(q.Components()) != 1 || len(q.Components()[0].Faces()) != 6 || q.ExactVolume().Cmp(big.NewRat(1, 1)) != 0 {
		t.Fatal(q)
	}
	merged := 0
	for _, wall := range q.Walls() {
		if len(wall.ArrangementEdges) > 1 {
			merged++
			if len(wall.Sources) < 2 {
				t.Fatal("lost collinear use", wall)
			}
		}
	}
	if merged != 2 {
		t.Fatal("missing exact straight-run simplification", merged)
	}
	if q, e := trimmed.ExtrudeAffineArrangement(context.Background(), arr, geom.Vec3{X: math.SmallestNonzeroFloat64, Z: 1}, trimmed.AffineArrangementPrismOptions{}); e == nil || q != nil {
		t.Fatal("rounded top vertex accepted")
	}
	trims[1].Outer = r(big.NewRat(1, 3), big.NewRat(2, 3))
	arr, e = intersect.ArrangeGeneralAffinePolygons(context.Background(), a, a, trims, intersect.AffineRingIntersection, geom.SurfacePatchOptions{}, intersect.AffinePolygonContactOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if q, e := trimmed.ExtrudeAffineArrangement(context.Background(), arr, geom.Vec3{Z: 1}, trimmed.AffineArrangementPrismOptions{}); e == nil || q != nil {
		t.Fatal("nonbinary native vertex accepted")
	}
}

func TestNativeAffineArrangementObliquePlane(t *testing.T) {
	s, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{}, {Y: 1, Z: 2}, {X: 1, Z: 1}, {X: 1, Y: 1, Z: 3}}, []float64{0, 0, 1, 1}, []float64{0, 0, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	a := intersect.Patch{Surface: s, Max: geom.Vec2{X: 1, Y: 1}}
	trim := intersect.AffinePolygonTrim{Outer: nativeArrangementBox(big.NewRat(0, 1), big.NewRat(1, 1), false)}
	arr, e := intersect.ArrangeGeneralAffinePolygons(context.Background(), a, a, [2]intersect.AffinePolygonTrim{trim, trim}, intersect.AffineRingUnion, geom.SurfacePatchOptions{}, intersect.AffinePolygonContactOptions{})
	if e != nil {
		t.Fatal(e)
	}
	q, e := trimmed.ExtrudeAffineArrangement(context.Background(), arr, geom.Vec3{X: 1}, trimmed.AffineArrangementPrismOptions{})
	if e != nil {
		t.Fatal(e)
	}
	m, e := q.Components()[0].MassProperties()
	if e != nil || math.Abs(m.Volume-1) > 1e-12 || q.ExactVolume().Cmp(big.NewRat(1, 1)) != 0 {
		t.Fatal(m, q.ExactVolume(), e)
	}
	for _, wall := range q.Walls() {
		if !wall.Reversed || len(wall.Sources) != 2 {
			t.Fatal("signed oblique source ownership", wall)
		}
	}
}
