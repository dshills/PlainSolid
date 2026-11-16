package exchange_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
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

func TestAcnodalCubicCutNeutralTransfer(t *testing.T) {
	ctx := context.Background()
	const name, units = "Acnodal Cut", .001
	source, err := os.ReadFile("testdata/native_acnodal_cubic_cut.json")
	if err != nil {
		t.Fatal(err)
	}
	cut, err := trimmed.ReadAcnodalCubicCutJSON(ctx, bytes.NewReader(source), trimmed.JSONLimits{}, trimmed.AcnodalCubicCutOptions{})
	if err != nil {
		t.Fatal(err)
	}
	// Independently integrate the authored cap gap using
	// y(t)=(1/8+t²)*t, dx=2t dt and 0<=t<=1/4 in its normalized
	// footprint. The physical determinant is six and the height factor 3/16.
	want := big.NewRat(4703, 1614807040)
	if cut.ExactVolume().Cmp(want) != 0 {
		t.Fatal("source receipt carries the wrong positive volume", cut.ExactVolume())
	}
	volume, _ := want.Float64()

	so := step.AcnodalCubicCutSTEPOptions{PoleTolerance: 1e-14}
	iopts := iges.AcnodalCubicCutIGESOptions{PoleTolerance: 1e-14}
	var st, ig bytes.Buffer
	if err := step.WriteAcnodalCubicCut(ctx, &st, cut, name, units, step.Limits{}, so); err != nil {
		t.Fatal(err)
	}
	if err := iges.WriteAcnodalCubicCut(ctx, &ig, cut, units, iges.Limits{}, iopts); err != nil {
		t.Fatal(err)
	}
	if err := step.VerifyAcnodalCubicCutSTEP(ctx, bytes.NewReader(st.Bytes()), cut, name, units, step.Limits{}, so); err != nil {
		t.Fatal(err)
	}
	if err := iges.VerifyAcnodalCubicCutIGES(ctx, bytes.NewReader(ig.Bytes()), cut, units, iges.Limits{}, iopts); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		format, ext, reader, digest string
		data                        []byte
	}{
		{"step", "step", os.Getenv("PLAINSOLID_OCCT_CLIPPED_CUT_STEP_COMPONENTS"),
			"f15618153144f4eefa77be280c37a7896edf1e72ee1623ad6b12e546817ee8be", st.Bytes()},
		{"iges", "igs", os.Getenv("PLAINSOLID_OCCT_CLIPPED_CUT_IGES"),
			"5fbda8930b2ef3dfa8a8efe4e10d62ffd874f1620a45357760fb83e23f63483d", ig.Bytes()},
	}
	for _, f := range tests {
		t.Run(f.format, func(t *testing.T) {
			if fmt.Sprintf("%x", sha256.Sum256(f.data)) != f.digest {
				t.Fatal("public writer changed the independently reopened analytic file")
			}
			if f.reader != "" {
				report := readPinnedExactComponents(t, ctx, f.ext, f.data, f.reader)
				if report.Solids != 1 || report.IndexedVertices != 5 || len(report.Components) != 1 {
					t.Fatal("readback changed the single cap or added a vertex", report)
				}
				body := report.Components[0]
				isolated := [3]float64{39.0 / 16, 1, 2243.0 / 1024}
				if body.Shells != 1 || body.Edges != 7 || body.Faces != 4 || slices.Contains(body.Vertices, isolated) ||
					!(math.Abs(body.Volume-volume) <= 1e-13) {
					t.Fatal("readback changed signed material or introduced an isolated contact", body)
				}
				vertices := map[[3]float64]bool{}
				for _, p := range cut.Boundary().Vertices() {
					vertices[[3]float64{p.X, p.Y, p.Z}] = true
				}
				for _, p := range body.Vertices {
					if !vertices[p] {
						t.Fatal("readback moved an exact original vertex", p)
					}
					delete(vertices, p)
				}
				if len(vertices) != 0 {
					t.Fatal("readback omitted an original vertex", vertices)
				}
				t.Logf("one solid, %d faces, %d edges, %d vertices, signed volume %.16g",
					body.Faces, body.Edges, report.IndexedVertices, body.Volume)
			}
			checkAcnodalCutReadbackJets(t, ctx, f.format, f.ext, f.data)
		})
	}

	altered := bytes.Replace(st.Bytes(), []byte(".CURVE_3D."), []byte(".PCURVE_S1."), 1)
	if bytes.Equal(altered, st.Bytes()) ||
		!errors.Is(step.VerifyAcnodalCubicCutSTEP(ctx, bytes.NewReader(altered), cut, name, units, step.Limits{}, so), geom.ErrConvergence) {
		t.Fatal("changed STEP arc owner passed verification")
	}
	if err := iges.VerifyAcnodalCubicCutIGES(ctx, bytes.NewReader(ig.Bytes()), cut, 1, iges.Limits{}, iopts); !errors.Is(err, geom.ErrConvergence) {
		t.Fatal("changed IGES unit passed verification", err)
	}
	var refused bytes.Buffer
	if err := step.WriteAcnodalCubicCut(ctx, &refused, cut, name, units, step.Limits{MaxEdgeUses: 13}, so); err == nil || refused.Len() != 0 {
		t.Fatal("limited STEP writer published a partial Cut", err)
	}
	if err := iges.WriteAcnodalCubicCut(ctx, &refused, cut, units, iges.Limits{MaxControlPoints: 1}, iopts); err == nil || refused.Len() != 0 {
		t.Fatal("limited IGES writer published a partial Cut", err)
	}
}

