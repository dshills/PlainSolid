package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"math/big"
	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"reflect"
	"testing"
)

func mappedSharedSeamFixture(t *testing.T) (Solid, int, int, SplineUV) {
	return mappedSharedSeamFixtureReversed(t, false)
}
func mappedSharedSeamFixtureReversed(t *testing.T, reverseSource bool) (Solid, int, int, SplineUV) {
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
			weights := source.Curve.Weights()
			for i := range weights {
				power := i
				if source.Reversed {
					power = len(weights) - 1 - i
				}
				weights[i] *= float64(uint64(1) << power)
			}
			c, e := geom.NewBSpline(source.Curve.Degree(), source.Curve.Poles(), source.Curve.Knots(), weights)
			if e != nil {
				t.Fatal(e)
			}
			source.Curve = c
			if reverseSource {
				poles, weights, knots := c.Poles(), c.Weights(), c.Knots()
				for i, j := 0, len(poles)-1; i < j; i, j = i+1, j-1 {
					poles[i], poles[j] = poles[j], poles[i]
					weights[i], weights[j] = weights[j], weights[i]
				}
				for i, j := 0, len(knots)-1; i <= j; i, j = i+1, j-1 {
					a, b := knots[i], knots[j]
					knots[i], knots[j] = source.Range.Min+source.Range.Max-b, source.Range.Min+source.Range.Max-a
				}
				c, e = geom.NewBSpline(c.Degree(), poles, knots, weights)
				if e != nil {
					t.Fatal(e)
				}
				source.Curve = c
				source.Reversed = !source.Reversed
				source, e = source.WithExactEndpoints()
				if e != nil {
					t.Fatal(e)
				}
			}
			// The original authored speed now differs on this owner only. An explicit
			// reciprocal map is necessary; changing a model-normalized t is not allowed.
			faces[fi].Outer[ui].Curve = source
			if _, e = NewSolid(s.Vertices(), s.Edges(), faces, s.tolerance); e == nil {
				t.Fatal("unmapped different source speed accepted")
			}
			mapped, e := NewMappedSplineUV(ctx, source, big.NewRat(1, 2), geom.SurfacePatchOptions{})
			if e != nil {
				t.Fatal(e)
			}
			faces[fi].Outer[ui].Curve = mapped
			out, e := NewSolid(s.Vertices(), s.Edges(), faces, s.tolerance)
			if e != nil {
				t.Fatal(e)
			}
			return out, edge, fi, source
		}
	}
	t.Fatal("no original owner")
	return Solid{}, 0, 0, SplineUV{}
}

