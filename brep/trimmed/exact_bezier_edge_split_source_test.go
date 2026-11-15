package trimmed

import (
	"bytes"
	"context"
	"errors"
	"math"
	"math/big"
	"plainsolid/brep/edgecheck"
	"plainsolid/geom"
	"reflect"
	"testing"
)

func authoredCurvedSplitFixture(t *testing.T) (Solid, int, map[int]SplineUV) {
	t.Helper()
	ctx := context.Background()
	weighted, weightedEdge := storedMixedWeightedKnotsFixture(t)
	// A binary physical/UV point does not justify rounding the raw authored
	// station. Width-four rational source domains cannot represent t=1/3.
	if out, _, e := weighted.SplitExactBezierTrimEdge(ctx, weightedEdge, big.NewRat(1, 3), edgecheck.Options{}); !errors.Is(e, geom.ErrUnrepresentable) || len(out.faces) != 0 {
		t.Fatal("nonbinary raw source split", e)
	}
	original, edge := storedMixedCurvedFixture(t, false)
	original, _ = storedMixedAffineSideFixture(t, original)
	faces := original.Faces()
	authored := map[int]SplineUV{}
	owner := 0
	for fi := range faces {
		for ui, u := range faces[fi].Outer {
			if u.Edge != edge {
				continue
			}
			q := u.Curve.(SplineUV)
			cuts := []*big.Rat{big.NewRat(1, 2)}
			if owner == 1 {
				cuts = []*big.Rat{big.NewRat(1, 4), big.NewRat(3, 4)}
			}
			q = knotOriginalMixedUse(t, q, cuts)
			if owner == 1 {
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
			authored[fi] = q
			owner++
		}
	}
	s, e := NewSolid(original.Vertices(), original.Edges(), faces, original.tolerance)
	if e != nil {
		t.Fatal(e)
	}
	return s, edge, authored
}

func TestSplitAuthoredCurvedSeam(t *testing.T) {
	ctx := context.Background()
	s, edge, authored := authoredCurvedSplitFixture(t)
	cut := big.NewRat(1, 2)
	out, h, e := s.SplitExactBezierTrimEdge(ctx, edge, cut, edgecheck.Options{})
	if e != nil {
		t.Fatal(e)
	}
	if h.First != edge || h.Second != len(s.edges) || h.Vertex != len(s.vertices) || len(out.vertices) != len(s.vertices)+1 {
		t.Fatal("topology history", h)
	}
	// t=1/2 on the original cubic seam, y=3/4-x^3/2.
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
	for fi, f := range out.faces {
		for _, u := range f.Outer {
			if u.Edge != h.First && u.Edge != h.Second {
				continue
			}
			q, ok := u.Curve.(SplineUV)
			if !ok {
				t.Fatal("original source replaced")
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
			if result, history, e := bad.SplitExactBezierTrimEdge(ctx, edge, cut, edgecheck.Options{}); e == nil || len(result.faces) != 0 || history != (EdgeSplit{}) {
				t.Fatal("forged owner accepted or partial split", e)
			}
		}
	}
	limited, _, e := s.SplitExactBezierTrimEdge(ctx, edge, cut, edgecheck.Options{MaxWork: 1})
	if e == nil || len(limited.faces) != 0 {
		t.Fatal("atomic proof quota")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if q, _, e := s.SplitExactBezierTrimEdge(canceled, edge, cut, edgecheck.Options{}); !errors.Is(e, context.Canceled) || len(q.faces) != 0 {
		t.Fatal("atomic cancellation", e)
	}
	t.Logf("triangles=%d volume=%g original edge=%d children=%d,%d", len(r.Mesh.Triangles), v, edge, h.First, h.Second)
}
