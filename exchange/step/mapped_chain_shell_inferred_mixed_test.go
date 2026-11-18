package step

import (
	"bytes"
	"context"
	"errors"
	"math"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/internal/slowtest"
	"strings"
	"testing"
	"time"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

const inferredMixedMappedChainPath = "testdata/occt_mapped_chain_inferred_mixed_knots.step"
const inferredMixedRedundantMappedChainPath = "testdata/occt_mapped_chain_inferred_mixed_redundant_knot.step"

// The integral covers the unsplit seam and each roof's two off-seam creases.
const inferredMixedMappedChainVolume = 1931883375.0 / 2147483648
const inferredMixedMappedChainArea = 5.923086577411750

func inferredMixedMappedChainOptions() MappedChainShellSTEPOptions {
	o := inferredSharedMappedChainOptions()
	o.AllowTwoInteriorRoofKnots, o.AllowThreeSupportKnots = true, true
	return o
}

func verifyInferredMixedMappedChain(t *testing.T, ctx context.Context, raw []byte) []byte {
	t.Helper()
	o := inferredMixedMappedChainOptions()
	m, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(raw), Limits{}, o)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Chain) != 3 || len(m.Crossings) != 3 || !allMappedChainCrossingsOnOneEdge(m.Crossings) {
		t.Fatal("the original Bézier must carry all three inferred support events")
	}
	verifyInferredMixedMaterial(t, ctx, m.Native)
	bases := m.Shell.IndependentOwnerBases()
	if bases[0] == bases[1] {
		t.Fatal("the single whole owner basis and independent bases must survive")
	}
	north := 0
	if !bases[0] {
		north = 1
	}
	south := 1 - north
	edges, vertices, faces := m.Native.Solid.Edges(), m.Native.Solid.Vertices(), m.Native.Solid.Faces()
	modelStops := [3]int64{2, 4, 6}
	pointsX := [3]float64{107.0 / 2048, 29.0 / 256, 369.0 / 2048}
	var stops [2][5]int64
	stops[north], stops[south] = [5]int64{2, 17, 37, 65, 107}, [5]int64{48, 40, 32, 24, 16}
	for i, c := range m.Crossings {
		x := pointsX[i]
		if c.EdgeID != m.Chain[0].EdgeID || c.StoredModelKnot ||
			c.SourceModelParameter.Cmp(big.NewRat(modelStops[i], 1)) != 0 || c.ModelStation.Cmp(big.NewRat(modelStops[i], 32)) != 0 ||
			c.RawParameters[north].Cmp(big.NewRat(stops[north][i+1], 1)) != 0 ||
			c.RawParameters[south].Cmp(big.NewRat(stops[south][i+1], 1)) != 0 ||
			c.OwnersChangingCells[north] != (i != 1) || c.OwnersChangingCells[south] != (i != 2) ||
			c.NativeEdges != [2]int{m.Native.ChainEdges[i], m.Native.ChainEdges[i+1]} {
			t.Fatal("one joint and two owner-only events must retain their raw stops and exact Bézier ancestry", i, c)
		}
		a, b := edges[c.NativeEdges[0]], edges[c.NativeEdges[1]]
		if a.End != b.Start || vertices[a.End] != (geom.Vec3{X: x, Y: .25, Z: x / 4}) {
			t.Fatal("all repaired edge pairs must meet at the corresponding exact original model point", i)
		}
	}
	f, err := Parse(bytes.NewReader(raw), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	verifyOriginalInferredModelBezier(t, f, m.Chain[0].ModelCurveID)
	for owner, use := range m.Chain[0].Owners {
		original, err := originalMappedChainStepOwner(f, use, true)
		if err != nil {
			t.Fatal(err)
		}
		for child := range 4 {
			q := faces[owner].Outer[child].Curve.(trimmed.MappedSplineUV)
			if !sameMixedMappedChainBasis(q.Source().Curve, original.Curve) {
				t.Fatal("each of the four children must retain its original owner basis", owner, child)
			}
			for end, want := range stops[owner][child : child+2] {
				rawParameter, err := q.AuthoredParameter(big.NewRat(int64(end), 1))
				if err != nil || rawParameter.Cmp(big.NewRat(want, 1)) != 0 {
					t.Fatal("the four chart restrictions must retain both original raw maps", owner, child, end, err)
				}
			}
		}
	}
	northRoof, southRoof := faces[north].Surface.(*geom.BSplineSurface), faces[south].Surface.(*geom.BSplineSurface)
	for _, x := range []float64{pointsX[0], pointsX[2]} {
		verifyMappedRoofSlopeJump(t, ctx, northRoof, 0, x, big.NewRat(1, 2), big.NewRat(1, 32))
		verifyMappedRoofSlopeJump(t, ctx, northRoof, 0, x, big.NewRat(1, 4), new(big.Rat))
	}
	for _, x := range pointsX[:2] {
		verifyMappedRoofSlopeJump(t, ctx, southRoof, 1, x, big.NewRat(1, 8), big.NewRat(-1, 64))
		verifyMappedRoofSlopeJump(t, ctx, southRoof, 1, x, big.NewRat(1, 4), new(big.Rat))
	}

	var recipe, canonical, replay bytes.Buffer
	nativeOptions := trimmed.MappedChainShellNativeOptions{}
	tag := []byte(`"Boundary":{"Version":7,`)
	if err := m.Shell.WriteJSON(ctx, &recipe, nativeOptions); err != nil || !bytes.Contains(recipe.Bytes(), tag) {
		t.Fatal("the existing v7 recipe must rederive all three original support events", err)
	}
	opened, err := trimmed.ReadMappedSplineChainShellJSON(ctx, bytes.NewReader(recipe.Bytes()), nativeOptions)
	if err != nil {
		t.Fatal(err)
	}
	native, err := opened.BuildRectangularSolid(ctx, 1e-9, o.Geometry)
	if err != nil {
		t.Fatal(err)
	}
	verifyInferredMixedMaterial(t, ctx, native)
	if err := opened.WriteJSON(ctx, &replay, nativeOptions); err != nil || !bytes.Equal(recipe.Bytes(), replay.Bytes()) {
		t.Fatal("the source-only v7 reload must retain the identical native record", err)
	}
	if err := WriteMappedChainRectangularOriginalShell(ctx, &canonical, m, Limits{}, o); err != nil {
		t.Fatal(err)
	}
	strict, err := ReadMappedChainRectangularShell(ctx, bytes.NewReader(canonical.Bytes()), Limits{}, o)
	if err != nil || len(strict.Chain) != 3 || len(strict.Crossings) != 3 || strict.Shell.IndependentOwnerBases() != bases {
		t.Fatal("strict reopen must retain the same original model and both owner bases", err)
	}
	verifyOriginalMappedChainSTEPGraph(t, m, strict)
	replay.Reset()
	if err := strict.Shell.WriteJSON(ctx, &replay, nativeOptions); err != nil || !bytes.Equal(recipe.Bytes(), replay.Bytes()) {
		t.Fatal("strict source transfer must rederive the identical native v7 recipe", err)
	}
	replay.Reset()
	if err := WriteMappedChainRectangularOriginalShell(ctx, &replay, strict, Limits{}, o); err != nil || !bytes.Equal(canonical.Bytes(), replay.Bytes()) {
		t.Fatal("canonical STEP must replay all original unstored event lineage", err)
	}
	return canonical.Bytes()
}

