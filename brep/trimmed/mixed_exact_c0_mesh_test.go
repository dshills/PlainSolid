package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func continuousCubicCrease(t *testing.T) *geom.SurfacePatch {
	t.Helper()
	var poles []geom.Vec3
	z := []float64{0, 0, 0, .125, .375, .625, 1}
	for i := 0; i < 7; i++ {
		for j := 0; j < 4; j++ {
			poles = append(poles, geom.Vec3{X: float64(i), Y: float64(j), Z: z[i] + float64(j)/4})
		}
	}
	s, e := geom.NewBSplineSurface(3, 3, 7, 4, poles, []float64{0, 0, 0, 0, .5, .5, .5, 1, 1, 1, 1}, []float64{0, 0, 0, 0, 1, 1, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	p, e := geom.NewSurfacePatch(s, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	return p
}

func TestMixedMeshSourceRejectsFullMultiplicity(t *testing.T) {
	// BSplineSurface is broader than the exact SurfacePatch proof domain. Even
	// coincident endpoint values cannot bypass the full-multiplicity exclusion.
	for _, discontinuous := range []bool{false, true} {
		var poles []geom.Vec3
		for i := 0; i < 8; i++ {
			for j := 0; j < 4; j++ {
				x := i
				z := 0.
				if i < 4 {
					if i == 3 {
						z = .125
					}
				} else {
					x = i - 1
					z = []float64{.125, .375, .625, 1}[i-4]
				}
				z += float64(j) / 4
				if discontinuous && i == 4 && j == 1 {
					z += 1. / 32
				}
				poles = append(poles, geom.Vec3{X: float64(x), Y: float64(j), Z: z})
			}
		}
		s, e := geom.NewBSplineSurface(3, 3, 8, 4, poles, []float64{0, 0, 0, 0, .5, .5, .5, .5, 1, 1, 1, 1}, []float64{0, 0, 0, 0, 1, 1, 1, 1}, nil)
		if e != nil {
			t.Fatal(e)
		}
		if p, e := geom.NewSurfacePatch(s, geom.SurfacePatchOptions{}); e == nil || p != nil {
			t.Fatal("full-multiplicity proof bypass", discontinuous)
		}
	}
}

func TestHigherDegreeMixedCreaseCertified(t *testing.T) {
	ctx := context.Background()
	p := continuousCubicCrease(t)
	if q, e := p.InsertUKnot(.5, 1, geom.SurfacePatchOptions{}); e == nil || q != nil {
		t.Fatal("refinement bypassed continuity invariant")
	}
	left, e := p.Trim(geom.ParameterRectangle{UMax: .5, VMax: 1})
	if e != nil {
		t.Fatal(e)
	}
	right, e := p.Trim(geom.ParameterRectangle{UMin: .5, UMax: 1, VMax: 1})
	if e != nil {
		t.Fatal(e)
	}
	a, e := left.EvaluateRational(ctx, big.NewRat(1, 2), big.NewRat(1, 3), geom.SurfacePatchEvaluationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	b, e := right.EvaluateRational(ctx, big.NewRat(1, 2), big.NewRat(1, 3), geom.SurfacePatchEvaluationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if a.Du[2].Cmp(big.NewRat(3, 4)) != 0 || b.Du[2].Cmp(big.NewRat(3, 2)) != 0 {
		t.Fatal("not a true crease")
	}
	for k := range a.Point {
		if a.Point[k].Cmp(b.Point[k]) != 0 {
			t.Fatal("crease position mismatch")
		}
	}

	corners := []geom.Vec2{{}, {X: 1}, {X: 1, Y: 1}, {Y: 1}}
	outer := make([]PCurve, 4)
	for i, a := range corners {
		var e error
		outer[i], e = mixedStoredUseCurve(Use{UVStart: a, UVEnd: corners[(i+1)%4]})
		if e != nil {
			t.Fatal(e)
		}
	}
	o := MixedExactMultiSpanMeshOptions{Span: MultiSpanTriangleBoundOptions{MaxDegree: 3}, Face: MixedExactCurvedMeshOptions{RectangularGrid: true, Interior: CurvedQuadraticTrimMeshOptions{Deflection: .1}}}
	o.Face.Cuts = make([][][]*big.Rat, 1)
	o.Face.Cuts[0] = make([][]*big.Rat, 4)
	for i := 0; i < 4; i++ {
		for j := 0; j <= 8; j++ {
			o.Face.Cuts[0][i] = append(o.Face.Cuts[0][i], big.NewRat(int64(j), 8))
		}
	}
	r, e := TessellateMixedExactMultiSpanRegionCertified(ctx, p, outer, nil, o)
	if e != nil {
		t.Fatal(e)
	}
	model := func(uv [2]*big.Rat) [3]*big.Rat {
		u, v := uv[0], uv[1]
		local := new(big.Rat).Mul(u, big.NewRat(2, 1))
		z := new(big.Rat)
		if u.Cmp(big.NewRat(1, 2)) > 0 {
			local.Sub(local, big.NewRat(1, 1))
			z.Add(z, big.NewRat(1, 8))
			z.Add(z, new(big.Rat).Mul(local, big.NewRat(3, 4)))
		}
		cube := new(big.Rat).Mul(local, local)
		cube.Mul(cube, local)
		cube.Quo(cube, big.NewRat(8, 1))
		z.Add(z, cube)
		z.Add(z, new(big.Rat).Mul(v, big.NewRat(3, 4)))
		return [3]*big.Rat{new(big.Rat).Mul(u, big.NewRat(6, 1)), new(big.Rat).Mul(v, big.NewRat(3, 1)), z}
	}
	for id, uv := range r.ExactUV {
		expected := model(uv)
		for k := range expected {
			if r.ExactPoints[id][k].Cmp(expected[k]) != 0 {
				t.Fatal("piecewise analytic source", id, k)
			}
		}
	}
	for _, tri := range r.Mesh.Triangles {
		uv := [2]*big.Rat{new(big.Rat), new(big.Rat)}
		display := [3]*big.Rat{new(big.Rat), new(big.Rat), new(big.Rat)}
		for _, id := range tri {
			for k := range uv {
				uv[k].Add(uv[k], r.ExactUV[id][k])
			}
			point := exactBoundPoint(r.Mesh.Vertices[id])
			for k := range display {
				display[k].Add(display[k], point[k])
			}
		}
		for k := range uv {
			uv[k].Quo(uv[k], big.NewRat(3, 1))
		}
		for k := range display {
			display[k].Quo(display[k], big.NewRat(3, 1))
		}
		delta := ddSub(model(uv), display)
		if ddDot(delta, delta).Cmp(r.FillSquaredDistance) > 0 {
			t.Fatal("piecewise interior residual")
		}
	}
	low := o
	low.MaxSpanCells = 1
	if q, e := TessellateMixedExactMultiSpanRegionCertified(ctx, p, outer, nil, low); e == nil || len(q.Mesh.Vertices) != 0 {
		t.Fatal("cumulative cell quota")
	}
	low = o
	low.MaxSpanPieces = 1
	if q, e := TessellateMixedExactMultiSpanRegionCertified(ctx, p, outer, nil, low); e == nil || len(q.Mesh.Vertices) != 0 {
		t.Fatal("cumulative piece quota")
	}

}
