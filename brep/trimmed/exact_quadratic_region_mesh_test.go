package trimmed

import (
	"context"
	"errors"
	"math"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func nativeNonbinaryRoundedLoop(t *testing.T, r *big.Rat, reverse bool) []ExactQuadraticUV {
	t.Helper()
	zero := new(big.Rat)
	negative := new(big.Rat).Neg(r)
	s := new(big.Rat).Mul(r, big.NewRat(10, 9))
	ns := new(big.Rat).Neg(s)
	ends := [][2]*big.Rat{{r, zero}, {zero, r}, {negative, zero}, {zero, negative}}
	mids := [][2]*big.Rat{{s, s}, {ns, s}, {ns, ns}, {s, ns}}
	var loop []ExactQuadraticUV
	for i, a := range ends {
		b := ends[(i+1)%4]
		var h [3][3]*big.Rat
		for j, p := range [][2]*big.Rat{a, mids[i], b} {
			h[j] = [3]*big.Rat{new(big.Rat).Add(big.NewRat(1, 2), p[0]), new(big.Rat).Add(big.NewRat(1, 2), p[1]), big.NewRat(1, 1)}
		}
		q, e := NewExactQuadraticUV(h)
		if e != nil {
			t.Fatal(e)
		}
		loop = append(loop, q)
	}
	if reverse {
		for i, j := 0, len(loop)-1; i < j; i, j = i+1, j-1 {
			loop[i], loop[j] = loop[j], loop[i]
		}
		for i := range loop {
			loop[i] = loop[i].reversed()
		}
	}
	return loop
}
func nativeObliquePatch(t *testing.T) *geom.SurfacePatch {
	t.Helper()
	var poles []geom.Vec3
	for _, u := range []float64{0, .5, 1} {
		for _, v := range []float64{0, 1} {
			poles = append(poles, geom.Vec3{X: 2 * u, Y: 3 * v, Z: u + v})
		}
	}
	s, e := geom.NewBSplineSurface(1, 1, 3, 2, poles, []float64{0, 0, .5, 1, 1}, []float64{0, 0, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	p, e := geom.NewSurfacePatch(s, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	return p
}
func TestNativeExactQuadraticHoledFace(t *testing.T) {
	outer, hole := nativeNonbinaryRoundedLoop(t, big.NewRat(3, 8), false), nativeNonbinaryRoundedLoop(t, big.NewRat(1, 8), true)
	cuts := [][][]*big.Rat{make([][]*big.Rat, 4), make([][]*big.Rat, 4)}
	cuts[0][0] = []*big.Rat{big.NewRat(1, 3)}
	cuts[1][0] = []*big.Rat{big.NewRat(1, 3)}
	opts := ExactQuadraticRegionMeshOptions{Face: MultiSpanCurvedTrimMeshOptions{Mesh: CurvedQuadraticTrimMeshOptions{Deflection: .07}}}
	for _, nonplanar := range []bool{false, true} {
		p := nativeObliquePatch(t)
		if nonplanar {
			p = partitionedSeamPatch(t, false)
		}
		got, e := TessellateExactQuadraticRegionCertified(context.Background(), p, outer, [][]ExactQuadraticUV{hole}, cuts, opts)
		if e != nil {
			t.Fatal(e)
		}
		if len(got.BoundaryLoops) != 2 || len(got.LoopTrims) != 2 {
			t.Fatal("missing hole")
		}
		coefficient := got.LoopTrims[0][0].Source.HomogeneousControls()[1][0]
		if coefficient.Cmp(big.NewRat(11, 12)) != 0 {
			t.Fatal("native coefficient changed", coefficient)
		}
		f, _ := coefficient.Float64()
		if ratInset(f).Cmp(coefficient) == 0 {
			t.Fatal("fixture is not genuinely nonbinary")
		}
		found := false
		for j, original := range got.Boundary.Region.OriginalSource[1] {
			if original == 0 {
				for _, q := range got.LoopTrims[1][j].Parameters {
					if q.Cmp(big.NewRat(2, 3)) == 0 {
						found = true
					}
				}
			}
		}
		if !found {
			t.Fatal("lost directed original hole cut")
		}
		sum := new(big.Rat)
		edges := map[[2]int]int{}
		for _, tri := range got.Mesh.Triangles {
			a, b, c := got.ExactUV[tri[0]], got.ExactUV[tri[1]], got.ExactUV[tri[2]]
			det := new(big.Rat).Sub(new(big.Rat).Mul(new(big.Rat).Sub(b[0], a[0]), new(big.Rat).Sub(c[1], a[1])), new(big.Rat).Mul(new(big.Rat).Sub(b[1], a[1]), new(big.Rat).Sub(c[0], a[0])))
			if det.Sign() <= 0 {
				t.Fatal("orientation")
			}
			sum.Add(sum, new(big.Rat).Quo(det, big.NewRat(2, 1)))
			for j, a := range tri {
				b := tri[(j+1)%3]
				if a > b {
					a, b = b, a
				}
				edges[[2]int{a, b}]++
			}
		}
		if sum.Cmp(got.Boundary.Region.PolygonUVArea) != 0 {
			t.Fatal("exact polygon coverage")
		}
		if len(got.Mesh.Vertices)-len(edges)+len(got.Mesh.Triangles) != 0 {
			t.Fatal("annular topology")
		}
		// Direct quadratic Green integration gives each loop area:
		// 4*(2*r*s/3+r²/6), s=10*r/9, hence annulus49/108.
		polygon, _ := sum.Float64()
		if math.Abs(polygon-49./108) > .006 {
			t.Fatal("independent rational source area", polygon)
		}
		if !nonplanar {
			if math.Abs(got.Mesh.Area()-7*polygon) > 1e-12 {
				t.Fatal("exact affine Jacobian")
			}
			if math.Abs(got.Mesh.Area()-343./108) > .025 {
				t.Fatal("independent model area", got.Mesh.Area())
			}
		}
		if got.FillSquaredDistance.Cmp(new(big.Rat).Mul(ratInset(.07), ratInset(.07))) > 0 {
			t.Fatal("uncertified distance")
		}
		t.Logf("nonplanar=%v tris=%d polygonUV=%g trueUV=%g cells=%d", nonplanar, len(got.Mesh.Triangles), polygon, 49./108, got.SpanCellChecks)
		if nonplanar {
			limited := opts
			limited.Face.MaxSpanCells = got.SpanCellChecks - 1
			r, e := TessellateExactQuadraticRegionCertified(context.Background(), p, outer, [][]ExactQuadraticUV{hole}, cuts, limited)
			if !errors.Is(e, geom.ErrConvergence) || len(r.Mesh.Triangles) != 0 {
				t.Fatal("non-atomic span limit", e)
			}
		}
	}
}
func TestNativeExactQuadraticFaceInvalidTopology(t *testing.T) {
	p := nativeObliquePatch(t)
	outer := nativeNonbinaryRoundedLoop(t, big.NewRat(3, 8), false)
	o := ExactQuadraticRegionMeshOptions{Face: MultiSpanCurvedTrimMeshOptions{Mesh: CurvedQuadraticTrimMeshOptions{Deflection: .1}}}
	for _, hole := range [][]ExactQuadraticUV{nativeNonbinaryRoundedLoop(t, big.NewRat(1, 8), false), nativeNonbinaryRoundedLoop(t, big.NewRat(3, 8), true)} {
		if r, e := TessellateExactQuadraticRegionCertified(context.Background(), p, outer, [][]ExactQuadraticUV{hole}, nil, o); e == nil || len(r.Mesh.Triangles) != 0 {
			t.Fatal("invalid hole accepted")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := TessellateExactQuadraticRegionCertified(ctx, p, outer, nil, nil, o); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	if _, e := TessellateExactQuadraticRegionCertified(context.Background(), p, []ExactQuadraticUV{{}, {}}, nil, nil, o); !errors.Is(e, geom.ErrInvalid) {
		t.Fatal(e)
	}
}

func TestNativeExactQuadraticOuterFace(t *testing.T) {
	p := partitionedSeamPatch(t, false)
	outer := nativeNonbinaryRoundedLoop(t, big.NewRat(3, 8), false)
	o := ExactQuadraticRegionMeshOptions{Face: MultiSpanCurvedTrimMeshOptions{Mesh: CurvedQuadraticTrimMeshOptions{Deflection: .1}}}
	got, e := TessellateExactQuadraticRegionCertified(context.Background(), p, outer, nil, nil, o)
	if e != nil {
		t.Fatal(e)
	}
	if len(got.BoundaryLoops) != 1 || len(got.LoopTrims) != 1 {
		t.Fatal("unexpected hole")
	}
	checks := 0
	for _, trim := range got.LoopTrims[0] {
		checks += trim.Checks
	}
	o.Face.Mesh.Boundary.Tessellation.MaxChecks = checks - 1
	r, e := TessellateExactQuadraticRegionCertified(context.Background(), p, outer, nil, nil, o)
	if !errors.Is(e, geom.ErrConvergence) || len(r.Mesh.Triangles) != 0 {
		t.Fatal("partial boundary check limit", e)
	}
}
