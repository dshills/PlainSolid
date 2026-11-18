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

func TestParabolaCircleFourCrossingsExchange(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	full := hasFourCrossingsPinnedReader()
	for layout := range 2 {
		for axis := range 2 {
			if !full && axis != layout {
				continue
			}
			stocks := readFourCrossingsExchangeStocks(t, ctx, axis, layout)
			for order := range 2 {
				if !full && order != 0 {
					continue
				}
				t.Run(fmt.Sprintf("%d/%d/%d", layout, axis, order), func(t *testing.T) {
					if full {
						t.Parallel()
					}
					src := [3]*geom.BSplineSurface{stocks[0], stocks[1+order], stocks[2-order]}
					check := axis == layout && order == 0
					for i, operation := range [2]parametric.BooleanOp{parametric.Intersection, parametric.Union} {
						name := [2]string{"common", "fuse"}[i]
						t.Run(name, func(t *testing.T) {
							checkFourCrossingsEnvelopeExchange(t, ctx, src, axis, layout, order, i, name, operation, check)
						})
					}
					t.Run("cut", func(t *testing.T) {
						checkFourCrossingsCutExchange(t, ctx, src, axis, layout, order, check)
					})
				})
			}
		}
	}
}

func readFourCrossingsExchangeStocks(t *testing.T, ctx context.Context, axis, layout int) [3]*geom.BSplineSurface {
	t.Helper()
	path := fmt.Sprintf("testdata/native_parabola_circle_four_crossings_axis_%d_layout_%d.json", axis, layout)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	r, err := trimmed.ReadParabolaCircleFourCrossingsEnvelopeJSON(ctx, bytes.NewReader(raw), trimmed.JSONLimits{}, trimmed.ParabolaCircleGraphOptions{})
	if err != nil {
		t.Fatal(err)
	}
	h, f := r.History(), r.Boundary().Faces()
	stocks := [3]*geom.BSplineSurface{f[h.Base].Surface.(*geom.BSplineSurface)}
	for i, id := range h.RoofFaces {
		stocks[1+h.RoofOwners[i]] = f[id].Surface.(*geom.BSplineSurface)
	}
	return stocks
}

func checkFourCrossingsEnvelopeExchange(t *testing.T, ctx context.Context, stocks [3]*geom.BSplineSurface,
	axis, layout, order, i int, name string, op parametric.BooleanOp, check bool) {
	t.Helper()
	const unit = .001
	proof := edgecheck.Options{}
	r, err := trimmed.CombineParabolaCircleFourCrossingsSplineGraphs(ctx, stocks[0], stocks[1], stocks[2], axis, op, 1e-9, trimmed.ParabolaCircleGraphOptions{})
	if err != nil {
		t.Fatal(err)
	}
	body, h := r.Boundary(), r.History()
	var st, ig bytes.Buffer
	if err := step.WriteParabolaCircleFourCrossingsEnvelope(ctx, &st, r, name, unit, step.Limits{}, proof); err != nil {
		t.Fatal(err)
	}
	if err := iges.WriteParabolaCircleFourCrossingsEnvelope(ctx, &ig, r, unit, iges.Limits{}, proof); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(st.Bytes(), []byte("TRIMMED_CURVE")) {
		t.Fatal("missing exact original quarter carrier")
	}
	if check {
		checkPortableFourCrossingsOwners(t, ctx, body, axis, layout, h.ParabolaEdges[:], h.CircleEdges)
		if err := step.VerifyParabolaCircleFourCrossingsEnvelopeSTEP(ctx, bytes.NewReader(st.Bytes()), r, name, unit, step.Limits{}, proof); err != nil {
			t.Fatal(err)
		}
		if err := iges.VerifyParabolaCircleFourCrossingsEnvelopeIGES(ctx, bytes.NewReader(ig.Bytes()), r, unit, iges.Limits{}, proof); err != nil {
			t.Fatal(err)
		}
	}
	// Exact independent Green integrals of the source residual.
	volumes := [2]float64{1.9904462736296447, 2.008815400934687}
	if layout != 0 {
		volumes = [2]float64{4 - volumes[1], 4 - volumes[0]}
	}
	checkTwoCirclesExchangeReadback(t, ctx, []trimmed.Solid{body}, []float64{volumes[i]}, st.Bytes(), ig.Bytes())
	checkFourCrossingsPinnedOwners(t, ctx, name, axis, layout, order, 1, st.Bytes(), ig.Bytes())
}

