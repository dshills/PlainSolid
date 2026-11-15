package trimmed

import (
	"bytes"
	"context"
	"errors"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"testing"
)

func TestStoredMixedCurvedWeightedSharedDomains(t *testing.T) {
	ctx := context.Background()
	stock, _ := polynomialClipStock(t)
	tool, _ := makeOpenSplineBox(t, [3]float64{-1, -1, -2}, [3]float64{3, 3, 3. / 16}, 0)
	cut, e := CombinePolynomialSplinePlaneClip(ctx, stock, tool, 5, parametric.Intersection, SplineRectangleEmbeddingOptions{})
	if e != nil {
		t.Fatal(e)
	}
	s := cut.Solid
	faces, edges := s.Faces(), s.Edges()
	chosen := -1
	weighted := func(c PCurve, reverse bool) [][3]*big.Rat {
		q, e := mixedStoredUseCurveContext(ctx, Use{Curve: c}, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		h, e := mixedExactControls(q)
		if e != nil {
			t.Fatal(e)
		}
		for i := range h {
			power := i
			if reverse {
				power = len(h) - 1 - i
			}
			factor := big.NewRat(int64(1<<power), 1)
			for k := 0; k < 3; k++ {
				h[i][k] = new(big.Rat).Mul(h[i][k], factor)
			}
		}
		return h
	}
	for _, id := range cut.SectionEdges {
		if m, ok := edges[id].Curve.(surfaceTrimCurve); ok && m.curve != nil {
			if q, ok := m.curve.(SplineUV); ok && q.Curve.Degree() == 4 {
				chosen = id
				m.curve, e = NewExactBezierUV(weighted(m.curve, false))
				if e != nil {
					t.Fatal(e)
				}
				edges[id].Curve = m
				break
			}
		}
	}
	if chosen < 0 {
		t.Fatal("no genuine cubic curved model seam")
	}
	count := 0
	for fi := range faces {
		for ui, u := range faces[fi].Outer {
			if u.Edge != chosen {
				continue
			}
			h := weighted(u.Curve, u.Reversed)
			var poles []geom.Vec3
			var weights []float64
			for _, p := range h {
				var xyz [2]float64
				for k := 0; k < 2; k++ {
					v := new(big.Rat).Quo(p[k], p[2])
					xyz[k], _ = v.Float64()
					if ratInset(xyz[k]).Cmp(v) != 0 {
						t.Fatal("nonbinary authored pole")
					}
				}
				w, _ := p[2].Float64()
				if ratInset(w).Cmp(p[2]) != 0 {
					t.Fatal("nonbinary authored weight")
				}
				poles = append(poles, geom.Vec3{X: xyz[0], Y: xyz[1]})
				weights = append(weights, w)
			}
			lo, hi := float64(2+fi), float64(6+fi)
			var knots []float64
			for i := 0; i < len(h); i++ {
				knots = append(knots, lo)
			}
			for i := 0; i < len(h); i++ {
				knots = append(knots, hi)
			}
			c, e := geom.NewBSpline(len(h)-1, poles, knots, weights)
			if e != nil {
				t.Fatal(e)
			}
			faces[fi].Outer[ui].Curve = SplineUV{Curve: c, Range: UVInterval{Min: lo, Max: hi}}
			count++
		}
	}
	if count != 2 {
		t.Fatal("two distinct source owners required", count)
	}
	supports := map[geom.Surface]geom.Surface{}
	support := func(src geom.Surface) geom.Surface {
		if dst, ok := supports[src]; ok {
			return dst
		}
		dst := minimalWeightedFixtureSupport(t, src)
		supports[src] = dst
		return dst
	}
	for i := range faces {
		faces[i].Surface = support(faces[i].Surface)
	}
	for i, edge := range edges {
		if m, ok := edge.Curve.(surfaceTrimCurve); ok {
			m.surface = support(m.surface)
			edges[i].Curve = m
		}
	}
	s, e = NewSolid(s.Vertices(), edges, faces, s.tolerance)
	if e != nil {
		t.Fatal(e)
	}
	var buf bytes.Buffer
	if e = WriteJSON(&buf, s); e != nil {
		t.Fatal(e)
	}
	s, e = ReadJSON(bytes.NewReader(buf.Bytes()), JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	for fi, f := range s.faces {
		for ui, u := range f.Outer {
			if u.Edge != chosen {
				continue
			}
			q := u.Curve.(SplineUV)
			for _, wrongRange := range []bool{false, true} {
				bad := s
				bad.faces = s.Faces()
				changed := q
				if wrongRange {
					changed.Range.Min += .125
				} else {
					poles := q.Curve.Poles()
					poles[len(poles)/2].Y += 1. / 128
					changed.Curve, e = geom.NewBSpline(q.Curve.Degree(), poles, q.Curve.Knots(), q.Curve.Weights())
					if e != nil {
						t.Fatal(e)
					}
				}
				bad.faces[fi].Outer[ui].Curve = changed
				var rejected bytes.Buffer
				if e := WriteJSON(&rejected, bad); e == nil || rejected.Len() != 0 {
					t.Fatal("persistence accepted altered original", wrongRange, e)
				}
			}
		}
	}
	o := StoredMixedClosedMeshOptions{Faces: make([]SolidMixedFaceMeshOptions, len(s.faces))}
	for i := range o.Faces {
		o.Faces[i] = SolidMixedFaceMeshOptions{Face: MixedExactMultiSpanMeshOptions{Span: MultiSpanTriangleBoundOptions{MaxDegree: 4}, Face: MixedExactCurvedMeshOptions{BoundaryModelBudget: .01, Interior: CurvedQuadraticTrimMeshOptions{Deflection: .04}}}}
	}
	// Straight UV closing chains can still curve in model space. Prescribe
	// four source intervals before the fixed-boundary lift; every resulting
	// chord and final surface residual is independently recertified.
	for fi, f := range s.faces {
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
			t.Fatalf("face %d: %v", fi, e)
		}
	}
	if _, e := s.TessellateMixedBoundaryNetworkCertified(ctx, o.Faces, 2); e != nil {
		t.Fatal("network", e)
	}
	// Native source correspondence does not imply that an arbitrary display
	// triangulation is embedded. This coarse closing-face lift has actual
	// nonincident coplanar contacts and must remain rejected globally.
	if _, e := s.TessellateMixedClosedCertified(ctx, o); !errors.Is(e, geom.ErrConvergence) {
		t.Fatal("expected unrefined global contact rejection", e)
	}
}

// Author the same original polynomial supports at their exact minimal tensor
// degrees. Independent full-domain comparison rejects any changed coefficient;
// this avoids spending mesh work on artificial degree elevation in the fixture.
func minimalWeightedFixtureSupport(t *testing.T, source geom.Surface) geom.Surface {
	s, ok := source.(*geom.BSplineSurface)
	if !ok {
		return source
	}
	p, e := geom.NewSurfacePatch(s, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if p.Domain() != (geom.ParameterRectangle{UMax: 1, VMax: 1}) {
		return source
	}
	c, _, ok := p.RationalBezierCoefficients(4)
	if !ok {
		return source
	}
	du, dv := 1, 1
	for i, row := range c {
		for j, h := range row {
			for _, x := range h {
				if x.Sign() != 0 {
					du = max(du, i)
					dv = max(dv, j)
				}
			}
		}
	}
	choose := func(n, k int) int64 {
		r := int64(1)
		for i := 1; i <= k; i++ {
			r = r * int64(n-i+1) / int64(i)
		}
		return r
	}
	var controls [][4]*big.Rat
	for i := 0; i <= du; i++ {
		for j := 0; j <= dv; j++ {
			var h [4]*big.Rat
			for axis := 0; axis < 4; axis++ {
				h[axis] = new(big.Rat)
				for k := 0; k <= i; k++ {
					for l := 0; l <= j; l++ {
						term := new(big.Rat).Mul(c[k][l][axis], big.NewRat(choose(i, k), choose(du, k)))
						term.Mul(term, big.NewRat(choose(j, l), choose(dv, l)))
						h[axis].Add(h[axis], term)
					}
				}
			}
			controls = append(controls, h)
		}
	}
	q, e := geom.NewExactHomogeneousBezierSurface(context.Background(), du, dv, controls, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	same, e := geom.CompareExactSurfaces(context.Background(), p, q, geom.ExactSurfaceComparisonOptions{})
	if e != nil || !same.Equal {
		t.Fatal("fixture changed support", e)
	}
	out, e := geom.NewExactBezierSurface(q)
	if e != nil {
		t.Fatal(e)
	}
	return out
}