func TestStoredMixedMappedSplineSeam(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		name := "forward"
		if reverse {
			name = "reversed_exact_endpoint_source"
		}
		t.Run(name, func(t *testing.T) { checkStoredMixedMappedSplineSeam(t, reverse) })
	}
}
func checkStoredMixedMappedSplineSeam(t *testing.T, reverseSource bool) {
	ctx := context.Background()
	s, edge, owner, original := mappedSharedSeamFixtureReversed(t, reverseSource)
	var data bytes.Buffer
	if e := WriteJSON(&data, s); e != nil {
		t.Fatal("write", e)
	}
	var n nativeSolid
	if e := json.Unmarshal(data.Bytes(), &n); e != nil || n.Version != 9 {
		t.Fatal("version", e, n.Version)
	}
	s, e := ReadJSON(bytes.NewReader(data.Bytes()), JSONLimits{})
	if e != nil {
		t.Fatal("reload", e)
	}
	var again bytes.Buffer
	if e = WriteJSON(&again, s); e != nil || !bytes.Equal(data.Bytes(), again.Bytes()) {
		t.Fatal("deterministic reload", e)
	}
	for _, u := range s.faces[owner].Outer {
		if u.Edge != edge {
			continue
		}
		q, ok := u.Curve.(MappedSplineUV)
		if !ok {
			t.Fatal("source map lost")
		}
		source := q.Source()
		if source.Range != original.Range || source.Reversed != original.Reversed || source.ExactEndpoints != original.ExactEndpoints || !reflect.DeepEqual(source.Curve.Poles(), original.Curve.Poles()) || !reflect.DeepEqual(source.Curve.Knots(), original.Curve.Knots()) || !reflect.DeepEqual(source.Curve.Weights(), original.Curve.Weights()) {
			t.Fatal("authored source changed")
		}
		w := q.ParameterWeight()
		w.SetInt64(100)
		if q.ParameterWeight().Cmp(big.NewRat(1, 2)) != 0 {
			t.Fatal("mutable map")
		}
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
		t.Fatal("closed contact", e)
	}
	if r.SquaredDistance.Cmp(new(big.Rat).Mul(ratInset(.04), ratInset(.04))) > 0 {
		t.Fatal("deflection")
	}
	v, _ := r.DisplayVolume.Float64()
	if math.Abs(v-23./448) > .002 {
		t.Fatal("independent volume", v)
	}
	differing := 0
	for _, use := range r.Network.Faces[owner].Coedges {
		if use.Edge != edge {
			continue
		}
		q := s.faces[owner].Outer[use.Use].Curve.(MappedSplineUV)
		patch, e := exactSplineSupportPatch(s.faces[owner].Surface, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		for j, raw := range use.AuthoredParameters {
			uv := authoredMixedSplinePoint(t, q.Source(), raw)
			point, e := patch.EvaluateRational(ctx, uv[0], uv[1], geom.SurfacePatchEvaluationOptions{})
			if e != nil {
				t.Fatal(e)
			}
			id := r.Network.FaceVertices[owner][use.Vertices[j]]
			for k := 0; k < 3; k++ {
				if point.Point[k].Cmp(r.Network.ExactPoints[id][k]) != 0 {
					t.Fatal("independent original raw parameter")
				}
			}
			linear := new(big.Rat).Set(use.TrimParameters[j])
			if original.Reversed {
				linear.Sub(big.NewRat(1, 1), linear)
			}
			linear.Mul(linear, ratInset(original.Range.Max-original.Range.Min))
			linear.Add(linear, ratInset(original.Range.Min))
			if raw.Cmp(linear) != 0 {
				differing++
			}
		}
	}
	if differing == 0 {
		t.Fatal("no genuinely distinct authored parameter")
	}
	limited := o
	limited.MaxTriangles = 1
	if out, e := s.TessellateMixedClosedCertified(ctx, limited); e == nil || len(out.Mesh.Triangles) != 0 {
		t.Fatal("atomic mesh quota")
	}
	t.Logf("triangles=%d volume=%g nonlinear authored stations=%d", len(r.Mesh.Triangles), v, differing)
}

func TestMappedSplineNativeProofGuards(t *testing.T) {
	s, edge, _, source := mappedSharedSeamFixture(t)
	var buf bytes.Buffer
	if e := WriteJSON(&buf, s); e != nil {
		t.Fatal(e)
	}
	tests := map[string]func(*nativeSolid, *nativeUse){
		"downgrade": func(n *nativeSolid, u *nativeUse) { n.Version = 8 },
		"wrong map": func(n *nativeSolid, u *nativeUse) { u.MappedSpline.Weight = "2" },
		"subfloat wrong map": func(n *nativeSolid, u *nativeUse) {
			q := new(big.Rat).Add(big.NewRat(1, 2), new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 100)))
			u.MappedSpline.Weight = q.RatString()
		},
		"negative map":     func(n *nativeSolid, u *nativeUse) { u.MappedSpline.Weight = "-1" },
		"mixed payload":    func(n *nativeSolid, u *nativeUse) { u.Quadratic = &QuadraticUV{} },
		"source ownership": func(n *nativeSolid, u *nativeUse) { u.MappedSpline.Source.Edge = 1 },
		"false reversal":   func(n *nativeSolid, u *nativeUse) { u.MappedSpline.Source.SplineReversed = !source.Reversed },
		"wrong range":      func(n *nativeSolid, u *nativeUse) { u.MappedSpline.Source.SplineRange.Min += .125 },
		"changed source interior": func(n *nativeSolid, u *nativeUse) {
			p := source.Curve.Poles()
			p[1].X += 1
			c, e := geom.NewBSpline(source.Curve.Degree(), p, source.Curve.Knots(), source.Curve.Weights())
			if e != nil {
				t.Fatal(e)
			}
			u.MappedSpline.Source.Spline, e = parametric.EncodeCurve(c)
			if e != nil {
				t.Fatal(e)
			}
		},
		"nested source payload": func(n *nativeSolid, u *nativeUse) { u.MappedSpline.Source.Quadratic = &QuadraticUV{} },
	}
	for name, change := range tests {
		t.Run(name, func(t *testing.T) {
			var n nativeSolid
			if e := json.Unmarshal(buf.Bytes(), &n); e != nil {
				t.Fatal(e)
			}
			found := false
			for i := range n.Faces {
				for j := range n.Faces[i].Outer {
					u := &n.Faces[i].Outer[j]
					if u.MappedSpline != nil {
						change(&n, u)
						found = true
						break
					}
				}
				if found {
					break
				}
			}
			if !found {
				t.Fatal("missing map")
			}
			b, e := json.Marshal(n)
			if e != nil {
				t.Fatal(e)
			}
			out, e := ReadJSON(bytes.NewReader(b), JSONLimits{})
			if e == nil || len(out.faces) != 0 {
				t.Fatal("accepted forged source/map", e)
			}
		})
	}
	// This perturbation is below floating evaluation's resolving power. Native
	// sampled topology may accept it; exact persisted model identity must not.
	tiny := new(big.Rat).Add(big.NewRat(1, 2), new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 100)))
	mapped, e := NewMappedSplineUV(context.Background(), source, tiny, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	changed := s.Faces()
	for fi := range changed {
		for ui, u := range changed[fi].Outer {
			if _, ok := u.Curve.(MappedSplineUV); ok {
				changed[fi].Outer[ui].Curve = mapped
			}
		}
	}
	sampled, e := NewSolid(s.Vertices(), s.Edges(), changed, s.tolerance)
	if e != nil {
		t.Fatal("subfloat fixture must pass sampled topology", e)
	}
	var exact bytes.Buffer
	if e = WriteJSON(&exact, sampled); e == nil || exact.Len() != 0 {
		t.Fatal("subfloat identity bypass")
	}
	// A stale privately cached normalized curve is not a serialization proof.
	bad := s
	bad.faces = s.Faces()
	for fi := range bad.faces {
		for ui, u := range bad.faces[fi].Outer {
			if q, ok := u.Curve.(MappedSplineUV); ok {
				q.weight = big.NewRat(2, 1)
				bad.faces[fi].Outer[ui].Curve = q
			}
		}
	}
	var out bytes.Buffer
	if e := WriteJSON(&out, bad); e == nil || out.Len() != 0 {
		t.Fatal("stale source cache escaped")
	}
	if result, _, e := s.SplitExactBezierTrimEdge(context.Background(), edge, big.NewRat(1, 2), edgecheck.Options{}); !errors.Is(e, geom.ErrUnrepresentable) || len(result.faces) != 0 {
		t.Fatal("nonbinary raw repair station accepted", e)
	}
}

