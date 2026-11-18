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

const sameCellAsynchronousMappedChainPath = "testdata/occt_mapped_chain_same_cell_asynchronous.step"

// Integrate xy and each owner's crease contribution over its original region.
const sameCellAsynchronousVolume = 1924853507.0 / 2147483648
const sameCellAsynchronousArea = 5.881962919399700

func sameCellAsynchronousOptions() MappedChainShellSTEPOptions {
	return MappedChainShellSTEPOptions{
		AllowMultiSpanRoofs: true, AllowUnsplitSupportKnot: true,
		AllowMixedOwnerBases: true, AllowSameCellAsynchronousSupportKnots: true,
	}
}

func readSameCellAsynchronousFixture(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile(sameCellAsynchronousMappedChainPath)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func verifySameCellAsynchronousSource(t *testing.T, ctx context.Context, raw []byte) []byte {
	t.Helper()
	o := sameCellAsynchronousOptions()
	m, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(raw), Limits{}, o)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Chain) != 3 || !mappedChainSameCellAsynchronousCrossings(m.Crossings) {
		t.Fatal("the two owner events must share one original model edge")
	}
	bases := m.Shell.IndependentOwnerBases()
	if bases[0] == bases[1] {
		t.Fatal("the original opposite owner basis organizations must survive")
	}
	north := 0
	if !bases[0] {
		north = 1
	}
	south := 1 - north
	verifySameCellAsynchronousMaterial(t, ctx, m.Native)
	edges, vertices, faces := m.Native.Solid.Edges(), m.Native.Solid.Vertices(), m.Native.Solid.Faces()
	x := [2]float64{107.0 / 2048, 369.0 / 2048}
	modelRaw := [2]int64{2, 6}
	rawStops := [2][4]int64{}
	rawStops[north] = [4]int64{2, 17, 65, 107}
	rawStops[south] = [4]int64{48, 40, 24, 16}
	for i, cut := range m.Crossings {
		if cut.EdgeID != m.Chain[0].EdgeID || cut.SourceModelParameter.Cmp(big.NewRat(modelRaw[i], 1)) != 0 ||
			cut.ModelStation.Cmp(big.NewRat(modelRaw[i], 32)) != 0 ||
			cut.RawParameters[north].Cmp(big.NewRat(rawStops[north][i+1], 1)) != 0 ||
			cut.RawParameters[south].Cmp(big.NewRat(rawStops[south][i+1], 1)) != 0 ||
			cut.OwnersChangingCells[north] != (i == 0) || cut.OwnersChangingCells[south] != (i == 1) ||
			cut.NativeEdges != ([2]int{m.Native.ChainEdges[i], m.Native.ChainEdges[i+1]}) {
			t.Fatal("the paired exact raw stations must preserve the original edge and opposite roof ownership", i, cut)
		}
		a, b := edges[cut.NativeEdges[0]], edges[cut.NativeEdges[1]]
		if a.End != b.Start || vertices[a.End] != (geom.Vec3{X: x[i], Y: .25, Z: x[i] / 4}) {
			t.Fatal("each repaired child pair must use one exact point of the original smooth model", i)
		}
	}
	f, err := Parse(bytes.NewReader(raw), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	model, err := readMappedChainStepSpline(f, m.Chain[0].ModelCurveID, 3, 16, 17, true)
	if err != nil || len(model.poles) != 4 || len(model.knots) != 8 {
		t.Fatal("the original model must be one authored cubic Bézier", err)
	}
	for i, px := range [4]float64{0, 1.0 / 16, 5.0 / 32, .25} {
		if model.knots[i].Sign() != 0 || model.knots[i+4].Cmp(big.NewRat(8, 1)) != 0 || model.weights[i].Cmp(big.NewRat(1, 1)) != 0 ||
			model.poles[i][0].Cmp(mappedChainStepRat(px)) != 0 || model.poles[i][1].Cmp(big.NewRat(1, 4)) != 0 ||
			model.poles[i][2].Cmp(mappedChainStepRat(px/4)) != 0 {
			t.Fatal("the original 3D source must retain its raw finite domain and exact cubic seam", i)
		}
	}
	for owner, use := range m.Chain[0].Owners {
		original, err := originalMappedChainStepOwner(f, use, true)
		if err != nil {
			t.Fatal(err)
		}
		for i := range 3 {
			q := faces[owner].Outer[i].Curve.(trimmed.MappedSplineUV)
			if !sameMixedMappedChainBasis(q.Source().Curve, original.Curve) {
				t.Fatal("the three repaired children must retain the same original owner basis", owner, i)
			}
			for side, want := range rawStops[owner][i : i+2] {
				stop, err := q.AuthoredParameter(big.NewRat(int64(side), 1))
				if err != nil || stop.Cmp(big.NewRat(want, 1)) != 0 {
					t.Fatal("a child lost its exact local original owner interval", owner, i, side, err)
				}
			}
		}
	}
	northRoof, southRoof := faces[north].Surface.(*geom.BSplineSurface), faces[south].Surface.(*geom.BSplineSurface)
	verifyMappedRoofSlopeJump(t, ctx, northRoof, 0, x[0], big.NewRat(1, 2), big.NewRat(1, 32))
	verifyMappedRoofSlopeJump(t, ctx, southRoof, 1, x[1], big.NewRat(1, 8), big.NewRat(-1, 64))
	verifyMappedRoofSlopeJump(t, ctx, northRoof, 0, x[0], big.NewRat(1, 4), new(big.Rat))
	verifyMappedRoofSlopeJump(t, ctx, southRoof, 1, x[1], big.NewRat(1, 4), new(big.Rat))

	var recipe, canonical, replay bytes.Buffer
	if err := m.Shell.WriteJSON(ctx, &recipe, trimmed.MappedChainShellNativeOptions{}); err != nil {
		t.Fatal(err)
	}
	tag := []byte(`"Boundary":{"Version":10`)
	if !bytes.Contains(recipe.Bytes(), tag) {
		t.Fatal("the native recipe must distinguish the same-original-cell proof")
	}
	if err := WriteMappedChainRectangularOriginalShell(ctx, &canonical, m, Limits{}, o); err != nil {
		t.Fatal(err)
	}
	strict, err := ReadMappedChainRectangularShell(ctx, bytes.NewReader(canonical.Bytes()), Limits{}, o)
	if err != nil || len(strict.Chain) != 3 || !mappedChainSameCellAsynchronousCrossings(strict.Crossings) || strict.Shell.IndependentOwnerBases() != bases {
		t.Fatal("strict reopen must preserve one original model Bézier and both original mixed owners", err)
	}
	verifyOriginalMappedChainSTEPGraph(t, m, strict)
	if err := strict.Shell.WriteJSON(ctx, &replay, trimmed.MappedChainShellNativeOptions{}); err != nil || !bytes.Equal(recipe.Bytes(), replay.Bytes()) {
		t.Fatal("strict re-export must reconstruct the original source-only native recipe", err)
	}
	opened, err := trimmed.ReadMappedSplineChainShellJSON(ctx, bytes.NewReader(recipe.Bytes()), trimmed.MappedChainShellNativeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	native, err := opened.BuildRectangularSolid(ctx, 1e-9, o.Geometry)
	if err != nil {
		t.Fatal(err)
	}
	verifySameCellAsynchronousMaterial(t, ctx, native)
	replay.Reset()
	if err := opened.WriteJSON(ctx, &replay, trimmed.MappedChainShellNativeOptions{}); err != nil || !bytes.Equal(recipe.Bytes(), replay.Bytes()) {
		t.Fatal("original source-only native bytes must replay exactly", err)
	}
	replay.Reset()
	if err := WriteMappedChainRectangularOriginalShell(ctx, &replay, strict, Limits{}, o); err != nil || !bytes.Equal(canonical.Bytes(), replay.Bytes()) {
		t.Fatal("strict canonical original STEP bytes must replay exactly", err)
	}
	legacy := bytes.Replace(recipe.Bytes(), tag, []byte(`"Boundary":{"Version":5`), 1)
	if _, err := trimmed.ReadMappedSplineChainShellJSON(ctx, bytes.NewReader(legacy), trimmed.MappedChainShellNativeOptions{}); !errors.Is(err, geom.ErrConvergence) {
		t.Fatal("v5 requires two distinct original edges", err)
	}
	return canonical.Bytes()
}

func verifySameCellAsynchronousMaterial(t *testing.T, ctx context.Context, m trimmed.MappedChainRectangularSolid) {
	t.Helper()
	s := m.Solid
	if len(m.ChainEdges) != 5 || len(s.Faces()) != 7 || len(s.Edges()) != 21 || len(s.Vertices()) != 16 {
		t.Fatal("the two event vertices must preserve the entire source topology")
	}
	proof, err := s.ValidateEdgeAgreement(ctx, edgecheck.Options{})
	if err != nil || !proof.Complete || len(proof.Uses) != 42 {
		t.Fatal("every native edge use must have complete source agreement", err)
	}
	mass, err := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
	if err != nil || math.Abs(mass.Volume-sameCellAsynchronousVolume) > 1e-8 || math.Abs(mass.Area-sameCellAsynchronousArea) > 1e-8 {
		t.Fatal("native material must match the independently authored source", mass, err)
	}
}

func TestOCCTMappedChainRectangularSameCellAsynchronousSupportKnots(t *testing.T) {
	verifySameCellAsynchronousSource(t, context.Background(), readSameCellAsynchronousFixture(t))
}

func TestOCCTMappedChainRectangularSameCellAsynchronousSupportKnotsClosedDisplay(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	m, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(readSameCellAsynchronousFixture(t)), Limits{}, sameCellAsynchronousOptions())
	if err != nil {
		t.Fatal(err)
	}
	verifyMappedChainCutDisplay(t, ctx, m)
}

