//go:build research

package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"testing"
)

func piecewiseSharedSeamFixture(t *testing.T) (Solid, int, int, PiecewiseMappedSplineUV) {
	t.Helper()
	ctx := context.Background()
	s, edge := storedMixedCurvedFixture(t, false)
	s, _ = storedMixedAffineSideFixture(t, s)
	faces := s.Faces()
	for fi, f := range faces {
		for ui, u := range f.Outer {
			if u.Edge != edge {
				continue
			}
			source := u.Curve.(SplineUV)
			base, e := mixedStoredHigherSplineUV(ctx, source, geom.SurfacePatchOptions{})
			if e != nil {
				t.Fatal(e)
			}
			station := big.NewRat(3, 8)
			a, b, e := base.(ExactBezierUV).Split(station)
			if e != nil {
				t.Fatal(e)
			}
			left, right := a.HomogeneousControls(), b.HomogeneousControls()
			for i := range left {
				for k := 0; k < 3; k++ {
					left[i][k].Mul(left[i][k], big.NewRat(int64(1<<i), 1))
					right[i][k].Mul(right[i][k], big.NewRat(1, int64(1<<i)))
				}
			}
			scale := new(big.Rat).Quo(left[len(left)-1][2], right[0][2])
			for i := range right {
				for k := 0; k < 3; k++ {
					right[i][k].Mul(right[i][k], scale)
				}
			}
			var poles []geom.Vec3
			var weights []float64
			for _, h := range append(left, right[1:]...) {
				var xy [2]float64
				for k := 0; k < 2; k++ {
					xy[k], e = geom.RepresentedExactScalar(new(big.Rat).Quo(h[k], h[2]))
					if e != nil {
						t.Fatal(e)
					}
				}
				w, e := geom.RepresentedExactScalar(h[2])
				if e != nil {
					t.Fatal(e)
				}
				poles = append(poles, geom.Vec3{X: xy[0], Y: xy[1]})
				weights = append(weights, w)
			}
			n := source.Curve.Degree()
			var knots []float64
			for i := 0; i <= n; i++ {
				knots = append(knots, 2)
			}
			for i := 0; i < n; i++ {
				knots = append(knots, 4)
			}
			for i := 0; i <= n; i++ {
				knots = append(knots, 10)
			}
			c, e := geom.NewBSpline(n, poles, knots, weights)
			if e != nil {
				t.Fatal(e)
			}
			source = SplineUV{Curve: c, Range: UVInterval{2, 10}}
			if _, e := NewMappedSplineUV(ctx, source, big.NewRat(1, 1), geom.SurfacePatchOptions{}); e == nil {
				t.Fatal("source must be genuinely two-span, not a redundant knot")
			}
			q, e := NewPiecewiseMappedSplineUV(ctx, source, station, [2]*big.Rat{big.NewRat(1, 2), big.NewRat(2, 1)}, geom.SurfacePatchOptions{})
			if e != nil {
				t.Fatal(e)
			}
			faces[fi].Outer[ui].Curve = q
			out, e := NewSolid(s.Vertices(), s.Edges(), faces, s.tolerance)
			if e != nil {
				t.Fatal(e)
			}
			return out, edge, fi, q
		}
	}
	t.Fatal("no curved source")
	return Solid{}, 0, 0, PiecewiseMappedSplineUV{}
}