func checkFourCrossingsCutExchange(t *testing.T, ctx context.Context, stocks [3]*geom.BSplineSurface, axis, layout, order int, check bool) {
	t.Helper()
	const name, unit = "Four original parabola-circle crossings Cut", .001
	proof := edgecheck.Options{}
	r, err := trimmed.CutParabolaCircleFourCrossingsSplineGraphs(ctx, stocks[0], stocks[1], stocks[2], axis, 1e-9, trimmed.ParabolaCircleGraphOptions{})
	if err != nil {
		t.Fatal(err)
	}
	parts, history := r.Components(), r.History()
	var st, ig bytes.Buffer
	if err := step.WriteParabolaCircleFourCrossingsCut(ctx, &st, r, name, unit, step.Limits{}, proof); err != nil {
		t.Fatal(err)
	}
	if err := iges.WriteParabolaCircleFourCrossingsCut(ctx, &ig, r, unit, iges.Limits{}, proof); err != nil {
		t.Fatal(err)
	}
	volumes := [6]float64{0.0062935814591662947, 0.0088153959330863688, 4.2984944571218707e-9,
		3.5155308778047057e-10, 3.5155308778047057e-10, 0.0032601449111889372}
	want := make([]float64, len(history))
	prefixControls := 0
	for i, h := range history {
		want[i] = volumes[h.Region]
		if check {
			b := checkPortableFourCrossingsOwners(t, ctx, parts[i], axis, layout, h.ParabolaEdges[:], h.CircleEdges)
			if i < len(parts)-1 {
				prefixControls += countTwoCirclesExchangeControls(t, b)
			}
		}
	}
	if check {
		if err := step.VerifyParabolaCircleFourCrossingsCutSTEP(ctx, bytes.NewReader(st.Bytes()), r, name, unit, step.Limits{}, proof); err != nil {
			t.Fatal(err)
		}
		if err := iges.VerifyParabolaCircleFourCrossingsCutIGES(ctx, bytes.NewReader(ig.Bytes()), r, unit, iges.Limits{}, proof); err != nil {
			t.Fatal(err)
		}
	}
	if check && len(parts) == 4 {
		var blocked bytes.Buffer
		if err := step.WriteParabolaCircleFourCrossingsCut(ctx, &blocked, r, name, unit, step.Limits{MaxSolids: 3}, proof); !errors.Is(err, geom.ErrConvergence) || blocked.Len() != 0 {
			t.Fatal("STEP published a partial contact cycle", err)
		}
		if err := iges.WriteParabolaCircleFourCrossingsCut(ctx, &blocked, r, unit, iges.Limits{MaxControlPoints: prefixControls}, proof); !errors.Is(err, geom.ErrConvergence) || blocked.Len() != 0 {
			t.Fatal("IGES published a partial contact cycle", err)
		}
	}
	checkTwoCirclesExchangeReadback(t, ctx, parts, want, st.Bytes(), ig.Bytes())
	checkFourCrossingsPinnedOwners(t, ctx, "cut", axis, layout, order, len(parts), st.Bytes(), ig.Bytes())
}

func checkPortableFourCrossingsOwners(t *testing.T, ctx context.Context, body trimmed.Solid, axis, layout int,
	parabola, circle []int) trimmed.ExactBezierBoundary {
	t.Helper()
	b, err := body.ExtractExactBezierBoundary(ctx, edgecheck.Options{})
	if err != nil {
		t.Fatal(err)
	}
	for branch, edges := range [2][]int{parabola, circle} {
		for _, id := range edges {
			if id < 0 {
				continue
			}
			checkPortableFourCrossingsLaw(t, b.Edges[id].Model, axis, layout, branch == 1, 2)
			owners, directions := make(map[*geom.BSplineSurface]bool), [2]int{}
			for _, face := range b.Faces {
				for _, use := range face.Loops[0] {
					if use.Edge != id {
						continue
					}
					if face.Spline == nil || owners[face.Spline] || len(face.Loops) != 1 {
						t.Fatal("changed original contour owner or simple source loop", id)
					}
					owners[face.Spline] = true
					if use.Reversed != face.Reversed {
						directions[1]++
					} else {
						directions[0]++
					}
					checkPortableFourCrossingsLaw(t, use.UV, axis, layout, branch == 1, 0)
				}
			}
			if len(owners) != 2 || directions != [2]int{1, 1} {
				t.Fatal("lost opposed original coedges", id, directions)
			}
		}
	}
	return b
}

