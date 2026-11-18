package exchange_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
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

func TestParabolaTwoCrossingCirclesExchange(t *testing.T) {
	slowtest.Skip(t)
	checkTwoCrossingCircleProjectionExchange(t, false)
}

func TestParabolaTwoCrossingCirclesOverlappingExchange(t *testing.T) {
	slowtest.Skip(t)
	checkTwoCrossingCircleProjectionExchange(t, true)
}

func checkTwoCrossingCircleProjectionExchange(t *testing.T, overlap bool) {
	t.Helper()
	ctx := context.Background()
	const unit = .001
	opts := trimmed.ParabolaCircleGraphOptions{}
	proof := edgecheck.Options{}
	envelopeVolumes := [2]float64{
		1483157107570343243947.0/741904988214506029056 - 10041325.0*math.Pi/211106232532992,
		1488777861138705791317.0/741904988214506029056 + 10041325.0*math.Pi/211106232532992,
	}
	cutVolumes := [2][3]float64{
		{3916874323195565375.0/4451429929287036174336 + 10041325.0*math.Pi/140737488355328,
			338828817319615.0/8902859858574072348672 - 10041325.0*math.Pi/844424930131968,
			338828817319615.0/8902859858574072348672 - 10041325.0*math.Pi/844424930131968},
		{29806969429345079615.0/4451429929287036174336 - 10041325.0*math.Pi/422212465065984,
			338828817319615.0/8902859858574072348672 + 10041325.0*math.Pi/281474976710656,
			338828817319615.0/8902859858574072348672 + 10041325.0*math.Pi/281474976710656},
	}
	if overlap {
		envelopeVolumes = [2]float64{
			3077966251710468651171107674069.0/1544949169028154583074107031552 - 24346835150740044425.0*math.Pi/75557863725914323419136,
			3107120598742298359868425660459.0/1544949169028154583074107031552 + 24346835150740044425.0*math.Pi/75557863725914323419136,
		}
		cutVolumes = [2][3]float64{
			{
				22285640997159849810594261035.0/3089898338056309166148214063104 + 73040505452220133275.0*math.Pi/151115727451828646838272,
				3157046895013458249341237195.0/6179796676112618332296428126208 - 48693421770505635725.0*math.Pi/302231454903657293676544,
				16494028902037895796875.0/6179796676112618332296428126208 - 248530974453125.0*math.Pi/302231454903657293676544,
			},
			{
				10955329892485735765601559225.0/1029966112685436388716071354368 - 24346835150740044425.0*math.Pi/151115727451828646838272,
				3157046895013458249341237195.0/6179796676112618332296428126208 + 146080265311516907175.0*math.Pi/302231454903657293676544,
				16494028902037895796875.0/6179796676112618332296428126208 + 745592923359375.0*math.Pi/302231454903657293676544,
			},
		}
	}
	// The ordinary regression includes both axes and Cut orders. A pinned
	// reader runs the full axis, operation and operand-order matrix.
	fullMatrix := os.Getenv("PLAINSOLID_OCCT_CLIPPED_CUT_STEP_COMPONENTS") != "" ||
		os.Getenv("PLAINSOLID_OCCT_CLIPPED_CUT_IGES") != "" ||
		os.Getenv("PLAINSOLID_OCCT_TWO_CROSSING_STEP"+twoCrossingCircleAuditSuffix(overlap)) != "" ||
		os.Getenv("PLAINSOLID_OCCT_TWO_CROSSING_IGES"+twoCrossingCircleAuditSuffix(overlap)) != ""
	for axis := range 2 {
		path := fmt.Sprintf("testdata/native_parabola_two_crossing_circles_axis_%d.json", axis)
		if overlap {
			path = [2]string{"testdata/native_parabola_two_crossing_circles_overlap.json",
				"testdata/native_parabola_two_crossing_circles_overlap_axis_1.json"}[axis]
		}
		src := readTwoCrossingCircleStocks(t, ctx, path)
		for order := range 2 {
			if !fullMatrix && axis != order {
				continue
			}
			for i, operation := range []parametric.BooleanOp{parametric.Intersection, parametric.Union} {
				if !fullMatrix && i != axis {
					continue
				}
				name := [2]string{"common", "fuse"}[i]
				t.Run(fmt.Sprintf("%s/%d/%d", name, axis, order), func(t *testing.T) {
					r, err := trimmed.CombineParabolaTwoCrossingCirclesSplineGraphs(ctx, src[0], src[1+order], src[2-order], axis, operation, 1e-9, opts)
					if err != nil {
						t.Fatal(err)
					}
					body, h := r.Boundary(), r.History()
					if axis == order && order == i {
						exact, err := body.ExtractExactBezierBoundary(ctx, proof)
						if err != nil {
							t.Fatal(err)
						}
						checkTwoCrossingCircleOwners(t, exact, axis, h.ParabolaEdges[:], h.CircleEdges, overlap)
					}
					var st, ig bytes.Buffer
					if err := step.WriteParabolaTwoCrossingCirclesEnvelope(ctx, &st, r, name, unit, step.Limits{}, proof); err != nil {
						t.Fatal(err)
					}
					if err := iges.WriteParabolaTwoCrossingCirclesEnvelope(ctx, &ig, r, unit, iges.Limits{}, proof); err != nil {
						t.Fatal(err)
					}
					if axis == order && order == i {
						if err := step.VerifyParabolaTwoCrossingCirclesEnvelopeSTEP(ctx, bytes.NewReader(st.Bytes()), r, name, unit, step.Limits{}, proof); err != nil {
							t.Fatal(err)
						}
						if err := iges.VerifyParabolaTwoCrossingCirclesEnvelopeIGES(ctx, bytes.NewReader(ig.Bytes()), r, unit, iges.Limits{}, proof); err != nil {
							t.Fatal(err)
						}
					}
					if overlap != bytes.Contains(st.Bytes(), []byte("TRIMMED_CURVE(")) {
						t.Fatal("wrong exact carrier mode")
					}
					checkTwoCirclesExchangeReadback(t, ctx, []trimmed.Solid{body}, []float64{envelopeVolumes[i]}, st.Bytes(), ig.Bytes())
					checkTwoCrossingCirclePinnedOwners(t, ctx, name, axis, order, overlap, st.Bytes(), ig.Bytes())
				})
			}
			t.Run(fmt.Sprintf("cut/%d/%d", axis, order), func(t *testing.T) {
				r, err := trimmed.CutParabolaTwoCrossingCirclesSplineGraphs(ctx, src[0], src[1+order], src[2-order], axis, 1e-9, opts)
				if err != nil {
					t.Fatal(err)
				}
				parts, history := r.Components(), r.History()
				controls := 0
				if axis == order {
					for i, part := range parts {
						b, err := part.ExtractExactBezierBoundary(ctx, proof)
						if err != nil {
							t.Fatal(err)
						}
						h := history[i]
						checkTwoCrossingCircleOwners(t, b, axis, h.ParabolaEdges[:], h.CircleEdges, overlap)
						if axis == 0 && order == 0 && i < 2 {
							controls += countTwoCirclesExchangeControls(t, b)
						}
					}
				}
				var st, ig bytes.Buffer
				if err := step.WriteParabolaTwoCrossingCirclesCut(ctx, &st, r, "cut", unit, step.Limits{}, proof); err != nil {
					t.Fatal(err)
				}
				if err := iges.WriteParabolaTwoCrossingCirclesCut(ctx, &ig, r, unit, iges.Limits{}, proof); err != nil {
					t.Fatal(err)
				}
				if axis == 1 && order == 1 {
					if err := step.VerifyParabolaTwoCrossingCirclesCutSTEP(ctx, bytes.NewReader(st.Bytes()), r, "cut", unit, step.Limits{}, proof); err != nil {
						t.Fatal(err)
					}
					if err := iges.VerifyParabolaTwoCrossingCirclesCutIGES(ctx, bytes.NewReader(ig.Bytes()), r, unit, iges.Limits{}, proof); err != nil {
						t.Fatal(err)
					}
				}
				checkTwoCirclesExchangeReadback(t, ctx, parts[:], cutVolumes[order][:], st.Bytes(), ig.Bytes())
				checkTwoCrossingCirclePinnedOwners(t, ctx, "cut", axis, order, overlap, st.Bytes(), ig.Bytes())
				if controls > 0 {
					var blocked bytes.Buffer
					if err := step.WriteParabolaTwoCrossingCirclesCut(ctx, &blocked, r, "cut", unit, step.Limits{MaxSolids: 2}, proof); !errors.Is(err, geom.ErrConvergence) || blocked.Len() != 0 {
						t.Fatal("STEP published fewer than three whole source components", err)
					}
					if err := iges.WriteParabolaTwoCrossingCirclesCut(ctx, &blocked, r, unit, iges.Limits{MaxControlPoints: controls}, proof); !errors.Is(err, geom.ErrConvergence) || blocked.Len() != 0 {
						t.Fatal("IGES published fewer than three whole source components", err)
					}
					changed := bytes.Replace(st.Bytes(), []byte(".CURVE_3D."), []byte(".PCURVE_S1."), 1)
					if bytes.Equal(changed, st.Bytes()) || !errors.Is(step.VerifyParabolaTwoCrossingCirclesCutSTEP(ctx, bytes.NewReader(changed), r, "cut", unit, step.Limits{}, proof), geom.ErrConvergence) {
						t.Fatal("changed STEP edge law accepted by source verifier")
					}
				}
			})
		}
	}
}