func verifyInferredMixedMaterial(t *testing.T, ctx context.Context, m trimmed.MappedChainRectangularSolid) {
	t.Helper()
	s := m.Solid
	if len(m.ChainEdges) != 6 || len(s.Faces()) != 7 || len(s.Edges()) != 24 || len(s.Vertices()) != 19 {
		t.Fatal("all three inferred vertices must retain the complete closed topology")
	}
	proof, err := s.ValidateEdgeAgreement(ctx, edgecheck.Options{})
	if err != nil || !proof.Complete || len(proof.Uses) != 48 {
		t.Fatal("all source incidences must have complete continuous agreement", err)
	}
	mass, err := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
	if err != nil || math.Abs(mass.Volume-inferredMixedMappedChainVolume) > 1e-8 || math.Abs(mass.Area-inferredMixedMappedChainArea) > 1e-8 {
		t.Fatal("native material must match the independent author's source", mass, err)
	}
}

func TestOCCTMappedChainRectangularInferredMixedSupportKnots(t *testing.T) {
	verifyInferredMixedMappedChain(t, context.Background(), readInferredSharedMappedChainFixture(t, inferredMixedMappedChainPath))
}

func TestOCCTMappedChainRectangularInferredMixedSupportKnotsClosedDisplay(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	raw := readInferredSharedMappedChainFixture(t, inferredMixedMappedChainPath)
	m, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(raw), Limits{}, inferredMixedMappedChainOptions())
	if err != nil {
		t.Fatal(err)
	}
	verifyMappedChainCutDisplay(t, ctx, m)
}

