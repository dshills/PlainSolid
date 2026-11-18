package exchange_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/internal/slowtest"
	"strconv"
	"testing"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/exchange/iges"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

func TestParabolaThreeCirclesEnvelopeExchange(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	proof := edgecheck.Options{}
	full := hasThreeCirclesPinnedReader()
	if full {
		t.Parallel()
	}
	const unit = .001
	// Independent integrals of the original residual over its seven signed regions.
	volumes := [2][2]float64{{1.9932183944638235, 2.2844857491023514}, {1.9909711399729113, 2.1709505882008608}}
	for side, masses := range volumes {
		for axis := range 2 {
			if !full && axis != side {
				continue
			}
			stocks := readThreeCirclesExchangeStocks(t, ctx, axis, side)
			for order := range 2 {
				for i, op := range [2]parametric.BooleanOp{parametric.Intersection, parametric.Union} {
					if !full && (order != side || i != side) {
						continue
					}
					name := [2]string{"common", "fuse"}[i]
					t.Run(fmt.Sprintf("%d/%d/%d/%s", side, axis, order, name), func(t *testing.T) {
						r, err := trimmed.CombineParabolaThreeCirclesSplineGraphs(ctx, stocks[0], stocks[1+order], stocks[2-order], axis, op,
							1e-9, trimmed.ParabolaCircleGraphOptions{})
						if err != nil {
							t.Fatal(err)
						}
						body, h := r.Boundary(), r.History()
						check := side == axis && axis == order && i == side
						if check {
							b, err := body.ExtractExactBezierBoundary(ctx, proof)
							if err != nil {
								t.Fatal(err)
							}
							checkPortableThreeCirclesOwners(t, b, axis, side, h.ParabolaEdges[:], h.CircleEdges, h.SeparatedEdges, 4)
						}
						var st, ig bytes.Buffer
						if err := step.WriteParabolaThreeCirclesEnvelope(ctx, &st, r, name, unit, step.Limits{}, proof); err != nil {
							t.Fatal(err)
						}
						if err := iges.WriteParabolaThreeCirclesEnvelope(ctx, &ig, r, unit, iges.Limits{}, proof); err != nil {
							t.Fatal(err)
						}
						if check {
							if err := step.VerifyParabolaThreeCirclesEnvelopeSTEP(ctx, bytes.NewReader(st.Bytes()), r, name, unit, step.Limits{}, proof); err != nil {
								t.Fatal(err)
							}
							if err := iges.VerifyParabolaThreeCirclesEnvelopeIGES(ctx, bytes.NewReader(ig.Bytes()), r, unit, iges.Limits{}, proof); err != nil {
								t.Fatal(err)
							}
						}
						checkTwoCirclesExchangeReadback(t, ctx, []trimmed.Solid{body}, []float64{masses[i]}, st.Bytes(), ig.Bytes())
						checkThreeCirclesPinnedOwners(t, ctx, name, axis, side, order, 1, st.Bytes(), ig.Bytes())
					})
				}
			}
		}
	}
}

