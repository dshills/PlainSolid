package exchange_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"
	"os"
	"testing"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/trimmed"
	"plainsolid/exchange/iges"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

func TestParabolaCircleCrossingsEnvelopeExchange(t *testing.T) {
	ctx := context.Background()
	const unit = .001
	proof := edgecheck.Options{}
	for _, c := range []struct {
		op          string
		axis, order int
		volume      float64
	}{
		{"common", 0, 0, 1071399207.0 / 536870912},
		{"fuse", 1, 1, 5429263229.0 / 2684354560},
	} {
		t.Run(c.op, func(t *testing.T) {
			raw := parabolaCircleCrossingsExchangeRecipe(t, c.op, c.axis, c.order)
			result, err := trimmed.ReadParabolaCircleCrossingsEnvelopeJSON(ctx, bytes.NewReader(raw), trimmed.JSONLimits{}, trimmed.ParabolaCircleGraphOptions{})
			if err != nil {
				t.Fatal(err)
			}
			history, solid := result.History(), result.Boundary()
			exact, err := solid.ExtractExactBezierBoundary(ctx, proof)
			if err != nil {
				t.Fatal(err)
			}
			checkParabolaCircleCrossingsExactEdges(t, exact, c.axis, history.ParabolaEdges, history.CircleEdges)
			var st, ig bytes.Buffer
			if err := step.WriteParabolaCircleCrossingsEnvelope(ctx, &st, result, c.op, unit, step.Limits{}, proof); err != nil {
				t.Fatal(err)
			}
			if err := iges.WriteParabolaCircleCrossingsEnvelope(ctx, &ig, result, unit, iges.Limits{}, proof); err != nil {
				t.Fatal(err)
			}
			if err := step.VerifyParabolaCircleCrossingsEnvelopeSTEP(ctx, bytes.NewReader(st.Bytes()), result, c.op, unit, step.Limits{}, proof); err != nil {
				t.Fatal(err)
			}
			if err := iges.VerifyParabolaCircleCrossingsEnvelopeIGES(ctx, bytes.NewReader(ig.Bytes()), result, unit, iges.Limits{}, proof); err != nil {
				t.Fatal(err)
			}
			checkTwoCirclesExchangeReadback(t, ctx, []trimmed.Solid{solid}, []float64{c.volume}, st.Bytes(), ig.Bytes())
		})
	}
}

func TestParabolaCircleCrossingsCutExchange(t *testing.T) {
	ctx := context.Background()
	const name, unit = "Transverse parabola-circle Cut", .001
	proof := edgecheck.Options{}
	const piTerm = 15.0 * math.Pi / 4194304
	volumes := [2][]float64{
		{237703427.0/56371445760 - piTerm, 4135679.0/28185722880 + piTerm},
		{1263364931.0/56371445760 + piTerm, 4135679.0/28185722880 - piTerm},
	}
	for axis := range 2 {
		for order, want := range volumes {
			t.Run(fmt.Sprintf("%d/%d", axis, order), func(t *testing.T) {
				raw := parabolaCircleCrossingsExchangeRecipe(t, "cut", axis, order)
				result, err := trimmed.ReadParabolaCircleCrossingsCutJSON(ctx, bytes.NewReader(raw), trimmed.JSONLimits{}, trimmed.ParabolaCircleGraphOptions{})
				if err != nil {
					t.Fatal(err)
				}
				parts, history, contacts := result.Components(), result.History(), result.Contacts()
				firstControls := 0
				for part, s := range parts {
					exact, err := s.ExtractExactBezierBoundary(ctx, proof)
					if err != nil {
						t.Fatal(err)
					}
					checkParabolaCircleCrossingsExactEdges(t, exact, axis, history[part].ParabolaEdges, history[part].CircleEdges)
					if axis == 0 && order == 0 && part == 0 {
						firstControls = countTwoCirclesExchangeControls(t, exact)
					}
					for i, node := range contacts {
						p := geom.Vec3{X: float64(2*i+1) / 4, Y: 49. / 128, Z: 2}
						if axis == 1 {
							p.X, p.Y = p.Y, p.X
						}
						id := node.Vertices[part]
						if exact.Vertices[id] != p {
							t.Fatalf("Cut changed the separate exact occurrence of node %d in shell %d", i, part)
						}
						for _, ray := range node.Rays[part] {
							edge := exact.Edges[ray]
							if (edge.Start == id) == (edge.End == id) {
								t.Fatalf("Cut changed the original directed arc at node %d in shell %d", i, part)
							}
						}
					}
				}
				var st, ig bytes.Buffer
				if err := step.WriteParabolaCircleCrossingsCut(ctx, &st, result, name, unit, step.Limits{}, proof); err != nil {
					t.Fatal(err)
				}
				if err := iges.WriteParabolaCircleCrossingsCut(ctx, &ig, result, unit, iges.Limits{}, proof); err != nil {
					t.Fatal(err)
				}
				if err := step.VerifyParabolaCircleCrossingsCutSTEP(ctx, bytes.NewReader(st.Bytes()), result, name, unit, step.Limits{}, proof); err != nil {
					t.Fatal(err)
				}
				if err := iges.VerifyParabolaCircleCrossingsCutIGES(ctx, bytes.NewReader(ig.Bytes()), result, unit, iges.Limits{}, proof); err != nil {
					t.Fatal(err)
				}
				if firstControls > 0 {
					var blocked bytes.Buffer
					err := step.WriteParabolaCircleCrossingsCut(ctx, &blocked, result, name, unit, step.Limits{MaxSolids: 1}, proof)
					if !errors.Is(err, geom.ErrConvergence) || blocked.Len() != 0 {
						t.Fatal("STEP published a partial two-body Cut", err)
					}
					err = iges.WriteParabolaCircleCrossingsCut(ctx, &blocked, result, unit, iges.Limits{MaxControlPoints: firstControls}, proof)
					if !errors.Is(err, geom.ErrConvergence) || blocked.Len() != 0 {
						t.Fatal("IGES published a partial two-body Cut", err)
					}
					original := []byte("CARTESIAN_POINT('',(0.25,0.3828125,2.))")
					changed := bytes.Replace(st.Bytes(), original, []byte("CARTESIAN_POINT('',(0.25,0.3828126,2.))"), 1)
					if bytes.Equal(changed, st.Bytes()) ||
						!errors.Is(step.VerifyParabolaCircleCrossingsCutSTEP(ctx, bytes.NewReader(changed), result, name, unit, step.Limits{}, proof), geom.ErrConvergence) {
						t.Fatal("STEP accepted a displaced original crossing")
					}
					// The type 502 vertex list of the three-vertex cap
					// contains its own occurrence of the same source node.
					original = []byte("502,3,0.25,0.3828125,2.")
					changed = bytes.Replace(ig.Bytes(), original, []byte("502,3,0.25,0.3828126,2."), 1)
					if bytes.Equal(changed, ig.Bytes()) ||
						!errors.Is(iges.VerifyParabolaCircleCrossingsCutIGES(ctx, bytes.NewReader(changed), result, unit, iges.Limits{}, proof), geom.ErrConvergence) {
						t.Fatal("IGES accepted a displaced cap occurrence of the original crossing")
					}
				}
				// The pinned readers must retain the original indexed vertices
				// and all edge incidences of each shell, including both exact
				// source point pairings and distinct local vertex identities.
				checkTwoCirclesExchangeReadback(t, ctx, parts[:], want, st.Bytes(), ig.Bytes())
			})
		}
	}
}

