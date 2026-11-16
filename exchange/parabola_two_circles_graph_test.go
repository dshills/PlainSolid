package exchange_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"
	"os"
	"slices"
	"testing"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/exchange/iges"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

func TestParabolaTwoCirclesGraphEnvelopeExchange(t *testing.T) {
	ctx := context.Background()
	const name, unit = "Parabola and two circles envelope", .001
	proof := edgecheck.Options{}
	diskVolume := 5347170325.0 * math.Pi / 2251799813685248
	for axis := range 2 {
		stock := twoCirclesExchangeStock(t, ctx, axis)
		for order := range 2 {
			for _, op := range []parametric.BooleanOp{parametric.Intersection, parametric.Union} {
				t.Run(fmt.Sprintf("%d/%d/%d", axis, order, op), func(t *testing.T) {
					r, err := trimmed.CombineParabolaTwoCirclesSplineGraphs(ctx, stock[0], stock[1+order], stock[2-order], axis, op, 1e-9, trimmed.ParabolaCircleGraphOptions{})
					if err != nil {
						t.Fatal(err)
					}
					h := r.History()
					if axis == 0 && order == 0 {
						checkTwoCirclesExchangeEdges(t, ctx, r.Boundary(), axis, h.ParabolaEdge, [2][]int{h.CircleEdges[0][:], h.CircleEdges[1][:]})
					}
					var st, ig bytes.Buffer
					if err := step.WriteParabolaTwoCirclesGraphEnvelope(ctx, &st, r, name, unit, step.Limits{}, proof); err != nil {
						t.Fatal(err)
					}
					if err := iges.WriteParabolaTwoCirclesGraphEnvelope(ctx, &ig, r, unit, iges.Limits{}, proof); err != nil {
						t.Fatal(err)
					}
					if err := step.VerifyParabolaTwoCirclesGraphEnvelopeSTEP(ctx, bytes.NewReader(st.Bytes()), r, name, unit, step.Limits{}, proof); err != nil {
						t.Fatal(err)
					}
					if err := iges.VerifyParabolaTwoCirclesGraphEnvelopeIGES(ctx, bytes.NewReader(ig.Bytes()), r, unit, iges.Limits{}, proof); err != nil {
						t.Fatal(err)
					}
					want := 88351926095467.0/44220983279616 - diskVolume
					if op == parametric.Union {
						want = 88462602167477.0/44220983279616 + diskVolume
					}
					checkTwoCirclesExchangeReadback(t, ctx, []trimmed.Solid{r.Boundary()}, []float64{want}, st.Bytes(), ig.Bytes())
				})
			}
		}
	}
}

func TestParabolaTwoCirclesGraphCutExchange(t *testing.T) {
	ctx := context.Background()
	const name, unit = "Parabola and two circles Cut", .001
	proof := edgecheck.Options{}
	diskVolume := 5347170325.0 * math.Pi / 2251799813685248
	volumes := [2][]float64{
		{90040463765.0 / 44220983279616, 39745125.0 * math.Pi / 17592186044416, 259794325.0 * math.Pi / 2251799813685248},
		{20635608245.0/44220983279616 + diskVolume},
	}
	for axis := range 2 {
		stock := twoCirclesExchangeStock(t, ctx, axis)
		for order, want := range volumes {
			t.Run(fmt.Sprintf("%d/%d", axis, order), func(t *testing.T) {
				r, err := trimmed.CutParabolaTwoCirclesSplineGraphs(ctx, stock[0], stock[1+order], stock[2-order], axis, 1e-9, trimmed.ParabolaCircleGraphOptions{})
				if err != nil {
					t.Fatal(err)
				}
				parts, owners := r.Components(), r.History()
				var firstTwoControls int
				if axis == 0 {
					for i, h := range owners {
						var groups [2][]int
						if h.Region >= 2 {
							groups[h.Region-2] = h.CircleEdges
						} else if len(h.CircleEdges) == 8 {
							groups = [2][]int{h.CircleEdges[:4], h.CircleEdges[4:]}
						}
						b := checkTwoCirclesExchangeEdges(t, ctx, parts[i], axis, h.ParabolaEdge, groups)
						if order == 0 && i < 2 {
							firstTwoControls += countTwoCirclesExchangeControls(t, b)
						}
					}
				}
				var st, ig bytes.Buffer
				if err := step.WriteParabolaTwoCirclesGraphCut(ctx, &st, r, name, unit, step.Limits{}, proof); err != nil {
					t.Fatal(err)
				}
				if err := iges.WriteParabolaTwoCirclesGraphCut(ctx, &ig, r, unit, iges.Limits{}, proof); err != nil {
					t.Fatal(err)
				}
				if err := step.VerifyParabolaTwoCirclesGraphCutSTEP(ctx, bytes.NewReader(st.Bytes()), r, name, unit, step.Limits{}, proof); err != nil {
					t.Fatal(err)
				}
				if err := iges.VerifyParabolaTwoCirclesGraphCutIGES(ctx, bytes.NewReader(ig.Bytes()), r, unit, iges.Limits{}, proof); err != nil {
					t.Fatal(err)
				}
				if axis == 0 && order == 0 {
					var blocked bytes.Buffer
					stepLimits := step.Limits{MaxSolids: 2}
					if err := step.WriteParabolaTwoCirclesGraphCut(ctx, &blocked, r, name, unit, stepLimits, proof); !errors.Is(err, geom.ErrConvergence) || blocked.Len() != 0 {
						t.Fatal("STEP published an incomplete three-component Cut", err)
					}
					stepLimits = step.Limits{MaxBytes: int64(st.Len() - 1)}
					if err := step.WriteParabolaTwoCirclesGraphCut(ctx, &blocked, r, name, unit, stepLimits, proof); !errors.Is(err, geom.ErrConvergence) || blocked.Len() != 0 {
						t.Fatal("STEP published a truncated Cut", err)
					}
					igesLimits := iges.Limits{MaxControlPoints: firstTwoControls}
					if err := iges.WriteParabolaTwoCirclesGraphCut(ctx, &blocked, r, unit, igesLimits, proof); !errors.Is(err, geom.ErrConvergence) || blocked.Len() != 0 {
						t.Fatal("IGES published two bodies without the final lens", err)
					}
					changed := bytes.Replace(st.Bytes(), []byte(".CURVE_3D."), []byte(".PCURVE_S1."), 1)
					if bytes.Equal(changed, st.Bytes()) || !errors.Is(step.VerifyParabolaTwoCirclesGraphCutSTEP(ctx, bytes.NewReader(changed), r, name, unit, step.Limits{}, proof), geom.ErrConvergence) {
						t.Fatal("changed contact model preference passed STEP source verification")
					}
				}
				checkTwoCirclesExchangeReadback(t, ctx, parts, want, st.Bytes(), ig.Bytes())
			})
		}
	}
}

