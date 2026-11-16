package exchange_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"
	"os"
	"sort"
	"testing"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/exchange/iges"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

func TestParabolaCircleGraphEnvelopeExchange(t *testing.T) {
	ctx := context.Background()
	const name, unit = "Parabola and circle envelope", .001
	proof := edgecheck.Options{}
	for axis := range 2 {
		stock := parabolaCircleExchangeStock(t, ctx, axis)
		for order := range 2 {
			for _, op := range []parametric.BooleanOp{parametric.Intersection, parametric.Union} {
				t.Run(fmt.Sprintf("%d/%d/%d", axis, order, op), func(t *testing.T) {
					r, err := trimmed.CombineParabolaCircleSplineGraphs(ctx, stock[0], stock[1+order], stock[2-order], axis, op, 1e-9, trimmed.ParabolaCircleGraphOptions{})
					if err != nil {
						t.Fatal(err)
					}
					h := r.History()
					checkExactParabolaCircleExchangeEdges(t, ctx, r.Boundary(), axis, h.ParabolaEdge, h.CircleEdges[:])
					var st, ig bytes.Buffer
					if err := step.WriteParabolaCircleGraphEnvelope(ctx, &st, r, name, unit, step.Limits{}, proof); err != nil {
						t.Fatal(err)
					}
					if err := iges.WriteParabolaCircleGraphEnvelope(ctx, &ig, r, unit, iges.Limits{}, proof); err != nil {
						t.Fatal(err)
					}
					if err := step.VerifyParabolaCircleGraphEnvelopeSTEP(ctx, bytes.NewReader(st.Bytes()), r, name, unit, step.Limits{}, proof); err != nil {
						t.Fatal(err)
					}
					if err := iges.VerifyParabolaCircleGraphEnvelopeIGES(ctx, bytes.NewReader(ig.Bytes()), r, unit, iges.Limits{}, proof); err != nil {
						t.Fatal(err)
					}
					want := 54715159.0/27525120 - 4317*math.Pi/134217728
					if op == parametric.Union {
						want = 55418249.0/27525120 + 4317*math.Pi/134217728
					}
					checkParabolaCircleGraphReadback(t, ctx, []trimmed.Solid{r.Boundary()}, []float64{want}, st.Bytes(), ig.Bytes())
				})
			}
		}
	}
}

func TestParabolaCircleGraphCutExchange(t *testing.T) {
	ctx := context.Background()
	const name, unit = "Parabola and circle Cut", .001
	proof := edgecheck.Options{}
	disk := 4317.0 * math.Pi / 134217728
	for axis := range 2 {
		stock := parabolaCircleExchangeStock(t, ctx, axis)
		for order, want := range [2][]float64{{335081.0 / 27525120, disk}, {368009.0/27525120 + disk}} {
			t.Run(fmt.Sprintf("%d/%d", axis, order), func(t *testing.T) {
				r, err := trimmed.CutParabolaCircleSplineGraphs(ctx, stock[0], stock[1+order], stock[2-order], axis, 1e-9, trimmed.ParabolaCircleGraphOptions{})
				if err != nil {
					t.Fatal(err)
				}
				parts, owners := r.Components(), r.History()
				for i, s := range parts {
					checkExactParabolaCircleExchangeEdges(t, ctx, s, axis, owners[i].ParabolaEdge, owners[i].CircleEdges)
				}
				var st, ig bytes.Buffer
				if err := step.WriteParabolaCircleGraphCut(ctx, &st, r, name, unit, step.Limits{}, proof); err != nil {
					t.Fatal(err)
				}
				if err := iges.WriteParabolaCircleGraphCut(ctx, &ig, r, unit, iges.Limits{}, proof); err != nil {
					t.Fatal(err)
				}
				if err := step.VerifyParabolaCircleGraphCutSTEP(ctx, bytes.NewReader(st.Bytes()), r, name, unit, step.Limits{}, proof); err != nil {
					t.Fatal(err)
				}
				if err := iges.VerifyParabolaCircleGraphCutIGES(ctx, bytes.NewReader(ig.Bytes()), r, unit, iges.Limits{}, proof); err != nil {
					t.Fatal(err)
				}
				if axis == 0 && order == 0 {
					var blocked bytes.Buffer
					if err := step.WriteParabolaCircleGraphCut(ctx, &blocked, r, name, unit, step.Limits{MaxSolids: 1}, proof); !errors.Is(err, geom.ErrConvergence) || blocked.Len() != 0 {
						t.Fatal("STEP published an incomplete two-component Cut", err)
					}
					if err := iges.WriteParabolaCircleGraphCut(ctx, &blocked, r, unit, iges.Limits{MaxControlPoints: 1}, proof); !errors.Is(err, geom.ErrConvergence) || blocked.Len() != 0 {
						t.Fatal("IGES published an incomplete two-component Cut", err)
					}
					changed := bytes.Replace(st.Bytes(), []byte(".CURVE_3D."), []byte(".PCURVE_S1."), 1)
					if bytes.Equal(changed, st.Bytes()) || !errors.Is(step.VerifyParabolaCircleGraphCutSTEP(ctx, bytes.NewReader(changed), r, name, unit, step.Limits{}, proof), geom.ErrConvergence) {
						t.Fatal("changed contact model preference passed source verification")
					}
				}
				checkParabolaCircleGraphReadback(t, ctx, parts, want, st.Bytes(), ig.Bytes())
			})
		}
	}
}

