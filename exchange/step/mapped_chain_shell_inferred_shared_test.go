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
	"strings"
	"testing"
	"time"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

const inferredSharedMappedChainPath = "testdata/occt_mapped_chain_inferred_shared_knot.step"
const redundantInferredSharedMappedChainPath = "testdata/occt_mapped_chain_inferred_shared_redundant_knot.step"

// The integral includes both off-seam crease terms on their original regions.
const inferredSharedMappedChainVolume = 30039003.0 / 33554432
const inferredSharedMappedChainArea = 5.874317046085239

func inferredSharedMappedChainOptions() MappedChainShellSTEPOptions {
	return MappedChainShellSTEPOptions{AllowMultiSpanRoofs: true, AllowUnsplitSupportKnot: true,
		AllowMixedOwnerBases: true, AllowInferredSharedSupportKnot: true}
}

func readInferredSharedMappedChainFixture(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func verifyInferredSharedMappedChain(t *testing.T, ctx context.Context, raw []byte) []byte {
	t.Helper()
	o := inferredSharedMappedChainOptions()
	m, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(raw), Limits{}, o)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Chain) != 3 || !mappedChainInferredSharedCrossing(m.Crossings) || m.Crossing == nil {
		t.Fatal("the original pair of owner changes must produce a single inferred shared station")
	}
	verifyInferredSharedMappedChainMaterial(t, ctx, m.Native)
	bases := m.Shell.IndependentOwnerBases()
	if bases[0] == bases[1] {
		t.Fatal("the two roofs must retain their original different basis organizations")
	}
	north := 0
	if !bases[0] {
		north = 1
	}
	south := 1 - north
	cut := m.Crossings[0]
	if cut.EdgeID != m.Chain[0].EdgeID || cut.SourceModelParameter.Cmp(big.NewRat(4, 1)) != 0 ||
		cut.ModelStation.Cmp(big.NewRat(1, 8)) != 0 || cut.RawParameters[north].Cmp(big.NewRat(37, 1)) != 0 ||
		cut.RawParameters[south].Cmp(big.NewRat(32, 1)) != 0 || cut.NativeEdges != [2]int{m.Native.ChainEdges[0], m.Native.ChainEdges[1]} {
		t.Fatal("the inferred shared event must keep one source edge, one model station and both original raw stations", cut)
	}
	const x = 29.0 / 256
	edges, vertices, faces := m.Native.Solid.Edges(), m.Native.Solid.Vertices(), m.Native.Solid.Faces()
	a, b := edges[cut.NativeEdges[0]], edges[cut.NativeEdges[1]]
	if a.End != b.Start || vertices[a.End] != (geom.Vec3{X: x, Y: .25, Z: x / 4}) {
		t.Fatal("the two native children must share the exact model point")
	}
	f, err := Parse(bytes.NewReader(raw), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	model, err := readMappedChainStepSpline(f, m.Chain[0].ModelCurveID, 3, 16, 17, true)
	if err != nil || len(model.poles) != 4 || len(model.knots) != 8 {
		t.Fatal("the shared support event must stay unstored in the original single model Bézier", err)
	}
	for i, px := range [4]float64{0, 1.0 / 16, 5.0 / 32, .25} {
		if model.knots[i].Sign() != 0 || model.knots[i+4].Cmp(big.NewRat(8, 1)) != 0 ||
			model.weights[i].Cmp(big.NewRat(1, 1)) != 0 || model.poles[i][0].Cmp(mappedChainStepRat(px)) != 0 ||
			model.poles[i][1].Cmp(big.NewRat(1, 4)) != 0 || model.poles[i][2].Cmp(mappedChainStepRat(px/4)) != 0 {
			t.Fatal("the model must retain the exact authored cubic and raw finite domain", i)
		}
	}
	var stops [2][3]int64
	stops[north], stops[south] = [3]int64{2, 37, 107}, [3]int64{48, 32, 16}
	for owner, use := range m.Chain[0].Owners {
		original, err := originalMappedChainStepOwner(f, use, true)
		if err != nil {
			t.Fatal(err)
		}
		for child := range 2 {
			q := faces[owner].Outer[child].Curve.(trimmed.MappedSplineUV)
			if !sameMixedMappedChainBasis(q.Source().Curve, original.Curve) {
				t.Fatal("each native child must retain the original owner's unmodified basis", owner, child)
			}
			for endpoint, expected := range stops[owner][child : child+2] {
				rawParameter, err := q.AuthoredParameter(big.NewRat(int64(endpoint), 1))
				if err != nil || rawParameter.Cmp(big.NewRat(expected, 1)) != 0 {
					t.Fatal("the original raw intervals must meet at the corresponding exact owner stop", owner, child, endpoint, err)
				}
			}
		}
	}
	northRoof, southRoof := faces[north].Surface.(*geom.BSplineSurface), faces[south].Surface.(*geom.BSplineSurface)
	verifyMappedRoofSlopeJump(t, ctx, northRoof, 0, x, big.NewRat(1, 2), big.NewRat(1, 32))
	verifyMappedRoofSlopeJump(t, ctx, southRoof, 1, x, big.NewRat(1, 8), big.NewRat(-1, 64))
	verifyMappedRoofSlopeJump(t, ctx, northRoof, 0, x, big.NewRat(1, 4), new(big.Rat))
	verifyMappedRoofSlopeJump(t, ctx, southRoof, 1, x, big.NewRat(1, 4), new(big.Rat))

	var recipe, canonical, replay bytes.Buffer
	nativeOptions := trimmed.MappedChainShellNativeOptions{}
	if err := m.Shell.WriteJSON(ctx, &recipe, nativeOptions); err != nil || !bytes.Contains(recipe.Bytes(), []byte(`"Boundary":{"Version":3,`)) {
		t.Fatal("the existing source-only native v3 proof must handle the shared inferred cut", err)
	}
	opened, err := trimmed.ReadMappedSplineChainShellJSON(ctx, bytes.NewReader(recipe.Bytes()), nativeOptions)
	if err != nil {
		t.Fatal(err)
	}
	native, err := opened.BuildRectangularSolid(ctx, 1e-9, o.Geometry)
	if err != nil {
		t.Fatal(err)
	}
	verifyInferredSharedMappedChainMaterial(t, ctx, native)
	if err := opened.WriteJSON(ctx, &replay, nativeOptions); err != nil || !bytes.Equal(recipe.Bytes(), replay.Bytes()) {
		t.Fatal("reloading the original v3 source must reproduce the native record bytes", err)
	}
	if err := WriteMappedChainRectangularOriginalShell(ctx, &canonical, m, Limits{}, o); err != nil {
		t.Fatal(err)
	}
	strict, err := ReadMappedChainRectangularShell(ctx, bytes.NewReader(canonical.Bytes()), Limits{}, o)
	if err != nil || len(strict.Chain) != 3 || !mappedChainInferredSharedCrossing(strict.Crossings) || strict.Shell.IndependentOwnerBases() != bases {
		t.Fatal("strict reopen must preserve the original unstored model event and mixed bases", err)
	}
	verifyOriginalMappedChainSTEPGraph(t, m, strict)
	replay.Reset()
	if err := strict.Shell.WriteJSON(ctx, &replay, nativeOptions); err != nil || !bytes.Equal(recipe.Bytes(), replay.Bytes()) {
		t.Fatal("canonical reopening must recreate the identical v3 source proof", err)
	}
	replay.Reset()
	if err := WriteMappedChainRectangularOriginalShell(ctx, &replay, strict, Limits{}, o); err != nil || !bytes.Equal(canonical.Bytes(), replay.Bytes()) {
		t.Fatal("strict canonical STEP must replay the original model graph byte-for-byte", err)
	}
	return canonical.Bytes()
}