func TestOCCTMappedChainRectangularInferredMixedSupportKnotsRejectsInvalidSource(t *testing.T) {
	ctx := context.Background()
	raw := readInferredSharedMappedChainFixture(t, inferredMixedMappedChainPath)
	o := inferredMixedMappedChainOptions()
	pairedOnly := o
	pairedOnly.AllowThreeSupportKnots = false
	bad, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(raw), Limits{}, pairedOnly)
	if !errors.Is(err, geom.ErrConvergence) || !strings.Contains(err.Error(), "must coincide") || len(bad.Crossings) != 0 {
		t.Fatal("the previous paired gate must still require two jointly inferred events", err)
	}
	rejectInferredMixedRedundantModelKnot(t, ctx, readInferredSharedMappedChainFixture(t, inferredMixedRedundantMappedChainPath))

	m, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(raw), Limits{}, o)
	if err != nil {
		t.Fatal(err)
	}
	f, err := Parse(bytes.NewReader(raw), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	north := 0
	if !m.Shell.IndependentOwnerBases()[0] {
		north = 1
	}
	_, knots, _, err := mappedChainStepSplineParts(f, m.Chain[0].Owners[north].BasisID)
	if err != nil {
		t.Fatal(err)
	}
	// The north raw stops at its own support events become 22 and 86.
	// Only the middle station from the other roof becomes nonbinary.
	knots[1].Items[1].Text = "142."
	bad, err = ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(serializeWrappedProofFile(f)), Limits{}, o)
	if !errors.Is(err, geom.ErrUnrepresentable) || !strings.Contains(err.Error(), "mixed original Bézier partition") ||
		len(bad.Crossings) != 0 || len(bad.Native.ChainEdges) != 0 {
		t.Fatal("both maps must be proved at every union station including the companion-only event", err)
	}
}

func rejectInferredMixedRedundantModelKnot(t *testing.T, ctx context.Context, raw []byte) {
	t.Helper()
	bad, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(raw), Limits{}, inferredMixedMappedChainOptions())
	if !errors.Is(err, geom.ErrConvergence) || !strings.Contains(err.Error(), "must match the stored model knot") ||
		len(bad.Crossings) != 0 || len(bad.Native.ChainEdges) != 0 {
		t.Fatal("the redundantly stored model knot at a south-only event must reject atomically", err)
	}
}

func TestOCCTMappedChainRectangularInferredMixedSupportKnotsLive(t *testing.T) {
	author, reader := os.Getenv("PLAINSOLID_OCCT_MAPPED_CHAIN_INFERRED_MIXED_AUTHOR"), os.Getenv("PLAINSOLID_OCCT_MAPPED_CHAIN_STEP")
	if author == "" || reader == "" {
		t.Skip("optional pinned OCCT source author and STEP consumer")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	authorFile := func(mode string) []byte {
		path := filepath.Join(t.TempDir(), mode+".step")
		if output, err := exec.CommandContext(ctx, author, mode, path).CombinedOutput(); err != nil {
			t.Fatalf("pinned OCCT author failed: %v\n%s", err, output)
		}
		return readInferredSharedMappedChainFixture(t, path)
	}
	raw := authorFile("write-shared-inferred-offset-pair")
	canonical := verifyInferredMixedMappedChain(t, ctx, raw)
	verifyPinnedMappedChainTransfer(t, ctx, reader, raw, canonical, inferredMixedMappedChainVolume, inferredMixedMappedChainArea)
	redundant := authorFile("write-mixed-inferred-redundant-knot")
	verifyPinnedMappedChainTransfer(t, ctx, reader, raw, redundant, inferredMixedMappedChainVolume, inferredMixedMappedChainArea)
	rejectInferredMixedRedundantModelKnot(t, ctx, redundant)
}
