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
	"strconv"
	"testing"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/trimmed"
	"plainsolid/exchange/iges"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

func TestParabolaMixedCirclesEnvelopeExchange(t *testing.T) {
	ctx := context.Background()
	const unit = .001
	proof := edgecheck.Options{}
	for _, f := range []struct {
		op                string
		axis, side, order int
		volume            float64
	}{
		{"common", 0, 0, 0, 20262849766147207.0/10133099161583616 - 9724805825.0*math.Pi/216172782113783808},
		{"common", 1, 1, 1, 20258070340374407.0/10133099161583616 - 18988606175.0*math.Pi/216172782113783808},
		{"fuse", 0, 1, 1, 6760879696404179.0/3377699720527872 + 18988606175.0*math.Pi/216172782113783808},
		{"fuse", 1, 0, 0, 6771686537652179.0/3377699720527872 + 9724805825.0*math.Pi/216172782113783808},
	} {
		t.Run(fmt.Sprintf("%s/%d", f.op, f.axis), func(t *testing.T) {
			raw := mixedCirclesExchangeRecipe(t, f.op, f.axis, f.side, f.order)
			result, err := trimmed.ReadParabolaMixedCirclesEnvelopeJSON(ctx, bytes.NewReader(raw), trimmed.JSONLimits{}, trimmed.ParabolaCircleGraphOptions{})
			if err != nil {
				t.Fatal(err)
			}
			body, history := result.Boundary(), result.History()
			exact, err := body.ExtractExactBezierBoundary(ctx, proof)
			if err != nil {
				t.Fatal(err)
			}
			checkParabolaCircleCrossingsExactEdges(t, exact, f.axis, history.ParabolaEdges, history.CircleEdges)
			checkMixedCirclesSeparatedRing(t, exact, f.axis, f.side, history.SeparatedEdges, 4)
			var st, ig bytes.Buffer
			if err := step.WriteParabolaMixedCirclesEnvelope(ctx, &st, result, f.op, unit, step.Limits{}, proof); err != nil {
				t.Fatal(err)
			}
			if err := iges.WriteParabolaMixedCirclesEnvelope(ctx, &ig, result, unit, iges.Limits{}, proof); err != nil {
				t.Fatal(err)
			}
			if err := step.VerifyParabolaMixedCirclesEnvelopeSTEP(ctx, bytes.NewReader(st.Bytes()), result, f.op, unit, step.Limits{}, proof); err != nil {
				t.Fatal(err)
			}
			if err := iges.VerifyParabolaMixedCirclesEnvelopeIGES(ctx, bytes.NewReader(ig.Bytes()), result, unit, iges.Limits{}, proof); err != nil {
				t.Fatal(err)
			}
			checkTwoCirclesExchangeReadback(t, ctx, []trimmed.Solid{body}, []float64{f.volume}, st.Bytes(), ig.Bytes())
			checkMixedCirclesPinnedJets(t, ctx, f.op, f.axis, f.side, f.order, 1, st.Bytes(), ig.Bytes())
		})
	}
}