func verifyInferredSharedMappedChainMaterial(t *testing.T, ctx context.Context, m trimmed.MappedChainRectangularSolid) {
	t.Helper()
	s := m.Solid
	if len(m.ChainEdges) != 4 || len(s.Faces()) != 7 || len(s.Edges()) != 20 || len(s.Vertices()) != 15 {
		t.Fatal("one exact inferred model vertex must retain the complete solid topology")
	}
	proof, err := s.ValidateEdgeAgreement(ctx, edgecheck.Options{})
	if err != nil || !proof.Complete || len(proof.Uses) != 40 {
		t.Fatal("all source coedges must retain complete continuous agreement", err)
	}
	mass, err := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
	if err != nil || math.Abs(mass.Volume-inferredSharedMappedChainVolume) > 1e-8 || math.Abs(mass.Area-inferredSharedMappedChainArea) > 1e-8 {
		t.Fatal("native material must match the separately authored solid", mass, err)
	}
}

func TestOCCTMappedChainRectangularInferredSharedSupportKnot(t *testing.T) {
	verifyInferredSharedMappedChain(t, context.Background(), readInferredSharedMappedChainFixture(t, inferredSharedMappedChainPath))
}

func TestOCCTMappedChainRectangularInferredSharedSupportKnotClosedDisplay(t *testing.T) {
	ctx := context.Background()
	raw := readInferredSharedMappedChainFixture(t, inferredSharedMappedChainPath)
	m, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(raw), Limits{}, inferredSharedMappedChainOptions())
	if err != nil {
		t.Fatal(err)
	}
	verifyMappedChainCutDisplay(t, ctx, m)
}

