//go:build research

package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"math/big"
	"plainsolid/brep/edgecheck"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"testing"
)

func independentlyMappedOwner(t *testing.T, source SplineUV) MultiSpanMappedSplineUV {
	t.Helper()
	ctx := context.Background()
	base, e := mixedStoredHigherSplineUV(ctx, source, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	count, reverse := 3, true
	stations := []*big.Rat{big.NewRat(1, 8), big.NewRat(1, 2)}
	speeds := []*big.Rat{big.NewRat(4, 1), big.NewRat(2, 1), big.NewRat(1, 2)}
	rawCuts := []float64{20, 25, 31, 38}
	var controls [][3]*big.Rat
	var weights []*big.Rat
	a := new(big.Rat)
	for i := 0; i < count; i++ {
		b := big.NewRat(1, 1)
		if i < len(stations) {
			b = stations[i]
		}
		piece, e := restrictMappedProofCurve(base.(ExactBezierUV), a, b)
		if e != nil {
			t.Fatal(e)
		}
		h := piece.HomogeneousControls()
		power := big.NewRat(1, 1)
		for j := range h {
			if j > 0 {
				power.Mul(power, speeds[i])
			}
			for k := 0; k < 3; k++ {
				h[j][k].Mul(h[j][k], power)
			}
		}
		if len(controls) > 0 {
			scale := new(big.Rat).Quo(controls[len(controls)-1][2], h[0][2])
			for j := range h {
				for k := 0; k < 3; k++ {
					h[j][k].Mul(h[j][k], scale)
				}
			}
			controls = append(controls, h[1:]...)
		} else {
			controls = append(controls, h...)
		}
		weights = append(weights, new(big.Rat).Inv(speeds[i]))
		a = b
	}
	var poles []geom.Vec3
	var rawWeights []float64
	for _, h := range controls {
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
		rawWeights = append(rawWeights, w)
	}
	n := source.Curve.Degree()
	var knots []float64
	for i, v := range rawCuts {
		m := n
		if i == 0 || i == count {
			m++
		}
		for j := 0; j < m; j++ {
			knots = append(knots, v)
		}
	}
	if reverse {
		for i, j := 0, len(poles)-1; i < j; i, j = i+1, j-1 {
			poles[i], poles[j] = poles[j], poles[i]
			rawWeights[i], rawWeights[j] = rawWeights[j], rawWeights[i]
		}
		sum := rawCuts[0] + rawCuts[count]
		for i, j := 0, len(knots)-1; i <= j; i, j = i+1, j-1 {
			a, b := knots[i], knots[j]
			knots[i], knots[j] = sum-b, sum-a
		}
	}
	c, e := geom.NewBSpline(n, poles, knots, rawWeights)
	if e != nil {
		t.Fatal(e)
	}
	source = SplineUV{Curve: c, Range: UVInterval{rawCuts[0], rawCuts[count]}, Reversed: reverse}
	if _, e = NewMappedSplineUV(ctx, source, big.NewRat(1, 1), geom.SurfacePatchOptions{}); e == nil {
		t.Fatal("not a genuine multispan source")
	}
	q, e := NewMultiSpanMappedSplineUV(ctx, source, stations, weights, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}

	return q
}
func TestStoredMixedBothMappedOwners(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	s, edge, first, original := multispanSharedSeamFixture(t, 3, false)
	faces := s.Faces()
	second := -1
	expected := map[int]MultiSpanMappedSplineUV{first: original}
	for fi, f := range faces {
		if fi == first {
			continue
		}
		for ui, u := range f.Outer {
			if u.Edge == edge {
				q := independentlyMappedOwner(t, u.Curve.(SplineUV))
				faces[fi].Outer[ui].Curve = q
				expected[fi] = q
				second = fi
			}
		}
	}
	if second < 0 {
		t.Fatal("missing second owner")
	}
	s, e := NewSolid(s.Vertices(), s.Edges(), faces, s.tolerance)
	if e != nil {
		t.Fatal(e)
	}
	reload := func(x Solid) Solid {
		var a, b bytes.Buffer
		if e := WriteJSON(&a, x); e != nil {
			t.Fatal(e)
		}
		out, e := ReadJSON(bytes.NewReader(a.Bytes()), JSONLimits{})
		if e != nil {
			t.Fatal(e)
		}
		if e = WriteJSON(&b, out); e != nil || !bytes.Equal(a.Bytes(), b.Bytes()) {
			t.Fatal("reload", e)
		}
		return out
	}
	s = reload(s)
	var payload bytes.Buffer
	if e := WriteJSON(&payload, s); e != nil {
		t.Fatal(e)
	}
	var record nativeSolid
	if e := json.Unmarshal(payload.Bytes(), &record); e != nil {
		t.Fatal(e)
	}
	altered := false
	for i, u := range record.Faces[second].Outer {
		if u.MultiSpanMappedSpline != nil {
			record.Faces[second].Outer[i].MultiSpanMappedSpline.Weights[0] = "3/4"
			altered = true
			break
		}
	}
	if !altered {
		t.Fatal("missing second original payload")
	}
	forged, e := json.Marshal(record)
	if e != nil {
		t.Fatal(e)
	}
	if out, e := ReadJSON(bytes.NewReader(forged), JSONLimits{}); e == nil || len(out.faces) != 0 {
		t.Fatal("changed second-owner speed accepted")
	}

	check := func(x Solid) {
		o := StoredMixedClosedMeshOptions{Faces: make([]SolidMixedFaceMeshOptions, len(x.faces))}
		for fi, f := range x.faces {
			o.Faces[fi].Face = MixedExactMultiSpanMeshOptions{Span: MultiSpanTriangleBoundOptions{MaxDegree: 4}, Face: MixedExactCurvedMeshOptions{BoundaryModelBudget: .01, Interior: CurvedQuadraticTrimMeshOptions{Deflection: .04}}}
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
		r, e := x.TessellateMixedClosedCertified(ctx, o)
		if e != nil {
			t.Fatal("mesh", e)
		}
		if !r.DisplayCollision.Complete || len(r.DisplayCollision.Contacts) != 0 {
			t.Fatal("contact")
		}
		knots := map[int]map[string]int{}
		for fi, original := range expected {
			knots[fi] = map[string]int{}
			patch, e := exactSplineSupportPatch(x.faces[fi].Surface, geom.SurfacePatchOptions{})
			if e != nil {
				t.Fatal(e)
			}
			for _, use := range r.Network.Faces[fi].Coedges {
				curve := x.faces[fi].Outer[use.Use].Curve
				_, raw, ok := mappedRepairSource(curve)
				if !ok {
					continue
				}
				if !sameAuthoredBezierSpline(raw.Curve, original.source.Curve) || raw.Reversed != original.source.Reversed {
					t.Fatal("source changed")
				}
				for j, p := range use.AuthoredParameters {
					uv := authoredMixedSplinePoint(t, raw, p)
					point, e := patch.EvaluateRational(ctx, uv[0], uv[1], geom.SurfacePatchEvaluationOptions{})
					if e != nil {
						t.Fatal(e)
					}
					id := r.Network.FaceVertices[fi][use.Vertices[j]]
					for k := 0; k < 3; k++ {
						if point.Point[k].Cmp(r.Network.ExactPoints[id][k]) != 0 {
							t.Fatal("source correspondence")
						}
					}
					key := point.Point[0].RatString()
					knots[fi][key] = id
				}
			}
		}
		for key, id := range knots[first] {
			if other, ok := knots[second][key]; !ok || other != id {
				t.Fatal("partition not common", key)
			}
		}
		if len(knots[first]) != len(knots[second]) {
			t.Fatal("asymmetric partition")
		}
		union := map[string]bool{}
		for fi, q := range expected {
			patch, e := exactSplineSupportPatch(x.faces[fi].Surface, geom.SurfacePatchOptions{})
			if e != nil {
				t.Fatal(e)
			}
			for _, station := range q.stations {
				raw, e := q.AuthoredParameter(station)
				if e != nil {
					t.Fatal(e)
				}
				uv := authoredMixedSplinePoint(t, q.source, raw)
				point, e := patch.EvaluateRational(ctx, uv[0], uv[1], geom.SurfacePatchEvaluationOptions{})
				if e != nil {
					t.Fatal(e)
				}
				key := point.Point[0].RatString()
				union[key] = true
				if _, ok := knots[first][key]; !ok {
					t.Fatal("missing original union station", key)
				}
			}
		}
		if len(union) != 4 {
			t.Fatal("fixture knots unexpectedly aligned", union)
		}

	}
	check(s)
	cut := big.NewRat(1, 8)
	for _, u := range s.faces[first].Outer {
		if u.Edge == edge && u.Reversed {
			cut.Sub(big.NewRat(1, 1), cut)
		}
	}
	split, h, e := s.SplitExactBezierTrimEdge(ctx, edge, cut, edgecheck.Options{})
	if e != nil {
		t.Fatal("split", e)
	}
	split = reload(split)
	check(split)
	merged, _, e := split.MergeExactBezierTrimEdges(ctx, h.First, h.Second, cut)
	if e != nil {
		t.Fatal("merge", e)
	}
	merged = reload(merged)
	check(merged)
}
