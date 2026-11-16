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
	"strconv"
	"testing"

	"plainsolid/brep/trimmed"
	"plainsolid/exchange/iges"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

func TestCuspidalCubicCommonExchange(t *testing.T) {
	ctx := context.Background()
	const name, units = "Cuspidal Common", .001
	for order := range 2 {
		t.Run(fmt.Sprint(order), func(t *testing.T) {
			raw, err := os.ReadFile(fmt.Sprintf("testdata/native_cuspidal_cubic_common_order_%d.json", order))
			if err != nil {
				t.Fatal(err)
			}
			common, err := trimmed.ReadCuspidalCubicCommonJSON(ctx, bytes.NewReader(raw), trimmed.JSONLimits{}, trimmed.CuspidalCubicCommonOptions{})
			if err != nil {
				t.Fatal(err)
			}
			so := step.CuspidalCubicSTEPOptions{PoleTolerance: 1e-14}
			iopts := iges.CuspidalCubicIGESOptions{PoleTolerance: 1e-14}
			var st, ig bytes.Buffer
			if err := step.WriteCuspidalCubicCommon(ctx, &st, common, name, units, step.Limits{}, so); err != nil {
				t.Fatal(err)
			}
			if err := iges.WriteCuspidalCubicCommon(ctx, &ig, common, units, iges.Limits{}, iopts); err != nil {
				t.Fatal(err)
			}
			// Both old operand orders produced the same canonical neutral
			// bytes before rational knot splitting became available.
			if fmt.Sprintf("%x", sha256.Sum256(st.Bytes())) != "50507a39cc2606e55dd0c490d0a67978af1cb4070e18f9df1b296c91a83e6a34" ||
				fmt.Sprintf("%x", sha256.Sum256(ig.Bytes())) != "00c6f4004ea066266e9a89b5693f414c6775ba38a7365d328db0a113cdecd3af" {
				t.Fatal("uncreased Common neutral bytes changed")
			}
			if err := step.VerifyCuspidalCubicCommonSTEP(ctx, bytes.NewReader(st.Bytes()), common, name, units, step.Limits{}, so); err != nil {
				t.Fatal(err)
			}
			if err := iges.VerifyCuspidalCubicCommonIGES(ctx, bytes.NewReader(ig.Bytes()), common, units, iges.Limits{}, iopts); err != nil {
				t.Fatal(err)
			}
			tampered := bytes.Replace(st.Bytes(), []byte(".CURVE_3D."), []byte(".PCURVE_S1."), 1)
			if bytes.Equal(tampered, st.Bytes()) || !errors.Is(step.VerifyCuspidalCubicCommonSTEP(ctx, bytes.NewReader(tampered), common, name, units, step.Limits{}, so), geom.ErrConvergence) {
				t.Fatal("changed cusp source owner was accepted")
			}
			if err := iges.VerifyCuspidalCubicCommonIGES(ctx, bytes.NewReader(ig.Bytes()), common, 1, iges.Limits{}, iopts); !errors.Is(err, geom.ErrConvergence) {
				t.Fatalf("changed physical units accepted: %v", err)
			}
			point := common.Boundary().Vertices()[common.History().Vertices[0]]
			for _, f := range []struct {
				name, ext, reader string
				data              []byte
			}{
				{"step", "step", os.Getenv("PLAINSOLID_OCCT_CLIPPED_CUT_STEP_COMPONENTS"), st.Bytes()},
				{"iges", "igs", os.Getenv("PLAINSOLID_OCCT_CLIPPED_CUT_IGES"), ig.Bytes()},
			} {
				if f.reader != "" {
					report := readPinnedExactComponents(t, ctx, f.ext, f.data, f.reader)
					if report.Solids != 1 || len(report.Components) != 1 || report.IndexedVertices != 13 {
						t.Fatalf("independent %s lost the original cusp shell: %+v", f.name, report)
					}
					part := report.Components[0]
					if part.Shells != 1 || part.Faces != 9 || part.Edges != 20 ||
						math.Abs(part.Volume-277485.0/22528) > 1e-8 || math.Abs(part.Area-32.5442218530794) > 1e-8 {
						t.Fatalf("independent %s changed material or topology: %+v", f.name, part)
					}
					original := map[[3]float64]bool{}
					for _, p := range common.Boundary().Vertices() {
						original[[3]float64{p.X, p.Y, p.Z}] = true
					}
					for _, p := range part.Vertices {
						if !original[p] {
							t.Fatalf("independent %s changed exact vertex %v", f.name, p)
						}
						delete(original, p)
					}
					if len(original) != 0 {
						t.Fatalf("independent %s lost an original vertex", f.name)
					}
				}
				checkCuspidalCubicReadbackJets(t, ctx, f.name, f.ext, f.data, point)
			}
		})
	}
}

func checkCuspidalCubicReadbackJets(t *testing.T, ctx context.Context, format, ext string, data []byte, p geom.Vec3) {
	t.Helper()
	checkCuspidalCubicJetsOnSpan(t, ctx, format, ext, data, p, 1)
}

func checkCuspidalCubicJetsOnSpan(t *testing.T, ctx context.Context, format, ext string, data []byte, p geom.Vec3, fraction float64) {
	t.Helper()
	reader := os.Getenv("PLAINSOLID_OCCT_CUSPIDAL_CUBIC_JETS")
	if reader == "" {
		return
	}
	path := filepath.Join(t.TempDir(), "cuspidal."+ext)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	args := []string{format, path}
	for _, x := range [...]float64{p.X, p.Y, p.Z} {
		args = append(args, strconv.FormatFloat(x, 'g', -1, 64))
	}
	raw, err := exec.CommandContext(ctx, reader, args...).CombinedOutput()
	if err != nil {
		t.Fatalf("independent %s cusp reader: %v\n%s", format, err, raw)
	}
	var report struct {
		Germs []struct{ First, Second, Third [3]float64 }
	}
	for _, line := range bytes.Split(raw, []byte{'\n'}) {
		if line = bytes.TrimSpace(line); bytes.HasPrefix(line, []byte{'{'}) {
			if err := json.Unmarshal(line, &report); err != nil {
				t.Fatal(err)
			}
		}
	}
	if len(report.Germs) != 2 {
		t.Fatalf("independent %s did not find both original germs", format)
	}
	// In the normalized outward edge parameter, the source chart gives
	// second X=9/2 and third Y=±9/2. At the cusp the original common
	// height law gives dZ/dX=5/64 and dZ/dY=3/256.
	wantSecond := [3]float64{4.5, 0, 45.0 / 128}
	wantThird := [3]float64{0, 4.5, 27.0 / 512}
	var sided [2]bool
	for _, germ := range report.Germs {
		if germ.First != ([3]float64{}) {
			t.Fatalf("independent %s regularized the original cusp: %+v", format, germ)
		}
		sign := math.Copysign(1, germ.Third[1])
		side := 0
		if sign > 0 {
			side = 1
		}
		if sided[side] {
			t.Fatalf("independent %s duplicated a cusp third-jet side: %+v", format, report.Germs)
		}
		sided[side] = true
		for axis := range 3 {
			second := wantSecond[axis] * fraction * fraction
			third := sign * wantThird[axis] * fraction * fraction * fraction
			if !(math.Abs(germ.Second[axis]-second) <= 1e-12) ||
				!(math.Abs(germ.Third[axis]-third) <= 1e-12) {
				t.Fatalf("independent %s changed the second or third derivative at axis %d: %+v", format, axis, germ)
			}
		}
	}
}