func checkPortableFourCrossingsLaw(t *testing.T, span geom.ExactBezierSpan, axis, layout int, circle bool, height int64) {
	t.Helper()
	if len(span.Control) != 3 || span.Start != 0 || span.End != 1 {
		t.Fatal("changed original rational quadratic span")
	}
	// Five exact homogeneous samples suffice for the degree-four
	// substitution of this quadratic Bézier into its source equation.
	for i := range int64(5) {
		basis := [3]int64{(4 - i) * (4 - i), 2 * i * (4 - i), i * i}
		var p [4]big.Rat
		for dim := range p {
			for j, q := range basis {
				p[dim].Add(&p[dim], new(big.Rat).Mul(span.Control[j][dim], big.NewRat(q, 1)))
			}
		}
		if new(big.Rat).Quo(&p[2], &p[3]).Cmp(big.NewRat(height, 1)) != 0 {
			t.Fatal("changed section plane")
		}
		x := new(big.Rat).Quo(&p[axis], &p[3])
		y := new(big.Rat).Quo(&p[1-axis], &p[3])
		x.Sub(x, big.NewRat(1, 2)).Mul(x, x)
		if circle {
			y.Sub(y, big.NewRat(1, 2)).Mul(y, y)
			if x.Add(x, y).Cmp(big.NewRat(7225, 65536)) != 0 {
				t.Fatal("moved an exact circle coedge")
			}
			continue
		}
		x.Mul(x, big.NewRat(2, 1)).Add(x, big.NewRat(327, 2048))
		if layout != 0 {
			x.Sub(big.NewRat(1, 1), x)
		}
		if x.Cmp(y) != 0 {
			t.Fatal("moved an exact parabola coedge")
		}
	}
}

func hasFourCrossingsPinnedReader() bool {
	return os.Getenv("PLAINSOLID_OCCT_FOUR_CROSSINGS_STEP_OWNERS") != "" || os.Getenv("PLAINSOLID_OCCT_FOUR_CROSSINGS_IGES_OWNERS") != "" ||
		os.Getenv("PLAINSOLID_OCCT_CLIPPED_CUT_STEP_COMPONENTS") != "" || os.Getenv("PLAINSOLID_OCCT_CLIPPED_CUT_IGES") != ""
}

func checkFourCrossingsPinnedOwners(t *testing.T, ctx context.Context, op string, axis, layout, order, parts int, st, ig []byte) {
	t.Helper()
	for _, f := range []struct {
		reader, extension string
		data              []byte
	}{
		{os.Getenv("PLAINSOLID_OCCT_FOUR_CROSSINGS_STEP_OWNERS"), "step", st},
		{os.Getenv("PLAINSOLID_OCCT_FOUR_CROSSINGS_IGES_OWNERS"), "igs", ig},
	} {
		if f.reader == "" {
			continue
		}
		path := filepath.Join(t.TempDir(), "four-crossings."+f.extension)
		if err := os.WriteFile(path, f.data, 0600); err != nil {
			t.Fatal(err)
		}
		raw, err := exec.CommandContext(ctx, f.reader, path, strconv.Itoa(axis), strconv.Itoa(layout), strconv.Itoa(order), op).CombinedOutput()
		if err != nil {
			t.Fatalf("%s original pocket or coedge audit failed: %v: %s", f.extension, err, raw)
		}
		at := bytes.Index(raw, []byte(`{"Pairs":`))
		var report struct {
			Pairs                            [4]int
			Solids, Coedges, Rays, RoofPoles int
			Subdivisions                     [3]int
		}
		if at < 0 || json.Unmarshal(bytes.TrimSpace(raw[at:]), &report) != nil {
			t.Fatalf("%s missing original roof and edge audit: %s", f.extension, raw)
		}
		pairs, poles := [4]int{}, 6*25
		if op == "cut" {
			pairs, poles = [4]int{1, 1, 1, 1}, 2*25*parts
		}
		if report.Solids != parts || report.Pairs != pairs || report.Coedges != 36 || report.Rays != 32 ||
			report.Subdivisions != [3]int{4, 4, 4} || report.RoofPoles != poles {
			t.Fatalf("%s lost original pocket, coedge or subdivision: %+v", f.extension, report)
		}
	}
}
