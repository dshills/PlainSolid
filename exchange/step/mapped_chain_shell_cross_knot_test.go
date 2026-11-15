package step

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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

const mappedChainCrossKnotVolume = 2022363.0 / 2097152

func crossKnotMappedSTEP(t *testing.T, ctx context.Context) ([]byte, trimmed.MappedSplineChainShell) {
	t.Helper()
	raw, err := os.ReadFile("testdata/mapped_chain_cross_knot_source.json")
	if err != nil {
		t.Fatal(err)
	}
	shell, err := trimmed.ReadMappedSplineChainShellJSON(ctx, bytes.NewReader(raw), trimmed.MappedChainShellNativeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := WriteMappedChainRectangularShell(ctx, &output, shell, "Repaired exact support crossing", .001, Limits{}, MappedChainShellSTEPOptions{
		AllowMultiSpanRoofs: true,
	}); err != nil {
		t.Fatal(err)
	}
	return output.Bytes(), shell
}

func TestMappedChainRectangularSTEPRepairedSupportCrossing(t *testing.T) {
	ctx := context.Background()
	raw, shell := crossKnotMappedSTEP(t, ctx)
	o := MappedChainShellSTEPOptions{AllowMultiSpanRoofs: true}
	if disabled, err := ReadMappedChainRectangularShell(ctx, bytes.NewReader(raw), Limits{}, MappedChainShellSTEPOptions{}); err == nil ||
		len(disabled.Native.Solid.Faces()) != 0 {
		t.Fatal("the repaired roof still requires the interior support knot capability")
	}
	before, err := shell.BuildRectangularSolid(ctx, 1e-9, o.Geometry)
	if err != nil {
		t.Fatal(err)
	}
	opened, err := ReadMappedChainRectangularShell(ctx, bytes.NewReader(raw), Limits{}, o)
	if err != nil {
		t.Fatal(err)
	}
	solid := opened.Native.Solid
	if len(opened.Chain) != 4 || len(solid.Vertices()) != 15 || len(solid.Edges()) != 20 || len(solid.Faces()) != 7 {
		t.Fatal("the rebuilt STEP wire did not retain the new model vertex and both split edges")
	}
	wantStation := big.NewRat(1, 8)
	if opened.Native.ChainStations[1].Cmp(wantStation) != 0 {
		t.Fatal("the original global model cut changed")
	}
	edges, faces, vertices := solid.Edges(), solid.Faces(), solid.Vertices()
	first, second := edges[opened.Native.ChainEdges[0]], edges[opened.Native.ChainEdges[1]]
	wantPoint := geom.Vec3{X: 29.0 / 256, Y: 23.0 / 128, Z: 667.0 / 32768}
	if first.End != second.Start || vertices[first.End] != wantPoint {
		t.Fatal("the two reopened model edges must share the exact support-knot vertex")
	}
	file, err := Parse(bytes.NewReader(raw), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	recordA, err := simple(file, opened.Chain[0].EdgeID, "EDGE_CURVE", 5)
	if err != nil {
		t.Fatal(err)
	}
	recordB, err := simple(file, opened.Chain[1].EdgeID, "EDGE_CURVE", 5)
	if err != nil || recordA.Args[2].Ref != recordB.Args[1].Ref {
		t.Fatal("standard STEP edges must reference the same VERTEX_POINT", err)
	}
	wantRaw := [2]float64{5, 27}
	expectedFaces := before.Solid.Faces()
	for owner := range 2 {
		if !geom.SameSplineSurface(faces[owner].Surface.(*geom.BSplineSurface), expectedFaces[owner].Surface.(*geom.BSplineSurface)) {
			t.Fatal("the original support or its chart was changed", owner)
		}
		a, b := opened.Chain[0].Owners[owner], opened.Chain[1].Owners[owner]
		if a.BasisID != b.BasisID || a.TrimmedCurveID == b.TrimmedCurveID || a.PCurveID == b.PCurveID {
			t.Fatal("STEP must retain two restrictions of the same original owner basis", owner)
		}
		for i, row := range [2]MappedChainSTEPCoedge{a, b} {
			s := faces[owner].Outer[i].Curve.(trimmed.MappedSplineUV).Source()
			prior := expectedFaces[owner].Outer[i].Curve.(trimmed.MappedSplineUV).Source()
			basisID, span, reversed, err := mappedChainStepRawUse(file, row.TrimmedCurveID)
			if err != nil || basisID != row.BasisID || span != prior.Range || reversed != prior.Reversed ||
				s.Range != prior.Range || s.Reversed != prior.Reversed ||
				s.Curve.Degree() != prior.Curve.Degree() || !slices.Equal(s.Curve.Poles(), prior.Curve.Poles()) ||
				!slices.Equal(s.Curve.Weights(), prior.Curve.Weights()) || !slices.Equal(s.Curve.Knots(), prior.Curve.Knots()) {
				t.Fatal("the original whole spline and raw parameter restriction must survive the transfer", owner, i, err)
			}
			end := span.Max
			if (i == 0) == reversed {
				end = span.Min
			}
			if end != wantRaw[owner] {
				t.Fatal("wrong raw parameter at the shared support vertex", owner, i, end)
			}
		}
	}
	proof, err := solid.ValidateEdgeAgreement(ctx, edgecheck.Options{})
	if err != nil || !proof.Complete || len(proof.Uses) != 40 {
		t.Fatal("reopened STEP coedges require full exact source agreement", err)
	}
	mass, err := solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
	if err != nil || math.Abs(mass.Volume-mappedChainCrossKnotVolume) > 1e-8 {
		t.Fatal("reopened STEP shell volume disagrees with the analytic creased roof", mass.Volume, err)
	}
	var replay bytes.Buffer
	if err := WriteMappedChainRectangularShell(ctx, &replay, opened.Shell, opened.Name, opened.LengthUnit, Limits{}, o); err != nil ||
		!bytes.Equal(raw, replay.Bytes()) {
		t.Fatal("explicit support-knot coedges must replay canonically", err)
	}

	restriction := opened.Chain[0].Owners[0].TrimmedCurveID
	for _, altered := range []string{"5.1", "5.125"} {
		file.Entities[restriction].Records[0].Args[3].Items[0].Items[0].Text = altered
		rejected, err := ReadMappedChainRectangularShell(ctx, bytes.NewReader(serializeWrappedProofFile(file)), Limits{}, o)
		if err == nil || len(rejected.Native.Solid.Faces()) != 0 || len(rejected.Chain) != 0 {
			t.Fatal("the STEP reader must reject the changed raw split atomically", altered, err)
		}
	}
	var basis mappedChainStepBasis
	var emitted int64
	emit := func(string) int64 {
		emitted++
		return emitted
	}
	if _, err := emitMappedChainStepUse(emit, expectedFaces[0].Outer[0], true, &basis); err != nil {
		t.Fatal(err)
	}
	count := emitted
	if _, err := emitMappedChainStepUse(emit, expectedFaces[1].Outer[1], true, &basis); !errors.Is(err, geom.ErrConvergence) || emitted != count {
		t.Fatal("cannot substitute the first spline basis for a different original basis", err)
	}
}

func TestMappedChainRectangularSTEPRejectsNonDecimalCrossing(t *testing.T) {
	ctx := context.Background()
	raw, err := os.ReadFile("testdata/mapped_chain_cross_knot_source.json")
	if err != nil {
		t.Fatal(err)
	}
	// Move only the global chain labels. The original raw curves, support
	// charts, split model point and both exact raw split parameters are equal.
	raw = bytes.Replace(raw, []byte(`"Stations":["1/4","1/2"]`), []byte(`"Stations":["1/3","2/3"]`), 1)
	raw = bytes.Replace(raw, []byte(`"Stations":["1/2"]`), []byte(`"Stations":["2/3"]`), 1)
	shell, err := trimmed.ReadMappedSplineChainShellJSON(ctx, bytes.NewReader(raw), trimmed.MappedChainShellNativeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	native, err := shell.BuildRectangularSolid(ctx, 1e-9, geom.SurfacePatchOptions{})
	if err != nil || native.ChainStations[1].Cmp(big.NewRat(1, 6)) != 0 {
		t.Fatal("the exact non-decimal model station must still be valid natively", err)
	}
	var output bytes.Buffer
	err = WriteMappedChainRectangularShell(ctx, &output, shell, "Non-decimal support crossing", .001, Limits{}, MappedChainShellSTEPOptions{AllowMultiSpanRoofs: true})
	if !errors.Is(err, geom.ErrUnrepresentable) || output.Len() != 0 {
		t.Fatal("STEP export must reject a non-terminating model station before writing bytes", err, output.Len())
	}
}

func TestMappedChainRectangularSTEPRepairedSupportCrossingOCCT(t *testing.T) {
	exe := os.Getenv("PLAINSOLID_OCCT_MAPPED_CHAIN_STEP")
	if exe == "" {
		t.Skip("optional pinned OCCT STEP consumer")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	raw, _ := crossKnotMappedSTEP(t, ctx)
	path := filepath.Join(t.TempDir(), "repaired-support-crossing.step")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	data, err := exec.CommandContext(ctx, exe, "read", path).CombinedOutput()
	if err != nil {
		t.Fatalf("independent OCCT transfer failed: %v\n%s", err, data)
	}
	var props struct {
		Volume, Area  float64
		Faces, Solids int
	}
	if err := json.Unmarshal(data, &props); err != nil {
		t.Fatal(err)
	}
	if props.Faces != 7 || props.Solids != 1 || math.Abs(props.Volume-mappedChainCrossKnotVolume) > 1e-8 ||
		math.Abs(props.Area-6.251604174083588) > 1e-8 {
		t.Fatal("independent OCCT recovered the wrong repaired material", props)
	}
	t.Logf("OCCT reopened one solid with %d faces, volume %.15g, area %.15g", props.Faces, props.Volume, props.Area)
}
