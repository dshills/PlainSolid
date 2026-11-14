package trimmed

import (
	"context"
	"errors"
	"math/big"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
	"testing"
)

func TestWrappedQuadraticCylinderPointQuery(t *testing.T) {
	ctx := context.Background()
	w := wrappedOverlapSupport(t)
	loop := wrappedQuadraticLoop(t)
	o := WrappedQuadraticFaceOptions{SurfaceInjectivityOptions: geom.SurfaceInjectivityOptions{ProjectionAxes: [2]int{1, 2}}}
	face, e := NewWrappedQuadraticFace(ctx, w, loop, nil, o)
	if e != nil {
		t.Fatal(e)
	}
	cases := []struct {
		uv      [2]*big.Rat
		want    trimvalidate.Location
		matched bool
	}{{[2]*big.Rat{new(big.Rat), new(big.Rat)}, trimvalidate.Inside, true}, {[2]*big.Rat{new(big.Rat), big.NewRat(1, 2)}, trimvalidate.Boundary, true}, {[2]*big.Rat{new(big.Rat), big.NewRat(3, 4)}, trimvalidate.Outside, true}, {[2]*big.Rat{big.NewRat(2, 1), new(big.Rat)}, trimvalidate.Outside, false}, {[2]*big.Rat{big.NewRat(1, 12), new(big.Rat)}, trimvalidate.Inside, true}}
	for _, c := range cases {
		for _, turn := range []int64{-2, 0, 3} {
			uv := [2]*big.Rat{new(big.Rat).Add(c.uv[0], big.NewRat(4*turn, 1)), new(big.Rat).Set(c.uv[1])}
			r, e := face.LocateCylinderRationalUV(ctx, uv, geom.SurfacePatchOptions{})
			if e != nil || !r.Complete || r.Location != c.want || r.MatchedLift != c.matched {
				t.Fatal("location", uv, r, e)
			}
			if r.SuppliedUV[0].Cmp(uv[0]) != 0 {
				t.Fatal("lost original supplied occurrence")
			}
			if c.matched && (r.FaceUV[0].Cmp(c.uv[0]) != 0 || r.PeriodShift[0].Cmp(big.NewInt(-turn)) != 0) {
				t.Fatal("wrong retained lift")
			}
			if !c.matched && (r.FaceUV[0] != nil || r.PeriodShift[0] != nil) {
				t.Fatal("invented retained lift")
			}
			if c.uv[0].Cmp(big.NewRat(1, 12)) == 0 {
				want := [3]*big.Rat{big.NewRat(143, 72), big.NewRat(23, 72), new(big.Rat)}
				for axis := 0; axis < 3; axis++ {
					if r.Point[axis].Cmp(want[axis]) != 0 {
						t.Fatal("nonbinary original model", r.Point)
					}
				}
				verified, e := face.ClassifyCylinderRationalPoint(ctx, want, uv, geom.SurfacePatchOptions{})
				if e != nil || !verified.Complete || verified.Location != trimvalidate.Inside {
					t.Fatal("exact model witness", e)
				}
				bad := [3]*big.Rat{new(big.Rat).Add(want[0], new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 100))), want[1], want[2]}
				af, _ := want[0].Float64()
				bf, _ := bad[0].Float64()
				if af != bf {
					t.Fatal("fixture not sub-ULP")
				}
				if q, e := face.ClassifyCylinderRationalPoint(ctx, bad, uv, geom.SurfacePatchOptions{}); !errors.Is(e, geom.ErrDomain) || q.Complete {
					t.Fatal("rounded model witness accepted", e)
				}
			}
			if q, e := face.LocateCylinderRationalUV(ctx, uv, geom.SurfacePatchOptions{MaxOperations: r.ExactOperations - 1}); e == nil || q.Complete || q.Point[0] != nil {
				t.Fatal("atomic query limit")
			}
		}
	}
	cancel, stop := context.WithCancel(ctx)
	stop()
	if q, e := face.LocateCylinderRationalUV(cancel, [2]*big.Rat{new(big.Rat), new(big.Rat)}, geom.SurfacePatchOptions{}); e == nil || q.Complete {
		t.Fatal("cancel publication")
	}
	refined, _, e := w.InsertKnot(ctx, 0, .125, 1, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	other, e := NewWrappedQuadraticFace(ctx, refined, loop, nil, o)
	if e != nil {
		t.Fatal(e)
	}
	if q, e := other.LocateCylinderRationalUV(ctx, [2]*big.Rat{new(big.Rat), new(big.Rat)}, geom.SurfacePatchOptions{}); e == nil || q.Complete {
		t.Fatal("unsupported global embedding inferred from local face")
	}
}
func TestWrappedQuadraticCylinderPointHole(t *testing.T) {
	ctx := context.Background()
	w := wrappedOverlapSupport(t)
	pts := [][2]*big.Rat{{big.NewRat(-1, 16), big.NewRat(-3, 4)}, {big.NewRat(-1, 16), big.NewRat(-1, 2)}, {big.NewRat(1, 16), big.NewRat(-1, 2)}, {big.NewRat(1, 16), big.NewRat(-3, 4)}}
	var hole []ExactQuadraticUV
	for i, p := range pts {
		q, e := quadraticLine(p, pts[(i+1)%4])
		if e != nil {
			t.Fatal(e)
		}
		hole = append(hole, q)
	}
	face, e := NewWrappedQuadraticFace(ctx, w, wrappedQuadraticLoop(t), [][]ExactQuadraticUV{hole}, WrappedQuadraticFaceOptions{SurfaceInjectivityOptions: geom.SurfaceInjectivityOptions{ProjectionAxes: [2]int{1, 2}}, Reversed: true})
	if e != nil {
		t.Fatal(e)
	}
	for _, test := range []struct {
		v   *big.Rat
		loc trimvalidate.Location
	}{{big.NewRat(-5, 8), trimvalidate.Outside}, {big.NewRat(-3, 4), trimvalidate.Boundary}, {big.NewRat(-7, 8), trimvalidate.Inside}} {
		r, e := face.LocateCylinderRationalUV(ctx, [2]*big.Rat{big.NewRat(4, 1), test.v}, geom.SurfacePatchOptions{})
		if e != nil || r.Location != test.loc || !r.MatchedLift || r.FaceUV[0].Sign() != 0 {
			t.Fatal("hole membership through seam", r, e)
		}
	}
}