func checkAcnodalCutReadbackJets(t *testing.T, ctx context.Context, format, ext string, data []byte) {
	t.Helper()
	reader := os.Getenv("PLAINSOLID_OCCT_ACNODAL_CUT_READBACK")
	if reader == "" {
		return
	}
	path := filepath.Join(t.TempDir(), "acnodal-cut."+ext)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	args := []string{format, path}
	for _, x := range [...]float64{45.0 / 16, 1, 2273.0 / 1024, 39.0 / 16, 1, 2243.0 / 1024} {
		args = append(args, strconv.FormatFloat(x, 'g', -1, 64))
	}
	raw, err := exec.CommandContext(ctx, reader, args...).CombinedOutput()
	if err != nil {
		t.Fatalf("independent %s regular arc reader: %v\n%s", format, err, raw)
	}
	var result struct {
		IsolatedOutside bool
		Germs           []struct {
			Exit, First [3]float64
			Owners      []struct {
				Orientation int
				UV, First   [2]float64
			}
		}
	}
	for _, line := range bytes.Split(raw, []byte{'\n'}) {
		if line = bytes.TrimSpace(line); bytes.HasPrefix(line, []byte{'{'}) {
			if err := json.Unmarshal(line, &result); err != nil {
				t.Fatal(err)
			}
		}
	}
	if !result.IsolatedOutside || len(result.Germs) != 2 {
		t.Fatal("independent arc readback changed the regular turn or isolated exclusion", string(raw))
	}
	exits := map[[3]float64]float64{
		{3, 29.0 / 32, 4567.0 / 2048}: -1,
		{3, 35.0 / 32, 4585.0 / 2048}: 1,
	}
	for _, germ := range result.Germs {
		sign, valid := exits[germ.Exit]
		if !valid {
			t.Fatal("regular germ lost its exact terminal point", germ.Exit)
		}
		delete(exits, germ.Exit)
		want := [3]float64{0, sign / 16, sign * 45.0 / 16384}
		for i, derivative := range germ.First {
			if !(math.Abs(derivative-want[i]) <= 1e-12) {
				t.Fatal("readback changed a regular model first derivative", germ.First, want)
			}
		}
		if len(germ.Owners) != 2 || germ.Owners[0].Orientation*germ.Owners[1].Orientation != -1 {
			t.Fatal("the original arc must retain opposite directed roof uses", germ)
		}
		owners := map[[2]float64][2]float64{
			{3, -11.0 / 4}:  {sign / 16, 0},
			{15.0 / 16, .5}: {0, sign / 32},
		}
		for _, use := range germ.Owners {
			want, valid := owners[use.UV]
			if !valid || !(math.Abs(use.First[0]-want[0]) <= 1e-12) || !(math.Abs(use.First[1]-want[1]) <= 1e-12) {
				t.Fatal("readback changed an original chart or its directed turn derivative", use, want)
			}
			delete(owners, use.UV)
		}
	}
	t.Log("original turning jets and both directed roof uses restored, isolated point outside")
}
