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
	"slices"
	"strings"
	"testing"
	"time"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

const separatedCellMappedChainPath = "testdata/occt_mapped_chain_separated_support_cells.step"
const mappedChainSeparatedCellVolume = 501639737.0 / 536870912
const mappedChainSeparatedCellArea = 6.098374182498745

func separatedCellMappedChainOptions() MappedChainShellSTEPOptions {
	return MappedChainShellSTEPOptions{
		AllowMultiSpanRoofs: true, AllowTwoInteriorRoofKnots: true, AllowUnsplitSupportKnot: true,
		AllowMixedOwnerBases: true, AllowSeparatedSupportKnotCells: true,
	}
}

func readSeparatedCellMappedChainFixture(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile(separatedCellMappedChainPath)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func verifySeparatedCellMappedChain(t *testing.T, ctx context.Context, raw []byte) []byte {
	t.Helper()
	o := separatedCellMappedChainOptions()
	m, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(raw), Limits{}, o)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Chain) != 3 || len(m.Crossings) != 3 || !mappedChainSeparatedSupportCells(m.Chain, m.Crossings) {
		t.Fatal("the original first and last model edges must retain their separated support events")
	}
	bases := m.Shell.IndependentOwnerBases()
	if bases[0] == bases[1] {
		t.Fatal("the north per-edge bases and south whole basis must survive")
	}
	north := 0
	if !bases[0] {
		north = 1
	}
	verifySeparatedCellMaterial(t, ctx, m.Native)
	edges, vertices, faces := m.Native.Solid.Edges(), m.Native.Solid.Vertices(), m.Native.Solid.Faces()
	eventCells, eventChildren := [3]int{0, 0, 2}, [3]int{0, 1, 4}
	modelStations := [3]int64{2, 4, 24}
	rawPairs := [3][2]int64{{17, 40}, {37, 32}, {858, -1}}
	points := [3]geom.Vec3{
		{X: 107.0 / 2048, Y: .25, Z: 107.0 / 8192},
		{X: 29.0 / 256, Y: .25, Z: 29.0 / 1024},
		{X: .75, Y: .25, Z: 547.0 / 2048},
	}
	for i, cut := range m.Crossings {
		cell, child := eventCells[i], eventChildren[i]
		if cut.EdgeID != m.Chain[cell].EdgeID || cut.StoredModelKnot != (i == 1) ||
			cut.SourceModelParameter.Cmp(big.NewRat(modelStations[i], 1)) != 0 ||
			cut.ModelStation.Cmp(big.NewRat(modelStations[i], 32)) != 0 ||
			cut.RawParameters[north].Cmp(big.NewRat(rawPairs[i][0], 1)) != 0 ||
			cut.RawParameters[1-north].Cmp(big.NewRat(rawPairs[i][1], 1)) != 0 ||
			cut.OwnersChangingCells[north] != (i < 2) || cut.OwnersChangingCells[1-north] != (i > 0) ||
			cut.NativeEdges != ([2]int{m.Native.ChainEdges[child], m.Native.ChainEdges[child+1]}) {
			t.Fatal("the model knot and owner-only roots must retain their original raw stations and edge ancestry", i, cut)
		}
		a, b := edges[cut.NativeEdges[0]], edges[cut.NativeEdges[1]]
		if a.End != b.Start || vertices[a.End] != points[i] {
			t.Fatal("a repaired edge pair must share one exact original model point", i)
		}
	}
	file, err := Parse(bytes.NewReader(raw), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	northStops := [3][]int64{{2, 17, 37, 107}, {700, 715}, {850, 858, 862}}
	southStops := [3][]int64{{48, 40, 32, 16}, {16, 1}, {1, -1, -2}}
	modelKnots := [3][]int64{{0, 0, 0, 0, 4, 4, 4, 8, 8, 8, 8}, {8, 8, 8, 8, 16, 16, 16, 16}, {16, 16, 16, 16, 32, 32, 32, 32}}
	firstChild := [3]int{0, 3, 4}
	for cell, source := range m.Chain {
		model, err := readMappedChainStepSpline(file, source.ModelCurveID, 3, 16, 33, true)
		if err != nil || !slices.EqualFunc(model.knots, modelKnots[cell], func(a *big.Rat, b int64) bool {
			return a.Cmp(big.NewRat(b, 1)) == 0
		}) || len(model.poles) != len(modelKnots[cell])-4 {
			t.Fatal("the original curve must retain all stored model knots and its exact finite domain", cell, err)
		}
		for owner, use := range source.Owners {
			a, err := originalMappedChainStepOwner(file, use, true)
			if err != nil {
				t.Fatal(err)
			}
			stops := southStops[cell]
			if owner == north {
				stops = northStops[cell]
			}
			for i := 0; i < len(stops)-1; i++ {
				q := faces[owner].Outer[firstChild[cell]+i].Curve.(trimmed.MappedSplineUV)
				if !sameMixedMappedChainBasis(q.Source().Curve, a.Curve) {
					t.Fatal("every native child must retain its own original owner basis", cell, owner, i)
				}
				for end, want := range stops[i : i+2] {
					stop, err := q.AuthoredParameter(big.NewRat(int64(end), 1))
					if err != nil || stop.Cmp(big.NewRat(want, 1)) != 0 {
						t.Fatal("the exact original owner restrictions must survive each cell boundary", cell, owner, i, end, err)
					}
				}
			}
		}
	}
	middle := edges[m.Native.ChainEdges[3]]
	if m.Native.ChainStations[3].Cmp(big.NewRat(1, 4)) != 0 || m.Native.ChainStations[4].Cmp(big.NewRat(1, 2)) != 0 ||
		edges[m.Native.ChainEdges[2]].End != middle.Start || middle.End != edges[m.Native.ChainEdges[4]].Start {
		t.Fatal("the event-free original middle curve must remain one complete model edge")
	}
	south := faces[1-north].Surface.(*geom.BSplineSurface)
	verifyMappedRoofSlopeJump(t, ctx, south, 1, .75, big.NewRat(1, 8), big.NewRat(-1, 64))
	verifyMappedRoofSlopeJump(t, ctx, south, 1, .75, big.NewRat(1, 4), new(big.Rat))

	var recipe, canonical, replay bytes.Buffer
	if err := m.Shell.WriteJSON(ctx, &recipe, trimmed.MappedChainShellNativeOptions{}); err != nil {
		t.Fatal(err)
	}
	v9 := []byte(`"Boundary":{"Version":9`)
	if !bytes.Contains(recipe.Bytes(), v9) {
		t.Fatal("the separated source partition must be recorded as v9")
	}
	if err := WriteMappedChainRectangularOriginalShell(ctx, &canonical, m, Limits{}, o); err != nil {
		t.Fatal(err)
	}
	strict, err := ReadMappedChainRectangularShell(ctx, bytes.NewReader(canonical.Bytes()), Limits{}, o)
	if err != nil || len(strict.Chain) != 3 || len(strict.Crossings) != 3 ||
		!mappedChainSeparatedSupportCells(strict.Chain, strict.Crossings) || strict.Shell.IndependentOwnerBases() != bases {
		t.Fatal("strict reopen must re-prove the exact separated original support partition", err)
	}
	verifyOriginalMappedChainSTEPGraph(t, m, strict)
	if err := strict.Shell.WriteJSON(ctx, &replay, trimmed.MappedChainShellNativeOptions{}); err != nil || !bytes.Equal(recipe.Bytes(), replay.Bytes()) {
		t.Fatal("strict canonical export must reconstruct the identical native v9 source recipe", err)
	}
	opened, err := trimmed.ReadMappedSplineChainShellJSON(ctx, bytes.NewReader(recipe.Bytes()), trimmed.MappedChainShellNativeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	native, err := opened.BuildRectangularSolid(ctx, 1e-9, o.Geometry)
	if err != nil {
		t.Fatal(err)
	}
	verifySeparatedCellMaterial(t, ctx, native)
	replay.Reset()
	if err := WriteMappedChainRectangularOriginalShell(ctx, &replay, strict, Limits{}, o); err != nil || !bytes.Equal(canonical.Bytes(), replay.Bytes()) {
		t.Fatal("the canonical original source graph must replay byte-for-byte", err)
	}
	downgraded := bytes.Replace(recipe.Bytes(), v9, []byte(`"Boundary":{"Version":7`), 1)
	if _, err := trimmed.ReadMappedSplineChainShellJSON(ctx, bytes.NewReader(downgraded), trimmed.MappedChainShellNativeOptions{}); !errors.Is(err, geom.ErrConvergence) {
		t.Fatal("native v7 must keep its one-original-cell proof", err)
	}
	return canonical.Bytes()
}

func verifySeparatedCellMaterial(t *testing.T, ctx context.Context, native trimmed.MappedChainRectangularSolid) {
	t.Helper()
	s := native.Solid
	if len(native.ChainEdges) != 6 || len(s.Faces()) != 7 || len(s.Edges()) != 24 || len(s.Vertices()) != 19 {
		t.Fatal("the two roofs and their closing faces must preserve the entire repaired topology")
	}
	proof, err := s.ValidateEdgeAgreement(ctx, edgecheck.Options{})
	if err != nil || !proof.Complete || len(proof.Uses) != 48 {
		t.Fatal("both original roof owners and all closing edges require continuous equality", err)
	}
	mass, err := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
	if err != nil || math.Abs(mass.Volume-mappedChainSeparatedCellVolume) > 1e-8 || math.Abs(mass.Area-mappedChainSeparatedCellArea) > 1e-8 {
		t.Fatal("native material must match the independently authored separated support fixture", mass, err)
	}
}

func TestOCCTMappedChainRectangularSeparatedSupportKnotCells(t *testing.T) {
	t.Parallel()
	verifySeparatedCellMappedChain(t, context.Background(), readSeparatedCellMappedChainFixture(t))
}

func TestOCCTMappedChainRectangularSeparatedSupportKnotCellsClosedDisplay(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	m, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(readSeparatedCellMappedChainFixture(t)), Limits{}, separatedCellMappedChainOptions())
	if err != nil {
		t.Fatal(err)
	}
	verifyMappedChainCutDisplay(t, ctx, m)
}