func TestMappedSplineLimits(t *testing.T) {
	_, _, _, source := mappedSharedSeamFixture(t)
	for _, w := range []*big.Rat{nil, new(big.Rat), big.NewRat(-1, 1)} {
		if q, e := NewMappedSplineUV(context.Background(), source, w, geom.SurfacePatchOptions{}); e == nil || q.ParameterWeight() != nil {
			t.Fatal("invalid map")
		}
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if q, e := NewMappedSplineUV(canceled, source, big.NewRat(1, 2), geom.SurfacePatchOptions{}); !errors.Is(e, context.Canceled) || q.ParameterWeight() != nil {
		t.Fatal("atomic cancellation", e)
	}
	if q, e := NewMappedSplineUV(context.Background(), source, big.NewRat(1, 2), geom.SurfacePatchOptions{MaxOperations: 1}); e == nil || q.ParameterWeight() != nil {
		t.Fatal("atomic budget", e)
	}
	w := big.NewRat(1, 2)
	q, e := NewMappedSplineUV(context.Background(), source, w, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	w.SetInt64(9)
	if q.ParameterWeight().Cmp(big.NewRat(1, 2)) != 0 {
		t.Fatal("input alias")
	}
	huge := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 4097))
	if _, e = q.AuthoredParameter(huge); e == nil {
		t.Fatal("unbounded parameter")
	}
	if _, _, e := coincidentUseCurve(context.Background(), Use{Curve: q}, geom.SurfacePatchOptions{}); e == nil {
		t.Fatal("unsupported coincidence path")
	}
}
