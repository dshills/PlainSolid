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

const localCellMappedChainPath = "testdata/occt_mapped_chain_two_support_cells.step"

// The integration covers xy and both shared crease terms over the whole roof,
// then each owner-only term over its original north or south region.
const mappedChainLocalCellVolume = 2043402795.0 / 2147483648
const mappedChainLocalCellArea = 6.223151117122341

func localCellMappedChainOptions() MappedChainShellSTEPOptions {
	return MappedChainShellSTEPOptions{AllowMultiSpanRoofs: true, AllowTwoInteriorRoofKnots: true,
		AllowUnsplitSupportKnot: true, AllowMultipleSupportKnotCells: true, AllowMixedOwnerBases: true}
}

func verifyLocalCellMappedChain(t *testing.T, ctx context.Context, raw []byte) []byte {
	t.Helper()
	o := localCellMappedChainOptions()
	// Both modes fit the authored roof count. Each original model graph must
	// select its own proof even when the v8 option is coenabled.
	o.AllowUpToSixSupportKnots = true
	m, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(raw), Limits{}, o)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Chain) != 3 || len(m.Crossings) != 4 || !mappedChainCrossingsOnTwoLocalCells(m.Crossings) {
		t.Fatal("four events must be distributed over the original two unsplit edges")
	}
	bases := m.Shell.IndependentOwnerBases()
	if bases[0] == bases[1] {
		t.Fatal("the original two roof basis organizations must survive")
	}
	north := 0
	if !bases[0] {
		north = 1
	}
	verifyLocalCellMaterial(t, ctx, m.Native)
	edges, vertices, faces := m.Native.Solid.Edges(), m.Native.Solid.Vertices(), m.Native.Solid.Faces()
	modelStations := [4]int64{2, 4, 12, 14}
	rawPairs := [4][2]int64{{17, 40}, {37, 32}, {706, 11}, {710, 7}}
	points := [4]geom.Vec3{{X: 107.0 / 2048, Y: .25, Z: 107.0 / 8192}, {X: 29.0 / 256, Y: .25, Z: 29.0 / 1024},
		{X: 99.0 / 256, Y: .25, Z: 67.0 / 512}, {X: 917.0 / 2048, Y: .25, Z: 661.0 / 4096}}
	for i, cut := range m.Crossings {
		cell, child := i/2, i+i/2
		if cut.EdgeID != m.Chain[cell].EdgeID || cut.StoredModelKnot != (i == 1 || i == 2) ||
			cut.SourceModelParameter.Cmp(big.NewRat(modelStations[i], 1)) != 0 || cut.ModelStation.Cmp(big.NewRat(modelStations[i], 32)) != 0 ||
			cut.RawParameters[north].Cmp(big.NewRat(rawPairs[i][0], 1)) != 0 || cut.RawParameters[1-north].Cmp(big.NewRat(rawPairs[i][1], 1)) != 0 ||
			cut.OwnersChangingCells[north] != (i != 3) || cut.OwnersChangingCells[1-north] != (i != 0) ||
			cut.NativeEdges != ([2]int{m.Native.ChainEdges[child], m.Native.ChainEdges[child+1]}) {
			t.Fatal("the local source cell must own both native edges and their exact raw event", i, cut)
		}
		a, b := edges[cut.NativeEdges[0]], edges[cut.NativeEdges[1]]
		if a.End != b.Start || vertices[a.End] != points[i] {
			t.Fatal("a repaired edge pair must share one exact native vertex", i)
		}
	}
	file, err := Parse(bytes.NewReader(raw), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	northStops := [][]int64{{2, 17, 37, 107}, {700, 706, 710, 715}, {850, 862}}
	southStops := [][]int64{{48, 40, 32, 16}, {16, 11, 7, 1}, {1, 0}}
	for cell, source := range m.Chain {
		model, err := readMappedChainStepSpline(file, source.ModelCurveID, 3, 16, 33, true)
		if err != nil {
			t.Fatal(err)
		}
		if cell < 2 {
			start := int64(8 * cell)
			if len(model.poles) != 7 || model.knots[0].Cmp(big.NewRat(start, 1)) != 0 || model.knots[10].Cmp(big.NewRat(start+8, 1)) != 0 {
				t.Fatal("the original cell must have two exact cubic pieces", cell)
			}
			for _, knot := range model.knots[4:7] {
				if knot.Cmp(big.NewRat(start+4, 1)) != 0 {
					t.Fatal("each original edge must retain its own stored material knot", cell)
				}
			}
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
				q := faces[owner].Outer[3*cell+i].Curve.(trimmed.MappedSplineUV)
				if !sameMixedMappedChainBasis(q.Source().Curve, a.Curve) {
					t.Fatal("repaired child must retain the basis of its own original model cell", cell, owner, i)
				}
				for end, want := range stops[i : i+2] {
					stop, err := q.AuthoredParameter(big.NewRat(int64(end), 1))
					if err != nil || stop.Cmp(big.NewRat(want, 1)) != 0 {
						t.Fatal("the local restriction lost its original, possibly one-sided raw parameter", cell, owner, i, end, err)
					}
				}
			}
		}
	}
	if m.Native.ChainStations[3].Cmp(big.NewRat(1, 4)) != 0 || edges[m.Native.ChainEdges[2]].End != edges[m.Native.ChainEdges[3]].Start {
		t.Fatal("the original junction between cells must keep one model vertex and both distinct owner endpoints")
	}
	verifyMappedRoofSlopeJump(t, ctx, faces[north].Surface.(*geom.BSplineSurface), 0, 107.0/2048, big.NewRat(1, 2), big.NewRat(1, 32))
	verifyMappedRoofSlopeJump(t, ctx, faces[1-north].Surface.(*geom.BSplineSurface), 1, 917.0/2048, big.NewRat(1, 8), big.NewRat(-1, 64))
	var recipe, canonical, replay bytes.Buffer
	if err := m.Shell.WriteJSON(ctx, &recipe, trimmed.MappedChainShellNativeOptions{}); err != nil {
		t.Fatal(err)
	}
	v9 := []byte(`"Boundary":{"Version":9`)
	if !bytes.Contains(recipe.Bytes(), v9) {
		t.Fatal("multiple original model cells need a distinct native proof identifier")
	}
	if err := WriteMappedChainRectangularOriginalShell(ctx, &canonical, m, Limits{}, o); err != nil {
		t.Fatal(err)
	}
	strict, err := ReadMappedChainRectangularShell(ctx, bytes.NewReader(canonical.Bytes()), Limits{}, o)
	if err != nil || len(strict.Chain) != 3 || len(strict.Crossings) != 4 || !mappedChainCrossingsOnTwoLocalCells(strict.Crossings) || strict.Shell.IndependentOwnerBases() != bases {
		t.Fatal("strict import must re-prove both original cell partitions", err)
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
	verifyLocalCellMaterial(t, ctx, native)
	replay.Reset()
	if err := WriteMappedChainRectangularOriginalShell(ctx, &replay, strict, Limits{}, o); err != nil || !bytes.Equal(canonical.Bytes(), replay.Bytes()) {
		t.Fatal("the two original canonical graphs must replay byte-for-byte", err)
	}
	downgraded := bytes.Replace(recipe.Bytes(), v9, []byte(`"Boundary":{"Version":8`), 1)
	if _, err := trimmed.ReadMappedSplineChainShellJSON(ctx, bytes.NewReader(downgraded), trimmed.MappedChainShellNativeOptions{}); !errors.Is(err, geom.ErrConvergence) {
		t.Fatal("native v8 must preserve its single-original-cell condition", err)
	}
	return canonical.Bytes()
}

