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
	a, b := report.Germs[0], report.Germs[1]
	if a.First != ([3]float64{}) || b.First != ([3]float64{}) || a.Second[0] <= 0 ||
		a.Third[1]*b.Third[1] >= 0 || math.Abs(a.Third[1]) < 4 {
		t.Fatalf("independent %s changed the genuine singularity or its two third-jet sides: %+v", format, report.Germs)
	}
	for axis := range 3 {
		if math.Abs(a.Second[axis]-b.Second[axis]) > 1e-12 || math.Abs(a.Third[axis]+b.Third[axis]) > 1e-12 {
			t.Fatalf("independent %s changed a shared second or opposite third derivative at axis %d", format, axis)
		}
	}
}
