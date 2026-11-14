package trimmed_test

import (
	"context"
	"math"
	"math/big"
	"plainsolid/brep"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"plainsolid/intersect"
	"testing"
)

func TestCombineAuthenticatedNativePrisms(t *testing.T) {
	a, _ := brep.Box(geom.Vec3{}, geom.Vec3{X: .75, Y: .75, Z: 2})
	b, _ := brep.Box(geom.Vec3{X: .25, Y: .25}, geom.Vec3{X: 1, Y: 1, Z: 2})
	for _, tc := range []struct {
		op     intersect.AffineRingOperation
		volume *big.Rat
	}{{intersect.AffineRingIntersection, big.NewRat(1, 2)}, {intersect.AffineRingUnion, big.NewRat(7, 4)}, {intersect.AffineRingDifference, big.NewRat(5, 8)}} {
		q, e := trimmed.CombineMatchingNativePrisms(context.Background(), a, b, tc.op, trimmed.NativePrismBooleanOptions{})
		if e != nil {
			t.Fatal(e)
		}
		if q.Axis() != 2 || q.Material().ExactVolume().Cmp(tc.volume) != 0 {
			t.Fatal(q.Axis(), q.Material().ExactVolume())
		}
		for _, s := range q.Material().Components() {
			if e := s.Validate(); e != nil {
				t.Fatal(e)
			}
		}
		vertices := q.Material().Arrangement().Area().Vertices()
		for _, u := range q.WallUses() {
			owner, e := q.OriginalOwner(u.Operand)
			if e != nil {
				t.Fatal(e)
			}
			edges, points, faces := owner.Edges(), owner.Vertices(), owner.Faces()
			if u.Wall < 0 || u.Wall >= len(faces) || u.UpperCap < 0 || u.LowerCap < 0 {
				t.Fatal(u)
			}
			for k := 0; k < 2; k++ {
				for _, pair := range []struct {
					edge      int
					parameter *big.Rat
					z         float64
				}{{u.LowerEdge, u.LowerParameter[k], 0}, {u.UpperEdge, u.UpperParameter[k], 2}} {
					edge := edges[pair.edge]
					p0, p1 := points[edge.Start], points[edge.End]
					x := new(big.Rat).Add(new(big.Rat).SetFloat64(p0.X), new(big.Rat).Mul(pair.parameter, new(big.Rat).Sub(new(big.Rat).SetFloat64(p1.X), new(big.Rat).SetFloat64(p0.X))))
					y := new(big.Rat).Add(new(big.Rat).SetFloat64(p0.Y), new(big.Rat).Mul(pair.parameter, new(big.Rat).Sub(new(big.Rat).SetFloat64(p1.Y), new(big.Rat).SetFloat64(p0.Y))))
					found := false
					for _, v := range vertices {
						if v.Point[0].Cmp(x) == 0 && v.Point[1].Cmp(y) == 0 {
							found = true
						}
					}
					if !found || p0.Z != pair.z || p1.Z != pair.z {
						t.Fatal("original edge provenance", u, x, y)
					}
				}
			}
		}
	}
}
func TestCombineNativePrismHolesAndAuthentication(t *testing.T) {
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
	q, e := trimmed.CombineMatchingNativePrisms(context.Background(), a, b, intersect.AffineRingDifference, trimmed.NativePrismBooleanOptions{})
	if e != nil || len(q.Material().Components()) != 2 || q.Material().ExactVolume().Cmp(big.NewRat(83, 128)) != 0 {
		t.Fatal(q, e)
	}
	for _, pose := range []geom.Affine{{{0, 0, 1, 3}, {1, 0, 0, -2}, {0, 1, 0, 1}}, {{1, 0, 0, 4}, {0, 0, -1, 5}, {0, 1, 0, -3}}} {
		pa, e := a.Transformed(pose)
		if e != nil {
			t.Fatal(e)
		}
		pb, e := b.Transformed(pose)
		if e != nil {
			t.Fatal(e)
		}
		r, e := trimmed.CombineMatchingNativePrisms(context.Background(), pa, pb, intersect.AffineRingDifference, trimmed.NativePrismBooleanOptions{})
		if e != nil {
			t.Fatal(e)
		}
		if r.Material().ExactVolume().Cmp(big.NewRat(83, 128)) != 0 {
			t.Fatal(r.Material().ExactVolume())
		}
		v := 0.
		for _, s := range r.Material().Components() {
			mass, e := s.MassProperties()
			if e != nil {
				t.Fatal(e)
			}
			v += mass.Volume
		}
		if math.Abs(v-83./128) > 1e-11 {
			t.Fatal(v)
		}
	}
	shifted, _ := brep.Box(geom.Vec3{X: .25, Y: .25, Z: .5}, geom.Vec3{X: 1, Y: 1, Z: 2.5})
	if r, e := trimmed.CombineMatchingNativePrisms(context.Background(), a, shifted, intersect.AffineRingUnion, trimmed.NativePrismBooleanOptions{}); e == nil || r != nil {
		t.Fatal("unmatched cap interval accepted")
	}
	tapered, e := brep.DraftPrism(polygon(.25, 1), nil, geom.Vec3{X: .5, Y: .5}, geom.Vec3{Z: 2}, .5)
	if e != nil {
		t.Fatal(e)
	}
	if r, e := trimmed.CombineMatchingNativePrisms(context.Background(), a, tapered, intersect.AffineRingUnion, trimmed.NativePrismBooleanOptions{}); e == nil || r != nil {
		t.Fatal("tapered wall accepted")
	}
	if r, e := trimmed.CombineMatchingNativePrisms(context.Background(), a, b, intersect.AffineRingDifference, trimmed.NativePrismBooleanOptions{Native: trimmed.AffineArrangementPrismOptions{MaxComponents: 1}}); e == nil || r != nil {
		t.Fatal("partial component publication")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if r, e := trimmed.CombineMatchingNativePrisms(ctx, a, b, intersect.AffineRingUnion, trimmed.NativePrismBooleanOptions{}); e != context.Canceled || r != nil {
		t.Fatal(r, e)
	}
}

func TestCombineNativePrismsSlantedProfile(t *testing.T) {
	a, _ := brep.Box(geom.Vec3{}, geom.Vec3{X: .75, Y: .75, Z: 2})
	b, _ := brep.Box(geom.Vec3{X: .25, Y: .25}, geom.Vec3{X: 1, Y: 1, Z: 2})
	shear := geom.Affine{{1, .5, 0, 2}, {0, 1, 0, -1}, {0, 0, 1, 3}}
	a, e := a.Transformed(shear)
	if e != nil {
		t.Fatal(e)
	}
	b, e = b.Transformed(shear)
	if e != nil {
		t.Fatal(e)
	}
	q, e := trimmed.CombineMatchingNativePrisms(context.Background(), a, b, intersect.AffineRingDifference, trimmed.NativePrismBooleanOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if q.Axis() != 2 || q.Material().ExactVolume().Cmp(big.NewRat(5, 8)) != 0 {
		t.Fatal(q.Axis(), q.Material().ExactVolume())
	}
	caps := q.OriginalCaps()
	for i := 0; i < 2; i++ {
		if caps[i][0] < 0 || caps[i][1] < 0 || caps[i][0] == caps[i][1] {
			t.Fatal(caps)
		}
	}
	mass, e := q.Material().Components()[0].MassProperties()
	if e != nil || math.Abs(mass.Volume-5./8) > 1e-11 {
		t.Fatal(mass, e)
	}
	uses := q.WallUses()
	uses[0].UpperParameter[0].SetInt64(99)
	if q.WallUses()[0].UpperParameter[0].Cmp(big.NewRat(99, 1)) == 0 {
		t.Fatal("mutable original edge parameters")
	}
}
