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

func multispanSharedSeamFixture(t *testing.T, count int, reverse bool) (Solid, int, int, MultiSpanMappedSplineUV) {
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
			var stations []*big.Rat
			if count == 3 {
				stations = []*big.Rat{big.NewRat(1, 4), big.NewRat(5, 8)}
			} else {
				for i := 1; i < count; i++ {
					stations = append(stations, big.NewRat(int64(i), int64(count)))
				}
			}
			speeds := []*big.Rat{big.NewRat(2, 1), big.NewRat(1, 2), big.NewRat(4, 1), big.NewRat(1, 1), big.NewRat(2, 1), big.NewRat(1, 4), big.NewRat(1, 2), big.NewRat(4, 1)}
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
			rawCuts := []float64{2}
			for i := 0; i < count; i++ {
				rawCuts = append(rawCuts, rawCuts[len(rawCuts)-1]+float64(i+3))
			}
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
			faces[fi].Outer[ui].Curve = q
			out, e := NewSolid(s.Vertices(), s.Edges(), faces, s.tolerance)
			if e != nil {
				t.Fatal(e)
			}
			return out, edge, fi, q
		}
	}
	t.Fatal("no owner")
	return Solid{}, 0, 0, MultiSpanMappedSplineUV{}
}

func TestStoredMixedMultiSpanMappedSeam(t *testing.T) {
	for _, v := range []struct {
		name    string
		count   int
		reverse bool
	}{{"three", 3, false}, {"three_reversed", 3, true}, {"eight", 8, false}} {
		t.Run(v.name, func(t *testing.T) { checkStoredMixedMultiSpan(t, v.count, v.reverse) })
	}
}
func checkStoredMixedMultiSpan(t *testing.T, count int, reverse bool) {
	ctx := context.Background()
	s, edge, owner, original := multispanSharedSeamFixture(t, count, reverse)
	var data bytes.Buffer
	if e := WriteJSON(&data, s); e != nil {
		t.Fatal(e)
	}
	var n nativeSolid
	if e := json.Unmarshal(data.Bytes(), &n); e != nil || n.Version != 11 {
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
		t.Fatal("analytic mass/deflection", v)
	}
	verified := 0
	for _, use := range r.Network.Faces[owner].Coedges {
		if use.Edge != edge {
			continue
		}
		q, ok := s.faces[owner].Outer[use.Use].Curve.(MultiSpanMappedSplineUV)
		if !ok || !sameAuthoredBezierSpline(q.source.Curve, original.source.Curve) || q.source.Range != original.source.Range || q.source.Reversed != original.source.Reversed {
			t.Fatal("source identity")
		}
		seen := make([]bool, count-1)
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
					t.Fatal("raw multispan authored point")
				}
			}
			for k, station := range q.stations {
				if use.TrimParameters[j].Cmp(station) != 0 {
					continue
				}
				seen[k] = true
				opposite := 0
				for fi, f := range r.Network.Faces {
					if fi == owner {
						continue
					}
					for _, other := range f.Coedges {
						if other.Edge != edge {
							continue
						}
						for m, parameter := range other.ModelParameters {
							if parameter.Cmp(use.ModelParameters[j]) == 0 {
								if r.Network.FaceVertices[fi][other.Vertices[m]] != id {
									t.Fatal("shared knot identity")
								}
								opposite++
							}
						}
					}
				}
				if opposite != 1 {
					t.Fatal("other owner missing source knot")
				}
				verified++
			}
		}
		for _, yes := range seen {
			if !yes {
				t.Fatal("missing original source knot")
			}
		}
	}
	if verified != count-1 {
		t.Fatal("knot count", verified)
	}

	t.Logf("spans=%d triangles=%d volume=%g shared original knots=%d", count, len(r.Mesh.Triangles), v, verified)
}

