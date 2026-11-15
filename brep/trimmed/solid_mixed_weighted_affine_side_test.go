package trimmed

import (
	"bytes"
	"context"
	"math"
	"math/big"
	"plainsolid/brep/edgecheck"
	"plainsolid/geom"
	"testing"
)

// The independently described material is 0<=x<=1,
// 0<=y<=3/4-x^3/2, x^3/8+y/4<=z<=3/16. Its interior thickness
// is positive and its exact volume is integral_0^1 (3-2*x^3)^2/128 dx
// =23/448. The y=0 closing face is authored in affine model coordinates;
// its cubic lower trim is authenticated against the original roof edge.
func storedMixedAffineSideFixture(t *testing.T, s Solid) (Solid, int) {
	t.Helper()
	ctx := context.Background()
	faces := s.Faces()
	changed := 0
	sideFace := -1
	for fi, f := range faces {
		p, e := exactSplineSupportPatch(f.Surface, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		c, _, ok := p.RationalBezierCoefficients(4)
		if !ok {
			t.Fatal("coefficients")
		}
		planar := true
		for _, row := range c {
			for _, v := range row {
				if v[1].Sign() != 0 {
					planar = false
				}
			}
		}
		if !planar {
			continue
		}
		// Independently authored original physical side y=0, with affine chart
		// (u,v)=(x,z). Its lower boundary is the exact cubic z=x^3/8.
		dst, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{}, {Z: 1}, {X: 1}, {X: 1, Z: 1}}, []float64{0, 0, 1, 1}, []float64{0, 0, 1, 1}, nil)
		if e != nil {
			t.Fatal(e)
		}
		for ui, u := range f.Outer {
			uv, rev, e := coincidentUseCurve(ctx, u, geom.SurfacePatchOptions{})
			if e != nil {
				t.Fatal(e)
			}
			uv, e = uv.Reparameterize(0, 1, rev, geom.SurfacePatchOptions{})
			if e != nil {
				t.Fatal(e)
			}
			model, e := p.ComposeParameterCurve(ctx, uv, geom.SurfacePatchOptions{})
			if e != nil {
				t.Fatal(e)
			}
			net, _, e := model.BezierControlNet(ctx, geom.SurfacePatchOptions{})
			if e != nil {
				t.Fatal(e)
			}
			h := make([][3]*big.Rat, len(net.Control))
			for i, v := range net.Control {
				if v[1].Sign() != 0 {
					t.Fatal("not planar")
				}
				h[i] = [3]*big.Rat{v[0], v[2], v[3]}
			}
			ends := [2]geom.Vec2{}
			for i, j := range []int{0, len(h) - 1} {
				ends[i], e = exactGraphPartitionUV([2]*big.Rat{new(big.Rat).Quo(h[j][0], h[j][2]), new(big.Rat).Quo(h[j][1], h[j][2])})
				if e != nil {
					t.Fatal(e)
				}
			}
			u.UVStart, u.UVEnd = ends[0], ends[1]
			for len(h) < 4 {
				n := len(h) - 1
				elevated := make([][3]*big.Rat, n+2)
				elevated[0], elevated[n+1] = h[0], h[n]
				for i := 1; i <= n; i++ {
					a := big.NewRat(int64(i), int64(n+1))
					for k := 0; k < 3; k++ {
						elevated[i][k] = new(big.Rat).Add(new(big.Rat).Mul(a, h[i-1][k]), new(big.Rat).Mul(new(big.Rat).Sub(big.NewRat(1, 1), a), h[i][k]))
					}
				}
				h = elevated
			}
			u.Curve, e = NewExactBezierUV(h)
			if e != nil {
				t.Fatal(e)
			}
			faces[fi].Outer[ui] = u
		}
		faces[fi].Surface = dst
		faces[fi].Reversed = false
		changed++
		sideFace = fi
	}
	if changed != 1 {
		t.Fatal("one independently authored planar side", changed)
	}
	var e error
	s, e = NewSolid(s.Vertices(), s.Edges(), faces, s.tolerance)
	if e != nil {
		t.Fatal(e)
	}
	for fi, f := range s.faces {
		for _, u := range f.Outer {
			var ok bool
			var ee error
			if u.Curve == nil {
				ok, ee = compareSplineEdgeUse(ctx, s.edges[u.Edge], f.Surface, u, edgecheck.Options{})
			} else {
				ok, ee = compareSplineRationalTrimUse(ctx, s.edges[u.Edge], f.Surface, u, edgecheck.Options{})
			}
			if !ok || ee != nil {
				t.Fatalf("compare fi%d edge%d curve%T model%T ok%v err%v", fi, u.Edge, u.Curve, s.edges[u.Edge].Curve, ok, ee)
			}
		}
	}
	var data bytes.Buffer
	if e = WriteJSON(&data, s); e != nil {
		t.Fatal("write", e)
	}
	s, e = ReadJSON(bytes.NewReader(data.Bytes()), JSONLimits{})
	if e != nil {
		t.Fatal("reload", e)
	}
	return s, sideFace
}

