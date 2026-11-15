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

const mappedChainThreeVolume = 2004002051.0 / 2147483648
const mappedChainThreeArea = 6.091662229085

func mappedChainThreeSTEP(t *testing.T, ctx context.Context) ([]byte, MappedChainShellSTEPOptions, trimmed.MappedSplineChainShell) {
	t.Helper()
	shell := mappedChainThreeSource(t, ctx)
	o := MappedChainShellSTEPOptions{AllowMultiSpanRoofs: true, AllowTwoInteriorRoofKnots: true}
	var b bytes.Buffer
	if err := WriteMappedChainRectangularShell(ctx, &b, shell, "Three original roof crossings", .001, Limits{}, o); err != nil {
		t.Fatal(err)
	}
	return b.Bytes(), o, shell
}

func mappedChainThreeSource(t *testing.T, ctx context.Context) trimmed.MappedSplineChainShell {
	t.Helper()
	raw, err := os.ReadFile("testdata/mapped_chain_three_crossings_source.json")
	if err != nil {
		t.Fatal(err)
	}
	shell, err := trimmed.ReadMappedSplineChainShellJSON(ctx, bytes.NewReader(raw), trimmed.MappedChainShellNativeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	return shell
}

func TestMappedChainRectangularSTEPThreeSupportKnots(t *testing.T) {
	ctx := context.Background()
	raw, o, shell := mappedChainThreeSTEP(t, ctx)
	m, err := ReadMappedChainRectangularShell(ctx, bytes.NewReader(raw), Limits{}, o)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Chain) != 6 {
		t.Fatal("explicit STEP must retain the complete six original shared wire cells")
	}
	verifyThreeKnotMaterial(t, ctx, m)
	wantStation := [3]*big.Rat{big.NewRat(1, 16), big.NewRat(1, 8), big.NewRat(3, 16)}
	f, err := Parse(bytes.NewReader(raw), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	for i, station := range wantStation {
		a, err := simple(f, m.Chain[i].EdgeID, "EDGE_CURVE", 5)
		if err != nil {
			t.Fatal(err)
		}
		b, err := simple(f, m.Chain[i+1].EdgeID, "EDGE_CURVE", 5)
		if err != nil || a.Args[2].Ref != b.Args[1].Ref || m.Native.ChainStations[i+1].Cmp(station) != 0 {
			t.Fatal("explicit STEP coedges must share the original model station and vertex", i, err)
		}
	}
	source, err := shell.BuildRectangularSolid(ctx, 1e-9, o.Geometry)
	if err != nil {
		t.Fatal(err)
	}
	faces, originalFaces := m.Native.Solid.Faces(), source.Solid.Faces()
	wantRaw := [2][3]int64{{93, 197, 317}, {443, 387, 283}}
	for owner := range 2 {
		if !geom.SameSplineSurface(faces[owner].Surface.(*geom.BSplineSurface), originalFaces[owner].Surface.(*geom.BSplineSurface)) {
			t.Fatal("STEP changed an original support chart", owner)
		}
		for cell := range 4 {
			p := faces[owner].Outer[cell].Curve.(trimmed.MappedSplineUV)
			q := originalFaces[owner].Outer[cell].Curve.(trimmed.MappedSplineUV)
			a, b := p.Source(), q.Source()
			if a.Range != b.Range || a.Reversed != b.Reversed || !slices.Equal(a.Curve.Poles(), b.Curve.Poles()) ||
				!slices.Equal(a.Curve.Knots(), b.Curve.Knots()) || !slices.Equal(a.Curve.Weights(), b.Curve.Weights()) ||
				m.Chain[cell].Owners[owner].BasisID != m.Chain[0].Owners[owner].BasisID {
				t.Fatal("the four STEP coedges must retain the complete original owner basis and raw interval", owner, cell)
			}
			if cell < 3 {
				x, err := p.AuthoredParameter(big.NewRat(1, 1))
				if err != nil || x.Cmp(big.NewRat(wantRaw[owner][cell], 1)) != 0 {
					t.Fatal("STEP changed an exact raw crossing limit", owner, cell, err)
				}
			}
		}
	}
	var replay bytes.Buffer
	if err := WriteMappedChainRectangularShell(ctx, &replay, m.Shell, m.Name, m.LengthUnit, Limits{}, o); err != nil ||
		!bytes.Equal(raw, replay.Bytes()) {
		t.Fatal("reopened explicit wire must rewrite byte-identically", err)
	}
}

func verifyThreeKnotMaterial(t *testing.T, ctx context.Context, m MappedChainSTEPModel) {
	t.Helper()
	solid := m.Native.Solid
	if len(m.Native.ChainEdges) != 6 || len(solid.Faces()) != 7 || len(solid.Edges()) != 24 || len(solid.Vertices()) != 19 {
		t.Fatal("three-cut fixture lost one of its original source coedges")
	}
	proof, err := solid.ValidateEdgeAgreement(ctx, edgecheck.Options{})
	if err != nil || !proof.Complete || len(proof.Uses) != 48 {
		t.Fatal("all paired source edges require complete continuous equality", err)
	}
	mass, err := solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
	if err != nil || math.Abs(mass.Volume-mappedChainThreeVolume) > 1e-8 || math.Abs(mass.Area-mappedChainThreeArea) > 1e-8 {
		t.Fatal("analytic three-crease roof changed during STEP exchange", mass, err)
	}
}

func TestMappedChainRectangularSTEPThreeSupportKnotsOCCT(t *testing.T) {
	exe := os.Getenv("PLAINSOLID_OCCT_MAPPED_CHAIN_STEP")
	if exe == "" {
		t.Skip("optional pinned OCCT STEP consumer")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	raw, _, _ := mappedChainThreeSTEP(t, ctx)
	path := filepath.Join(t.TempDir(), "three-original-support-crossings.step")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	data, err := exec.CommandContext(ctx, exe, "read", path).CombinedOutput()
	if err != nil {
		t.Fatalf("pinned OCCT rejected the explicit three-cut solid: %v\n%s", err, data)
	}
	var p struct {
		Volume, Area  float64
		Faces, Solids int
	}
	if err := json.Unmarshal(data, &p); err != nil {
		t.Fatal(err)
	}
	if p.Solids != 1 || p.Faces != 7 || math.Abs(p.Volume-mappedChainThreeVolume) > 1e-8 ||
		math.Abs(p.Area-mappedChainThreeArea) > 1e-8 {
		t.Fatal("pinned OCCT must recover the correct three-crease material", p)
	}
	t.Logf("OCCT reopened one seven-face solid, volume %.15g, area %.15g", p.Volume, p.Area)
}

func verifyIndependentlyAuthoredThreeKnots(t *testing.T, ctx context.Context, raw []byte) {
	t.Helper()
	o := MappedChainShellSTEPOptions{AllowMultiSpanRoofs: true, AllowTwoInteriorRoofKnots: true}
	m, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(raw), Limits{}, o)
	if err != nil {
		t.Fatal(err)
	}
	if m.OriginalSTEP() != string(raw) || len(m.Chain) != 6 || len(m.Crossings) != 0 || len(m.Planes) != 5 {
		t.Fatal("independent author must supply explicit original roof coedges and signed-axis planes")
	}
	verifyThreeKnotMaterial(t, ctx, m)
	source, err := mappedChainThreeSource(t, ctx).BuildRectangularSolid(ctx, 1e-9, o.Geometry)
	if err != nil {
		t.Fatal(err)
	}
	expected := source.Solid.Faces()
	var matched [2]bool
	for _, face := range m.Native.Solid.Faces()[:2] {
		patch := face.Surface.(*geom.BSplineSurface)
		for i := range matched {
			if geom.SameSplineSurface(patch, expected[i].Surface.(*geom.BSplineSurface)) {
				matched[i] = true
			}
		}
	}
	if matched != [2]bool{true, true} {
		t.Fatal("the two independently authored roofs must retain the same exact original controls and cuts")
	}
	var recipe, replay bytes.Buffer
	if err := m.Shell.WriteJSON(ctx, &recipe, trimmed.MappedChainShellNativeOptions{}); err != nil {
		t.Fatal(err)
	}
	opened, err := trimmed.ReadMappedSplineChainShellJSON(ctx, bytes.NewReader(recipe.Bytes()), trimmed.MappedChainShellNativeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if err := opened.WriteJSON(ctx, &replay, trimmed.MappedChainShellNativeOptions{}); err != nil ||
		!bytes.Equal(recipe.Bytes(), replay.Bytes()) {
		t.Fatal("independently authored UV bases and source intervals must survive native reload", err)
	}
	native, err := opened.BuildRectangularSolid(ctx, 1e-9, o.Geometry)
	if err != nil {
		t.Fatal(err)
	}
	m.Native = native
	verifyThreeKnotMaterial(t, ctx, m)
}

func TestOCCTMappedChainRectangularThreeSupportKnotsRecorded(t *testing.T) {
	raw, err := os.ReadFile("testdata/occt_mapped_chain_three_crossings.step")
	if err != nil {
		t.Fatal(err)
	}
	verifyIndependentlyAuthoredThreeKnots(t, context.Background(), raw)
}

func TestOCCTMappedChainRectangularThreeSupportKnotsLive(t *testing.T) {
	exe := os.Getenv("PLAINSOLID_OCCT_MAPPED_CHAIN_THREE_AUTHOR")
	if exe == "" {
		t.Skip("optional pinned OCCT three-crossing STEP author")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	path := filepath.Join(t.TempDir(), "occt-authored-three-crossings.step")
	if out, err := exec.CommandContext(ctx, exe, "write-three-knots", path).CombinedOutput(); err != nil {
		t.Fatalf("pinned OCCT author could not write the independent three-crease fixture: %v\n%s", err, out)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	verifyIndependentlyAuthoredThreeKnots(t, ctx, raw)
}