func readTwoCrossingCircleStocks(t *testing.T, ctx context.Context, path string) [3]*geom.BSplineSurface {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	r, err := trimmed.ReadParabolaTwoCrossingCirclesEnvelopeJSON(ctx, bytes.NewReader(raw), trimmed.JSONLimits{}, trimmed.ParabolaCircleGraphOptions{})
	if err != nil {
		t.Fatal(err)
	}
	h, faces := r.History(), r.Boundary().Faces()
	stock := [3]*geom.BSplineSurface{faces[h.Base].Surface.(*geom.BSplineSurface)}
	for i, owner := range h.RoofOwners {
		stock[1+owner] = faces[h.RoofFaces[i]].Surface.(*geom.BSplineSurface)
	}
	return stock
}

func checkTwoCrossingCircleOwners(t *testing.T, b trimmed.ExactBezierBoundary, axis int, parabola []int, circles [2][]int, overlap bool) {
	t.Helper()
	for group, edges := range [3][]int{parabola, circles[0], circles[1]} {
		for _, id := range edges {
			if id < 0 {
				continue
			}
			checkTwoCrossingCircleLaw(t, b.Edges[id].Model, axis, group, 2, overlap)
			owners, oriented := make(map[*geom.BSplineSurface]bool), [2]int{}
			for _, f := range b.Faces {
				for _, loop := range f.Loops {
					for _, use := range loop {
						if use.Edge != id {
							continue
						}
						if f.Spline == nil || owners[f.Spline] {
							t.Fatal("original distinct roof owner missing", id)
						}
						owners[f.Spline] = true
						if use.Reversed != f.Reversed {
							oriented[1]++
						} else {
							oriented[0]++
						}
						checkTwoCrossingCircleLaw(t, use.UV, axis, group, 0, overlap)
					}
				}
			}
			if len(owners) != 2 || oriented != [2]int{1, 1} {
				t.Fatal("lost opposed original seam uses", id, oriented)
			}
		}
	}
}

