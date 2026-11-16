package exchange_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
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

func TestParabolaCircleOsculationEnvelopeExchange(t *testing.T) {
	ctx := context.Background()
	const unit = .001
	proof := edgecheck.Options{}
	for _, fixture := range []struct {
		op                string
		axis, side, order int
		want              float64
	}{
		{"common", 0, 0, 0, 54382063.0/27525120 - 33.0*math.Pi/131072},
		{"common", 1, 1, 1, 54918991.0/27525120 - 33.0*math.Pi/131072},
		{"fuse", 0, 1, 0, 55718417.0/27525120 + 33.0*math.Pi/131072},
		{"fuse", 1, 0, 1, 55181489.0/27525120 + 33.0*math.Pi/131072},
	} {
		t.Run(fmt.Sprintf("%s/%d/%d", fixture.op, fixture.axis, fixture.side), func(t *testing.T) {
			raw := osculationExchangeSource(t, fixture.op, fixture.axis, fixture.side, fixture.order)
			body, err := trimmed.ReadParabolaCircleOsculationEnvelopeJSON(ctx, bytes.NewReader(raw), trimmed.JSONLimits{}, trimmed.ParabolaCircleGraphOptions{})
			if err != nil {
				t.Fatal(err)
			}
			var st, ig bytes.Buffer
			if err := step.WriteParabolaCircleOsculationEnvelope(ctx, &st, body, fixture.op, unit, step.Limits{}, proof); err != nil {
				t.Fatal(err)
			}
			if err := iges.WriteParabolaCircleOsculationEnvelope(ctx, &ig, body, unit, iges.Limits{}, proof); err != nil {
				t.Fatal(err)
			}
			if err := step.VerifyParabolaCircleOsculationEnvelopeSTEP(ctx, bytes.NewReader(st.Bytes()), body, fixture.op, unit, step.Limits{}, proof); err != nil {
				t.Fatal(err)
			}
			if err := iges.VerifyParabolaCircleOsculationEnvelopeIGES(ctx, bytes.NewReader(ig.Bytes()), body, unit, iges.Limits{}, proof); err != nil {
				t.Fatal(err)
			}
			if fixture.op == "common" && fixture.axis == 0 {
				var blocked bytes.Buffer
				short := edgecheck.Options{MaxWork: 1}
				if err := step.WriteParabolaCircleOsculationEnvelope(ctx, &blocked, body, fixture.op, unit, step.Limits{}, short); !errors.Is(err, geom.ErrConvergence) || blocked.Len() != 0 {
					t.Fatal("STEP published an incompletely proved osculation shell", err)
				}
				if err := iges.WriteParabolaCircleOsculationEnvelope(ctx, &blocked, body, unit, iges.Limits{}, short); !errors.Is(err, geom.ErrConvergence) || blocked.Len() != 0 {
					t.Fatal("IGES published an incompletely proved osculation shell", err)
				}
			}
			checkTwoCirclesExchangeReadback(t, ctx, []trimmed.Solid{body.Boundary()}, []float64{fixture.want}, st.Bytes(), ig.Bytes())
			checkOsculationNeutralReadback(t, ctx, fixture.op, fixture.axis, fixture.side, st.Bytes(), ig.Bytes())
		})
	}
}

func TestParabolaCircleOsculationCutExchange(t *testing.T) {
	ctx := context.Background()
	const name, unit = "Parabola-circle osculation Cut", .001
	proof := edgecheck.Options{}
	volumes := []float64{131249.0 / 27525120, 33.0 * math.Pi / 131072}
	for axis := range 2 {
		for side := range 2 {
			t.Run(fmt.Sprintf("%d/%d", axis, side), func(t *testing.T) {
				raw := osculationExchangeSource(t, "cut", axis, side, 1-side)
				body, err := trimmed.ReadParabolaCircleOsculationCutJSON(ctx, bytes.NewReader(raw), trimmed.JSONLimits{}, trimmed.ParabolaCircleGraphOptions{})
				if err != nil {
					t.Fatal(err)
				}
				var st, ig bytes.Buffer
				if err := step.WriteParabolaCircleOsculationCut(ctx, &st, body, name, unit, step.Limits{}, proof); err != nil {
					t.Fatal(err)
				}
				if err := iges.WriteParabolaCircleOsculationCut(ctx, &ig, body, unit, iges.Limits{}, proof); err != nil {
					t.Fatal(err)
				}
				if err := step.VerifyParabolaCircleOsculationCutSTEP(ctx, bytes.NewReader(st.Bytes()), body, name, unit, step.Limits{}, proof); err != nil {
					t.Fatal(err)
				}
				if err := iges.VerifyParabolaCircleOsculationCutIGES(ctx, bytes.NewReader(ig.Bytes()), body, unit, iges.Limits{}, proof); err != nil {
					t.Fatal(err)
				}
				parts := body.Components()
				if axis == 0 && side == 1 {
					exactWall, err := parts[0].ExtractExactBezierBoundary(ctx, proof)
					if err != nil {
						t.Fatal(err)
					}
					var blocked bytes.Buffer
					if err := step.WriteParabolaCircleOsculationCut(ctx, &blocked, body, name, unit, step.Limits{MaxSolids: 1}, proof); !errors.Is(err, geom.ErrConvergence) || blocked.Len() != 0 {
						t.Fatal("STEP published a partial source contact pair", err)
					}
					limit := iges.Limits{MaxControlPoints: countTwoCirclesExchangeControls(t, exactWall)}
					if err := iges.WriteParabolaCircleOsculationCut(ctx, &blocked, body, unit, limit, proof); !errors.Is(err, geom.ErrConvergence) || blocked.Len() != 0 {
						t.Fatal("IGES published a partial source contact pair", err)
					}
				}
				checkTwoCirclesExchangeReadback(t, ctx, parts[:], volumes, st.Bytes(), ig.Bytes())
				checkOsculationNeutralReadback(t, ctx, "cut", axis, side, st.Bytes(), ig.Bytes())
			})
		}
	}
}

