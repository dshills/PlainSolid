package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"math/big"
	"plainsolid/brep/edgecheck"
	"plainsolid/geom"
	"testing"
)

func TestPiecewiseMappedSourceRepair(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		prefix := "forward"
		if reverse {
			prefix = "reversed_source"
		}
		for _, v := range []struct {
			name string
			cut  *big.Rat
		}{{"inside_first", big.NewRat(3, 16)}, {"original_knot", big.NewRat(3, 8)}, {"inside_second", big.NewRat(11, 16)}} {
			t.Run(prefix+"/"+v.name, func(t *testing.T) { checkPiecewiseMappedSourceRepair(t, v.cut, reverse) })
		}
	}
}
func checkPiecewiseMappedSourceRepair(t *testing.T, local *big.Rat, reverse bool) {
	ctx := context.Background()
	s, edge, owner, original := piecewiseSharedSeamFixture(t)
	source := original.Source()
	knots := source.Curve.Knots()
	for i, k := range knots {
		if k == 4 {
			knots[i] = 8
		}
		if k == 10 {
			knots[i] = 14
		}
	}
	c, e := geom.NewBSpline(source.Curve.Degree(), source.Curve.Poles(), knots, source.Curve.Weights())
	if e != nil {
		t.Fatal(e)
	}
	source.Curve = c
	source.Range.Max = 14
	if reverse {
		poles, weights, ks := source.Curve.Poles(), source.Curve.Weights(), source.Curve.Knots()
		for i, j := 0, len(poles)-1; i < j; i, j = i+1, j-1 {
			poles[i], poles[j] = poles[j], poles[i]
			weights[i], weights[j] = weights[j], weights[i]
		}
		for i, j := 0, len(ks)-1; i <= j; i, j = i+1, j-1 {
			a, b := ks[i], ks[j]
			ks[i], ks[j] = 16-b, 16-a
		}
		source.Curve, e = geom.NewBSpline(source.Curve.Degree(), poles, ks, weights)
		if e != nil {
			t.Fatal(e)
		}
		source.Reversed = true
	}
	original, e = NewPiecewiseMappedSplineUV(ctx, source, original.ModelStation(), original.ParameterWeights(), geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	faces := s.Faces()
	cut := new(big.Rat).Set(local)
	for ui, u := range faces[owner].Outer {
		if u.Edge == edge {
			faces[owner].Outer[ui].Curve = original
			if u.Reversed {
				cut.Sub(big.NewRat(1, 1), local)
			}
		}
	}
	s, e = NewSolid(s.Vertices(), s.Edges(), faces, s.tolerance)
	if e != nil {
		t.Fatal(e)
	}
	split, h, e := s.SplitExactBezierTrimEdge(ctx, edge, cut, edgecheck.Options{})
	if e != nil {
		t.Fatal("split", e)
	}
	x0, x1 := s.vertices[s.edges[edge].Start].X, s.vertices[s.edges[edge].End].X
	tf, _ := cut.Float64()
	x := x0 + (x1-x0)*tf
	if split.vertices[h.Vertex] != (geom.Vec3{X: x, Y: .75 - .5*x*x*x, Z: 3. / 16}) {
		t.Fatal("independent dyadic model point", split.vertices[h.Vertex])
	}
	reload := func(s Solid, version int) Solid {
		t.Helper()
		var b bytes.Buffer
		if e := WriteJSON(&b, s); e != nil {
			t.Fatal(e)
		}
		var n nativeSolid
		if e := json.Unmarshal(b.Bytes(), &n); e != nil || n.Version != version {
			t.Fatal("recipe version", e, n.Version)
		}
		out, e := ReadJSON(bytes.NewReader(b.Bytes()), JSONLimits{})
		if e != nil {
			t.Fatal("reload", e)
		}
		var again bytes.Buffer
		if e = WriteJSON(&again, out); e != nil || !bytes.Equal(again.Bytes(), b.Bytes()) {
			t.Fatal("deterministic reload", e)
		}
		return out
	}
	version := 10
	if local.Cmp(original.station) == 0 {
		version = 9
	}
	split = reload(split, version)
	children := 0
	for _, u := range split.faces[owner].Outer {
		if u.Edge != h.First && u.Edge != h.Second {
			continue
		}
		_, q, ok := mappedRepairSource(u.Curve)
		if !ok || !sameAuthoredBezierSpline(q.Curve, original.source.Curve) || q.Reversed != original.source.Reversed || q.Range.Min < source.Range.Min || q.Range.Max > source.Range.Max {
			t.Fatal("original source child")
		}
		children++
	}
	if children != 2 {
		t.Fatal("child ownership")
	}
	// Genuine changed source parameter identity is rejected even when the child
	// stays geometrically identical and passes native persistence by itself.
	bad := split
	bad.faces = split.Faces()
	mutated := false
	for ui, u := range bad.faces[owner].Outer {
		q, ok := u.Curve.(MappedSplineUV)
		if !ok {
			continue
		}
		raw := q.Source()
		ks := raw.Curve.Knots()
		for i := range ks {
			ks[i] += 16
		}
		raw.Range.Min += 16
		raw.Range.Max += 16
		raw.Curve, e = geom.NewBSpline(raw.Curve.Degree(), raw.Curve.Poles(), ks, raw.Curve.Weights())
		if e != nil {
			t.Fatal(e)
		}
		mapped, e := NewMappedSplineUV(ctx, raw, q.ParameterWeight(), geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		bad.faces[owner].Outer[ui].Curve = mapped
		mutated = true
		break
	}
	if !mutated {
		t.Fatal("missing single-span child")
	}
	var valid bytes.Buffer
	if e = WriteJSON(&valid, bad); e != nil {
		t.Fatal("reauthored child must remain geometrically valid", e)
	}
	if out, history, e := bad.MergeExactBezierTrimEdges(ctx, h.First, h.Second, cut); e == nil || len(out.faces) != 0 || len(history.Edges) != 0 {
		t.Fatal("changed source merged", e)
	}
	merged, history, e := split.MergeExactBezierTrimEdges(ctx, h.First, h.Second, cut)
	if e != nil {
		t.Fatal("merge", e)
	}
	merged = reload(merged, 10)
	found := false
	for _, u := range merged.faces[owner].Outer {
		if u.Edge != history.Edge {
			continue
		}
		q, ok := u.Curve.(PiecewiseMappedSplineUV)
		if !ok || q.source.Range != original.source.Range || !sameAuthoredBezierSpline(q.source.Curve, original.source.Curve) || q.station.Cmp(original.station) != 0 {
			t.Fatal("source graph not restored")
		}
		for i, w := range q.ParameterWeights() {
			if w.Cmp(original.ParameterWeights()[i]) != 0 {
				t.Fatal("piece speed not restored")
			}
		}
		found = true
	}
	if !found {
		t.Fatal("lost owner")
	}
	for phase, solid := range []Solid{split, merged} {
		o := StoredMixedClosedMeshOptions{Faces: make([]SolidMixedFaceMeshOptions, len(solid.faces))}
		for fi, f := range solid.faces {
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
		r, e := solid.TessellateMixedClosedCertified(ctx, o)
		if e != nil {
			t.Fatal("closed mesh", e)
		}
		if e = r.Mesh.ValidateSolid(); e != nil || !r.DisplayCollision.Complete || len(r.DisplayCollision.Contacts) != 0 {
			t.Fatal("closed contact", e)
		}
		v, _ := r.DisplayVolume.Float64()
		if math.Abs(v-23./448) > .002 || r.SquaredDistance.Cmp(new(big.Rat).Mul(ratInset(.04), ratInset(.04))) > 0 {
			t.Fatal("mass/deflection", v)
		}
		if phase == 0 {
			id, ok := r.Network.CornerVertices[h.Vertex]
			if !ok || r.Mesh.Vertices[id] != split.vertices[h.Vertex] {
				t.Fatal("original split corner identity")
			}
		}
		for _, use := range r.Network.Faces[owner].Coedges {
			_, raw, ok := mappedRepairSource(solid.faces[owner].Outer[use.Use].Curve)
			if !ok {
				continue
			}
			var knot *big.Rat
			if q, ok := solid.faces[owner].Outer[use.Use].Curve.(PiecewiseMappedSplineUV); ok {
				knot = q.ModelStation()
			}
			seen := knot == nil
			patch, e := exactSplineSupportPatch(solid.faces[owner].Surface, geom.SurfacePatchOptions{})
			if e != nil {
				t.Fatal(e)
			}
			for j, p := range use.AuthoredParameters {
				uv := authoredMixedSplinePoint(t, raw, p)
				point, e := patch.EvaluateRational(ctx, uv[0], uv[1], geom.SurfacePatchEvaluationOptions{})
				if e != nil {
					t.Fatal(e)
				}
				id := r.Network.FaceVertices[owner][use.Vertices[j]]
				if knot != nil && use.TrimParameters[j].Cmp(knot) == 0 {
					seen = true
					if p.Cmp(big.NewRat(8, 1)) != 0 {
						t.Fatal("active source knot")
					}
					opposite := 0
					for fi, f := range r.Network.Faces {
						if fi == owner {
							continue
						}
						for _, other := range f.Coedges {
							if other.Edge != use.Edge {
								continue
							}
							for k, parameter := range other.ModelParameters {
								if parameter.Cmp(use.ModelParameters[j]) == 0 {
									if r.Network.FaceVertices[fi][other.Vertices[k]] != id {
										t.Fatal("shared restricted knot ID")
									}
									opposite++
								}
							}
						}
					}
					if opposite != 1 {
						t.Fatal("opposite owner knot missing")
					}
				}
				for k := 0; k < 3; k++ {
					if point.Point[k].Cmp(r.Network.ExactPoints[id][k]) != 0 {
						t.Fatal("original raw source station")
					}
				}
			}
			if !seen {
				t.Fatal("restricted original knot not sampled")
			}
		}
		t.Logf("triangles=%d volume=%g", len(r.Mesh.Triangles), v)
	}
	if out, _, e := s.SplitExactBezierTrimEdge(ctx, edge, cut, edgecheck.Options{MaxWork: 1}); e == nil || len(out.faces) != 0 {
		t.Fatal("atomic quota")
	}
}

func TestPiecewiseMappedRepairOwnerGuards(t *testing.T) {
	_, _, _, q := piecewiseSharedSeamFixture(t)
	for _, a := range []PCurve{q, q.parts[0]} {
		if result, _, e := joinPiecewiseMappedSources(context.Background(), a, q.Source(), big.NewRat(1, 2)); !errors.Is(e, geom.ErrInvalid) || result != nil {
			t.Fatal("unsupported owner type", e)
		}
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := restrictPiecewiseMappedSource(canceled, q, new(big.Rat), big.NewRat(1, 1), geom.SurfacePatchOptions{}); !errors.Is(e, context.Canceled) {
		t.Fatal("restriction cancellation", e)
	}
}
