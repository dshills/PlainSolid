package trimmed

import (
	"bytes"
	"context"
	"errors"
	"math"
	"math/big"
	"plainsolid/brep/edgecheck"
	"plainsolid/geom"
	"testing"
)

func TestMappedSplineSourceRepair(t *testing.T) {
	for _, rev := range []bool{false, true} {
		name := "forward"
		if rev {
			name = "reversed_source"
		}
		t.Run(name, func(t *testing.T) { checkMappedSplineSourceRepair(t, rev) })
	}
}
func checkMappedSplineSourceRepair(t *testing.T, reverse bool) {
	ctx := context.Background()
	original, edge, owner, _ := mappedSharedSeamFixtureReversed(t, reverse)
	if out, _, e := original.SplitExactBezierTrimEdge(ctx, edge, big.NewRat(1, 2), edgecheck.Options{}); !errors.Is(e, geom.ErrUnrepresentable) || len(out.faces) != 0 {
		t.Fatal("nonbinary original raw station must reject", e)
	}
	faces := original.Faces()
	var authored MappedSplineUV
	for ui, u := range faces[owner].Outer {
		if u.Edge != edge {
			continue
		}
		q := u.Curve.(MappedSplineUV)
		source := q.Source()
		knots := source.Curve.Knots()
		for i := range knots {
			knots[i] = source.Range.Min + (knots[i]-source.Range.Min)*.75
		}
		c, e := geom.NewBSpline(source.Curve.Degree(), source.Curve.Poles(), knots, source.Curve.Weights())
		if e != nil {
			t.Fatal(e)
		}
		source.Curve = c
		source.Range.Max = source.Range.Min + 3
		authored, e = NewMappedSplineUV(ctx, source, q.ParameterWeight(), geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		faces[owner].Outer[ui].Curve = authored
	}
	var e error
	original, e = NewSolid(original.Vertices(), original.Edges(), faces, original.tolerance)
	if e != nil {
		t.Fatal(e)
	}
	split, h, e := original.SplitExactBezierTrimEdge(ctx, edge, big.NewRat(1, 2), edgecheck.Options{})
	if e != nil {
		t.Fatal("mapped split", e)
	}
	if split.vertices[h.Vertex] != (geom.Vec3{X: .5, Y: 11. / 16, Z: 3. / 16}) {
		t.Fatal("independent seam station", split.vertices[h.Vertex])
	}
	reload := func(s Solid) Solid {
		t.Helper()
		var data bytes.Buffer
		if e := WriteJSON(&data, s); e != nil {
			t.Fatal(e)
		}
		out, e := ReadJSON(bytes.NewReader(data.Bytes()), JSONLimits{})
		if e != nil {
			t.Fatal(e)
		}
		var again bytes.Buffer
		if e = WriteJSON(&again, out); e != nil || !bytes.Equal(data.Bytes(), again.Bytes()) {
			t.Fatal("native deterministic reload", e)
		}
		return out
	}
	split = reload(split)
	count := 0
	for _, u := range split.faces[owner].Outer {
		if u.Edge != h.First && u.Edge != h.Second {
			continue
		}
		q, ok := u.Curve.(MappedSplineUV)
		if !ok {
			t.Fatal("mapped source discarded")
		}
		source := q.Source()
		if !sameAuthoredBezierSpline(source.Curve, authored.source.Curve) || source.Reversed != authored.source.Reversed || source.ExactEndpoints != authored.source.ExactEndpoints {
			t.Fatal("source history changed")
		}
		if q.weight.Cmp(big.NewRat(3, 4)) != 0 && q.weight.Cmp(big.NewRat(2, 3)) != 0 {
			t.Fatal("child map coefficient", q.weight)
		}
		count++
	}
	if count != 2 {
		t.Fatal("child owner count")
	}
	reject := func(s Solid, t0 *big.Rat) {
		t.Helper()
		out, h, e := s.MergeExactBezierTrimEdges(ctx, h.First, h.Second, t0)
		if e == nil || len(out.faces) != 0 || len(h.Edges) != 0 {
			t.Fatal("non-atomic forged merge", e)
		}
	}
	reject(split, big.NewRat(1, 3))
	for ui, u := range split.faces[owner].Outer {
		q, ok := u.Curve.(MappedSplineUV)
		if !ok {
			continue
		}
		for _, mutate := range []string{"map", "range", "source"} {
			bad := split
			bad.faces = split.Faces()
			source := q.Source()
			weight := q.ParameterWeight()
			switch mutate {
			case "map":
				weight.Mul(weight, big.NewRat(2, 1))
			case "range":
				source.Range.Min += .125
			case "source":
				knots := source.Curve.Knots()
				for i := range knots {
					knots[i] += 16
				}
				source.Range.Min += 16
				source.Range.Max += 16
				source.Curve, e = geom.NewBSpline(source.Curve.Degree(), source.Curve.Poles(), knots, source.Curve.Weights())
				if e != nil {
					t.Fatal(e)
				}
			}
			forged, e := NewMappedSplineUV(ctx, source, weight, geom.SurfacePatchOptions{})
			if e != nil {
				if mutate == "range" {
					continue
				}
				t.Fatal(e)
			}
			bad.faces[owner].Outer[ui].Curve = forged
			if mutate == "source" {
				var valid bytes.Buffer
				if e = WriteJSON(&valid, bad); e != nil {
					t.Fatal("alternate source must remain geometrically valid", e)
				}
			}
			reject(bad, big.NewRat(1, 2))
		}
		break
	}
	merged, history, e := split.MergeExactBezierTrimEdges(ctx, h.First, h.Second, big.NewRat(1, 2))
	if e != nil {
		t.Fatal("mapped merge", e)
	}
	if history.RemovedVertex != h.Vertex || len(merged.vertices) != len(original.vertices) || len(merged.edges) != len(original.edges) {
		t.Fatal("topology history")
	}
	merged = reload(merged)
	before := original.edges[edge].Curve.(surfaceTrimCurve).curve.(ExactBezierUV).HomogeneousControls()
	after := merged.edges[history.Edge].Curve.(surfaceTrimCurve).curve.(ExactBezierUV).HomogeneousControls()
	if len(before) != len(after) {
		t.Fatal("model degree changed")
	}
	for i := range before {
		for k := 0; k < 3; k++ {
			if before[i][k].Cmp(after[i][k]) != 0 {
				t.Fatal("original model coefficients changed")
			}
		}
	}

	for _, u := range merged.faces[owner].Outer {
		if u.Edge != history.Edge {
			continue
		}
		q := u.Curve.(MappedSplineUV)
		if q.source.Range != authored.source.Range || !sameAuthoredBezierSpline(q.source.Curve, authored.source.Curve) || q.weight.Cmp(authored.weight) != 0 || q.source.Reversed != authored.source.Reversed || q.source.ExactEndpoints != authored.source.ExactEndpoints {
			t.Fatal("whole authored source/map not restored")
		}
	}
	for _, s := range []Solid{split, merged} {
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
			t.Fatal("closed", e)
		}
		v, _ := r.DisplayVolume.Float64()
		if math.Abs(v-23./448) > .002 || r.SquaredDistance.Cmp(new(big.Rat).Mul(ratInset(.04), ratInset(.04))) > 0 {
			t.Fatal("analytic volume/distance", v)
		}
		for _, use := range r.Network.Faces[owner].Coedges {
			q, ok := s.faces[owner].Outer[use.Use].Curve.(MappedSplineUV)
			if !ok {
				continue
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
						t.Fatal("repaired authored parameter lineage")
					}
				}
			}
		}
		t.Logf("triangles=%d volume=%g", len(r.Mesh.Triangles), v)
	}
	if out, _, e := original.SplitExactBezierTrimEdge(ctx, edge, big.NewRat(1, 2), edgecheck.Options{MaxWork: 1}); e == nil || len(out.faces) != 0 {
		t.Fatal("split quota")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if out, _, e := split.MergeExactBezierTrimEdges(canceled, h.First, h.Second, big.NewRat(1, 2)); !errors.Is(e, context.Canceled) || len(out.faces) != 0 {
		t.Fatal("merge cancellation", e)
	}
}