func TestOCCTMappedChainRectangularSeparatedSupportKnotCellsRejectsInvalidSource(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	raw := readSeparatedCellMappedChainFixture(t)
	o := separatedCellMappedChainOptions()
	m, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(raw), Limits{}, o)
	if err != nil {
		t.Fatal(err)
	}
	older := o
	older.AllowSeparatedSupportKnotCells, older.AllowThreeSupportKnots = false, true
	if bad, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(raw), Limits{}, older); err == nil || len(bad.Crossings) != 0 {
		t.Fatal("the older source gate must retain its one-original-cell bound", err)
	}
	north := 0
	if !m.Shell.IndependentOwnerBases()[0] {
		north = 1
	}
	for _, tc := range []struct {
		name, diagnosis string
		want            error
		edit            func(*File)
	}{
		{"owner-only knot moved to seam endpoint", "required separated support event is missing", geom.ErrConvergence, func(f *File) {
			moveSeparatedSouthCrease(t, f, m.FaceIDs[1-north], .5)
		}},
		{"owner-only knot moved into the intervening cell", "intervening original edge without a cut", geom.ErrConvergence, func(f *File) {
			moveSeparatedSouthCrease(t, f, m.FaceIDs[1-north], 99.0/256)
		}},
		{"final companion raw stop no longer representable", "not representable", geom.ErrUnrepresentable, func(f *File) {
			_, knots, _, err := mappedChainStepSplineParts(f, m.Chain[2].Owners[north].BasisID)
			if err != nil {
				t.Fatal(err)
			}
			knots[1].Items[1].Text = "863."
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, err := Parse(bytes.NewReader(raw), Limits{})
			if err != nil {
				t.Fatal(err)
			}
			tc.edit(f)
			bad, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(serializeWrappedProofFile(f)), Limits{}, o)
			if !errors.Is(err, tc.want) || !strings.Contains(err.Error(), tc.diagnosis) || len(bad.Crossings) != 0 || len(bad.Native.ChainEdges) != 0 {
				t.Fatal("an unproved source partition must reject before exposing any repaired edge", err)
			}
		})
	}
	t.Run("native recipe loses the last original interior event", func(t *testing.T) {
		f, err := Parse(bytes.NewReader(raw), Limits{})
		if err != nil {
			t.Fatal(err)
		}
		south := 1 - north
		moveSeparatedSouthCrease(t, f, m.FaceIDs[south], .5)
		face := f.Entities[m.FaceIDs[south]].Records[0]
		changed, err := DecodeBSplineSurface(f, face.Args[2].Ref, 81)
		if err != nil {
			t.Fatal(err)
		}
		before, err := parametric.EncodeSurface(m.Native.Solid.Faces()[south].Surface)
		if err != nil {
			t.Fatal(err)
		}
		after, err := parametric.EncodeSurface(changed.Geometry)
		if err != nil {
			t.Fatal(err)
		}
		var recipe bytes.Buffer
		if err := m.Shell.WriteJSON(ctx, &recipe, trimmed.MappedChainShellNativeOptions{}); err != nil {
			t.Fatal(err)
		}
		if !bytes.Contains(recipe.Bytes(), before) {
			t.Fatal("the original south support must be stored in the native source recipe")
		}
		mutated := bytes.Replace(recipe.Bytes(), before, after, 1)
		bad, err := trimmed.ReadMappedSplineChainShellJSON(ctx, bytes.NewReader(mutated), trimmed.MappedChainShellNativeOptions{})
		if !errors.Is(err, geom.ErrConvergence) || !strings.Contains(err.Error(), "one interior event per original support knot") || bad.Topology().Faces != 0 {
			t.Fatal("source-only native reload must discover every original support event again", err)
		}
	})
}