func parabolaCircleCrossingsExchangeRecipe(t *testing.T, op string, axis, order int) []byte {
	t.Helper()
	name := fmt.Sprintf("testdata/native_parabola_circle_crossings_%s-axis-%d-order-%d.json", op, axis, order)
	raw, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func checkParabolaCircleCrossingsExactEdges(t *testing.T, b trimmed.ExactBezierBoundary, axis int, parabola [3]int, circles [4]int) {
	t.Helper()
	for kind, ids := range [2][]int{parabola[:], circles[:]} {
		for _, id := range ids {
			if id < 0 {
				continue
			}
			checkParabolaCircleCrossingsExactLaw(t, b.Edges[id].Model, axis, kind == 1, 2)
			forward, reversed := 0, 0
			for _, face := range b.Faces {
				for _, loop := range face.Loops {
					for _, use := range loop {
						if use.Edge != id {
							continue
						}
						if face.Spline == nil {
							t.Fatal("contact edge lost its original spline roof owner", id)
						}
						checkParabolaCircleCrossingsExactLaw(t, use.UV, axis, kind == 1, 0)
						if face.Reversed != use.Reversed {
							reversed++
						} else {
							forward++
						}
					}
				}
			}
			if forward != 1 || reversed != 1 {
				t.Fatal("lost opposed original roof coedges", id, forward, reversed)
			}
		}
	}
}

func checkParabolaCircleCrossingsExactLaw(t *testing.T, span geom.ExactBezierSpan, axis int, circle bool, plane int64) {
	t.Helper()
	if len(span.Control) != 3 || span.Start != 0 || span.End != 1 {
		t.Fatal("lost exact quadratic source law or shared clock")
	}
	// Homogeneous conic equations have degree at most four; five
	// distinct exact rational evaluations certify the entire curve law.
	for i := range int64(5) {
		basis := [3]int64{(4 - i) * (4 - i), 2 * i * (4 - i), i * i}
		var h [4]big.Rat
		for k := range h {
			for j, coefficient := range basis {
				h[k].Add(&h[k], new(big.Rat).Mul(span.Control[j][k], big.NewRat(coefficient, 1)))
			}
		}
		x := new(big.Rat).Quo(&h[axis], &h[3])
		y := new(big.Rat).Quo(&h[1-axis], &h[3])
		if new(big.Rat).Quo(&h[2], &h[3]).Cmp(big.NewRat(plane, 1)) != 0 {
			t.Fatal("original arc left its exact plane")
		}
		x.Sub(x, big.NewRat(1, 2)).Mul(x, x)
		if circle {
			y.Sub(y, big.NewRat(49, 128)).Mul(y, y)
			if x.Add(x, y).Cmp(big.NewRat(1, 16)) != 0 {
				t.Fatal("original circle equation changed")
			}
		} else if x.Quo(x, big.NewRat(8, 1)).Add(x, big.NewRat(3, 8)).Cmp(y) != 0 {
			t.Fatal("original parabola equation changed")
		}
	}
}