func verifyLocalCellMaterial(t *testing.T, ctx context.Context, native trimmed.MappedChainRectangularSolid) {
	t.Helper()
	s := native.Solid
	if len(native.ChainEdges) != 7 || len(s.Faces()) != 7 || len(s.Edges()) != 27 || len(s.Vertices()) != 22 {
		t.Fatal("both original roofs and the two split edges require their original closed topology")
	}
	proof, err := s.ValidateEdgeAgreement(ctx, edgecheck.Options{})
	if err != nil || !proof.Complete || len(proof.Uses) != 54 {
		t.Fatal("both owners must agree continuously on each of the original and repaired edges", err)
	}
	mass, err := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
	if err != nil || math.Abs(mass.Volume-mappedChainLocalCellVolume) > 1e-8 || math.Abs(mass.Area-mappedChainLocalCellArea) > 1e-8 {
		t.Fatal("native material must match the independently authored two-cell fixture", mass, err)
	}
}

func TestOCCTMappedChainRectangularLocalSupportKnotCells(t *testing.T) {
	raw, err := os.ReadFile(localCellMappedChainPath)
	if err != nil {
		t.Fatal(err)
	}
	verifyLocalCellMappedChain(t, context.Background(), raw)
}

func TestOCCTMappedChainRectangularLocalSupportKnotCellsClosedDisplay(t *testing.T) {
	ctx := context.Background()
	raw, err := os.ReadFile(localCellMappedChainPath)
	if err != nil {
		t.Fatal(err)
	}
	m, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(raw), Limits{}, localCellMappedChainOptions())
	if err != nil {
		t.Fatal(err)
	}
	verifyMappedChainCutDisplay(t, ctx, m)
}

