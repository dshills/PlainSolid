//go:build research

package trimmed

import (
	"bytes"
	"context"
	"errors"
	"math"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func knotOriginalMixedUse(t *testing.T, q SplineUV, cuts []*big.Rat) SplineUV {
	t.Helper()
	ctx := context.Background()
	curve, e := mixedStoredUseCurveContext(ctx, Use{Curve: q}, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	h, e := mixedExactControls(curve)
	if e != nil {
		t.Fatal(e)
	}
	degree := len(h) - 1
	previous := new(big.Rat)
	rest := h
	var pieces [][][3]*big.Rat
	for _, cut := range cuts {
		relative := new(big.Rat).Quo(new(big.Rat).Sub(cut, previous), new(big.Rat).Sub(big.NewRat(1, 1), previous))
		a, b := splitHomogeneousUV(rest, relative)
		pieces = append(pieces, a)
		rest = b
		previous = cut
	}
	pieces = append(pieces, rest)
	var poles []geom.Vec3
	var weights []float64
	for pi, piece := range pieces {
		for i, p := range piece {
			if pi > 0 && i == 0 {
				continue
			}
			var xyz [2]float64
			for k := 0; k < 2; k++ {
				r := new(big.Rat).Quo(p[k], p[2])
				xyz[k], _ = r.Float64()
				if ratInset(xyz[k]).Cmp(r) != 0 {
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
	}
	var knots []float64
	for i := 0; i <= degree; i++ {
		knots = append(knots, q.Range.Min)
	}
	for _, cut := range cuts {
		raw := new(big.Rat).Add(ratInset(q.Range.Min), new(big.Rat).Mul(cut, new(big.Rat).Sub(ratInset(q.Range.Max), ratInset(q.Range.Min))))
		k, _ := raw.Float64()
		if ratInset(k).Cmp(raw) != 0 {
			t.Fatal("nonbinary authored knot")
		}
		for i := 0; i < degree; i++ {
			knots = append(knots, k)
		}
	}
	for i := 0; i <= degree; i++ {
		knots = append(knots, q.Range.Max)
	}
	c, e := geom.NewBSpline(degree, poles, knots, weights)
	if e != nil {
		t.Fatal(e)
	}
	return SplineUV{Curve: c, Range: q.Range}
}

func TestStoredMixedIndependentCurvedKnots(t *testing.T) {
	ctx := context.Background()
	s, edge := storedMixedCurvedFixture(t, false)
	s, _ = storedMixedAffineSideFixture(t, s)
	faces := s.Faces()
	owners := 0
	for fi := range faces {
		for ui, u := range faces[fi].Outer {
			if u.Edge != edge {
				continue
			}
			q, ok := u.Curve.(SplineUV)
			if !ok {
				t.Fatal("authored spline")
			}
			cuts := []*big.Rat{big.NewRat(1, 2)}
			if owners == 1 {
				cuts = []*big.Rat{big.NewRat(1, 4), big.NewRat(3, 4)}
			}
			faces[fi].Outer[ui].Curve = knotOriginalMixedUse(t, q, cuts)
			owners++
		}
	}
	if owners != 2 {
		t.Fatal("owners", owners)
	}
	var e error
	s, e = NewSolid(s.Vertices(), s.Edges(), faces, s.tolerance)
	if e != nil {
		t.Fatal(e)
	}
	var data bytes.Buffer
	if e = WriteJSON(&data, s); e != nil {
		t.Fatal("write", e)
	}
	s, e = ReadJSON(bytes.NewReader(data.Bytes()), JSONLimits{})
	if e != nil {
		t.Fatal("reload", e)
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
	for _, cut := range []*big.Rat{big.NewRat(1, 4), big.NewRat(1, 2), big.NewRat(3, 4)} {
		found := false
		for _, p := range r.Network.EdgeParameters[edge] {
			if p.Cmp(cut) == 0 {
				found = true
			}
		}
		if !found {
			t.Fatal("lost shared knot parameter", cut)
		}
	}
	for fi, f := range s.faces {
		for ui, u := range f.Outer {
			if u.Edge != edge {
				continue
			}
			q := u.Curve.(SplineUV)
			if candidate, e := mixedStoredHigherSplineUV(ctx, q, geom.SurfacePatchOptions{MaxOperations: 1}); e == nil || candidate != nil {
				t.Fatal("non-atomic exact proof budget")
			}
			var excessive []*big.Rat
			for i := 1; i <= 8; i++ {
				excessive = append(excessive, big.NewRat(int64(i), 16))
			}
			many := knotOriginalMixedUse(t, q, excessive)
			if candidate, e := mixedStoredHigherSplineUV(ctx, many, geom.SurfacePatchOptions{}); e == nil || candidate != nil {
				t.Fatal("unbounded source spans")
			}
			if len(q.Curve.Poles()) <= 5 {
				t.Fatal("original knot history replaced")
			}
			// A changed second cell with unchanged global endpoints is not a redundant
			// knot representation. Exact whole-curve proof must fail atomically.
			bad := s
			bad.faces = s.Faces()
			poles := q.Curve.Poles()
			poles[len(poles)-2].Y += 1. / 1024
			c, e := geom.NewBSpline(q.Curve.Degree(), poles, q.Curve.Knots(), q.Curve.Weights())
			if e != nil {
				t.Fatal(e)
			}
			q.Curve = c
			bad.faces[fi].Outer[ui].Curve = q
			var rejected bytes.Buffer
			if e = WriteJSON(&rejected, bad); e == nil || rejected.Len() != 0 {
				t.Fatal("changed later cell accepted")
			}
			if mesh, e := bad.TessellateMixedClosedCertified(ctx, o); e == nil || len(mesh.Mesh.Vertices) != 0 {
				t.Fatal("non-atomic changed cell")
			}
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e := s.TessellateMixedClosedCertified(canceled, o); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	t.Logf("triangles=%d volume=%g seam samples=%d", len(r.Mesh.Triangles), v, len(r.Network.EdgeParameters[edge]))
}