func twoCirclesExchangeStock(t *testing.T, ctx context.Context, axis int) [3]*geom.BSplineSurface {
	t.Helper()
	raw, err := os.ReadFile(fmt.Sprintf("testdata/native_parabola_two_circles_graph_axis_%d.json", axis))
	if err != nil {
		t.Fatal(err)
	}
	r, err := trimmed.ReadParabolaTwoCirclesGraphEnvelopeJSON(ctx, bytes.NewReader(raw), trimmed.JSONLimits{}, trimmed.ParabolaCircleGraphOptions{})
	if err != nil {
		t.Fatal(err)
	}
	h := r.History()
	f := r.Boundary().Faces()
	stock := [3]*geom.BSplineSurface{f[h.Base].Surface.(*geom.BSplineSurface)}
	for region, owner := range h.RoofOwners {
		stock[1+owner] = f[h.RoofFaces[region]].Surface.(*geom.BSplineSurface)
	}
	return stock
}

func checkTwoCirclesExchangeEdges(t *testing.T, ctx context.Context, s trimmed.Solid, axis, parabola int, circles [2][]int) trimmed.ExactBezierBoundary {
	t.Helper()
	b, err := s.ExtractExactBezierBoundary(ctx, edgecheck.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if parabola >= 0 {
		checkTwoCirclesExchangeEdge(t, b, axis, parabola, -1)
	}
	for group, ring := range circles {
		for _, edge := range ring {
			checkTwoCirclesExchangeEdge(t, b, axis, edge, group)
		}
	}
	return b
}

func checkTwoCirclesExchangeEdge(t *testing.T, b trimmed.ExactBezierBoundary, axis, edge, circle int) {
	t.Helper()
	checkTwoCirclesExchangeConic(t, b.Edges[edge].Model, axis, circle, 2)
	owners := 0
	for _, f := range b.Faces {
		for _, loop := range f.Loops {
			for _, u := range loop {
				if u.Edge == edge {
					checkTwoCirclesExchangeConic(t, u.UV, axis, circle, 0)
					owners++
				}
			}
		}
	}
	if owners != 2 {
		t.Fatal("expected both exact original roof coedges", edge, owners)
	}
}

func checkTwoCirclesExchangeConic(t *testing.T, span geom.ExactBezierSpan, axis, circle int, z int64) {
	t.Helper()
	if len(span.Control) != 3 || span.Start != 0 || span.End != 1 {
		t.Fatal("lost exact quadratic law or shared edge clock", span)
	}
	// Each homogeneous conic identity has degree at most four. These
	// five distinct rational evaluations prove the whole control law.
	for i := range int64(5) {
		basis := [3]int64{(4 - i) * (4 - i), 2 * i * (4 - i), i * i}
		var h [4]*big.Rat
		for coordinate := range h {
			h[coordinate] = new(big.Rat)
			for j, b := range basis {
				h[coordinate].Add(h[coordinate], new(big.Rat).Mul(span.Control[j][coordinate], big.NewRat(b, 1)))
			}
		}
		x := new(big.Rat).Quo(h[axis], h[3])
		y := new(big.Rat).Quo(h[1-axis], h[3])
		if new(big.Rat).Quo(h[2], h[3]).Cmp(big.NewRat(z, 1)) != 0 {
			t.Fatal("curve left the exact section plane", i)
		}
		if circle < 0 {
			x.Mul(x, x).Quo(x, big.NewRat(8, 1)).Add(x, big.NewRat(1, 4))
			if x.Cmp(y) != 0 {
				t.Fatal("original parabola factor changed", i)
			}
		} else {
			x.Sub(x, big.NewRat(int64(1+2*circle), 4))
			y.Sub(y, big.NewRat(3, 4))
			x.Mul(x, x).Add(x, y.Mul(y, y))
			if x.Cmp(big.NewRat(1, [2]int64{64, 256}[circle])) != 0 {
				t.Fatal("original circle factor changed or swapped owners", circle, i)
			}
		}
	}
}

func countTwoCirclesExchangeControls(t *testing.T, b trimmed.ExactBezierBoundary) int {
	t.Helper()
	n := 0
	for _, edge := range b.Edges {
		n += len(edge.Model.Control)
	}
	for _, face := range b.Faces {
		support, err := face.BSplineSupport()
		if err != nil {
			t.Fatal(err)
		}
		n += len(support.Data().Poles)
		for _, loop := range face.Loops {
			for _, use := range loop {
				n += len(use.UV.Control)
			}
		}
	}
	return n
}

func checkTwoCirclesExchangeReadback(t *testing.T, ctx context.Context, sources []trimmed.Solid, want []float64, st, ig []byte) {
	t.Helper()
	for _, f := range []struct {
		reader, format string
		data           []byte
	}{
		{os.Getenv("PLAINSOLID_OCCT_CLIPPED_CUT_STEP_COMPONENTS"), "step", st},
		{os.Getenv("PLAINSOLID_OCCT_CLIPPED_CUT_IGES"), "igs", ig},
	} {
		if f.reader == "" {
			continue
		}
		report := readPinnedExactComponents(t, ctx, f.format, f.data, f.reader)
		if report.Solids != len(sources) || len(report.Components) != len(sources) {
			t.Fatalf("%s changed the number of separate solid bodies: %+v", f.format, report)
		}
		unmatched := slices.Clone(report.Components)
		vertices := 0
		for i, body := range sources {
			topo := body.Topology()
			vertices += topo.Vertices
			index := slices.IndexFunc(unmatched, func(part pinnedExactComponentReport) bool {
				return matchTwoCirclesExchangeVertices(body.Vertices(), part.Vertices)
			})
			if index < 0 {
				t.Fatalf("%s changed the original vertices of component %d", f.format, i)
			}
			part := unmatched[index]
			unmatched = slices.Delete(unmatched, index, index+1)
			tolerance := max(1e-13, math.Abs(want[i])*1e-9)
			if part.Shells != 1 || part.Faces != topo.Faces || part.Edges != topo.Edges ||
				!(part.Volume > 0) || !(math.Abs(part.Volume-want[i]) <= tolerance) {
				t.Fatalf("%s changed component %d topology or signed volume: %+v", f.format, i, part)
			}
			checkTwoCirclesExchangeIncidence(t, f.format, body, part)
			t.Logf("%s component %d has %d faces, %d edges, %d vertices and signed volume %.16g", f.format, i, part.Faces, part.Edges, len(part.Vertices), part.Volume)
		}
		if report.IndexedVertices != vertices {
			t.Fatalf("%s changed aggregate vertex identity", f.format)
		}
	}
}

func matchTwoCirclesExchangeVertices(native []geom.Vec3, neutral [][3]float64) bool {
	if len(native) != len(neutral) {
		return false
	}
	expected := make(map[[3]float64]bool, len(native))
	for _, p := range native {
		expected[[3]float64{p.X, p.Y, p.Z}] = true
	}
	for _, p := range neutral {
		if !expected[p] {
			return false
		}
		delete(expected, p)
	}
	return len(expected) == 0
}

func checkTwoCirclesExchangeIncidence(t *testing.T, format string, source trimmed.Solid, part pinnedExactComponentReport) {
	t.Helper()
	vertexID := make(map[[3]float64]int, len(part.Vertices))
	for i, v := range part.Vertices {
		vertexID[v] = i
	}
	points := source.Vertices()
	unmatched := make(map[[2]int]int)
	for _, e := range source.Edges() {
		a, b := points[e.Start], points[e.End]
		u, v := vertexID[[3]float64{a.X, a.Y, a.Z}], vertexID[[3]float64{b.X, b.Y, b.Z}]
		unmatched[[2]int{min(u, v), max(u, v)}]++
	}
	for _, e := range part.EdgeVertices {
		id := [2]int{min(e[0], e[1]), max(e[0], e[1])}
		if unmatched[id] < 1 {
			t.Fatalf("%s added or rewired an original edge %v", format, e)
		}
		unmatched[id]--
	}
	for _, n := range unmatched {
		if n != 0 {
			t.Fatalf("%s omitted an original edge", format)
		}
	}
}