func TestStoredMixedPiecewiseMappedSeam(t *testing.T) {
	ctx := context.Background()
	s, edge, owner, original := piecewiseSharedSeamFixture(t)
	var data bytes.Buffer
	if e := WriteJSON(&data, s); e != nil {
		t.Fatal("write", e)
	}
	var n nativeSolid
	if e := json.Unmarshal(data.Bytes(), &n); e != nil || n.Version != 10 {
		t.Fatal("version", e)
	}
	s, e := ReadJSON(bytes.NewReader(data.Bytes()), JSONLimits{})
	if e != nil {
		t.Fatal("reload", e)
	}
	var again bytes.Buffer
	if e = WriteJSON(&again, s); e != nil || !bytes.Equal(data.Bytes(), again.Bytes()) {
		t.Fatal("reload deterministic", e)
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
		t.Fatal("closed", e)
	}
	if e = r.Mesh.ValidateSolid(); e != nil || !r.DisplayCollision.Complete || len(r.DisplayCollision.Contacts) != 0 {
		t.Fatal("contact", e)
	}
	v, _ := r.DisplayVolume.Float64()
	if math.Abs(v-23./448) > .002 || r.SquaredDistance.Cmp(new(big.Rat).Mul(ratInset(.04), ratInset(.04))) > 0 {
		t.Fatal("independent volume/distance", v)
	}
	left, right, junction := 0, 0, -1
	var modelStation *big.Rat
	for _, use := range r.Network.Faces[owner].Coedges {
		if use.Edge != edge {
			continue
		}
		q, ok := s.faces[owner].Outer[use.Use].Curve.(PiecewiseMappedSplineUV)
		if !ok || !sameAuthoredBezierSpline(q.source.Curve, original.source.Curve) || q.source.Range != original.source.Range {
			t.Fatal("source history")
		}
		patch, e := exactSplineSupportPatch(s.faces[owner].Surface, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		for j, p := range use.AuthoredParameters {
			uv := authoredMixedSplinePoint(t, q.Source(), p)
			point, e := patch.EvaluateRational(ctx, uv[0], uv[1], geom.SurfacePatchEvaluationOptions{})
			if e != nil {
				t.Fatal(e)
			}
			id := r.Network.FaceVertices[owner][use.Vertices[j]]
			for k := 0; k < 3; k++ {
				if point.Point[k].Cmp(r.Network.ExactPoints[id][k]) != 0 {
					t.Fatal("raw two-span source identity")
				}
			}
			switch use.TrimParameters[j].Cmp(q.station) {
			case -1:
				left++
			case 1:
				right++
			case 0:
				if p.Cmp(big.NewRat(4, 1)) != 0 {
					t.Fatal("source knot station")
				}
				junction = id
				modelStation = use.ModelParameters[j]
			}
		}
	}
	if left == 0 || right == 0 || junction < 0 {
		t.Fatal("missing piecewise station/side samples")
	}
	other := 0
	for fi, f := range r.Network.Faces {
		if fi == owner {
			continue
		}
		for _, use := range f.Coedges {
			if use.Edge != edge {
				continue
			}
			for j, p := range use.ModelParameters {
				if p.Cmp(modelStation) == 0 {
					if r.Network.FaceVertices[fi][use.Vertices[j]] != junction {
						t.Fatal("shared knot ID")
					}
					other++
				}
			}
		}
	}
	if other != 1 {
		t.Fatal("other original owner missed knot", other)
	}
	t.Logf("triangles=%d volume=%g source-side samples=%d/%d shared knot=%d", len(r.Mesh.Triangles), v, left, right, junction)
}

func TestPiecewiseMappedSourceGuards(t *testing.T) {
	ctx := context.Background()
	s, _, _, q := piecewiseSharedSeamFixture(t)
	source := q.Source()
	n := source.Curve.Degree()
	p, w := source.Curve.Poles(), source.Curve.Weights()
	// Exact C0 uses the one shared original pole. One-sided raw derivatives
	// differ, independently demonstrating a genuine authored parameter-speed kink.
	different := false
	for _, axis := range []int{0, 1} {
		coord := func(v geom.Vec3) float64 {
			if axis == 0 {
				return v.X
			}
			return v.Y
		}
		left := new(big.Rat).Mul(big.NewRat(int64(n), 2), new(big.Rat).Mul(new(big.Rat).Quo(ratInset(w[n-1]), ratInset(w[n])), new(big.Rat).Sub(ratInset(coord(p[n])), ratInset(coord(p[n-1])))))
		right := new(big.Rat).Mul(big.NewRat(int64(n), 6), new(big.Rat).Mul(new(big.Rat).Quo(ratInset(w[n+1]), ratInset(w[n])), new(big.Rat).Sub(ratInset(coord(p[n+1])), ratInset(coord(p[n])))))
		if left.Cmp(right) != 0 {
			different = true
		}
	}
	if !different {
		t.Fatal("source speed must genuinely change at knot")
	}
	for _, station := range []*big.Rat{nil, new(big.Rat), big.NewRat(1, 1), big.NewRat(1, 2)} {
		if out, e := NewPiecewiseMappedSplineUV(ctx, source, station, q.ParameterWeights(), geom.SurfacePatchOptions{}); e == nil || out.ModelStation() != nil {
			t.Fatal("wrong station", station, e)
		}
	}
	if _, e := NewPiecewiseMappedSplineUV(ctx, source, q.ModelStation(), [2]*big.Rat{big.NewRat(1, 1), big.NewRat(2, 1)}, geom.SurfacePatchOptions{}); e == nil {
		t.Fatal("wrong original speed")
	}
	// Full-multiplicity discontinuity is not silently joined at a rounded point.
	poles := append([]geom.Vec3(nil), p[:n+1]...)
	shifted := p[n]
	shifted.Y += .125
	poles = append(poles, shifted)
	poles = append(poles, p[n+1:]...)
	weights := append([]float64(nil), w[:n+1]...)
	weights = append(weights, w[n:]...)
	var knots []float64
	for _, v := range []float64{2, 4, 10} {
		for i := 0; i <= n; i++ {
			knots = append(knots, v)
		}
	}
	broken, e := geom.NewBSpline(n, poles, knots, weights)
	if e != nil {
		t.Fatal(e)
	}
	bad := source
	bad.Curve = broken
	if _, e := NewPiecewiseMappedSplineUV(ctx, bad, q.ModelStation(), q.ParameterWeights(), geom.SurfacePatchOptions{}); e == nil {
		t.Fatal("discontinuous source accepted")
	}
	if out, e := NewPiecewiseMappedSplineUV(ctx, source, q.ModelStation(), q.ParameterWeights(), geom.SurfacePatchOptions{MaxOperations: 1}); e == nil || out.ModelStation() != nil {
		t.Fatal("quota")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if out, e := NewPiecewiseMappedSplineUV(canceled, source, q.ModelStation(), q.ParameterWeights(), geom.SurfacePatchOptions{}); !errors.Is(e, context.Canceled) || out.ModelStation() != nil {
		t.Fatal("cancellation", e)
	}
	station := q.ModelStation()
	station.SetInt64(1)
	ws := q.ParameterWeights()
	ws[0].SetInt64(99)
	if q.ModelStation().Cmp(big.NewRat(3, 8)) != 0 || q.ParameterWeights()[0].Cmp(big.NewRat(1, 2)) != 0 {
		t.Fatal("mutable evidence")
	}
	var data bytes.Buffer
	if e := WriteJSON(&data, s); e != nil {
		t.Fatal(e)
	}
	tests := map[string]func(*nativeSolid, *nativeUse){
		"downgrade":         func(n *nativeSolid, u *nativeUse) { n.Version = 9 },
		"wrong station":     func(n *nativeSolid, u *nativeUse) { u.PiecewiseMappedSpline.Station = "1/2" },
		"wrong speed":       func(n *nativeSolid, u *nativeUse) { u.PiecewiseMappedSpline.Weights[1] = "1" },
		"competing payload": func(n *nativeSolid, u *nativeUse) { u.MappedSpline = &nativeMappedSplineUse{} },
		"nested payload":    func(n *nativeSolid, u *nativeUse) { u.PiecewiseMappedSpline.Source.Quadratic = &QuadraticUV{} },
		"wrong source": func(n *nativeSolid, u *nativeUse) {
			p := source.Curve.Poles()
			p[len(p)-2].Y += .125
			c, e := geom.NewBSpline(source.Curve.Degree(), p, source.Curve.Knots(), source.Curve.Weights())
			if e != nil {
				t.Fatal(e)
			}
			u.PiecewiseMappedSpline.Source.Spline, e = parametric.EncodeCurve(c)
			if e != nil {
				t.Fatal(e)
			}
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			var n nativeSolid
			if e := json.Unmarshal(data.Bytes(), &n); e != nil {
				t.Fatal(e)
			}
			found := false
			for i := range n.Faces {
				for j := range n.Faces[i].Outer {
					u := &n.Faces[i].Outer[j]
					if u.PiecewiseMappedSpline != nil {
						mutate(&n, u)
						found = true
						break
					}
				}
				if found {
					break
				}
			}
			b, e := json.Marshal(n)
			if e != nil {
				t.Fatal(e)
			}
			if out, e := ReadJSON(bytes.NewReader(b), JSONLimits{}); e == nil || len(out.faces) != 0 {
				t.Fatal("forged recipe accepted", e)
			}
		})
	}
}

func TestPiecewiseMappedRequiredCuts(t *testing.T) {
	_, _, _, q := piecewiseSharedSeamFixture(t)
	uses := [][]Use{{{Curve: q}}}
	original := [][][]*big.Rat{{{new(big.Rat), big.NewRat(1, 1)}}}
	cuts, e := piecewiseMappedFaceCuts(uses, original)
	if e != nil || len(cuts[0][0]) != 3 || cuts[0][0][1].Cmp(q.station) != 0 || len(original[0][0]) != 2 {
		t.Fatal("mandatory station/owned cuts", e)
	}
	cuts[0][0][1].SetInt64(99)
	if q.ModelStation().Cmp(big.NewRat(3, 8)) != 0 {
		t.Fatal("station alias")
	}
	bad := [][][]*big.Rat{{{big.NewRat(1, 1), new(big.Rat)}}}
	if _, e := piecewiseMappedFaceCuts(uses, bad); e == nil {
		t.Fatal("invalid cuts normalized away")
	}
	huge := make([]*big.Rat, 2049)
	if _, e := piecewiseMappedFaceCuts(uses, [][][]*big.Rat{{huge}}); e == nil {
		t.Fatal("raw cut quota")
	}
}

func TestPiecewiseMappedNativeMixedModes(t *testing.T) {
	s, edge, owner, _ := piecewiseSharedSeamFixture(t)
	faces := s.Faces()
	found := false
	for fi := range faces {
		if fi == owner {
			continue
		}
		for ui, u := range faces[fi].Outer {
			if u.Edge != edge {
				continue
			}
			source := u.Curve.(SplineUV)
			source, e := source.WithExactEndpoints()
			if e != nil {
				t.Fatal(e)
			}
			q, e := NewMappedSplineUV(context.Background(), source, big.NewRat(1, 1), geom.SurfacePatchOptions{})
			if e != nil {
				t.Fatal(e)
			}
			faces[fi].Outer[ui].Curve = q
			found = true
		}
	}
	if !found {
		t.Fatal("no opposite owner")
	}
	mixed, e := NewSolid(s.Vertices(), s.Edges(), faces, s.tolerance)
	if e != nil {
		t.Fatal(e)
	}
	var data bytes.Buffer
	if e = WriteJSON(&data, mixed); e != nil {
		t.Fatal(e)
	}
	out, e := ReadJSON(bytes.NewReader(data.Bytes()), JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	if !hasNativeMappedSpline(out) || !hasNativePiecewiseMappedSpline(out) {
		t.Fatal("mixed original recipe types lost")
	}
	bad := s
	bad.faces = s.Faces()
	for ui, u := range bad.faces[owner].Outer {
		if q, ok := u.Curve.(PiecewiseMappedSplineUV); ok {
			q.station = big.NewRat(1, 2)
			bad.faces[owner].Outer[ui].Curve = q
		}
	}
	var rejected bytes.Buffer
	if e = WriteJSON(&rejected, bad); e == nil || rejected.Len() != 0 {
		t.Fatal("stale normalized cache accepted")
	}
}
