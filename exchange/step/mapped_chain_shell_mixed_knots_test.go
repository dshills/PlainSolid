package step

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

const mappedChainMixedVolume = 251787937.0 / 268435456

func mappedChainMixedSTEP(t *testing.T, ctx context.Context) ([]byte, MappedChainShellSTEPOptions, trimmed.MappedSplineChainShell) {
	t.Helper()
	raw, err := os.ReadFile("testdata/mapped_chain_mixed_crossings_source.json")
	if err != nil {
		t.Fatal(err)
	}
	shell, err := trimmed.ReadMappedSplineChainShellJSON(ctx, bytes.NewReader(raw), trimmed.MappedChainShellNativeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	o := MappedChainShellSTEPOptions{AllowMultiSpanRoofs: true, AllowTwoInteriorRoofKnots: true}
	var data bytes.Buffer
	if err := WriteMappedChainRectangularShell(ctx, &data, shell, "Mixed original roof crossings", .001, Limits{}, o); err != nil {
		t.Fatal(err)
	}
	return data.Bytes(), o, shell
}

func TestMappedChainRectangularSTEPMixedSupportKnots(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	raw, o, shell := mappedChainMixedSTEP(t, ctx)
	if out, err := ReadMappedChainRectangularShell(ctx, bytes.NewReader(raw), Limits{}, MappedChainShellSTEPOptions{AllowMultiSpanRoofs: true}); err == nil ||
		len(out.Native.Solid.Faces()) != 0 {
		t.Fatal("the mixed shell has an original roof with two support knots", err)
	}
	m, err := ReadMappedChainRectangularShell(ctx, bytes.NewReader(raw), Limits{}, o)
	if err != nil {
		t.Fatal(err)
	}
	solid := m.Native.Solid
	if len(m.Chain) != 5 || len(solid.Faces()) != 7 || len(solid.Edges()) != 22 || len(solid.Vertices()) != 17 ||
		m.Native.ChainStations[1].Cmp(big.NewRat(1, 8)) != 0 || m.Native.ChainStations[2].Cmp(big.NewRat(3, 16)) != 0 {
		t.Fatal("explicit STEP output lost one mixed crossing or its common model station")
	}
	prior, err := shell.BuildRectangularSolid(ctx, 1e-9, o.Geometry)
	if err != nil {
		t.Fatal(err)
	}
	faces, sourceFaces := solid.Faces(), prior.Solid.Faces()
	wantedRaw := [2][2]int64{{41, 65}, {51, 43}}
	for owner := range 2 {
		if !geom.SameSplineSurface(faces[owner].Surface.(*geom.BSplineSurface), sourceFaces[owner].Surface.(*geom.BSplineSurface)) {
			t.Fatal("reopened STEP must retain both original support knot vectors and nets", owner)
		}
		for i := range 3 {
			a := faces[owner].Outer[i].Curve.(trimmed.MappedSplineUV)
			b := sourceFaces[owner].Outer[i].Curve.(trimmed.MappedSplineUV)
			x, y := a.Source(), b.Source()
			if x.Range != y.Range || x.Reversed != y.Reversed ||
				!slices.Equal(x.Curve.Poles(), y.Curve.Poles()) || !slices.Equal(x.Curve.Weights(), y.Curve.Weights()) ||
				!slices.Equal(x.Curve.Knots(), y.Curve.Knots()) || m.Chain[i].Owners[owner].BasisID != m.Chain[0].Owners[owner].BasisID {
				t.Fatal("STEP changed one exact restriction or original whole basis", owner, i)
			}
			if i < 2 {
				t0, err := a.AuthoredParameter(big.NewRat(1, 1))
				if err != nil || t0.Cmp(big.NewRat(wantedRaw[owner][i], 1)) != 0 {
					t.Fatal("wrong original raw crossing parameter", owner, i, err)
				}
			}
		}
	}
	proof, err := solid.ValidateEdgeAgreement(ctx, edgecheck.Options{})
	if err != nil || !proof.Complete || len(proof.Uses) != 44 {
		t.Fatal("reopened material must prove every exact source edge incidence", err)
	}
	mass, err := solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
	if err != nil || math.Abs(mass.Volume-mappedChainMixedVolume) > 1e-8 {
		t.Fatal("reopened mixed-crease material volume disagrees with the analytic roof", mass.Volume, err)
	}
	var replay bytes.Buffer
	if err := WriteMappedChainRectangularShell(ctx, &replay, m.Shell, m.Name, m.LengthUnit, Limits{}, o); err != nil ||
		!bytes.Equal(raw, replay.Bytes()) {
		t.Fatal("explicit repaired coedges must replay byte-identically", err)
	}
}

func TestMappedChainSupportKnotNativeRecordCompatibility(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	for _, name := range []string{"mapped_chain_two_crossings_source.json", "mapped_chain_asynchronous_crossings_source.json", "mapped_chain_mixed_crossings_source.json"} {
		raw, err := os.ReadFile(filepath.Join("testdata", name))
		if err != nil {
			t.Fatal(err)
		}
		shell, err := trimmed.ReadMappedSplineChainShellJSON(ctx, bytes.NewReader(raw), trimmed.MappedChainShellNativeOptions{})
		if err != nil {
			t.Fatal(err)
		}
		var replay bytes.Buffer
		if err := shell.WriteJSON(ctx, &replay, trimmed.MappedChainShellNativeOptions{}); err != nil || !bytes.Equal(raw, replay.Bytes()) {
			t.Fatal("existing synchronized, asynchronous, and mixed boundary records must remain byte-identical", name, err)
		}
	}
}

func TestMappedChainRectangularSTEPMixedSupportKnotsOCCT(t *testing.T) {
	t.Parallel()
	exe := os.Getenv("PLAINSOLID_OCCT_MAPPED_CHAIN_STEP")
	if exe == "" {
		t.Skip("optional pinned OCCT STEP consumer")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	raw, _, _ := mappedChainMixedSTEP(t, ctx)
	path := filepath.Join(t.TempDir(), "mixed-support-crossings.step")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	data, err := exec.CommandContext(ctx, exe, "read", path).CombinedOutput()
	if err != nil {
		t.Fatalf("pinned OCCT rejected the mixed-crease material: %v\n%s", err, data)
	}
	var p struct {
		Volume, Area  float64
		Faces, Solids int
	}
	if err := json.Unmarshal(data, &p); err != nil {
		t.Fatal(err)
	}
	if p.Solids != 1 || p.Faces != 7 || math.Abs(p.Volume-mappedChainMixedVolume) > 1e-8 ||
		math.Abs(p.Area-6.1387980719367325) > 1e-8 {
		t.Fatal("pinned OCCT reopened the wrong mixed-crease material", p)
	}
	t.Logf("OCCT reopened one seven-face solid, volume %.15g, area %.15g", p.Volume, p.Area)
}
