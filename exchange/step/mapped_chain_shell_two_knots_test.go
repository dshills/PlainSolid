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

const mappedChainTwoKnotVolume = 257426019.0 / 268435456

func mappedChainTwoCrossingsSTEP(t *testing.T, ctx context.Context) ([]byte, trimmed.MappedSplineChainShell) {
	t.Helper()
	original, err := os.ReadFile("testdata/mapped_chain_two_crossings_source.json")
	if err != nil {
		t.Fatal(err)
	}
	source, err := trimmed.ReadMappedSplineChainShellJSON(ctx, bytes.NewReader(original), trimmed.MappedChainShellNativeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	var data bytes.Buffer
	o := MappedChainShellSTEPOptions{AllowMultiSpanRoofs: true, AllowTwoInteriorRoofKnots: true}
	if err := WriteMappedChainRectangularShell(ctx, &data, source, "Two synchronized roof crossings", .001, Limits{}, o); err != nil {
		t.Fatal(err)
	}
	return data.Bytes(), source
}

func TestMappedChainRectangularSTEPTwoSupportKnots(t *testing.T) {
	ctx := context.Background()
	data, source := mappedChainTwoCrossingsSTEP(t, ctx)
	for _, disabled := range []MappedChainShellSTEPOptions{{AllowMultiSpanRoofs: true}, {AllowTwoInteriorRoofKnots: true}} {
		rejected, err := ReadMappedChainRectangularShell(ctx, bytes.NewReader(data), Limits{}, disabled)
		if err == nil || len(rejected.Native.Solid.Faces()) != 0 {
			t.Fatal("reading two original support knots requires both capabilities", err)
		}
		var output bytes.Buffer
		if err := WriteMappedChainRectangularShell(ctx, &output, source, "Two knots disabled", .001, Limits{}, disabled); err == nil || output.Len() != 0 {
			t.Fatal("writing two original support knots requires both capabilities", err)
		}
	}
	o := MappedChainShellSTEPOptions{AllowMultiSpanRoofs: true, AllowTwoInteriorRoofKnots: true}
	model, err := ReadMappedChainRectangularShell(ctx, bytes.NewReader(data), Limits{}, o)
	if err != nil {
		t.Fatal(err)
	}
	solid := model.Native.Solid
	if len(model.Chain) != 5 || len(solid.Vertices()) != 18 || len(solid.Edges()) != 23 || len(solid.Faces()) != 7 {
		t.Fatal("two-knot STEP transfer must retain the explicit split coedges and shared vertices")
	}
	before, err := source.BuildRectangularSolid(ctx, 1e-9, o.Geometry)
	if err != nil {
		t.Fatal(err)
	}
	file, err := Parse(bytes.NewReader(data), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	wantStation := [2]*big.Rat{big.NewRat(1, 8), big.NewRat(3, 16)}
	wantRaw := [2][2]int64{{41, 65}, {51, 43}}
	faces, originalFaces := solid.Faces(), before.Solid.Faces()
	for i := range wantStation {
		left, err := simple(file, model.Chain[i].EdgeID, "EDGE_CURVE", 5)
		if err != nil {
			t.Fatal(err)
		}
		right, err := simple(file, model.Chain[i+1].EdgeID, "EDGE_CURVE", 5)
		if err != nil || left.Args[2].Ref != right.Args[1].Ref || model.Native.ChainStations[i+1].Cmp(wantStation[i]) != 0 {
			t.Fatal("model sides must refer to the same original crossing vertex and station", i, err)
		}
	}
	for owner := range 2 {
		if !geom.SameSplineSurface(faces[owner].Surface.(*geom.BSplineSurface), originalFaces[owner].Surface.(*geom.BSplineSurface)) {
			t.Fatal("STEP changed an original roof support chart", owner)
		}
		basis := model.Chain[0].Owners[owner].BasisID
		for cell := range 3 {
			trace := model.Chain[cell].Owners[owner]
			id, raw, reversed, err := mappedChainStepRawUse(file, trace.TrimmedCurveID)
			prior := originalFaces[owner].Outer[cell].Curve.(trimmed.MappedSplineUV).Source()
			got := faces[owner].Outer[cell].Curve.(trimmed.MappedSplineUV).Source()
			if err != nil || id != basis || trace.BasisID != basis || raw != prior.Range || raw != got.Range ||
				reversed != prior.Reversed || reversed != got.Reversed ||
				!slices.Equal(prior.Curve.Knots(), got.Curve.Knots()) || !slices.Equal(prior.Curve.Poles(), got.Curve.Poles()) ||
				!slices.Equal(prior.Curve.Weights(), got.Curve.Weights()) {
				t.Fatal("STEP changed an original whole owner basis or its coedge restriction", owner, cell, err)
			}
			if cell < 2 {
				tau, err := faces[owner].Outer[cell].Curve.(trimmed.MappedSplineUV).AuthoredParameter(big.NewRat(1, 1))
				if err != nil || tau.Cmp(big.NewRat(wantRaw[owner][cell], 1)) != 0 {
					t.Fatal("wrong original owner raw split station", owner, cell, err)
				}
			}
		}
	}
	proof, err := solid.ValidateEdgeAgreement(ctx, edgecheck.Options{})
	if err != nil || !proof.Complete || len(proof.Uses) != 46 {
		t.Fatal("exact STEP read must prove all source coedges after both splits", err)
	}
	mass, err := solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
	if err != nil || math.Abs(mass.Volume-mappedChainTwoKnotVolume) > 1e-8 {
		t.Fatal("reopened two-crease shell disagrees with analytic volume", mass.Volume, err)
	}
	var replay bytes.Buffer
	if err := WriteMappedChainRectangularShell(ctx, &replay, model.Shell, model.Name, model.LengthUnit, Limits{}, o); err != nil ||
		!bytes.Equal(data, replay.Bytes()) {
		t.Fatal("original two-knot STEP wire and raw restrictions must replay byte-identically", err)
	}
}

func TestMappedChainRectangularSTEPTwoSupportKnotsOCCT(t *testing.T) {
	exe := os.Getenv("PLAINSOLID_OCCT_MAPPED_CHAIN_STEP")
	if exe == "" {
		t.Skip("optional pinned OCCT STEP consumer")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	data, _ := mappedChainTwoCrossingsSTEP(t, ctx)
	path := filepath.Join(t.TempDir(), "two-support-crossings.step")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	encoded, err := exec.CommandContext(ctx, exe, "read", path).CombinedOutput()
	if err != nil {
		t.Fatalf("independent two-knot STEP transfer failed: %v\n%s", err, encoded)
	}
	var props struct {
		Volume, Area  float64
		Faces, Solids int
	}
	if err := json.Unmarshal(encoded, &props); err != nil {
		t.Fatal(err)
	}
	if props.Faces != 7 || props.Solids != 1 || math.Abs(props.Volume-mappedChainTwoKnotVolume) > 1e-8 ||
		math.Abs(props.Area-6.228134630379123) > 1e-8 {
		t.Fatal("independent OCCT recovered the wrong two-knot material", props)
	}
	t.Logf("OCCT reopened one solid with %d faces, volume %.15g, area %.15g", props.Faces, props.Volume, props.Area)
}