func TestParabolaThreeCirclesCutExchange(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	proof := edgecheck.Options{}
	full := hasThreeCirclesPinnedReader()
	if full {
		t.Parallel()
	}
	const name, unit = "Three-circle ordered Cut", .001
	// Positive masses from the independent signed source-height Green integrals.
	volumes := [2][7]float64{
		{.00678122243255969570, .284379425887643229, .0000531525239913044500, 1.91551808399345852e-7, .0000531525239913044500, 1.91551808399345852e-7, 1.81667256547182025e-8},
		{.00902861162935336052, .170850479767487537, .0000500542166867359336, 1.11245215674448245e-7, .0000500542166867359336, 1.11245215674448245e-7, 2.59073038281078706e-8},
	}
	for side, masses := range volumes {
		for axis := range 2 {
			if !full && axis != side {
				continue
			}
			stocks := readThreeCirclesExchangeStocks(t, ctx, axis, side)
			for order := range 2 {
				if !full && order != 0 {
					continue
				}
				t.Run(fmt.Sprintf("%d/%d/%d", side, axis, order), func(t *testing.T) {
					r, err := trimmed.CutParabolaThreeCirclesSplineGraphs(ctx, stocks[0], stocks[1+order], stocks[2-order], axis,
						1e-9, trimmed.ParabolaCircleGraphOptions{})
					if err != nil {
						t.Fatal(err)
					}
					parts, history := r.Components(), r.History()
					want := make([]float64, 0, len(parts))
					for _, h := range history {
						want = append(want, masses[h.Region])
					}
					controls := 0
					check := side == axis && order == 0
					if check {
						for i, h := range history {
							b, err := parts[i].ExtractExactBezierBoundary(ctx, proof)
							if err != nil {
								t.Fatal(err)
							}
							inner := 0
							if i == 0 && side == order {
								inner = 8
							}
							checkPortableThreeCirclesOwners(t, b, axis, side, h.ParabolaEdges[:], h.CircleEdges, h.SeparatedEdges, inner)
							if i < len(parts)-1 {
								controls += countTwoCirclesExchangeControls(t, b)
							}
						}
					}
					var st, ig bytes.Buffer
					if err := step.WriteParabolaThreeCirclesCut(ctx, &st, r, name, unit, step.Limits{}, proof); err != nil {
						t.Fatal(err)
					}
					if err := iges.WriteParabolaThreeCirclesCut(ctx, &ig, r, unit, iges.Limits{}, proof); err != nil {
						t.Fatal(err)
					}
					if check {
						if err := step.VerifyParabolaThreeCirclesCutSTEP(ctx, bytes.NewReader(st.Bytes()), r, name, unit, step.Limits{}, proof); err != nil {
							t.Fatal(err)
						}
						if err := iges.VerifyParabolaThreeCirclesCutIGES(ctx, bytes.NewReader(ig.Bytes()), r, unit, iges.Limits{}, proof); err != nil {
							t.Fatal(err)
						}
					}
					if check && len(parts) == 4 {
						var blocked bytes.Buffer
						if err := step.WriteParabolaThreeCirclesCut(ctx, &blocked, r, name, unit, step.Limits{MaxSolids: 3}, proof); !errors.Is(err, geom.ErrConvergence) || blocked.Len() != 0 {
							t.Fatal("STEP published a partial three-circle Cut", err)
						}
						if err := iges.WriteParabolaThreeCirclesCut(ctx, &blocked, r, unit, iges.Limits{MaxControlPoints: controls}, proof); !errors.Is(err, geom.ErrConvergence) || blocked.Len() != 0 {
							t.Fatal("IGES published a partial three-circle Cut", err)
						}
					}
					checkTwoCirclesExchangeReadback(t, ctx, parts, want, st.Bytes(), ig.Bytes())
					checkThreeCirclesPinnedOwners(t, ctx, "cut", axis, side, order, len(parts), st.Bytes(), ig.Bytes())
				})
			}
		}
	}
}

func readThreeCirclesExchangeStocks(t *testing.T, ctx context.Context, axis, side int) [3]*geom.BSplineSurface {
	t.Helper()
	path := fmt.Sprintf("testdata/native_parabola_three_circles_axis_%d_side_%d.json", axis, side)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	r, err := trimmed.ReadParabolaThreeCirclesEnvelopeJSON(ctx, bytes.NewReader(raw), trimmed.JSONLimits{}, trimmed.ParabolaCircleGraphOptions{})
	if err != nil {
		t.Fatal(err)
	}
	h, faces := r.History(), r.Boundary().Faces()
	stocks := [3]*geom.BSplineSurface{faces[h.Base].Surface.(*geom.BSplineSurface)}
	for i, owner := range h.RoofOwners {
		stocks[1+owner] = faces[h.RoofFaces[i]].Surface.(*geom.BSplineSurface)
	}
	return stocks
}

func checkPortableThreeCirclesOwners(t *testing.T, b trimmed.ExactBezierBoundary, axis, side int,
	parabola []int, crossing [2][]int, isolated [4]int, innerUses int) {
	t.Helper()
	foundInner := 0
	for group, edges := range [4][]int{parabola, crossing[0], crossing[1], isolated[:]} {
		for _, id := range edges {
			if id < 0 {
				continue
			}
			checkPortableThreeCirclesLaw(t, b.Edges[id].Model, axis, side, group, 2)
			owners, directions := make(map[*geom.BSplineSurface]bool), [2]int{}
			for _, face := range b.Faces {
				for ring, loop := range face.Loops {
					for _, use := range loop {
						if use.Edge != id {
							continue
						}
						if face.Spline == nil || owners[face.Spline] {
							t.Fatal("model edge lost a distinct original spline owner", id)
						}
						owners[face.Spline] = true
						if use.Reversed != face.Reversed {
							directions[1]++
						} else {
							directions[0]++
						}
						if group == 3 && ring > 0 {
							foundInner++
						}
						checkPortableThreeCirclesLaw(t, use.UV, axis, side, group, 0)
					}
				}
			}
			if len(owners) != 2 || directions != ([2]int{1, 1}) {
				t.Fatal("lost opposite original coedge ownership", id, directions)
			}
		}
	}
	if foundInner != innerUses {
		t.Fatal("isolated trim changed hole or disk incidence", foundInner, innerUses)
	}
}

