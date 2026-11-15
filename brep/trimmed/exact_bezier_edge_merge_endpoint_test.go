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

func TestMergeWeightedEndpointSources(t *testing.T) {
	ctx := context.Background()
	original, edge, authored := weightedEndpointFixture(t)
	cut := big.NewRat(1, 3)
	split, h, e := original.SplitExactBezierTrimEdgeWithExactEndpoints(ctx, edge, cut, edgecheck.Options{})
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
	reject(split, big.NewRat(1, 4))
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
			if q.ExactEndpoints {
				t.Fatal("restored exact legacy endpoints must normalize mode")
			}
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
	if bytes.Contains(encoded.Bytes(), []byte("SplineExactEndpoints")) {
		t.Fatal("redundant v8 mode persisted")
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
	if mesh.SquaredDistance.Cmp(new(big.Rat).Mul(ratInset(.04), ratInset(.04))) > 0 {
		t.Fatal("deflection bound")
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

func TestJoinStoredEndpointModeNormalization(t *testing.T) {
	ctx := context.Background()
	// Independent curved polynomial: changing the mode does not change source
	// identity when the actual exact endpoints agree on each original child.
	c, e := geom.NewBSpline(3, []geom.Vec3{{}, {X: 1, Y: 2}, {X: 2, Y: 1}, {X: 3}}, []float64{0, 0, 0, 0, 4, 4, 4, 4}, nil)
	if e != nil {
		t.Fatal(e)
	}
	a := SplineUV{Curve: c, Range: UVInterval{0, 2}, ExactEndpoints: true}
	b := SplineUV{Curve: c, Range: UVInterval{2, 4}}
	for _, reverse := range []bool{false, true} {
		x, y := a, b
		if reverse {
			x, y = b, a
			x.Reversed = true
			y.Reversed = true
		}
		result, _, e := joinStoredBezierSources(ctx, x, y, big.NewRat(1, 2))
		if e != nil {
			t.Fatal(e)
		}
		q := result.(SplineUV)
		if q.ExactEndpoints || q.Range != (UVInterval{0, 4}) || q.Reversed != reverse || q.Curve != c {
			t.Fatal("source mode normalization", q)
		}
	}
	// An incomplete original source range may still need exact endpoint mode.
	// The forward rational speed maps fractions 1/3,1/2,1 to x=1/2,2/3,1;
	// this original owner stores that traversal in reverse.
	// the independently scaled UV chart makes all three exact binary points.
	_, _, sources := weightedEndpointFixture(t)
	for _, source := range sources {
		if !source.Reversed {
			continue
		}
		q := source
		start := q.Range.Min
		c := q.Curve
		q.Range = UVInterval{start, start + 2}
		q, e = q.WithExactEndpoints()
		if e != nil {
			t.Fatal(e)
		}
		x, y := q, q
		x.Range.Min = start + 1.5
		y.Range.Max = start + 1.5
		result, ends, e := joinStoredBezierSources(ctx, x, y, big.NewRat(1, 4))
		if e != nil {
			t.Fatal(e)
		}
		got := result.(SplineUV)
		if !got.ExactEndpoints || got.Range != q.Range || got.Curve != c {
			t.Fatal("required mode lost")
		}
		for j := 0; j < 2; j++ {
			v, e := got.EvalUV(float64(j))
			if e != nil || v != ends[j] {
				t.Fatal("exact restricted endpoint", e)
			}
		}
		x.ExactEndpoints = false
		if _, _, e = joinStoredBezierSources(ctx, x, y, big.NewRat(1, 4)); e == nil {
			t.Fatal("silently repaired rounded child")
		}
		return
	}
	t.Fatal("no reversed weighted source")
}