func TestOCCTMappedChainRectangularInferredSharedSupportKnotRejectsInvalidSource(t *testing.T) {
	ctx := context.Background()
	raw, o := readInferredSharedMappedChainFixture(t, inferredSharedMappedChainPath), inferredSharedMappedChainOptions()
	previous := sameCellAsynchronousOptions()
	bad, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(raw), Limits{}, previous)
	if !errors.Is(err, geom.ErrConvergence) || !strings.Contains(err.Error(), "stations must be distinct") || len(bad.Crossings) != 0 {
		t.Fatal("the previous same-cell gate requires distinct model stations", err)
	}
	asynchronous := readSameCellAsynchronousFixture(t)
	bad, err = ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(asynchronous), Limits{}, o)
	if !errors.Is(err, geom.ErrConvergence) || !strings.Contains(err.Error(), "must coincide") || len(bad.Crossings) != 0 {
		t.Fatal("the shared inferred gate requires exact equality of both model stations", err)
	}
	rejectInferredSharedStoredKnot(t, ctx, readInferredSharedMappedChainFixture(t, redundantInferredSharedMappedChainPath))

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
	// The original model curve and shared event stay fixed while the
	// independently authored north raw root becomes nonbinary.
	knots[1].Items[1].Text = "108."
	bad, err = ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(serializeWrappedProofFile(f)), Limits{}, o)
	if !errors.Is(err, geom.ErrUnrepresentable) || len(bad.Crossings) != 0 || len(bad.Native.ChainEdges) != 0 {
		t.Fatal("both original owner stops must be independently exact and representable", err)
	}
}

func rejectInferredSharedStoredKnot(t *testing.T, ctx context.Context, raw []byte) {
	t.Helper()
	bad, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(raw), Limits{}, inferredSharedMappedChainOptions())
	if !errors.Is(err, geom.ErrConvergence) || !strings.Contains(err.Error(), "without a stored knot") ||
		len(bad.Crossings) != 0 || len(bad.Native.ChainEdges) != 0 {
		t.Fatal("a redundant stored model knot at the shared event must reject atomically", err)
	}
}

func TestOCCTMappedChainRectangularInferredSharedSupportKnotLive(t *testing.T) {
	author, reader := os.Getenv("PLAINSOLID_OCCT_MAPPED_CHAIN_INFERRED_SHARED_AUTHOR"), os.Getenv("PLAINSOLID_OCCT_MAPPED_CHAIN_STEP")
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
	raw := authorFile("write-shared-inferred-knot")
	canonical := verifyInferredSharedMappedChain(t, ctx, raw)
	verifyPinnedMappedChainTransfer(t, ctx, reader, raw, canonical, inferredSharedMappedChainVolume, inferredSharedMappedChainArea)
	redundant := authorFile("write-shared-inferred-redundant-knot")
	verifyPinnedMappedChainTransfer(t, ctx, reader, raw, redundant, inferredSharedMappedChainVolume, inferredSharedMappedChainArea)
	rejectInferredSharedStoredKnot(t, ctx, redundant)
}
