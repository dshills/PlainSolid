package exchange_test

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"plainsolid/brep/trimmed"
	"plainsolid/exchange/iges"
	"plainsolid/exchange/step"
)

func TestFourTransverseClippedFuseExchange(t *testing.T) {
	ctx := context.Background()
	for _, c := range []struct {
		layout                 string
		faces, edges, vertices int
		volume, area           float64
	}{
		{"partial", 22, 54, 34, 5205974907560435.0 / 314460325543936.0, 40.256084657152932},
		{"dependent", 23, 57, 36, 2570722139241407.0 / 157230162771968.0, 39.788493372566357},
	} {
		t.Run(c.layout, func(t *testing.T) {
			recipe, err := os.ReadFile("testdata/native_clipped_polynomial_four_fuse_" + c.layout + ".json")
			if err != nil {
				t.Fatal(err)
			}
			fuse, err := trimmed.ReadClippedPolynomialFuseJSON(ctx, bytes.NewReader(recipe), trimmed.JSONLimits{}, trimmed.PolynomialCubicCrossingTangencyEnvelopeOptions{})
			if err != nil {
				t.Fatal(err)
			}
			const name = "Four-contact Fuse"
			const unit = .001
			var stepData, igesData bytes.Buffer
			so := step.ClippedPolynomialFuseSTEPOptions{PoleTolerance: 1e-14}
			if err := step.WriteClippedPolynomialFuse(ctx, &stepData, fuse, name, unit, step.Limits{}, so); err != nil {
				t.Fatal(err)
			}
			if err := step.VerifyClippedPolynomialFuseSTEP(ctx, bytes.NewReader(stepData.Bytes()), fuse, name, unit, step.Limits{}, so); err != nil {
				t.Fatal(err)
			}
			f, err := step.Parse(bytes.NewReader(stepData.Bytes()), step.Limits{})
			if err != nil {
				t.Fatal(err)
			}
			counts := map[string]int{}
			for _, e := range f.Entities {
				for _, r := range e.Records {
					counts[r.Type]++
				}
			}
			if counts["MANIFOLD_SOLID_BREP"] != 1 || counts["ADVANCED_FACE"] != c.faces ||
				counts["EDGE_CURVE"] != c.edges || counts["VERTEX_POINT"] != c.vertices || counts["PCURVE"] != 2*c.edges {
				t.Fatal("STEP must keep the original stepped topology and both chart owners per edge", counts)
			}
			io := iges.ClippedPolynomialFuseIGESOptions{PoleTolerance: 1e-14}
			if err := iges.WriteClippedPolynomialFuse(ctx, &igesData, fuse, unit, iges.Limits{}, io); err != nil {
				t.Fatal(err)
			}
			if err := iges.VerifyClippedPolynomialFuseIGES(ctx, bytes.NewReader(igesData.Bytes()), fuse, unit, iges.Limits{}, io); err != nil {
				t.Fatal(err)
			}

			check := func(format string, data []byte, reader string, readArgs ...string) {
				t.Helper()
				if reader == "" {
					return
				}
				path := filepath.Join(t.TempDir(), "four-fuse."+format)
				if err := os.WriteFile(path, data, 0600); err != nil {
					t.Fatal(err)
				}
				raw, err := exec.CommandContext(ctx, reader, append(readArgs, path)...).CombinedOutput()
				if err != nil {
					t.Fatalf("independent %s reader: %v\n%s", format, err, raw)
				}
				var got struct {
					Faces, Solids, IndexedVertices int
					Volume, Area                   float64
					Components                     []struct {
						Edges, Shells int
						Volume, Area  float64
					}
				}
				var report []byte
				for _, line := range bytes.Split(raw, []byte{'\n'}) {
					if line = bytes.TrimSpace(line); bytes.HasPrefix(line, []byte{'{'}) {
						report = line
					}
				}
				if err := json.Unmarshal(report, &got); err != nil {
					t.Fatalf("independent %s material report: %v\n%s", format, err, raw)
				}
				if format == "igs" {
					if len(got.Components) != 1 || got.IndexedVertices != c.vertices || got.Components[0].Edges != c.edges || got.Components[0].Shells != 1 {
						t.Fatalf("independent IGES lost original topology: %+v", got)
					}
					got.Volume, got.Area = got.Components[0].Volume, got.Components[0].Area
				}
				if got.Solids != 1 || got.Faces != c.faces || math.Abs(got.Volume-c.volume) > 1e-7 || math.Abs(got.Area-c.area) > 1e-7 {
					t.Fatalf("independent %s material: %+v, exact volume %.16g and reference area %.16g", format, got, c.volume, c.area)
				}
				t.Logf("independent %s: %d faces, volume %.16g, area %.16g", format, got.Faces, got.Volume, got.Area)
			}
			check("step", stepData.Bytes(), os.Getenv("PLAINSOLID_OCCT_CLIPPED_FUSE_STEP"), "read")
			check("igs", igesData.Bytes(), os.Getenv("PLAINSOLID_OCCT_CLIPPED_CUT_IGES"))
		})
	}
}
