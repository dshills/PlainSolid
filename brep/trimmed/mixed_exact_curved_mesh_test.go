package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func TestMixedExactCurvedHoledSaddle(t *testing.T) {
	outer, hole := mixedAffineLoop(t, big.NewRat(3, 8), false), mixedAffineLoop(t, big.NewRat(1, 8), true)
	h := outer[0].(ExactBezierUV).HomogeneousControls()
	h[1][1].Add(h[1][1], big.NewRat(1, 64))
	outer[0], _ = NewExactBezierUV(h)
	s, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{}, {Y: 1}, {X: 1}, {X: 1, Y: 1, Z: .25}}, []float64{0, 0, 1, 1}, []float64{0, 0, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	p, e := geom.NewSurfacePatch(s, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	o := MixedExactCurvedMeshOptions{Interior: CurvedQuadraticTrimMeshOptions{Deflection: .02, MaxSubdivisions: 64}}
	r, e := TessellateMixedExactCurvedRegionCertified(context.Background(), p, outer, [][]PCurve{hole}, o)
	if e != nil {
		t.Fatal(e)
	}
	if len(r.BoundaryLoops) != 2 || r.FillSquaredDistance.Cmp(big.NewRat(1, 2500)) > 0 {
		t.Fatal("missing holed certificate")
	}
	for _, sample := range r.UVRegion.Samples {
		for j, id := range sample.Vertices {
			uv, e := mixedExactEvaluate(r.UVRegion.Source[sample.Loop][sample.Trim], sample.Parameters[j])
			if e != nil {
				t.Fatal(e)
			}
			for k := range uv {
				if r.ExactUV[id][k].Cmp(uv[k]) != 0 {
					t.Fatal("source index changed")
				}
			}
			want := new(big.Rat).Quo(new(big.Rat).Mul(uv[0], uv[1]), big.NewRat(4, 1))
			if r.ExactPoints[id][2].Cmp(want) != 0 {
				t.Fatal("independent model boundary")
			}
		}
	}
	// Independently evaluate z=uv/4 at every UV triangle barycenter, and
	// compare the same barycentric displayed point against its local certificate.
	for i, tri := range r.Mesh.Triangles {
		uv := [2]*big.Rat{new(big.Rat), new(big.Rat)}
		display := [3]*big.Rat{new(big.Rat), new(big.Rat), new(big.Rat)}
		for _, id := range tri {
			for k := range uv {
				uv[k].Add(uv[k], r.ExactUV[id][k])
			}
			v := r.Mesh.Vertices[id]
			for k, f := range []float64{v.X, v.Y, v.Z} {
				display[k].Add(display[k], ratInset(f))
			}
		}
		for k := range uv {
			uv[k].Quo(uv[k], big.NewRat(3, 1))
		}
		for k := range display {
			display[k].Quo(display[k], big.NewRat(3, 1))
		}
		exact := [3]*big.Rat{uv[0], uv[1], new(big.Rat).Quo(new(big.Rat).Mul(uv[0], uv[1]), big.NewRat(4, 1))}
		sq := new(big.Rat)
		for k := range exact {
			d := new(big.Rat).Sub(exact[k], display[k])
			sq.Add(sq, new(big.Rat).Mul(d, d))
		}
		if sq.Cmp(r.SquaredErrors[i]) > 0 {
			t.Fatal("independent triangle residual")
		}
	}
	small := o
	small.Interior.MaxTriangleChecks = 1
	if denied, e := TessellateMixedExactCurvedRegionCertified(context.Background(), p, outer, [][]PCurve{hole}, small); e == nil || len(denied.Mesh.Vertices) != 0 {
		t.Fatal("budget partial success")
	}
}

func TestMixedExactCurvedRationalSupport(t *testing.T) {
	s, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{}, {Y: 1}, {X: 1}, {X: 1, Y: 1, Z: .25}}, []float64{0, 0, 1, 1}, []float64{0, 0, 1, 1}, []float64{1, 1, 1, .875})
	if e != nil {
		t.Fatal(e)
	}
	p, e := geom.NewSurfacePatch(s, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	outer, hole := mixedAffineLoop(t, big.NewRat(3, 8), false), mixedAffineLoop(t, big.NewRat(1, 8), true)
	o := MixedExactCurvedMeshOptions{Interior: CurvedQuadraticTrimMeshOptions{Deflection: .06, MaxSubdivisions: 64}}
	r, e := TessellateMixedExactCurvedRegionCertified(context.Background(), p, outer, [][]PCurve{hole}, o)
	if e != nil {
		t.Fatal(e)
	}
	for i, tri := range r.Mesh.Triangles {
		u, v := new(big.Rat), new(big.Rat)
		display := [3]*big.Rat{new(big.Rat), new(big.Rat), new(big.Rat)}
		for _, id := range tri {
			u.Add(u, r.ExactUV[id][0])
			v.Add(v, r.ExactUV[id][1])
			p := r.Mesh.Vertices[id]
			for k, x := range []float64{p.X, p.Y, p.Z} {
				display[k].Add(display[k], ratInset(x))
			}
		}
		u.Quo(u, big.NewRat(3, 1))
		v.Quo(v, big.NewRat(3, 1))
		for k := range display {
			display[k].Quo(display[k], big.NewRat(3, 1))
		}
		uv := new(big.Rat).Mul(u, v)
		w := new(big.Rat).Sub(big.NewRat(1, 1), new(big.Rat).Quo(uv, big.NewRat(8, 1)))
		exact := [3]*big.Rat{new(big.Rat).Quo(new(big.Rat).Sub(u, new(big.Rat).Quo(uv, big.NewRat(8, 1))), w), new(big.Rat).Quo(new(big.Rat).Sub(v, new(big.Rat).Quo(uv, big.NewRat(8, 1))), w), new(big.Rat).Quo(new(big.Rat).Mul(big.NewRat(7, 1), uv), new(big.Rat).Mul(big.NewRat(32, 1), w))}
		sq := new(big.Rat)
		for k := range exact {
			d := new(big.Rat).Sub(exact[k], display[k])
			sq.Add(sq, new(big.Rat).Mul(d, d))
		}
		if sq.Cmp(r.SquaredErrors[i]) > 0 {
			t.Fatal("independent rational residual")
		}
	}
	if r.FillSquaredDistance.Cmp(big.NewRat(9, 2500)) > 0 {
		t.Fatal("global bound")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if r, e := TessellateMixedExactCurvedRegionCertified(ctx, p, outer, nil, o); e != context.Canceled || len(r.Mesh.Vertices) != 0 {
		t.Fatal("cancel", e)
	}
}
