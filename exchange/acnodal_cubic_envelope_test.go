package exchange_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"testing"

	"plainsolid/brep/trimmed"
	"plainsolid/exchange/iges"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

func authoredAcnodalCubicStock(t *testing.T, arc bool) [3]*geom.BSplineSurface {
	t.Helper()
	// A−B=(3/16)*((v-1/2)^2-(u-13/16)^2*(u-13/16-gap)).
	// The controls below are the original degree-elevated free-coordinate term.
	q := [4]float64{-8619, 1157, -75, -27}
	if arc {
		q = [4]float64{-7605, 1339, -213, 27}
	}
	var lower, b, a []geom.Vec3
	for i := range 2 {
		for j := range 2 {
			u, v := float64(i), float64(j)
			lower = append(lower, geom.Vec3{X: 3 * u, Y: 2 * v, Z: (u + v) / 16})
			b = append(b, geom.Vec3{X: 3 * (1 - v), Y: 2 * u, Z: 2 + 3*(1-v)/16 + 3*(1-v)*u/32})
		}
	}
	for i, offset := range q {
		for j, dep := range [3]float64{3. / 64, -3. / 64, 3. / 64} {
			u, v := float64(i), float64(j)/2
			a = append(a, geom.Vec3{X: u, Y: 2 * v, Z: 2 + u/16 + u*v/32 + dep - offset/65536})
		}
	}
	var result [3]*geom.BSplineSurface
	for i, x := range []struct {
		u, v   int
		poles  []geom.Vec3
		ku, kv []float64
	}{
		{1, 1, lower, []float64{0, 0, 3, 3}, []float64{0, 0, 2, 2}},
		{3, 2, a, []float64{0, 0, 0, 0, 1, 1, 1, 1}, []float64{0, 0, 0, 1, 1, 1}},
		{1, 1, b, []float64{2, 2, 4, 4}, []float64{-3, -3, 1, 1}},
	} {
		var err error
		result[i], err = geom.NewBSplineSurface(x.u, x.v, x.u+1, x.v+1, x.poles, x.ku, x.kv, nil)
		if err != nil {
			t.Fatal(err)
		}
	}
	return result
}

