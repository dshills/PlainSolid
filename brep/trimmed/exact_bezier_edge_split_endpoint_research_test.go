//go:build research

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
	"plainsolid/internal/slowtest"
	"reflect"
	"strings"
	"testing"
)

func weightedEndpointFixture(t *testing.T) (Solid, int, map[int]SplineUV) {
	t.Helper()
	s, edge := storedMixedWeightedKnotsFixture(t)
	faces := s.Faces()
	authored := map[int]SplineUV{}
	for fi := range faces {
		for ui, u := range faces[fi].Outer {
			if u.Edge != edge {
				continue
			}
			q := u.Curve.(SplineUV)
			knots := q.Curve.Knots()
			for i := range knots {
				knots[i] = q.Range.Min + (knots[i]-q.Range.Min)*.75
			}
			c, e := geom.NewBSpline(q.Curve.Degree(), q.Curve.Poles(), knots, q.Curve.Weights())
			if e != nil {
				t.Fatal(e)
			}
			q.Curve = c
			q.Range.Max = q.Range.Min + 3
			faces[fi].Outer[ui].Curve = q
			authored[fi] = q
		}
	}
	out, e := NewSolid(s.Vertices(), s.Edges(), faces, s.tolerance)
	if e != nil {
		t.Fatal(e)
	}
	if bad, _, e := out.SplitExactBezierTrimEdge(context.Background(), edge, big.NewRat(1, 3), edgecheck.Options{}); e == nil || !strings.Contains(e.Error(), "p-curve endpoints differ") || len(bad.faces) != 0 {
		t.Fatal("legacy rounding failure required", e)
	}
	return out, edge, authored
}

