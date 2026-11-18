package trimmed

import (
	"bytes"
	"context"
	"errors"
	"math"
	"math/big"
	"plainsolid/brep/edgecheck"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"reflect"
	"testing"
)

func TestMergeAuthoredCurvedSeam(t *testing.T) {
	slowtest.Skip(t)
	for _, v := range []struct {
		name    string
		reverse bool
	}{{"forward", false}, {"reversed_model", true}} {
		t.Run(v.name, func(t *testing.T) { checkMergeAuthoredCurvedSeam(t, v.reverse) })
	}
}
func checkMergeAuthoredCurvedSeam(t *testing.T, reverseModel bool) {
	ctx := context.Background()
	original, edge, authored := authoredCurvedSplitFixture(t)
	cut := big.NewRat(1, 4)
	if reverseModel {
		edges, faces := original.Edges(), original.Faces()
		m := edges[edge].Curve.(surfaceTrimCurve)
		m.a, m.b = m.b, m.a
		m.curve = m.curve.(ExactBezierUV).reversed()
		edges[edge].Curve = m
		edges[edge].Start, edges[edge].End = edges[edge].End, edges[edge].Start
		for fi := range faces {
			for ui := range faces[fi].Outer {
				if faces[fi].Outer[ui].Edge == edge {
					faces[fi].Outer[ui].Reversed = !faces[fi].Outer[ui].Reversed
				}
			}
		}
		var e error
		original, e = NewSolid(original.Vertices(), edges, faces, original.tolerance)
		if e != nil {
			t.Fatal(e)
		}
	}
	split, h, e := original.SplitExactBezierTrimEdge(ctx, edge, cut, edgecheck.Options{})
	if e != nil {
		t.Fatal(e)
	}
	var data bytes.Buffer
	if e = WriteJSON(&data, split); e != nil {
		t.Fatal(e)
	}
	split, e = ReadJSON(bytes.NewReader(data.Bytes()), JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	reject := func(s Solid, t0 *big.Rat) {
		t.Helper()
		out, h, e := s.MergeExactBezierTrimEdges(ctx, h.First, h.Second, t0)
		if e == nil || len(out.faces) != 0 || len(h.Edges) != 0 || len(h.Vertices) != 0 {
			t.Fatal("non-atomic forged merge", e)
		}
	}
	reject(split, big.NewRat(2, 5))
	for fi, f := range split.faces {
		for ui, u := range f.Outer {
			if u.Edge != h.Second {
				continue
			}
			source := u.Curve.(SplineUV)
			for _, delta := range []float64{-.125, .125} {
				bad := split
				bad.faces = split.Faces()
				q := source
				q.Range.Min += delta
				bad.faces[fi].Outer[ui].Curve = q
				reject(bad, cut)
			}
			// Reauthoring just this child's identical trace on a fresh knot
			// net is geometrically valid but is not the unchanged full source
			// shared by the two child ranges.
			bad := split
			bad.faces = split.Faces()
			q := knotOriginalMixedUse(t, source, []*big.Rat{big.NewRat(1, 4)})
			bad.faces[fi].Outer[ui].Curve = q
			var valid bytes.Buffer
			if e := WriteJSON(&valid, bad); e != nil {
				t.Fatal("replacement must remain geometrically valid", e)
			}
			reject(bad, cut)
		}
	}
	// Complete same-domain exact support identity is required after reload;
	// endpoint coincidence does not authorize changing the source support.
	bad := split
	bad.edges = split.Edges()
	m := bad.edges[h.Second].Curve.(surfaceTrimCurve)
	p := m.surface.(*geom.ExactBezierSurface).ExactPatch()
	recipe, e := p.ExportExactBezierRecipe()
	if e != nil {
		t.Fatal(e)
	}
	z, ok := new(big.Rat).SetString(recipe.Controls[1][2])
	if !ok {
		t.Fatal("coefficient")
	}
	recipe.Controls[1][2] = z.Add(z, big.NewRat(1, 1024)).RatString()
	p, e = geom.ImportExactBezierSurfaceRecipe(ctx, recipe, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	m.surface, e = geom.NewExactBezierSurface(p)
	if e != nil {
		t.Fatal(e)
	}
	bad.edges[h.Second].Curve = m
	reject(bad, cut)
	merged, history, e := split.MergeExactBezierTrimEdges(ctx, h.First, h.Second, cut)
	if e != nil {
		t.Fatal(e)
	}
	if history.RemovedVertex != h.Vertex || history.Edges[h.First] != history.Edges[h.Second] || len(merged.vertices) != len(original.vertices) || len(merged.edges) != len(original.edges) {
		t.Fatal("merge topology history", history)
	}
	before := original.edges[edge].Curve.(surfaceTrimCurve).curve.(ExactBezierUV).HomogeneousControls()
	after := merged.edges[history.Edge].Curve.(surfaceTrimCurve).curve.(ExactBezierUV).HomogeneousControls()
	for i := range before {
		for k := 0; k < 3; k++ {
			if before[i][k].Cmp(after[i][k]) != 0 {
				t.Fatal("original model coefficients not restored")
			}
		}
	}
	owners, reversed := 0, 0
	for fi, f := range merged.faces {
		for _, u := range f.Outer {
			if u.Edge != history.Edge {
				continue
			}
			q := u.Curve.(SplineUV)
			old := authored[fi]
			if q.Range != old.Range || q.Reversed != old.Reversed || !reflect.DeepEqual(q.Curve.Knots(), old.Curve.Knots()) || !reflect.DeepEqual(q.Curve.Poles(), old.Curve.Poles()) || !reflect.DeepEqual(q.Curve.Weights(), old.Curve.Weights()) {
				t.Fatal("original source not restored")
			}
			owners++
			if q.Reversed {
				reversed++
			}
		}
	}
	if owners != 2 || reversed != 1 {
		t.Fatal("original opposite/reversed owner history")
	}
	var encoded bytes.Buffer
	if e = WriteJSON(&encoded, merged); e != nil {
		t.Fatal(e)
	}
	merged, e = ReadJSON(bytes.NewReader(encoded.Bytes()), JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	var repeat bytes.Buffer
	if e = WriteJSON(&repeat, merged); e != nil || !bytes.Equal(repeat.Bytes(), encoded.Bytes()) {
		t.Fatal("non-deterministic source recovery", e)
	}
	o := StoredMixedClosedMeshOptions{Faces: make([]SolidMixedFaceMeshOptions, len(merged.faces))}
	for fi, f := range merged.faces {
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
	mesh, e := merged.TessellateMixedClosedCertified(ctx, o)
	if e != nil {
		t.Fatal(e)
	}
	if e = mesh.Mesh.ValidateSolid(); e != nil || !mesh.DisplayCollision.Complete || len(mesh.DisplayCollision.Contacts) != 0 {
		t.Fatal("closed display", e)
	}
	v, _ := mesh.DisplayVolume.Float64()
	if math.Abs(v-23./448) > .002 {
		t.Fatal("independent analytic volume", v)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if q, _, e := split.MergeExactBezierTrimEdges(canceled, h.First, h.Second, cut); !errors.Is(e, context.Canceled) || len(q.faces) != 0 {
		t.Fatal("cancellation", e)
	}
	t.Logf("triangles=%d volume=%g restored edge=%d", len(mesh.Mesh.Triangles), v, history.Edge)
}