func TestAcnodalCubicEnvelopeNeutralTransfer(t *testing.T) {
	ctx := context.Background()
	const unit = .001
	so := step.AcnodalCubicSTEPOptions{PoleTolerance: 1e-14}
	io := iges.AcnodalCubicIGESOptions{PoleTolerance: 1e-14}
	for _, arc := range []bool{false, true} {
		source := authoredAcnodalCubicStock(t, arc)
		volumes := [2]float64{789. / 64, 412713. / 32768}
		counts := [3]int{6, 12, 8}
		if arc {
			volumes = [2]float64{19907538337. / 1614807040, 20297401823. / 1614807040}
			counts = [3]int{9, 20, 13}
		}
		for order := range 2 {
			for i, build := range []func(context.Context, *geom.BSplineSurface, *geom.BSplineSurface, *geom.BSplineSurface, int, float64, trimmed.AcnodalCubicEnvelopeOptions) (*trimmed.AcnodalCubicEnvelope, error){
				trimmed.CommonAcnodalCubicSplineGraphs, trimmed.FuseAcnodalCubicSplineGraphs,
			} {
				op := [2]string{"Common", "Fuse"}[i]
				t.Run(fmt.Sprintf("arc_%v/order_%d/%s", arc, order, op), func(t *testing.T) {
					id := "Acnodal " + op
					model, err := build(ctx, source[0], source[1+order], source[2-order], order, 1e-9, trimmed.AcnodalCubicEnvelopeOptions{})
					if err != nil {
						t.Fatal(err)
					}
					var stepFile, igesFile bytes.Buffer
					if err := step.WriteAcnodalCubicEnvelope(ctx, &stepFile, model, id, unit, step.Limits{}, so); err != nil {
						t.Fatal(err)
					}
					if err := iges.WriteAcnodalCubicEnvelope(ctx, &igesFile, model, unit, iges.Limits{}, io); err != nil {
						t.Fatal(err)
					}
					if err := step.VerifyAcnodalCubicEnvelopeSTEP(ctx, bytes.NewReader(stepFile.Bytes()), model, id, unit, step.Limits{}, so); err != nil {
						t.Fatal(err)
					}
					if err := iges.VerifyAcnodalCubicEnvelopeIGES(ctx, bytes.NewReader(igesFile.Bytes()), model, unit, iges.Limits{}, io); err != nil {
						t.Fatal(err)
					}
					point, err := geom.RepresentedExactPoint(model.IsolatedContact())
					if err != nil {
						t.Fatal(err)
					}
					wantPoint := [3]float64{point.X, point.Y, point.Z}
					oracle := os.Getenv("PLAINSOLID_OCCT_ACNODAL_INTERIOR")
					for _, media := range []struct {
						fmt, ext, components string
						data                 []byte
					}{
						{"step", "step", os.Getenv("PLAINSOLID_OCCT_CLIPPED_CUT_STEP_COMPONENTS"), stepFile.Bytes()},
						{"iges", "igs", os.Getenv("PLAINSOLID_OCCT_CLIPPED_CUT_IGES"), igesFile.Bytes()},
					} {
						if media.components != "" {
							report := readPinnedExactComponents(t, ctx, media.ext, media.data, media.components)
							if report.Solids != 1 || report.IndexedVertices != counts[2] || len(report.Components) != 1 {
								t.Fatal("reopen did not retain one closed original shell", media.fmt, report)
							}
							body := report.Components[0]
							if body.Shells != 1 || body.Faces != counts[0] || body.Edges != counts[1] ||
								math.Abs(body.Volume-volumes[i]) > 1e-9 || slices.Contains(body.Vertices, wantPoint) {
								t.Fatal("changed topology, isolated contact occurrence or material", media.fmt, body)
							}
							digest := sha256.Sum256(media.data)
							t.Logf("%s %d faces %d edges %d vertices, signed volume %.16g; sha256=%x",
								media.fmt, body.Faces, body.Edges, report.IndexedVertices, body.Volume, digest)
						}
						if oracle != "" {
							path := filepath.Join(t.TempDir(), "acnodal."+media.ext)
							if err := os.WriteFile(path, media.data, 0600); err != nil {
								t.Fatal(err)
							}
							args := []string{media.fmt, path}
							for _, q := range wantPoint {
								args = append(args, strconv.FormatFloat(q, 'g', -1, 64))
							}
							raw, err := exec.CommandContext(ctx, oracle, args...).CombinedOutput()
							if err != nil {
								t.Fatalf("classify reopened isolated point: %v: %s", err, raw)
							}
							line := bytes.LastIndexByte(raw, '{')
							var state struct{ InteriorFaces, BoundaryFaces int }
							if line == -1 || json.Unmarshal(raw[line:], &state) != nil || state.InteriorFaces != 1 || state.BoundaryFaces != 0 {
								t.Fatalf("isolated point regained a seam: %s", raw)
							}
						}
					}
					if arc && order == 0 && i == 0 {
						var none bytes.Buffer
						if err := step.WriteAcnodalCubicEnvelope(ctx, &none, model, id, unit, step.Limits{MaxFaces: 1}, so); err == nil || none.Len() != 0 {
							t.Fatal("STEP published an incomplete envelope", err)
						}
						if err := iges.WriteAcnodalCubicEnvelope(ctx, &none, model, unit, iges.Limits{MaxControlPoints: 1}, io); err == nil || none.Len() != 0 {
							t.Fatal("IGES published an incomplete envelope", err)
						}
						tampered := bytes.Replace(stepFile.Bytes(), []byte(".CURVE_3D."), []byte(".PCURVE_S1."), 1)
						if !errors.Is(step.VerifyAcnodalCubicEnvelopeSTEP(ctx, bytes.NewReader(tampered), model, id, unit, step.Limits{}, so), geom.ErrConvergence) {
							t.Fatal("changed STEP curve ownership was accepted")
						}
					}
				})
			}
		}
	}
}