func TestParabolaMixedCirclesCutExchange(t *testing.T) {
	ctx := context.Background()
	const name, unit = "Mixed parabola-circle Cut", .001
	proof := edgecheck.Options{}
	for _, f := range []struct {
		axis, side, order int
		volumes           []float64
	}{
		{0, 0, 0, []float64{2793913668193235.0/8694199080638742528 - 68480808725.0*math.Pi/72057594037927936,
			79148254988215.0/8694199080638742528 + 105062125.0*math.Pi/105553116266496}},
		{0, 1, 0, []float64{6874310365806835.0/8694199080638742528 + 88781875.0*math.Pi/105553116266496,
			99498870437015.0/8694199080638742528 - 88781875.0*math.Pi/105553116266496, 18988606175.0 * math.Pi / 216172782113783808}},
		{1, 0, 1, []float64{41843838384235475.0/8694199080638742528 + 105062125.0*math.Pi/105553116266496,
			79148254988215.0/8694199080638742528 - 105062125.0*math.Pi/105553116266496, 9724805825.0 * math.Pi / 216172782113783808}},
		{1, 1, 1, []float64{14006678396434675.0/8694199080638742528 - 54278891275.0*math.Pi/72057594037927936,
			99498870437015.0/8694199080638742528 + 88781875.0*math.Pi/105553116266496}},
	} {
		t.Run(fmt.Sprintf("%d/%d/%d", f.axis, f.side, f.order), func(t *testing.T) {
			raw := mixedCirclesExchangeRecipe(t, "cut", f.axis, f.side, f.order)
			result, err := trimmed.ReadParabolaMixedCirclesCutJSON(ctx, bytes.NewReader(raw), trimmed.JSONLimits{}, trimmed.ParabolaCircleGraphOptions{})
			if err != nil {
				t.Fatal(err)
			}
			parts, history := result.Components(), result.History()
			controlsForFirstTwo := 0
			for i, part := range parts {
				exact, err := part.ExtractExactBezierBoundary(ctx, proof)
				if err != nil {
					t.Fatal(err)
				}
				h := history[i]
				checkParabolaCircleCrossingsExactEdges(t, exact, f.axis, h.ParabolaEdges, h.CircleEdges)
				if h.SeparatedEdges[0] >= 0 {
					holes := 0
					if i == 0 {
						holes = 8
					}
					checkMixedCirclesSeparatedRing(t, exact, f.axis, f.side, h.SeparatedEdges, holes)
				}
				if f.axis == 0 && len(parts) == 3 && i < 2 {
					controlsForFirstTwo += countTwoCirclesExchangeControls(t, exact)
				}
			}
			var st, ig bytes.Buffer
			if err := step.WriteParabolaMixedCirclesCut(ctx, &st, result, name, unit, step.Limits{}, proof); err != nil {
				t.Fatal(err)
			}
			if err := iges.WriteParabolaMixedCirclesCut(ctx, &ig, result, unit, iges.Limits{}, proof); err != nil {
				t.Fatal(err)
			}
			if err := step.VerifyParabolaMixedCirclesCutSTEP(ctx, bytes.NewReader(st.Bytes()), result, name, unit, step.Limits{}, proof); err != nil {
				t.Fatal(err)
			}
			if err := iges.VerifyParabolaMixedCirclesCutIGES(ctx, bytes.NewReader(ig.Bytes()), result, unit, iges.Limits{}, proof); err != nil {
				t.Fatal(err)
			}
			if controlsForFirstTwo > 0 {
				var blocked bytes.Buffer
				if err := step.WriteParabolaMixedCirclesCut(ctx, &blocked, result, name, unit, step.Limits{MaxSolids: 2}, proof); !errors.Is(err, geom.ErrConvergence) || blocked.Len() != 0 {
					t.Fatal("STEP published a partial mixed-circle Cut", err)
				}
				if err := iges.WriteParabolaMixedCirclesCut(ctx, &blocked, result, unit, iges.Limits{MaxControlPoints: controlsForFirstTwo}, proof); !errors.Is(err, geom.ErrConvergence) || blocked.Len() != 0 {
					t.Fatal("IGES published a partial mixed-circle Cut", err)
				}
			}
			checkTwoCirclesExchangeReadback(t, ctx, parts, f.volumes, st.Bytes(), ig.Bytes())
			checkMixedCirclesPinnedJets(t, ctx, "cut", f.axis, f.side, f.order, len(parts), st.Bytes(), ig.Bytes())
		})
	}
}