func TestSplitWeightedExactEndpoints(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	s, edge, authored := weightedEndpointFixture(t)
	cut := big.NewRat(1, 3)
	out, h, e := s.SplitExactBezierTrimEdgeWithExactEndpoints(ctx, edge, cut, edgecheck.Options{})
	if e != nil {
		t.Fatal(e)
	}
	if h.First != edge || h.Second != len(s.edges) || h.Vertex != len(s.vertices) || len(out.vertices) != len(s.vertices)+1 {
		t.Fatal("topology history", h)
	}
	// Weighted parameter 1/3 maps to physical x=1/2.
	if out.vertices[h.Vertex] != (geom.Vec3{X: .5, Y: 11. / 16, Z: 3. / 16}) {
		t.Fatal("independent split station", out.vertices[h.Vertex])
	}
	var data bytes.Buffer
	if e = WriteJSON(&data, out); e != nil {
		t.Fatal("write", e)
	}
	out, e = ReadJSON(bytes.NewReader(data.Bytes()), JSONLimits{})
	if e != nil {
		t.Fatal("reload", e)
	}
	owners := 0
	roundingDifferences := 0
	for fi, f := range out.faces {
		for _, u := range f.Outer {
			if u.Edge != h.First && u.Edge != h.Second {
				continue
			}
			q, ok := u.Curve.(SplineUV)
			if !ok {
				t.Fatal("original source replaced")
			}
			if !q.ExactEndpoints {
				t.Fatal("missing endpoint mode")
			}
			legacy := q
			legacy.ExactEndpoints = false
			for _, t0 := range []float64{0, 1} {
				a, _ := q.EvalUV(t0)
				b, _ := legacy.EvalUV(t0)
				if a != b {
					roundingDifferences++
				}
			}
			old := authored[fi]
			if q.Reversed != old.Reversed || !reflect.DeepEqual(q.Curve.Knots(), old.Curve.Knots()) || !reflect.DeepEqual(q.Curve.Poles(), old.Curve.Poles()) || !reflect.DeepEqual(q.Curve.Weights(), old.Curve.Weights()) {
				t.Fatal("original author history lost")
			}
			if q.Range == old.Range || q.Range.Min < old.Range.Min || q.Range.Max > old.Range.Max {
				t.Fatal("incorrect restriction")
			}
			owners++
		}
	}
	if roundingDifferences == 0 {
		t.Fatal("fixture did not expose original rounding")
	}
	if owners != 4 {
		t.Fatal("both new seams need both old owners", owners)
	}
	o := StoredMixedClosedMeshOptions{Faces: make([]SolidMixedFaceMeshOptions, len(out.faces))}
	for fi, f := range out.faces {
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
	r, e := out.TessellateMixedClosedCertified(ctx, o)
	if e != nil {
		t.Fatal(e)
	}
	if r.SquaredDistance.Cmp(new(big.Rat).Mul(ratInset(.04), ratInset(.04))) > 0 {
		t.Fatal("two-sided deflection bound")
	}
	if e = r.Mesh.ValidateSolid(); e != nil || !r.DisplayCollision.Complete || len(r.DisplayCollision.Contacts) != 0 {
		t.Fatal("closed display", e)
	}
	if id, ok := r.Network.CornerVertices[h.Vertex]; !ok || r.Mesh.Vertices[id] != out.vertices[h.Vertex] {
		t.Fatal("new shared corner identity")
	}
	v, _ := r.DisplayVolume.Float64()
	if math.Abs(v-23./448) > .002 {
		t.Fatal("analytic volume", v)
	}
	for fi, f := range r.Network.Faces {
		for _, use := range f.Coedges {
			if use.Edge != h.First && use.Edge != h.Second {
				continue
			}
			q := out.faces[fi].Outer[use.Use].Curve.(SplineUV)
			patch, e := exactSplineSupportPatch(out.faces[fi].Surface, geom.SurfacePatchOptions{})
			if e != nil {
				t.Fatal(e)
			}
			for j, p := range use.AuthoredParameters {
				uv := authoredMixedSplinePoint(t, q, p)
				point, e := patch.EvaluateRational(ctx, uv[0], uv[1], geom.SurfacePatchEvaluationOptions{})
				if e != nil {
					t.Fatal(e)
				}
				id := r.Network.FaceVertices[fi][use.Vertices[j]]
				for k := 0; k < 3; k++ {
					if point.Point[k].Cmp(r.Network.ExactPoints[id][k]) != 0 {
						t.Fatal("split raw source identity")
					}
				}
			}
		}
	}
	for fi, f := range s.faces {
		for ui, u := range f.Outer {
			if u.Edge != edge {
				continue
			}
			q := u.Curve.(SplineUV)
			weights := q.Curve.Weights()
			weights[2] *= 2
			c, e := geom.NewBSpline(q.Curve.Degree(), q.Curve.Poles(), q.Curve.Knots(), weights)
			if e != nil {
				t.Fatal(e)
			}
			q.Curve = c
			bad := s
			bad.faces = s.Faces()
			bad.faces[fi].Outer[ui].Curve = q
			if result, history, e := bad.SplitExactBezierTrimEdgeWithExactEndpoints(ctx, edge, cut, edgecheck.Options{}); e == nil || len(result.faces) != 0 || history != (EdgeSplit{}) {
				t.Fatal("forged owner accepted or partial split", e)
			}
		}
	}
	limited, _, e := s.SplitExactBezierTrimEdgeWithExactEndpoints(ctx, edge, cut, edgecheck.Options{MaxWork: 1})
	if e == nil || len(limited.faces) != 0 {
		t.Fatal("atomic proof quota")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if q, _, e := s.SplitExactBezierTrimEdgeWithExactEndpoints(canceled, edge, cut, edgecheck.Options{}); !errors.Is(e, context.Canceled) || len(q.faces) != 0 {
		t.Fatal("atomic cancellation", e)
	}
	t.Logf("triangles=%d volume=%g original edge=%d children=%d,%d", len(r.Mesh.Triangles), v, edge, h.First, h.Second)
}

func TestExactSplineEndpointNativeGuards(t *testing.T) {
	s, edge, _ := weightedEndpointFixture(t)
	var legacy bytes.Buffer
	if e := WriteJSON(&legacy, s); e != nil {
		t.Fatal(e)
	}
	var old nativeSolid
	if e := json.Unmarshal(legacy.Bytes(), &old); e != nil {
		t.Fatal(e)
	}
	if old.Version >= 8 || bytes.Contains(legacy.Bytes(), []byte("SplineExactEndpoints")) {
		t.Fatal("legacy schema changed")
	}
	out, _, e := s.SplitExactBezierTrimEdgeWithExactEndpoints(context.Background(), edge, big.NewRat(1, 3), edgecheck.Options{})
	if e != nil {
		t.Fatal(e)
	}
	var data bytes.Buffer
	if e = WriteJSON(&data, out); e != nil {
		t.Fatal(e)
	}
	reloaded, e := ReadJSON(bytes.NewReader(data.Bytes()), JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	var again bytes.Buffer
	if e = WriteJSON(&again, reloaded); e != nil || !bytes.Equal(data.Bytes(), again.Bytes()) {
		t.Fatal("nondeterministic reload", e)
	}
	tests := map[string]func(*nativeSolid){
		"downgrade": func(n *nativeSolid) { n.Version = 7 },
		"mode removed": func(n *nativeSolid) {
			for i := range n.Faces {
				for j := range n.Faces[i].Outer {
					n.Faces[i].Outer[j].SplineExactEndpoints = false
				}
			}
		},
		"unsupported payload": func(n *nativeSolid) { n.Edges[0].P = &nativeUse{SplineExactEndpoints: true} },
		"false endpoint": func(n *nativeSolid) {
			for i := range n.Faces {
				for j := range n.Faces[i].Outer {
					u := &n.Faces[i].Outer[j]
					if u.SplineExactEndpoints {
						u.UVStart.X += 1
						return
					}
				}
			}
		},
		"source interior": func(n *nativeSolid) {
			for i := range n.Faces {
				for j := range n.Faces[i].Outer {
					u := &n.Faces[i].Outer[j]
					if !u.SplineExactEndpoints {
						continue
					}
					q := out.faces[i].Outer[j].Curve.(SplineUV)
					poles := q.Curve.Poles()
					poles[1].X += 1
					c, e := geom.NewBSpline(q.Curve.Degree(), poles, q.Curve.Knots(), q.Curve.Weights())
					if e != nil {
						t.Fatal(e)
					}
					u.Spline, e = parametric.EncodeCurve(c)
					if e != nil {
						t.Fatal(e)
					}
					return
				}
			}
		},
		"wrong range": func(n *nativeSolid) {
			for i := range n.Faces {
				for j := range n.Faces[i].Outer {
					u := &n.Faces[i].Outer[j]
					if u.SplineExactEndpoints {
						u.SplineRange.Min += .125
						return
					}
				}
			}
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			var n nativeSolid
			if e := json.Unmarshal(data.Bytes(), &n); e != nil {
				t.Fatal(e)
			}
			mutate(&n)
			b, e := json.Marshal(n)
			if e != nil {
				t.Fatal(e)
			}
			if result, e := ReadJSON(bytes.NewReader(b), JSONLimits{}); e == nil || len(result.faces) != 0 {
				t.Fatal("accepted forged mode/source", e)
			}
		})
	}
	// No cached endpoint or digest is trusted: the schema rejects extra claims.
	forged := bytes.Replace(data.Bytes(), []byte(`"SplineExactEndpoints":true`), []byte(`"SplineExactEndpoints":true,"SourceDigest":"stale"`), 1)
	if _, e := ReadJSON(bytes.NewReader(forged), JSONLimits{}); e == nil {
		t.Fatal("accepted stale unknown evidence")
	}
}
