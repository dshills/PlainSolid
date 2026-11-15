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

const mappedChainAsynchronousVolume = 15123675.0 / 16777216

func asynchronousMappedChainSTEP(t *testing.T, ctx context.Context) ([]byte, trimmed.MappedSplineChainShell) {
	t.Helper()
	raw, err := os.ReadFile("testdata/mapped_chain_asynchronous_crossings_source.json")
	if err != nil {
		t.Fatal(err)
	}
	source, err := trimmed.ReadMappedSplineChainShellJSON(ctx, bytes.NewReader(raw), trimmed.MappedChainShellNativeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := WriteMappedChainRectangularShell(ctx, &output, source, "Asynchronous roof crossings", .001, Limits{}, MappedChainShellSTEPOptions{
		AllowMultiSpanRoofs: true,
	}); err != nil {
		t.Fatal(err)
	}
	return output.Bytes(), source
}

func TestMappedChainRectangularSTEPAsynchronousSupportKnots(t *testing.T) {
	ctx := context.Background()
	raw, source := asynchronousMappedChainSTEP(t, ctx)
	o := MappedChainShellSTEPOptions{AllowMultiSpanRoofs: true}
	opened, err := ReadMappedChainRectangularShell(ctx, bytes.NewReader(raw), Limits{}, o)
	if err != nil {
		t.Fatal(err)
	}
	solid := opened.Native.Solid
	if len(opened.Chain) != 5 || len(solid.Vertices()) != 16 || len(solid.Edges()) != 21 || len(solid.Faces()) != 7 {
		t.Fatal("two asynchronous crosses must be explicit shared STEP coedges")
	}
	original, err := source.BuildRectangularSolid(ctx, 1e-9, o.Geometry)
	if err != nil {
		t.Fatal(err)
	}
	file, err := Parse(bytes.NewReader(raw), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	wantStations := [2]*big.Rat{big.NewRat(1, 8), big.NewRat(3, 8)}
	wantRaw := [2][2]int64{{17, 27}, {65, 21}}
	faces, originalFaces := solid.Faces(), original.Solid.Faces()
	for cross, left := range [2]int{0, 2} {
		a, err := simple(file, opened.Chain[left].EdgeID, "EDGE_CURVE", 5)
		if err != nil {
			t.Fatal(err)
		}
		b, err := simple(file, opened.Chain[left+1].EdgeID, "EDGE_CURVE", 5)
		if err != nil || a.Args[2].Ref != b.Args[1].Ref || opened.Native.ChainStations[left+1].Cmp(wantStations[cross]) != 0 {
			t.Fatal("paired STEP sides lost the same exact model station or vertex identity", cross, err)
		}
		for owner := range 2 {
			if !geom.SameSplineSurface(faces[owner].Surface.(*geom.BSplineSurface), originalFaces[owner].Surface.(*geom.BSplineSurface)) {
				t.Fatal("STEP changed the original chart or its asynchronous knot", owner)
			}
			for offset := range 2 {
				cell := left + offset
				s := faces[owner].Outer[cell].Curve.(trimmed.MappedSplineUV)
				p, err := s.AuthoredParameter(big.NewRat(int64(1-offset), 1))
				if err != nil || p.Cmp(big.NewRat(wantRaw[cross][owner], 1)) != 0 {
					t.Fatal("STEP lost the untouched companion's original raw split parameter", cross, owner, err)
				}
				ownerSpline := s.Source()
				prior := originalFaces[owner].Outer[cell].Curve.(trimmed.MappedSplineUV).Source()
				id, span, reversed, err := mappedChainStepRawUse(file, opened.Chain[cell].Owners[owner].TrimmedCurveID)
				if err != nil || id != opened.Chain[0].Owners[owner].BasisID || span != prior.Range || reversed != prior.Reversed ||
					span != ownerSpline.Range || reversed != ownerSpline.Reversed ||
					!slices.Equal(ownerSpline.Curve.Poles(), prior.Curve.Poles()) ||
					!slices.Equal(ownerSpline.Curve.Knots(), prior.Curve.Knots()) ||
					!slices.Equal(ownerSpline.Curve.Weights(), prior.Curve.Weights()) {
					t.Fatal("both owners must retain original whole STEP bases and parameter restrictions", cross, owner, err)
				}
			}
		}
	}
	proof, err := solid.ValidateEdgeAgreement(ctx, edgecheck.Options{})
	if err != nil || !proof.Complete || len(proof.Uses) != 42 {
		t.Fatal("STEP reopen must verify continuous equality on every exact source coedge", err)
	}
	mass, err := solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
	if err != nil || math.Abs(mass.Volume-mappedChainAsynchronousVolume) > 1e-8 {
		t.Fatal("STEP retained the original material above both distinct roofs", mass.Volume, err)
	}
	var replay bytes.Buffer
	if err := WriteMappedChainRectangularShell(ctx, &replay, opened.Shell, opened.Name, opened.LengthUnit, Limits{}, o); err != nil ||
		!bytes.Equal(raw, replay.Bytes()) {
		t.Fatal("explicit asynchronous STEP coedges must replay byte-identically", err)
	}
}

func TestMappedChainRectangularSTEPAsynchronousSupportKnotsOCCT(t *testing.T) {
	exe := os.Getenv("PLAINSOLID_OCCT_MAPPED_CHAIN_STEP")
	if exe == "" {
		t.Skip("optional pinned OCCT STEP consumer")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	raw, _ := asynchronousMappedChainSTEP(t, ctx)
	path := filepath.Join(t.TempDir(), "asynchronous-support-knots.step")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	result, err := exec.CommandContext(ctx, exe, "read", path).CombinedOutput()
	if err != nil {
		t.Fatalf("pinned OCCT cannot reopen the original asymmetric roof material: %v\n%s", err, result)
	}
	var props struct {
		Volume, Area  float64
		Faces, Solids int
	}
	if err := json.Unmarshal(result, &props); err != nil {
		t.Fatal(err)
	}
	if props.Solids != 1 || props.Faces != 7 || math.Abs(props.Volume-mappedChainAsynchronousVolume) > 1e-8 ||
		math.Abs(props.Area-5.9325913022728294) > 1e-8 {
		t.Fatal("pinned OCCT must recover the same independently creased roof material", props)
	}
	t.Logf("OCCT reopened one seven-face solid, volume %.15g, area %.15g", props.Volume, props.Area)
}