func moveSeparatedSouthCrease(t *testing.T, f *File, faceID int64, x float64) {
	t.Helper()
	face := f.Entities[faceID].Records[0]
	roof := f.Entities[face.Args[2].Ref].Records[0]
	decimal := func(number float64) string {
		text, err := mappedChainStepDecimal(mappedChainStepRat(number))
		if err != nil {
			t.Fatal(err)
		}
		return text
	}
	roof.Args[11].Items[2].Text = decimal(x)
	// The model curves stay exactly the same at y=1/4.
	knots := [4]float64{0, 29.0 / 256, x, 1}
	for i, row := range roof.Args[3].Items {
		for j, pole := range row.Items {
			xp, y := knots[j], float64(i)
			z := xp*y + .125*max(0, xp-29.0/256) + .125*(y-.25)*max(0, xp-x)
			p := f.Entities[pole.Ref].Records[0].Args[1].Items
			p[0].Text, p[2].Text = decimal(xp), decimal(z)
		}
	}
}

func TestOCCTMappedChainRectangularSeparatedSupportKnotCellsLive(t *testing.T) {
	t.Parallel()
	author, reader := os.Getenv("PLAINSOLID_OCCT_MAPPED_CHAIN_SEPARATED_AUTHOR"), os.Getenv("PLAINSOLID_OCCT_MAPPED_CHAIN_STEP")
	if author == "" || reader == "" {
		t.Skip("optional pinned OCCT source author and STEP consumer")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	path := filepath.Join(t.TempDir(), "occt-authored-separated-support-cells.step")
	if out, err := exec.CommandContext(ctx, author, "write-separated-support-cells", path).CombinedOutput(); err != nil {
		t.Fatalf("pinned OCCT author failed: %v\n%s", err, out)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	canonical := verifySeparatedCellMappedChain(t, ctx, raw)
	verifyPinnedMappedChainTransfer(t, ctx, reader, raw, canonical, mappedChainSeparatedCellVolume, mappedChainSeparatedCellArea)
}