func TestMultiSpanMappedGuards(t *testing.T) {
	ctx := context.Background()
	s, _, owner, q := multispanSharedSeamFixture(t, 3, false)
	source := q.Source()
	stations, weights := q.ModelStations(), q.ParameterWeights()
	for _, bad := range [][]*big.Rat{{stations[1], stations[0]}, {stations[0], stations[0]}, {stations[0], nil}, {stations[0], big.NewRat(3, 4)}} {
		if out, e := NewMultiSpanMappedSplineUV(ctx, source, bad, weights, geom.SurfacePatchOptions{}); e == nil || len(out.ModelStations()) != 0 {
			t.Fatal("incorrect station", e)
		}
	}
	if out, e := NewMultiSpanMappedSplineUV(ctx, source, make([]*big.Rat, 8), make([]*big.Rat, 9), geom.SurfacePatchOptions{}); e == nil || len(out.ModelStations()) != 0 {
		t.Fatal("span quota")
	}
	wrong := q.ParameterWeights()
	wrong[2] = big.NewRat(1, 1)
	if _, e := NewMultiSpanMappedSplineUV(ctx, source, stations, wrong, geom.SurfacePatchOptions{}); e == nil {
		t.Fatal("last span speed unchecked")
	}
	partial := source
	partial.Range.Max -= .125
	if _, e := NewMultiSpanMappedSplineUV(ctx, partial, stations, weights, geom.SurfacePatchOptions{}); e == nil {
		t.Fatal("unproved omitted source tail")
	}
	poles := source.Curve.Poles()
	poles[len(poles)-2].Y = math.Nextafter(poles[len(poles)-2].Y, math.Inf(1))
	changed, e := geom.NewBSpline(source.Curve.Degree(), poles, source.Curve.Knots(), source.Curve.Weights())
	if e != nil {
		t.Fatal(e)
	}
	badSource := source
	badSource.Curve = changed
	if _, e := NewMultiSpanMappedSplineUV(ctx, badSource, stations, weights, geom.SurfacePatchOptions{}); e == nil {
		t.Fatal("subfloat last span changed")
	}
	if out, e := NewMultiSpanMappedSplineUV(ctx, source, stations, weights, geom.SurfacePatchOptions{MaxOperations: 1}); e == nil || len(out.ModelStations()) != 0 {
		t.Fatal("operation quota")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if out, e := NewMultiSpanMappedSplineUV(canceled, source, stations, weights, geom.SurfacePatchOptions{}); !errors.Is(e, context.Canceled) || len(out.ModelStations()) != 0 {
		t.Fatal("cancellation", e)
	}
	stations[0].SetInt64(1)
	weights[0].SetInt64(99)
	if q.ModelStations()[0].Cmp(big.NewRat(1, 4)) != 0 || q.ParameterWeights()[0].Cmp(big.NewRat(1, 2)) != 0 {
		t.Fatal("mutable source evidence")
	}
	var data bytes.Buffer
	if e := WriteJSON(&data, s); e != nil {
		t.Fatal(e)
	}
	tests := map[string]func(*nativeSolid, *nativeUse){
		"downgrade":         func(n *nativeSolid, u *nativeUse) { n.Version = 10 },
		"wrong station":     func(n *nativeSolid, u *nativeUse) { u.MultiSpanMappedSpline.Stations[1] = "3/4" },
		"wrong final speed": func(n *nativeSolid, u *nativeUse) { u.MultiSpanMappedSpline.Weights[2] = "1" },
		"surplus coefficient": func(n *nativeSolid, u *nativeUse) {
			u.MultiSpanMappedSpline.Weights = append(u.MultiSpanMappedSpline.Weights, "1")
		},
		"competing payload": func(n *nativeSolid, u *nativeUse) { u.MappedSpline = &nativeMappedSplineUse{} },
		"source payload":    func(n *nativeSolid, u *nativeUse) { u.MultiSpanMappedSpline.Source.Quadratic = &QuadraticUV{} },
		"subfloat final span": func(n *nativeSolid, u *nativeUse) {
			var e error
			u.MultiSpanMappedSpline.Source.Spline, e = parametric.EncodeCurve(changed)
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
					if u.MultiSpanMappedSpline != nil {
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
				t.Fatal("forged multispan recipe accepted", e)
			}
		})
	}
	bad := s
	bad.faces = s.Faces()
	for ui, u := range bad.faces[owner].Outer {
		if p, ok := u.Curve.(MultiSpanMappedSplineUV); ok {
			p.stations = p.ModelStations()
			p.stations[1] = big.NewRat(3, 4)
			bad.faces[owner].Outer[ui].Curve = p
		}
	}
	var output bytes.Buffer
	if e := WriteJSON(&output, bad); e == nil || output.Len() != 0 {
		t.Fatal("stale cache accepted")
	}
}

func TestMappedNativeCoefficientCounts(t *testing.T) {
	s, _, _, _ := piecewiseSharedSeamFixture(t)
	var data bytes.Buffer
	if e := WriteJSON(&data, s); e != nil {
		t.Fatal(e)
	}
	var n nativeSolid
	if e := json.Unmarshal(data.Bytes(), &n); e != nil {
		t.Fatal(e)
	}
	for i := range n.Faces {
		for j := range n.Faces[i].Outer {
			u := &n.Faces[i].Outer[j]
			if u.PiecewiseMappedSpline != nil {
				u.PiecewiseMappedSpline.Weights = append(u.PiecewiseMappedSpline.Weights, "123")
			}
		}
	}
	b, e := json.Marshal(n)
	if e != nil {
		t.Fatal(e)
	}
	if _, e := ReadJSON(bytes.NewReader(b), JSONLimits{}); e == nil {
		t.Fatal("v10 surplus coefficient silently discarded")
	}
}

func TestMultiSpanMappedRequiredCuts(t *testing.T) {
	_, _, _, q := multispanSharedSeamFixture(t, 8, false)
	uses := [][]Use{{{Curve: q}}}
	cuts, e := piecewiseMappedFaceCuts(uses, nil)
	if e != nil || len(cuts[0][0]) != 9 {
		t.Fatal("all mandatory original knots", e)
	}
	for i, station := range q.ModelStations() {
		if cuts[0][0][i+1].Cmp(station) != 0 {
			t.Fatal("station order")
		}
	}
	cuts[0][0][1].SetInt64(99)
	if q.ModelStations()[0].Cmp(big.NewRat(1, 8)) != 0 {
		t.Fatal("station ownership")
	}
	full := make([]*big.Rat, 2048)
	for i := range full {
		full[i] = big.NewRat(int64(i), 2047)
	}
	if out, e := piecewiseMappedFaceCuts(uses, [][][]*big.Rat{{full}}); e == nil || out != nil {
		t.Fatal("mandatory cuts exceeded aggregate quota")
	}
}