func TestMappedSplineSourceRepairMaps(t *testing.T) {
	ctx := context.Background()
	_, _, _, source := mappedSharedSeamFixture(t)
	knots := source.Curve.Knots()
	lo := source.Range.Min
	for i := range knots {
		knots[i] = lo + (knots[i]-lo)*35/4
	}
	c, e := geom.NewBSpline(source.Curve.Degree(), source.Curve.Poles(), knots, source.Curve.Weights())
	if e != nil {
		t.Fatal(e)
	}
	source.Curve = c
	source.Range.Max = lo + 35
	q, e := NewMappedSplineUV(ctx, source, big.NewRat(1, 2), geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for _, cut := range []*big.Rat{big.NewRat(1, 4), big.NewRat(3, 4)} {
		a, b, e := q.curve.Split(cut)
		if e != nil {
			t.Fatal(e)
		}
		first, second, e := splitMappedSplineSource(ctx, q, cut, a, b, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		for i, part := range []PCurve{first, second} {
			child := part.(MappedSplineUV)
			for _, t0 := range []*big.Rat{new(big.Rat), big.NewRat(1, 3), big.NewRat(1, 2), big.NewRat(2, 3), big.NewRat(1, 1)} {
				global := new(big.Rat).Mul(cut, t0)
				if i == 1 {
					global.Add(cut, new(big.Rat).Mul(new(big.Rat).Sub(big.NewRat(1, 1), cut), t0))
				}
				want, e := q.AuthoredParameter(global)
				if e != nil {
					t.Fatal(e)
				}
				got, e := child.AuthoredParameter(t0)
				if e != nil || got.Cmp(want) != 0 {
					t.Fatal("independent raw parameter composition", cut, i, t0, e)
				}
			}
		}
		joined, _, e := joinMappedSplineSources(ctx, first.(MappedSplineUV), second.(MappedSplineUV), cut)
		if e != nil {
			t.Fatal(e)
		}
		got := joined.(MappedSplineUV)
		if got.source.Range != q.source.Range || got.weight.Cmp(q.weight) != 0 {
			t.Fatal("asymmetric map restoration")
		}
	}
}