func TestOCCTMappedChainRectangularSameCellAsynchronousSupportKnotsRejectsInvalidSource(t *testing.T) {
	ctx := context.Background()
	raw, o := readSameCellAsynchronousFixture(t), sameCellAsynchronousOptions()
	older := o
	older.AllowSameCellAsynchronousSupportKnots, older.AllowAsynchronousSupportKnots = false, true
	if bad, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(raw), Limits{}, older); err == nil || len(bad.Crossings) != 0 {
		t.Fatal("the earlier option cannot certify both owner events on one original edge", err)
	}
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
	// The north event stays exact at raw 18. The south event pulls back to
	// a nonbinary north parameter on the common partition.
	knots[1].Items[1].Text = "114."
	bad, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(serializeWrappedProofFile(f)), Limits{}, o)
	if !errors.Is(err, geom.ErrUnrepresentable) || !strings.Contains(err.Error(), "paired original Bézier partition") ||
		len(bad.Crossings) != 0 || len(bad.Native.ChainEdges) != 0 {
		t.Fatal("the same-cell proof must reject a nonrepresentable original restriction atomically", err)
	}
}

func TestOCCTMappedChainRectangularSameCellAsynchronousSupportKnotsLive(t *testing.T) {
	author, reader := os.Getenv("PLAINSOLID_OCCT_MAPPED_CHAIN_SAME_CELL_ASYNC_AUTHOR"), os.Getenv("PLAINSOLID_OCCT_MAPPED_CHAIN_STEP")
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
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	raw := authorFile("write-same-cell-asynchronous")
	canonical := verifySameCellAsynchronousSource(t, ctx, raw)
	verifyPinnedMappedChainTransfer(t, ctx, reader, raw, canonical, sameCellAsynchronousVolume, sameCellAsynchronousArea)
	redundant := authorFile("write-same-cell-redundant-knot")
	verifyPinnedMappedChainTransfer(t, ctx, reader, raw, redundant, sameCellAsynchronousVolume, sameCellAsynchronousArea)
	bad, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(redundant), Limits{}, sameCellAsynchronousOptions())
	if !errors.Is(err, geom.ErrConvergence) || !strings.Contains(err.Error(), "without a stored knot") || len(bad.Crossings) != 0 || len(bad.Native.ChainEdges) != 0 {
		t.Fatal("the independently authored exact but redundant model knot must fail the single-Bézier graph gate", err)
	}
}