func TestStoredMixedWeightedCurvedSeamAffineSides(t *testing.T) {
	ctx := context.Background()
	s, chosen := storedMixedCurvedWeightedFixture(t)
	s, sideFace := storedMixedAffineSideFixture(t, s)
	var e error
	// The new affine side's cubic lower trace is compared against an
	// original straight UV line on the curved support. Equal endpoints cannot
	// authenticate a changed interior in that composition path.
	tampered := false
	for ui, u := range s.faces[sideFace].Outer {
		m, ok := s.edges[u.Edge].Curve.(surfaceTrimCurve)
		if !ok || m.curve != nil {
			continue
		}
		q, ok := u.Curve.(ExactBezierUV)
		if !ok || u.UVStart.X == u.UVEnd.X {
			continue
		}
		h := q.HomogeneousControls()
		h[1][1].Add(h[1][1], new(big.Rat).Mul(h[1][2], big.NewRat(1, 128)))
		wrong, e := NewExactBezierUV(h)
		if e != nil {
			t.Fatal(e)
		}
		bad := s
		bad.faces = s.Faces()
		bad.faces[sideFace].Outer[ui].Curve = wrong
		var out bytes.Buffer
		if e = WriteJSON(&out, bad); e == nil || out.Len() != 0 {
			t.Fatal("changed cubic interior accepted", e)
		}
		tampered = true
		break
	}
	if !tampered {
		t.Fatal("missing curved original line composition")
	}
	o := StoredMixedClosedMeshOptions{Faces: make([]SolidMixedFaceMeshOptions, len(s.faces))}
	for fi, f := range s.faces {
		o.Faces[fi] = SolidMixedFaceMeshOptions{Face: MixedExactMultiSpanMeshOptions{Span: MultiSpanTriangleBoundOptions{MaxDegree: 4}, Face: MixedExactCurvedMeshOptions{BoundaryModelBudget: .01, Interior: CurvedQuadraticTrimMeshOptions{Deflection: .04}}}}
		for _, loop := range append([][]Use{f.Outer}, f.Holes...) {
			cuts := make([][]*big.Rat, len(loop))
			for i := range cuts {
				for k := 0; k <= 4; k++ {
					cuts[i] = append(cuts[i], big.NewRat(int64(k), 4))
				}
			}
			o.Faces[fi].Face.Face.Cuts = append(o.Faces[fi].Face.Face.Cuts, cuts)
		}
	}
	for fi := range s.faces {
		if _, e := s.TessellateMixedFaceCertified(ctx, fi, o.Faces[fi]); e != nil {
			t.Fatalf("face%d %v", fi, e)
		}
	}
	r, e := s.TessellateMixedClosedCertified(ctx, o)
	if e != nil {
		t.Fatal("closed", e)
	}
	if e = r.Mesh.ValidateSolid(); e != nil {
		t.Fatal(e)
	}
	if !r.DisplayCollision.Complete || len(r.DisplayCollision.Contacts) != 0 {
		t.Fatal("global contacts")
	}
	v, _ := r.DisplayVolume.Float64()
	if math.Abs(v-23./448) > .002 {
		t.Fatal("analytic volume", v)
	}
	if len(r.Network.EdgeVertices) != len(s.edges) || len(r.Network.EdgeVertices[chosen]) < 3 {
		t.Fatal("missing original seam")
	}
	for fi, ids := range r.FaceVertices {
		for local, global := range ids {
			if r.Mesh.Vertices[global] != r.Network.Faces[fi].Mesh.Vertices[local] {
				t.Fatal("changed source face vertex")
			}
		}
	}
	limit := new(big.Rat).Mul(ratInset(.04), ratInset(.04))
	if r.SquaredDistance.Cmp(limit) > 0 {
		t.Fatal("source deflection", r.SquaredDistance)
	}
	limited := o
	limited.MaxTriangles = 1
	if q, e := s.TessellateMixedClosedCertified(ctx, limited); e == nil || len(q.Mesh.Vertices) != 0 {
		t.Fatal("non-atomic quota")
	}
	t.Logf("triangles=%d volume=%g", len(r.Mesh.Triangles), v)
}