func checkTwoCrossingCircleLaw(t *testing.T, span geom.ExactBezierSpan, axis, group int, z int64, overlap bool) {
	t.Helper()
	if len(span.Control) != 3 || span.Start != 0 || span.End != 1 {
		t.Fatal("changed exact source section or native edge clock")
	}
	// Five distinct exact stations determine the entire degree-four
	// homogeneous section equation for this quadratic span.
	for i := range int64(5) {
		basis := [3]int64{(4 - i) * (4 - i), 2 * i * (4 - i), i * i}
		var h [4]big.Rat
		for k := range h {
			for j, b := range basis {
				h[k].Add(&h[k], new(big.Rat).Mul(span.Control[j][k], big.NewRat(b, 1)))
			}
		}
		x, y := new(big.Rat).Quo(&h[axis], &h[3]), new(big.Rat).Quo(&h[1-axis], &h[3])
		if new(big.Rat).Quo(&h[2], &h[3]).Cmp(big.NewRat(z, 1)) != 0 {
			t.Fatal("original section plane changed")
		}
		if group == 0 {
			if overlap {
				x.Mul(new(big.Rat).Add(new(big.Rat).Mul(x, big.NewRat(-2, 1)), big.NewRat(119, 64)), x).Add(x, big.NewRat(171, 512))
			} else {
				x.Sub(x, big.NewRat(1, 2)).Mul(x, x).Mul(x, big.NewRat(2, 1)).Add(x, big.NewRat(1, 4))
			}
			if x.Cmp(y) != 0 {
				t.Fatal("original parabola changed")
			}
		} else {
			cx, cy, radiusSquared := big.NewRat([2]int64{7, 25}[group-1], 32), big.NewRat(177, 512), big.NewRat(1, 256)
			if overlap {
				cx = big.NewRat([2]int64{48, 89}[group-1], 128)
				cy = big.NewRat([2]int64{1760, 2543}[group-1], 4096)
				radiusSquared = big.NewRat([2]int64{1681, 25}[group-1], 16384)
			}
			x.Sub(x, cx)
			y.Sub(y, cy)
			if x.Mul(x, x).Add(x, y.Mul(y, y)).Cmp(radiusSquared) != 0 {
				t.Fatal("original exact circle changed", group)
			}
		}
	}
}