func parabolaCircleExchangeStock(t *testing.T, ctx context.Context, axis int) [3]*geom.BSplineSurface {
	t.Helper()
	raw, err := os.ReadFile(fmt.Sprintf("testdata/native_parabola_circle_graph_axis_%d.json", axis))
	if err != nil {
		t.Fatal(err)
	}
	r, err := trimmed.ReadParabolaCircleGraphEnvelopeJSON(ctx, bytes.NewReader(raw), trimmed.JSONLimits{}, trimmed.ParabolaCircleGraphOptions{})
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

func checkExactParabolaCircleExchangeEdges(t *testing.T, ctx context.Context, s trimmed.Solid, axis, parabola int, circle []int) {
	t.Helper()
	b, err := s.ExtractExactBezierBoundary(ctx, edgecheck.Options{})
	if err != nil {
		t.Fatal(err)
	}
	contact := map[int]bool{}
	if parabola >= 0 {
		contact[parabola] = false
	}
	for _, id := range circle {
		contact[id] = true
	}
	for id, inside := range contact {
		exactParabolaCircleExchangeControl(t, b.Edges[id].Model, axis, inside, 2)
		uses := 0
		for _, f := range b.Faces {
			for _, loop := range f.Loops {
				for _, u := range loop {
					if u.Edge == id {
						exactParabolaCircleExchangeControl(t, u.UV, axis, inside, 0)
						uses++
					}
				}
			}
		}
		if uses != 2 {
			t.Fatal("expected both original coedges", id, uses)
		}
	}
}

func exactParabolaCircleExchangeControl(t *testing.T, span geom.ExactBezierSpan, axis int, circle bool, z int64) {
	t.Helper()
	ctrl := span.Control
	if len(ctrl) != 3 || span.Start != 0 || span.End != 1 {
		t.Fatal("lost exact rational quadratic and its shared edge clock", span)
	}
	// Substitution gives a degree at most four polynomial. Check the
	// identity at five distinct rational parameters, for both actual owners.
	for i := range int64(5) {
		basis := [3]int64{(4 - i) * (4 - i), 2 * i * (4 - i), i * i}
		var h [4]*big.Rat
		for ax := range h {
			h[ax] = new(big.Rat)
			for j, b := range basis {
				h[ax].Add(h[ax], new(big.Rat).Mul(ctrl[j][ax], big.NewRat(b, 1)))
			}
		}
		x := new(big.Rat).Quo(h[axis], h[3])
		y := new(big.Rat).Quo(h[1-axis], h[3])
		if new(big.Rat).Quo(h[2], h[3]).Cmp(big.NewRat(z, 1)) != 0 {
			t.Fatal("curve left its exact source plane", i)
		}
		if circle {
			x.Sub(x, big.NewRat(1, 2))
			y.Sub(y, big.NewRat(3, 4))
			x.Mul(x, x).Add(x, y.Mul(y, y))
			if x.Cmp(big.NewRat(1, 64)) != 0 {
				t.Fatal("circle control net does not reproduce the original implicit factor", i)
			}
		} else {
			x.Mul(x, x).Quo(x, big.NewRat(8, 1)).Add(x, big.NewRat(1, 4))
			if x.Cmp(y) != 0 {
				t.Fatal("parabola control net does not reproduce the original implicit factor", i)
			}
		}
	}
}

func checkParabolaCircleGraphReadback(t *testing.T, ctx context.Context, originals []trimmed.Solid, volume []float64, st, ig []byte) {
	t.Helper()
	for _, format := range []struct {
		reader, ext string
		data        []byte
	}{
		{os.Getenv("PLAINSOLID_OCCT_CLIPPED_CUT_STEP_COMPONENTS"), "step", st},
		{os.Getenv("PLAINSOLID_OCCT_CLIPPED_CUT_IGES"), "igs", ig},
	} {
		if format.reader == "" {
			continue
		}
		report := readPinnedExactComponents(t, ctx, format.ext, format.data, format.reader)
		if report.Solids != len(originals) || len(report.Components) != len(originals) {
			t.Fatalf("%s changed the number of separate solid bodies: %+v", format.ext, report)
		}
		sort.Slice(report.Components, func(i, j int) bool { return report.Components[i].Faces > report.Components[j].Faces })
		totalVertices := 0
		for i, body := range originals {
			part, topo := report.Components[i], body.Topology()
			totalVertices += topo.Vertices
			if part.Shells != 1 || part.Faces != topo.Faces || part.Edges != topo.Edges || len(part.Vertices) != topo.Vertices ||
				!(part.Volume > 0) || !(math.Abs(part.Volume-volume[i]) <= 1e-9) {
				t.Fatalf("%s changed the signed volume or topology of component %d: %+v", format.ext, i, part)
			}
			seen := make(map[[3]float64]bool, topo.Vertices)
			for _, p := range body.Vertices() {
				seen[[3]float64{p.X, p.Y, p.Z}] = true
			}
			for _, p := range part.Vertices {
				if !seen[p] {
					t.Fatalf("%s displaced an original indexed endpoint %v", format.ext, p)
				}
				delete(seen, p)
			}
			t.Logf("%s component %d has %d faces, %d edges, %d vertices and signed volume %.16g", format.ext, i, part.Faces, part.Edges, len(part.Vertices), part.Volume)
		}
		if report.IndexedVertices != totalVertices {
			t.Fatalf("%s changed global component vertex identity", format.ext)
		}
	}
}