func mixedCirclesExchangeRecipe(t *testing.T, op string, axis, side, order int) []byte {
	t.Helper()
	name := fmt.Sprintf("testdata/native_parabola_mixed_circles_%s-axis-%d-side-%d-order-%d.json", op, axis, side, order)
	raw, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func checkMixedCirclesSeparatedRing(t *testing.T, b trimmed.ExactBezierBoundary, axis, side int, ids [4]int, wantInnerUses int) {
	t.Helper()
	cx, cy := big.NewRat(7, 8), big.NewRat(1, 8)
	if side == 1 {
		cx, cy = big.NewRat(1, 8), big.NewRat(3, 4)
	}
	// These coefficients define a homogeneous polynomial of degree at most
	// four. Five exact stations therefore prove the complete circle law.
	check := func(span geom.ExactBezierSpan, z int64) {
		t.Helper()
		if len(span.Control) != 3 || span.Start != 0 || span.End != 1 {
			t.Fatal("lost the original quadratic circle clock")
		}
		for i := range int64(5) {
			factors := [3]int64{(4 - i) * (4 - i), 2 * i * (4 - i), i * i}
			var point [4]big.Rat
			for k := range point {
				for j, f := range factors {
					point[k].Add(&point[k], new(big.Rat).Mul(span.Control[j][k], big.NewRat(f, 1)))
				}
			}
			x := new(big.Rat).Quo(&point[axis], &point[3])
			y := new(big.Rat).Quo(&point[1-axis], &point[3])
			x.Sub(x, cx).Mul(x, x)
			y.Sub(y, cy).Mul(y, y)
			if x.Add(x, y).Cmp(big.NewRat(1, 256)) != 0 ||
				new(big.Rat).Quo(&point[2], &point[3]).Cmp(big.NewRat(z, 1)) != 0 {
				t.Fatal("separated circle left its exact original law")
			}
		}
	}
	innerUses := 0
	for _, id := range ids {
		check(b.Edges[id].Model, 2)
		owners := make(map[*geom.BSplineSurface]bool)
		forward, reverse := 0, 0
		for _, face := range b.Faces {
			for i, loop := range face.Loops {
				for _, use := range loop {
					if use.Edge != id {
						continue
					}
					if face.Spline == nil || owners[face.Spline] {
						t.Fatal("separated circle lost a distinct original owner")
					}
					owners[face.Spline] = true
					check(use.UV, 0)
					if face.Reversed == use.Reversed {
						forward++
					} else {
						reverse++
					}
					if i > 0 {
						innerUses++
					}
				}
			}
		}
		if len(owners) != 2 || forward != 1 || reverse != 1 {
			t.Fatal("separated rim lost opposite original uses")
		}
	}
	if innerUses != wantInnerUses {
		t.Fatal("circle rim migrated between the original hole and disk", innerUses)
	}
}

func checkMixedCirclesPinnedJets(t *testing.T, ctx context.Context, op string, axis, side, order, parts int, st, ig []byte) {
	t.Helper()
	for _, f := range []struct {
		reader, format string
		data           []byte
	}{
		{os.Getenv("PLAINSOLID_OCCT_MIXED_CIRCLES_STEP_JETS"), "step", st},
		{os.Getenv("PLAINSOLID_OCCT_MIXED_CIRCLES_IGES_JETS"), "igs", ig},
	} {
		if f.reader == "" {
			continue
		}
		path := filepath.Join(t.TempDir(), "mixed-circles."+f.format)
		if err := os.WriteFile(path, f.data, 0600); err != nil {
			t.Fatal(err)
		}
		raw, err := exec.CommandContext(ctx, f.reader, path, strconv.Itoa(axis), strconv.Itoa(side), strconv.Itoa(order), op).CombinedOutput()
		if err != nil {
			t.Fatalf("%s original-owner and jet readback: %v: %s", f.format, err, raw)
		}
		at := bytes.Index(raw, []byte(`{"Pairs":`))
		if at < 0 {
			t.Fatalf("%s missing independent jet report: %s", f.format, raw)
		}
		var report struct {
			Pairs      [2]int
			Components []struct {
				OpposedCoedges, OwnerRays, IsolatedOwnerJets, OriginalOwnerPoles int
			}
		}
		if err := json.Unmarshal(bytes.TrimSpace(raw[at:]), &report); err != nil {
			t.Fatal(err)
		}
		pairs, wantPoles := [2]int{}, 405
		if op == "cut" {
			pairs, wantPoles = [2]int{1, 1}, 162*parts
		}
		var coedges, rays, isolated, poles int
		for _, part := range report.Components {
			coedges += part.OpposedCoedges
			rays += part.OwnerRays
			isolated += part.IsolatedOwnerJets
			poles += part.OriginalOwnerPoles
		}
		if len(report.Components) != parts || report.Pairs != pairs || coedges != 22 || rays != 16 || isolated != 16 || poles != wantPoles {
			t.Fatalf("%s lost an original owner jet or contact pair: %+v", f.format, report)
		}
	}
}