func checkTwoCrossingCirclePinnedOwners(t *testing.T, ctx context.Context, op string, axis, order int, overlap bool, st, ig []byte) {
	t.Helper()
	suffix, coedges := twoCrossingCircleAuditSuffix(overlap), 30
	if overlap {
		coedges = 36
	}
	for _, file := range []struct {
		reader, format string
		data           []byte
	}{
		{os.Getenv("PLAINSOLID_OCCT_TWO_CROSSING_STEP" + suffix), "step", st},
		{os.Getenv("PLAINSOLID_OCCT_TWO_CROSSING_IGES" + suffix), "igs", ig},
	} {
		if file.reader == "" {
			continue
		}
		path := filepath.Join(t.TempDir(), "two-crossing-circles."+file.format)
		if err := os.WriteFile(path, file.data, 0600); err != nil {
			t.Fatal(err)
		}
		raw, err := exec.CommandContext(ctx, file.reader, path, strconv.Itoa(axis), strconv.Itoa(order), op).CombinedOutput()
		if err != nil {
			t.Fatalf("%s original owner or crossing jet: %v: %s", file.format, err, raw)
		}
		off := bytes.Index(raw, []byte(`{"Pairs":`))
		var report struct {
			Pairs                            [4]int
			Solids, Coedges, Rays, RoofPoles int
		}
		if off < 0 || json.Unmarshal(bytes.TrimSpace(raw[off:]), &report) != nil {
			t.Fatalf("%s missing independent owner report: %s", file.format, raw)
		}
		pairs, solids := [4]int{}, 1
		if op == "cut" {
			pairs, solids = [4]int{1, 1, 1, 1}, 3
		}
		if report.Pairs != pairs || report.Solids != solids || report.Coedges != coedges || report.Rays != 32 || report.RoofPoles != 486 {
			t.Fatalf("%s changed a source owner, junction ray or component occurrence: %+v", file.format, report)
		}
	}
}

func twoCrossingCircleAuditSuffix(overlap bool) string {
	if overlap {
		return "_WINDOW_OWNERS"
	}
	return "_OWNERS"
}