func TestOCCTMappedChainRectangularLocalSupportKnotCellsRejectsInvalidSource(t *testing.T) {
	ctx := context.Background()
	raw, err := os.ReadFile(localCellMappedChainPath)
	if err != nil {
		t.Fatal(err)
	}
	o := localCellMappedChainOptions()
	m, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(raw), Limits{}, o)
	if err != nil {
		t.Fatal(err)
	}
	v8 := o
	v8.AllowMultipleSupportKnotCells, v8.AllowUpToSixSupportKnots = false, true
	if bad, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(raw), Limits{}, v8); err == nil || len(bad.Crossings) != 0 {
		t.Fatal("v8 only admits the previously proved one-original-cell graph", err)
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
		{"shifted second stored model knot", "stored model knot", geom.ErrConvergence, func(f *File) {
			_, k, _, err := mappedChainStepSplineParts(f, m.Chain[1].ModelCurveID)
			if err != nil {
				t.Fatal(err)
			}
			k[1].Items[1].Text = "11.875"
		}},
		{"second owner stop no longer representable", "not representable", geom.ErrUnrepresentable, func(f *File) {
			_, k, _, err := mappedChainStepSplineParts(f, m.Chain[1].Owners[north].BasisID)
			if err != nil {
				t.Fatal(err)
			}
			k[1].Items[1].Text = "716."
		}},
		{"both owner events moved into the first cell", "one owner-side crossing per original edge", geom.ErrConvergence, func(f *File) {
			moveLocalSouthCrease(t, f, m.FaceIDs[1-north], 369.0/2048)
		}},
		{"owner-only knot moved to common cell endpoint", "complete 2-cut repair", geom.ErrConvergence, func(f *File) {
			moveLocalSouthCrease(t, f, m.FaceIDs[1-north], .25)
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
				t.Fatal("a broken original support stream must reject before publishing any repaired edge", err)
			}
		})
	}
	t.Run("native recipe loses one original interior event", func(t *testing.T) {
		f, err := Parse(bytes.NewReader(raw), Limits{})
		if err != nil {
			t.Fatal(err)
		}
		south := 1 - north
		moveLocalSouthCrease(t, f, m.FaceIDs[south], .25)
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
			t.Fatal("the v9 recipe must contain the full original south support")
		}
		mutated := bytes.Replace(recipe.Bytes(), before, after, 1)
		bad, err := trimmed.ReadMappedSplineChainShellJSON(ctx, bytes.NewReader(mutated), trimmed.MappedChainShellNativeOptions{})
		if !errors.Is(err, geom.ErrConvergence) || !strings.Contains(err.Error(), "one interior event per original support knot") || bad.Topology().Faces != 0 {
			t.Fatal("each original support knot must be accounted for during source-only native reload", err)
		}
	})
}

func moveLocalSouthCrease(t *testing.T, f *File, faceID int64, x float64) {
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
	// Both values exercised here remain between the two stored shared knots.
	// The change is entirely off the seam at y=1/4.
	roof.Args[11].Items[2].Text, roof.Args[11].Items[3].Text = decimal(x), decimal(99.0/256)
	for i, row := range roof.Args[3].Items {
		for j, pole := range row.Items {
			xp, y := [5]float64{0, 29.0 / 256, x, 99.0 / 256, 1}[j], float64(i)
			z := xp*y + .125*(max(0, xp-29.0/256)+max(0, xp-99.0/256)) + .125*(y-.25)*max(0, xp-x)
			p := f.Entities[pole.Ref].Records[0].Args[1].Items
			p[0].Text, p[2].Text = decimal(xp), decimal(z)
		}
	}
}

func TestOCCTMappedChainRectangularLocalSupportKnotCellsLive(t *testing.T) {
	author, reader := os.Getenv("PLAINSOLID_OCCT_MAPPED_CHAIN_LOCAL_AUTHOR"), os.Getenv("PLAINSOLID_OCCT_MAPPED_CHAIN_STEP")
	if author == "" || reader == "" {
		t.Skip("optional pinned OCCT source author and STEP consumer")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	path := filepath.Join(t.TempDir(), "occt-authored-two-support-cells.step")
	if out, err := exec.CommandContext(ctx, author, "write-two-support-cells", path).CombinedOutput(); err != nil {
		t.Fatalf("pinned OCCT author failed: %v\n%s", err, out)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	canonical := verifyLocalCellMappedChain(t, ctx, raw)
	verifyPinnedMappedChainTransfer(t, ctx, reader, raw, canonical, mappedChainLocalCellVolume, mappedChainLocalCellArea)
}
