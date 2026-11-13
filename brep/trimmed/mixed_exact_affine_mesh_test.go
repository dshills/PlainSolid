package trimmed

import (
	"context"
	"math"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func mixedAffineLoop(t *testing.T, r *big.Rat, reverse bool) []PCurve {
	q := nativeNonbinaryRoundedLoop(t, r, reverse)
	out := make([]PCurve, len(q))
	for i, c := range q {
		out[i] = c
	}
	h := q[0].HomogeneousControls()
	elev := make([][3]*big.Rat, 4)
	for i := range elev {
		for k := 0; k < 3; k++ {
			switch i {
			case 0:
				elev[i][k] = new(big.Rat).Set(h[0][k])
			case 3:
				elev[i][k] = new(big.Rat).Set(h[2][k])
			default:
				elev[i][k] = new(big.Rat).Add(new(big.Rat).Mul(h[i-1][k], big.NewRat(int64(i), 3)), new(big.Rat).Mul(h[i][k], big.NewRat(int64(3-i), 3)))
			}
		}
	}
	b, e := NewExactBezierUV(elev)
	if e != nil {
		t.Fatal(e)
	}
	out[0] = b
	return out
}
func TestMixedExactAffineAnnulus(t *testing.T) {
	outer, hole := mixedAffineLoop(t, big.NewRat(3, 8), false), mixedAffineLoop(t, big.NewRat(1, 8), true)
	// A genuine cubic perturbation, not merely elevated quadratic geometry.
	h := outer[0].(ExactBezierUV).HomogeneousControls()
	h[1][1].Add(h[1][1], big.NewRat(1, 64))
	outer[0], _ = NewExactBezierUV(h)
	o := MixedExactAffineMeshOptions{Display: geom.ComposedCollectionPolylineOptions{ComposedCurvePolylineOptions: geom.ComposedCurvePolylineOptions{Deflection: .002}}}
	r, e := TessellateMixedExactAffineRegionCertified(context.Background(), nativeObliquePatch(t), outer, [][]PCurve{hole}, o)
	if e != nil {
		t.Fatal(e)
	}
	if len(r.BoundaryLoops) != 2 || len(r.Samples) != 8 {
		t.Fatal("missing ancestry")
	}
	// Each polynomial quadrant has exact area49*r²/54; four quadrants and
	// radii3/8,1/8 give49/108 in UV. Oblique chart Jacobian is exactly7.
	// Moving cubic control y1 by d changes area by -integral(d*B1^3*x').
	// With r=3/8,s=10r/9,d=1/64, that is17/15360 exactly.
	expected := (49.0/108 + 17.0/15360) * 7
	area := 0.
	for _, tri := range r.Mesh.Triangles {
		a, b, c := r.Mesh.Vertices[tri[0]], r.Mesh.Vertices[tri[1]], r.Mesh.Vertices[tri[2]]
		area += b.Sub(a).Cross(c.Sub(a)).Length() / 2
		uv := [2]*big.Rat{new(big.Rat), new(big.Rat)}
		for _, id := range tri {
			for k := 0; k < 2; k++ {
				uv[k].Add(uv[k], r.ExactUV[id][k])
			}
		}
		for k := range uv {
			uv[k].Quo(uv[k], big.NewRat(3, 1))
		}
		dx := new(big.Rat).Sub(uv[0], big.NewRat(1, 2))
		dy := new(big.Rat).Sub(uv[1], big.NewRat(1, 2))
		rad := new(big.Rat).Add(new(big.Rat).Mul(dx, dx), new(big.Rat).Mul(dy, dy))
		if rad.Cmp(big.NewRat(1, 128)) < 0 {
			t.Fatal("hole filled")
		}
	}
	if math.Abs(area-expected) > .015 {
		t.Fatal("independent analytic area", area, expected)
	}
	for _, s := range r.Samples {
		for j, id := range s.Vertices {
			uv, e := mixedExactEvaluate(r.Source[s.Loop][s.Trim], s.Parameters[j])
			if e != nil {
				t.Fatal(e)
			}
			for k := range uv {
				if uv[k].Cmp(r.ExactUV[id][k]) != 0 {
					t.Fatal("source direction ancestry")
				}
			}
		}
	}
	if r.FillSquaredDistance.Sign() < 0 {
		t.Fatal("bound")
	}
	o.MaxTriangles = 1
	if r, e := TessellateMixedExactAffineRegionCertified(context.Background(), nativeObliquePatch(t), outer, [][]PCurve{hole}, o); e == nil || len(r.Mesh.Vertices) != 0 {
		t.Fatal("budget")
	}
}
func TestMixedExactAffineContactRejected(t *testing.T) {
	outer := mixedAffineLoop(t, big.NewRat(3, 8), false)
	touch := mixedAffineLoop(t, big.NewRat(3, 8), true)
	if r, e := TessellateMixedExactAffineRegionCertified(context.Background(), nativeObliquePatch(t), outer, [][]PCurve{touch}, MixedExactAffineMeshOptions{}); e == nil || len(r.Mesh.Vertices) != 0 {
		t.Fatal("coincident hole")
	}
}

func TestMixedExactAffineTangentHoleAndCancellation(t *testing.T) {
	outer := mixedAffineLoop(t, big.NewRat(3, 8), false)
	hole := mixedAffineLoop(t, big.NewRat(1, 8), true)
	for i, c := range hole {
		h, e := mixedExactControls(c)
		if e != nil {
			t.Fatal(e)
		}
		for j := range h {
			h[j][0].Add(h[j][0], new(big.Rat).Quo(h[j][2], big.NewRat(4, 1)))
		}
		if len(h) == 3 {
			hole[i], e = NewExactQuadraticUV([3][3]*big.Rat{h[0], h[1], h[2]})
		} else {
			hole[i], e = NewExactBezierUV(h)
		}
		if e != nil {
			t.Fatal(e)
		}
	}
	if r, e := TessellateMixedExactAffineRegionCertified(context.Background(), nativeObliquePatch(t), outer, [][]PCurve{hole}, MixedExactAffineMeshOptions{}); e == nil || len(r.Mesh.Vertices) != 0 {
		t.Fatal("tangent hole accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if r, e := TessellateMixedExactAffineRegionCertified(ctx, nativeObliquePatch(t), outer, nil, MixedExactAffineMeshOptions{}); e != context.Canceled || len(r.Mesh.Vertices) != 0 {
		t.Fatal("cancellation", e)
	}
}
