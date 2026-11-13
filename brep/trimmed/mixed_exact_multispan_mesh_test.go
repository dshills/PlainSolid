package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func TestMixedExactMultiSpanCreasedAnnulus(t *testing.T) {
	for _, weighted := range []bool{false, true} {
		outer, hole := mixedAffineLoop(t, big.NewRat(3, 8), false), mixedAffineLoop(t, big.NewRat(1, 8), true)
		h := outer[0].(ExactBezierUV).HomogeneousControls()
		h[1][1].Add(h[1][1], big.NewRat(1, 64))
		outer[0], _ = NewExactBezierUV(h)
		// z=u*v/4 + |u-7/16|/8: continuous, with a real slope jump1/4.
		const knot = .4375
		poles := []geom.Vec3{}
		for _, u := range []float64{0, knot, 1} {
			for _, v := range []float64{0, 1} {
				delta := u - knot
				if delta < 0 {
					delta = -delta
				}
				poles = append(poles, geom.Vec3{X: u, Y: v, Z: u*v/4 + delta/8})
			}
		}
		var weights []float64
		if weighted {
			weights = []float64{1, 1, .875, .875, 1, 1}
		}
		s, e := geom.NewBSplineSurface(1, 1, 3, 2, poles, []float64{0, 0, knot, 1, 1}, []float64{0, 0, 1, 1}, weights)
		if e != nil {
			t.Fatal(e)
		}
		p, e := geom.NewSurfacePatch(s, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		o := MixedExactMultiSpanMeshOptions{Face: MixedExactCurvedMeshOptions{Interior: CurvedQuadraticTrimMeshOptions{Deflection: .03, MaxSubdivisions: 64}}}
		r, e := TessellateMixedExactMultiSpanRegionCertified(context.Background(), p, outer, [][]PCurve{hole}, o)
		if e != nil {
			t.Fatal(e)
		}
		if len(r.BoundaryLoops) != 2 || r.SpanCellChecks <= r.TriangleChecks || r.SpanPieceChecks <= r.TriangleChecks || r.FillSquaredDistance.Cmp(big.NewRat(9, 10000)) > 0 {
			t.Fatal("missing span proof")
		}
		crossed := 0
		for i, tri := range r.Mesh.Triangles {
			uv := [2]*big.Rat{new(big.Rat), new(big.Rat)}
			display := [3]*big.Rat{new(big.Rat), new(big.Rat), new(big.Rat)}
			lo, hi := false, false
			for _, id := range tri {
				u := r.ExactUV[id][0]
				lo = lo || u.Cmp(big.NewRat(7, 16)) < 0
				hi = hi || u.Cmp(big.NewRat(7, 16)) > 0
				for k := range uv {
					uv[k].Add(uv[k], r.ExactUV[id][k])
				}
				v := r.Mesh.Vertices[id]
				for k, f := range []float64{v.X, v.Y, v.Z} {
					display[k].Add(display[k], ratInset(f))
				}
			}
			if lo && hi {
				crossed++
			}
			for k := range uv {
				uv[k].Quo(uv[k], big.NewRat(3, 1))
			}
			for k := range display {
				display[k].Quo(display[k], big.NewRat(3, 1))
			}
			x := new(big.Rat).Set(uv[0])
			if weighted {
				if x.Cmp(big.NewRat(7, 16)) <= 0 {
					tau := new(big.Rat).Quo(x, big.NewRat(7, 16))
					x.Quo(new(big.Rat).Mul(tau, big.NewRat(49, 128)), new(big.Rat).Sub(big.NewRat(1, 1), new(big.Rat).Quo(tau, big.NewRat(8, 1))))
				} else {
					tau := new(big.Rat).Quo(new(big.Rat).Sub(x, big.NewRat(7, 16)), big.NewRat(9, 16))
					x.Quo(new(big.Rat).Add(new(big.Rat).Mul(new(big.Rat).Sub(big.NewRat(1, 1), tau), big.NewRat(49, 128)), tau), new(big.Rat).Add(big.NewRat(7, 8), new(big.Rat).Quo(tau, big.NewRat(8, 1))))
				}
			}
			delta := new(big.Rat).Sub(x, big.NewRat(7, 16))
			delta.Abs(delta)
			z := new(big.Rat).Add(new(big.Rat).Quo(new(big.Rat).Mul(x, uv[1]), big.NewRat(4, 1)), new(big.Rat).Quo(delta, big.NewRat(8, 1)))
			exact := [3]*big.Rat{x, uv[1], z}
			sq := new(big.Rat)
			for k := range exact {
				d := new(big.Rat).Sub(exact[k], display[k])
				sq.Add(sq, new(big.Rat).Mul(d, d))
			}
			if sq.Cmp(r.SquaredErrors[i]) > 0 {
				t.Fatal("independent crease residual")
			}
		}
		if crossed == 0 {
			t.Fatal("fixture has no knot-crossing triangles")
		}
		for _, sample := range r.UVRegion.Samples {
			for j, id := range sample.Vertices {
				uv, e := mixedExactEvaluate(r.UVRegion.Source[sample.Loop][sample.Trim], sample.Parameters[j])
				if e != nil {
					t.Fatal(e)
				}
				for k := range uv {
					if uv[k].Cmp(r.ExactUV[id][k]) != 0 {
						t.Fatal("lost source ownership")
					}
				}
			}
		}
		small := o
		small.MaxSpanCells = 1
		if r, e := TessellateMixedExactMultiSpanRegionCertified(context.Background(), p, outer, [][]PCurve{hole}, small); e == nil || len(r.Mesh.Vertices) != 0 {
			t.Fatal("global cell budget")
		}
		small = o
		small.MaxSpanPieces = 1
		if r, e := TessellateMixedExactMultiSpanRegionCertified(context.Background(), p, outer, [][]PCurve{hole}, small); e == nil || len(r.Mesh.Vertices) != 0 {
			t.Fatal("global piece budget")
		}
	}
}