func osculationExchangeSource(t *testing.T, op string, axis, side, order int) []byte {
	t.Helper()
	raw, err := os.ReadFile(fmt.Sprintf("testdata/native_parabola_circle_osculation_%s-axis-%d-side-%d-order-%d.json", op, axis, side, order))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func checkOsculationNeutralReadback(t *testing.T, ctx context.Context, op string, axis, side int, st, ig []byte) {
	t.Helper()
	for _, f := range []struct {
		reader, format string
		data           []byte
	}{
		{os.Getenv("PLAINSOLID_OCCT_OSCULATION_STEP_JETS"), "step", st},
		{os.Getenv("PLAINSOLID_OCCT_OSCULATION_IGES_JETS"), "igs", ig},
	} {
		if f.reader == "" {
			continue
		}
		path := filepath.Join(t.TempDir(), "osculating."+f.format)
		if err := os.WriteFile(path, f.data, 0600); err != nil {
			t.Fatal(err)
		}
		out, err := exec.CommandContext(ctx, f.reader, path, strconv.Itoa(axis), strconv.Itoa(side), op).CombinedOutput()
		if err != nil {
			t.Fatalf("%s osculation readback: %v: %s", f.format, err, out)
		}
		start := bytes.Index(out, []byte(`{"ApexPairs":`))
		if start < 0 {
			t.Fatalf("%s has no osculation readback report: %s", f.format, out)
		}
		var report struct {
			ApexPairs                    int
			MaximumFourthDerivativeError float64
			Components                   []struct {
				Faces, Parabola, Circle, Bridge, DirectedUses, BridgeUses, DirectedApexUses, ExactParabolicMidpoints int
			}
		}
		if err := json.Unmarshal(bytes.TrimSpace(out[start:]), &report); err != nil {
			t.Fatal(err)
		}
		pairs, shells := 0, 1
		if op == "cut" {
			pairs, shells = 1, 2
		}
		if report.ApexPairs != pairs || len(report.Components) != shells || !(report.MaximumFourthDerivativeError <= 2e-7) {
			t.Fatalf("%s changed the exact contact pairing or fourth jet: %+v", f.format, report)
		}
		faces := 0
		for _, p := range report.Components {
			var want [7]int
			switch p.Faces {
			case 12:
				want = [7]int{4, 4, 1, 16, 2, 8, 2}
			case 5:
				want = [7]int{2, 0, 0, 4, 0, 4, 0}
			case 2:
				want = [7]int{0, 4, 0, 8, 0, 4, 0}
			default:
				t.Fatalf("%s unexpected original shell faces: %+v", f.format, p)
			}
			faces += p.Faces
			got := [7]int{p.Parabola, p.Circle, p.Bridge, p.DirectedUses, p.BridgeUses, p.DirectedApexUses, p.ExactParabolicMidpoints}
			if got != want {
				t.Fatalf("%s changed original arc, subdivision or bridge: %+v", f.format, p)
			}
		}
		if (op == "cut" && faces != 7) || (op != "cut" && faces != 12) {
			t.Fatalf("%s changed original shell grouping: %+v", f.format, report)
		}
		t.Logf("%s retained the original fourth derivative with maximum error %g", f.format, report.MaximumFourthDerivativeError)
	}
}