func checkPortableThreeCirclesLaw(t *testing.T, span geom.ExactBezierSpan, axis, side, branch int, height int64) {
	t.Helper()
	if len(span.Control) != 3 || span.Start != 0 || span.End != 1 {
		t.Fatal("changed original polynomial span or edge clock")
	}
	// Five exact homogeneous samples recover the complete degree-four
	// composition of a rational quadratic span with its source equation.
	for i := range int64(5) {
		basis := [3]int64{(4 - i) * (4 - i), 2 * i * (4 - i), i * i}
		var point [4]big.Rat
		for dimension := range point {
			for j, weight := range basis {
				point[dimension].Add(&point[dimension], new(big.Rat).Mul(span.Control[j][dimension], big.NewRat(weight, 1)))
			}
		}
		x, y := new(big.Rat).Quo(&point[axis], &point[3]), new(big.Rat).Quo(&point[1-axis], &point[3])
		if new(big.Rat).Quo(&point[2], &point[3]).Cmp(big.NewRat(height, 1)) != 0 {
			t.Fatal("original section plane changed")
		}
		if branch == 0 {
			x.Sub(x, big.NewRat(1, 2)).Mul(x, x).Mul(x, big.NewRat(2, 1)).Add(x, big.NewRat(1, 4))
			if x.Cmp(y) != 0 {
				t.Fatal("parabola trim changed original law")
			}
			continue
		}
		cx, cy, rr := big.NewRat([3]int64{3, 13, 8}[branch-1], 16), big.NewRat(41, 128), big.NewRat(1, 64)
		if branch == 3 {
			cy, rr = big.NewRat(int64(3+3*side), 16), big.NewRat(1, 1024)
		}
		x.Sub(x, cx)
		y.Sub(y, cy)
		if x.Mul(x, x).Add(x, y.Mul(y, y)).Cmp(rr) != 0 {
			t.Fatal("original circle trim moved to a different source law", branch)
		}
	}
}

func hasThreeCirclesPinnedReader() bool {
	return os.Getenv("PLAINSOLID_OCCT_THREE_CIRCLES_STEP_OWNERS") != "" || os.Getenv("PLAINSOLID_OCCT_THREE_CIRCLES_IGES_OWNERS") != "" ||
		os.Getenv("PLAINSOLID_OCCT_CLIPPED_CUT_STEP_COMPONENTS") != "" || os.Getenv("PLAINSOLID_OCCT_CLIPPED_CUT_IGES") != ""
}

func checkThreeCirclesPinnedOwners(t *testing.T, ctx context.Context, op string, axis, side, order, parts int, st, ig []byte) {
	t.Helper()
	for _, f := range []struct {
		reader, extension string
		data              []byte
	}{
		{os.Getenv("PLAINSOLID_OCCT_THREE_CIRCLES_STEP_OWNERS"), "step", st},
		{os.Getenv("PLAINSOLID_OCCT_THREE_CIRCLES_IGES_OWNERS"), "igs", ig},
	} {
		if f.reader == "" {
			continue
		}
		path := filepath.Join(t.TempDir(), "three-circles."+f.extension)
		if err := os.WriteFile(path, f.data, 0600); err != nil {
			t.Fatal(err)
		}
		raw, err := exec.CommandContext(ctx, f.reader, path, strconv.Itoa(axis), strconv.Itoa(side), strconv.Itoa(order), op).CombinedOutput()
		if err != nil {
			t.Fatalf("%s original roof and wire audit failed: %v: %s", f.extension, err, raw)
		}
		at := bytes.Index(raw, []byte(`{"Pairs":`))
		var report struct {
			Pairs                                          [4]int
			Solids, Coedges, Rays, RoofPoles, IsolatedJets int
		}
		if at < 0 || json.Unmarshal(bytes.TrimSpace(raw[at:]), &report) != nil {
			t.Fatalf("%s missing independent ownership report: %s", f.extension, raw)
		}
		pairs, poles := [4]int{}, 7*81
		if op == "cut" {
			pairs, poles = [4]int{1, 1, 1, 1}, 2*81*parts
		}
		if report.Solids != parts || report.Pairs != pairs || report.Coedges != 38 || report.Rays != 32 ||
			report.IsolatedJets != 16 || report.RoofPoles != poles {
			t.Fatalf("%s changed source branch or separated ring incidence: %+v", f.extension, report)
		}
	}
}
