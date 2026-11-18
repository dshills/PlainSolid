package trimmed

import (
	"bytes"
	"context"
	"math"
	"math/big"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"reflect"
	"testing"
)

// Scaling chart coordinates by 81=3^4 exactly clears the odd denominators
// introduced by bisecting the degree-four Mobius weights 1,2,4,8,16. This is
// a change of source parameter units, not a rounded or enlarged physical solid.
func scaledMixedFixtureCharts(t *testing.T, s Solid) Solid {
	t.Helper()
	ctx := context.Background()
	factor := big.NewRat(81, 1)
	supports := map[geom.Surface]geom.Surface{}
	support := func(src geom.Surface) geom.Surface {
		if dst, ok := supports[src]; ok {
			return dst
		}
		p, e := exactSplineSupportPatch(src, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		r, e := p.ExportExactBezierRecipe()
		if e != nil {
			t.Fatal(e)
		}
		r.Domain.UMin *= 81
		r.Domain.UMax *= 81
		r.Domain.VMin *= 81
		r.Domain.VMax *= 81
		p, e = geom.ImportExactBezierSurfaceRecipe(ctx, r, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		dst, e := geom.NewExactBezierSurface(p)
		if e != nil {
			t.Fatal(e)
		}
		supports[src] = dst
		return dst
	}
	uv := func(p geom.Vec2) geom.Vec2 { return geom.Vec2{X: 81 * p.X, Y: 81 * p.Y} }
	curve := func(c PCurve) PCurve {
		switch q := c.(type) {
		case nil:
			return nil
		case SplineUV:
			poles := q.Curve.Poles()
			for i := range poles {
				poles[i].X *= 81
				poles[i].Y *= 81
			}
			v, e := geom.NewBSpline(q.Curve.Degree(), poles, q.Curve.Knots(), q.Curve.Weights())
			if e != nil {
				t.Fatal(e)
			}
			q.Curve = v
			return q
		case ExactBezierUV:
			h := q.HomogeneousControls()
			for i := range h {
				h[i][0].Mul(h[i][0], factor)
				h[i][1].Mul(h[i][1], factor)
			}
			v, e := NewExactBezierUV(h)
			if e != nil {
				t.Fatal(e)
			}
			return v
		case ExactQuadraticUV:
			h := q.HomogeneousControls()
			for i := range h {
				h[i][0].Mul(h[i][0], factor)
				h[i][1].Mul(h[i][1], factor)
			}
			v, e := NewExactQuadraticUV(h)
			if e != nil {
				t.Fatal(e)
			}
			return v
		}
		t.Fatalf("curve %T", c)
		return nil
	}
	faces, edges := s.Faces(), s.Edges()
	for fi := range faces {
		faces[fi].Surface = support(faces[fi].Surface)
		for ui, u := range faces[fi].Outer {
			u.UVStart, u.UVEnd = uv(u.UVStart), uv(u.UVEnd)
			u.Curve = curve(u.Curve)
			faces[fi].Outer[ui] = u
		}
	}
	for i, e := range edges {
		m, ok := e.Curve.(surfaceTrimCurve)
		if !ok {
			t.Fatalf("model %T", e.Curve)
		}
		m.surface = support(m.surface)
		m.a, m.b = uv(m.a), uv(m.b)
		m.curve = curve(m.curve)
		edges[i].Curve = m
	}
	out, e := NewSolid(s.Vertices(), edges, faces, s.tolerance)
	if e != nil {
		t.Fatal(e)
	}
	return out
}

func storedMixedWeightedKnotsFixture(t *testing.T) (Solid, int) {
	t.Helper()
	s, edge := storedMixedCurvedWeightedFixture(t)
	s, _ = storedMixedAffineSideFixture(t, s)
	s = scaledMixedFixtureCharts(t, s)
	faces := s.Faces()
	owners := 0
	for fi := range faces {
		for ui, u := range faces[fi].Outer {
			if u.Edge != edge {
				continue
			}
			q := u.Curve.(SplineUV)
			q = knotOriginalMixedUse(t, q, []*big.Rat{big.NewRat(1, 2)})
			if owners == 1 {
				poles, weights, knots := q.Curve.Poles(), q.Curve.Weights(), q.Curve.Knots()
				for i, j := 0, len(poles)-1; i < j; i, j = i+1, j-1 {
					poles[i], poles[j] = poles[j], poles[i]
					weights[i], weights[j] = weights[j], weights[i]
				}
				for i, j := 0, len(knots)-1; i <= j; i, j = i+1, j-1 {
					a, b := knots[i], knots[j]
					knots[i], knots[j] = q.Range.Min+q.Range.Max-b, q.Range.Min+q.Range.Max-a
				}
				c, e := geom.NewBSpline(q.Curve.Degree(), poles, knots, weights)
				if e != nil {
					t.Fatal(e)
				}
				q.Curve = c
				q.Reversed = true
			}
			faces[fi].Outer[ui].Curve = q
			owners++
		}
	}
	if owners != 2 {
		t.Fatal("owners")
	}
	var e error
	s, e = NewSolid(s.Vertices(), s.Edges(), faces, s.tolerance)
	if e != nil {
		t.Fatal(e)
	}
	authored := map[[2]int]SplineUV{}
	for fi, f := range s.faces {
		for ui, u := range f.Outer {
			if u.Edge == edge {
				authored[[2]int{fi, ui}] = u.Curve.(SplineUV)
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
	for id, original := range authored {
		q := s.faces[id[0]].Outer[id[1]].Curve.(SplineUV)
		if q.Range != original.Range || q.Reversed != original.Reversed || !reflect.DeepEqual(q.Curve.Knots(), original.Curve.Knots()) || !reflect.DeepEqual(q.Curve.Poles(), original.Curve.Poles()) || !reflect.DeepEqual(q.Curve.Weights(), original.Curve.Weights()) {
			t.Fatal("native authored source history changed")
		}
	}
	return s, edge
}

func TestStoredMixedWeightedReversedCurvedKnots(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	s, edge := storedMixedWeightedKnotsFixture(t)
	var e error
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
	r, e := s.TessellateMixedClosedCertified(ctx, o)
	if e != nil {
		t.Fatal(e)
	}
	if e = r.Mesh.ValidateSolid(); e != nil || !r.DisplayCollision.Complete || len(r.DisplayCollision.Contacts) != 0 {
		t.Fatal("closed display", e)
	}
	v, _ := r.DisplayVolume.Float64()
	if math.Abs(v-23./448) > .002 {
		t.Fatal("analytic volume", v)
	}
	for fi, face := range r.Network.Faces {
		for _, use := range face.Coedges {
			if use.Edge != edge {
				continue
			}
			q := s.faces[fi].Outer[use.Use].Curve.(SplineUV)
			patch, e := exactSplineSupportPatch(s.faces[fi].Surface, geom.SurfacePatchOptions{})
			if e != nil {
				t.Fatal(e)
			}
			if len(use.AuthoredParameters) != len(use.TrimParameters) {
				t.Fatal("missing authored history")
			}
			for j, parameter := range use.AuthoredParameters {
				normalized := new(big.Rat).Set(use.TrimParameters[j])
				if q.Reversed {
					normalized.Sub(big.NewRat(1, 1), normalized)
				}
				expected := new(big.Rat).Add(ratInset(q.Range.Min), new(big.Rat).Mul(normalized, new(big.Rat).Sub(ratInset(q.Range.Max), ratInset(q.Range.Min))))
				if parameter.Cmp(expected) != 0 {
					t.Fatal("authored parameter map")
				}
				uv := authoredMixedSplinePoint(t, q, parameter)
				world, e := patch.EvaluateRational(ctx, uv[0], uv[1], geom.SurfacePatchEvaluationOptions{})
				if e != nil {
					t.Fatal(e)
				}
				global, ok := r.Network.FaceVertices[fi][use.Vertices[j]]
				if !ok {
					t.Fatal("missing shared identity")
				}
				for axis := 0; axis < 3; axis++ {
					if world.Point[axis].Cmp(r.Network.ExactPoints[global][axis]) != 0 {
						t.Fatal("independent authored de Boor/shared model mismatch")
					}
				}
			}
			trim, model := new(big.Rat).Set(use.TrimParameters[0]), new(big.Rat).Set(use.ModelParameters[0])
			use.AuthoredParameters[0].SetInt64(-99)
			if use.TrimParameters[0].Cmp(trim) != 0 || use.ModelParameters[0].Cmp(model) != 0 {
				t.Fatal("parameter histories alias")
			}
		}
	}
	reversed := 0
	for fi, f := range s.faces {
		for ui, u := range f.Outer {
			if u.Edge != edge {
				continue
			}
			q := u.Curve.(SplineUV)
			if len(q.Curve.Poles()) != 9 {
				t.Fatal("lost knot history")
			}
			w := q.Curve.Weights()
			if w[0] == w[len(w)-1] {
				t.Fatal("not genuine rational speed")
			}
			if q.Reversed {
				reversed++
			}
			// A changed weight preserves both endpoint positions but changes the
			// original rational trace. It must not be accepted as a redundant knot.
			bad := s
			bad.faces = s.Faces()
			w[2] *= 2
			c, e := geom.NewBSpline(q.Curve.Degree(), q.Curve.Poles(), q.Curve.Knots(), w)
			if e != nil {
				t.Fatal(e)
			}
			q.Curve = c
			bad.faces[fi].Outer[ui].Curve = q
			var out bytes.Buffer
			if e = WriteJSON(&out, bad); e == nil || out.Len() != 0 {
				t.Fatal("altered rational source accepted")
			}
		}
	}
	if reversed != 1 {
		t.Fatal("missing reversed source history")
	}
	t.Logf("triangles=%d volume=%g seam samples=%d", len(r.Mesh.Triangles), v, len(r.Network.EdgeParameters[edge]))
}

// Independent exact de Boor evaluation in the ORIGINAL authored knot domain.
// This does not use the candidate Bezier normalization under test.
func authoredMixedSplinePoint(t *testing.T, q SplineUV, parameter *big.Rat) [2]*big.Rat {
	t.Helper()
	poles, weights, knots := q.Curve.Poles(), q.Curve.Weights(), q.Curve.Knots()
	p := q.Curve.Degree()
	span := len(poles) - 1
	for i := p; i < len(poles); i++ {
		if parameter.Cmp(ratInset(knots[i+1])) < 0 {
			span = i
			break
		}
	}
	d := make([][3]*big.Rat, p+1)
	for j := 0; j <= p; j++ {
		i := span - p + j
		w := big.NewRat(1, 1)
		if len(weights) > 0 {
			w = ratInset(weights[i])
		}
		d[j] = [3]*big.Rat{new(big.Rat).Mul(ratInset(poles[i].X), w), new(big.Rat).Mul(ratInset(poles[i].Y), w), new(big.Rat).Set(w)}
	}
	for r := 1; r <= p; r++ {
		for j := p; j >= r; j-- {
			lo := ratInset(knots[j+span-p])
			hi := ratInset(knots[j+span-r+1])
			a := new(big.Rat).Quo(new(big.Rat).Sub(parameter, lo), new(big.Rat).Sub(hi, lo))
			b := new(big.Rat).Sub(big.NewRat(1, 1), a)
			for k := 0; k < 3; k++ {
				d[j][k] = new(big.Rat).Add(new(big.Rat).Mul(b, d[j-1][k]), new(big.Rat).Mul(a, d[j][k]))
			}
		}
	}
	return [2]*big.Rat{new(big.Rat).Quo(d[p][0], d[p][2]), new(big.Rat).Quo(d[p][1], d[p][2])}
}
