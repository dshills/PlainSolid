package step

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

const mixedBasisUnsplitMappedChainPath = "testdata/occt_mapped_chain_three_unsplit_mixed_bases.step"

func unsplitMappedChainMixedBasisOptions() MappedChainShellSTEPOptions {
	return MappedChainShellSTEPOptions{AllowMultiSpanRoofs: true, AllowTwoInteriorRoofKnots: true,
		AllowUnsplitSupportKnot: true, AllowThreeSupportKnots: true, AllowMixedOwnerBases: true}
}

func verifyUnsplitMixedBasisMappedChain(t *testing.T, ctx context.Context, raw []byte) {
	t.Helper()
	o := unsplitMappedChainMixedBasisOptions()
	m, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(raw), Limits{}, o)
	if err != nil {
		t.Fatal(err)
	}
	mode := m.Shell.IndependentOwnerBases()
	if mode[0] == mode[1] || len(m.Chain) != 3 || len(m.Crossings) != 3 || m.Crossing != nil || m.OriginalSTEP() != string(raw) {
		t.Fatal("the mixed owner bases must retain all three original edges and their three support crossings")
	}
	independent, common := 0, 1
	if !mode[independent] {
		independent, common = common, independent
	}
	verifyThreeKnotMaterial(t, ctx, m)
	f, err := Parse(bytes.NewReader(raw), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	parent := m.Chain[0]
	model, err := readMappedChainStepSpline(f, parent.ModelCurveID, 3, 16, 33, true)
	if err != nil || model.degree != 3 || len(model.poles) != 7 || len(model.knots) != 11 ||
		model.knots[0].Sign() != 0 || model.knots[10].Cmp(big.NewRat(4, 1)) != 0 {
		t.Fatal("the original unsplit model edge must retain its two exact cubic pieces and full source domain", err)
	}
	for _, knot := range model.knots[4:7] {
		if knot.Cmp(big.NewRat(2, 1)) != 0 {
			t.Fatal("only the shared material kink is an original model spline knot")
		}
	}
	commonRaw := []int64{478, 443, 387, 283, 23, 17, 10}
	independentRaw := [][2]int64{{2, 93}, {93, 197}, {197, 317}, {317, 457}, {700, 706}, {850, 862}}
	vertices, edges := m.Native.Solid.Vertices(), m.Native.Solid.Edges()
	points := [3]geom.Vec3{{X: 107.0 / 2048, Y: .25, Z: 107.0 / 8192},
		{X: 29.0 / 256, Y: .25, Z: 29.0 / 1024}, {X: 369.0 / 2048, Y: .25, Z: 875.0 / 16384}}
	for i, crossing := range m.Crossings {
		if crossing.EdgeID != parent.EdgeID || crossing.ModelStation.Cmp(big.NewRat(int64(i+1), 16)) != 0 ||
			crossing.SourceModelParameter.Cmp(big.NewRat(int64(i+1), 1)) != 0 ||
			crossing.RawParameters[independent].Cmp(big.NewRat(independentRaw[i][1], 1)) != 0 ||
			crossing.RawParameters[common].Cmp(big.NewRat(commonRaw[i+1], 1)) != 0 ||
			crossing.OwnersChangingCells[independent] != (i <= 1) || crossing.OwnersChangingCells[common] != (i >= 1) {
			t.Fatal("an original model or raw owner event changed during paired repair", i, crossing)
		}
		if crossing.NativeEdges != [2]int{m.Native.ChainEdges[i], m.Native.ChainEdges[i+1]} {
			t.Fatal("the original unsplit edge must own both adjacent native edges", i)
		}
		a, b := edges[crossing.NativeEdges[0]], edges[crossing.NativeEdges[1]]
		if a.End != b.Start || vertices[a.End] != points[i] {
			t.Fatal("the native repair requires one shared exact model vertex", i)
		}
	}
	var bases [2][]*geom.BSpline
	seen := make(map[int64]bool)
	for i, cell := range m.Chain {
		for owner, use := range cell.Owners {
			if (use.TrimmedCurveID == 0) != mode[owner] || mode[owner] && seen[use.BasisID] ||
				!mode[owner] && use.BasisID != parent.Owners[owner].BasisID {
				t.Fatal("mixed ownership must keep one common basis and distinct whole original coedge bases", i, owner)
			}
			seen[use.BasisID] = true
			spline, err := readMappedChainStepSpline(f, use.BasisID, 2, 4, 64, true)
			if err != nil {
				t.Fatal(err)
			}
			basis, err := spline.ownerBasis()
			if err != nil {
				t.Fatal(err)
			}
			bases[owner] = append(bases[owner], basis)
		}
	}
	if !slices.Equal(bases[independent][1].Weights(), []float64{8, 12, 18, 27}) {
		t.Fatal("the authored weight ratios and scale must survive without rounding")
	}
	var recipe, replay bytes.Buffer
	if err := m.Shell.WriteJSON(ctx, &recipe, trimmed.MappedChainShellNativeOptions{}); err != nil {
		t.Fatal(err)
	}
	var record struct {
		Boundary struct {
			Version int
			Owners  []struct {
				Source  json.RawMessage
				Sources []json.RawMessage
			}
		}
	}
	if err := json.Unmarshal(recipe.Bytes(), &record); err != nil || record.Boundary.Version != 7 || len(record.Boundary.Owners) != 2 ||
		len(record.Boundary.Owners[independent].Sources) != 3 || len(record.Boundary.Owners[common].Source) == 0 ||
		!bytes.Contains(record.Boundary.Owners[independent].Sources[1], []byte(`"Kind":"bspline-exact-weights"`)) {
		t.Fatal("v7 native recipe must retain the original unsplit owner organization", err)
	}
	shell, err := trimmed.ReadMappedSplineChainShellJSON(ctx, bytes.NewReader(recipe.Bytes()), trimmed.MappedChainShellNativeOptions{})
	if err != nil || shell.IndependentOwnerBases() != mode {
		t.Fatal("native reload changed the original mixed owner basis modes", err)
	}
	if err := shell.WriteJSON(ctx, &replay, trimmed.MappedChainShellNativeOptions{}); err != nil || !bytes.Equal(recipe.Bytes(), replay.Bytes()) {
		t.Fatal("the source-only recipe must replay byte-identically", err)
	}
	native, err := shell.BuildRectangularSolid(ctx, 1e-9, geom.SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	reopened := m
	reopened.Native = native
	verifyThreeKnotMaterial(t, ctx, reopened)
	original, loaded := m.Native.Solid.Faces(), native.Solid.Faces()
	for cell, want := range independentRaw {
		ancestry := max(cell-3, 0)
		for owner := range mode {
			starts, ends := commonRaw[cell], commonRaw[cell+1]
			if owner == independent {
				starts, ends = want[0], want[1]
			}
			a, b := original[owner].Outer[cell].Curve.(trimmed.MappedSplineUV), loaded[owner].Outer[cell].Curve.(trimmed.MappedSplineUV)
			if a.Source().Range != b.Source().Range || a.Source().Reversed != b.Source().Reversed || a.ParameterWeight().Cmp(b.ParameterWeight()) != 0 ||
				!sameMixedMappedChainBasis(bases[owner][ancestry], a.Source().Curve) || !sameMixedMappedChainBasis(a.Source().Curve, b.Source().Curve) {
				t.Fatal("the rebuilt restriction must use the exact original parent basis and map", owner, cell)
			}
			for j, endpoint := range []int64{starts, ends} {
				q, err := b.AuthoredParameter(big.NewRat(int64(j), 1))
				if err != nil || q.Cmp(big.NewRat(endpoint, 1)) != 0 {
					t.Fatal("lost an exact one-sided raw endpoint", owner, cell, j, err)
				}
			}
		}
	}
	// At the original joint the incoming and outgoing independent bases assign
	// different raw parameters to the same topological vertex.
	a, b := edges[m.Native.ChainEdges[3]], edges[m.Native.ChainEdges[4]]
	if a.End != b.Start || independentRaw[3][1] == independentRaw[4][0] {
		t.Fatal("independent one-sided parameters must keep their common original model vertex")
	}
}

func TestOCCTMappedChainRectangularUnsplitThreeSupportKnotsMixedBases(t *testing.T) {
	raw, err := os.ReadFile(mixedBasisUnsplitMappedChainPath)
	if err != nil {
		t.Fatal(err)
	}
	verifyUnsplitMixedBasisMappedChain(t, context.Background(), raw)
}

func TestOCCTMappedChainRectangularUnsplitThreeSupportKnotsMixedBasesRejectsInvalidSource(t *testing.T) {
	ctx := context.Background()
	raw, err := os.ReadFile(mixedBasisUnsplitMappedChainPath)
	if err != nil {
		t.Fatal(err)
	}
	o := unsplitMappedChainMixedBasisOptions()
	m, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(raw), Limits{}, o)
	if err != nil {
		t.Fatal(err)
	}
	for _, flag := range []func(*MappedChainShellSTEPOptions){
		func(x *MappedChainShellSTEPOptions) { x.AllowMixedOwnerBases = false },
		func(x *MappedChainShellSTEPOptions) { x.AllowThreeSupportKnots = false },
	} {
		disabled := o
		flag(&disabled)
		bad, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(raw), Limits{}, disabled)
		if err == nil || len(bad.Crossings) != 0 || len(bad.Native.Solid.Edges()) != 0 {
			t.Fatal("both unresolved source proof gates are required", err)
		}
	}
	independent := 0
	if !m.Shell.IndependentOwnerBases()[0] {
		independent = 1
	}
	for _, test := range []struct {
		name, message string
		id            int64
		station       string
		want          error
	}{
		{"stored material knot moved", "stored model knot", m.Chain[0].ModelCurveID, "1.875", geom.ErrConvergence},
		{"same trace with unrepresentable independent raw root", "not representable", m.Chain[0].Owners[independent].BasisID, "458.", geom.ErrUnrepresentable},
	} {
		t.Run(test.name, func(t *testing.T) {
			f, err := Parse(bytes.NewReader(raw), Limits{})
			if err != nil {
				t.Fatal(err)
			}
			_, knots, _, err := mappedChainStepSplineParts(f, test.id)
			if err != nil {
				t.Fatal(err)
			}
			knots[1].Items[1].Text = test.station
			bad, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(serializeWrappedProofFile(f)), Limits{}, o)
			if !errors.Is(err, test.want) || !strings.Contains(err.Error(), test.message) || len(bad.Crossings) != 0 || len(bad.Native.Solid.Edges()) != 0 {
				t.Fatal("the exact source edge and both raw owner restrictions must agree before repair", err)
			}
		})
	}
	t.Run("the two owner-only events become coincident on identical seam geometry", func(t *testing.T) {
		f, err := Parse(bytes.NewReader(raw), Limits{})
		if err != nil {
			t.Fatal(err)
		}
		common := f.Entities[m.FaceIDs[1-independent]].Records[0]
		roof := f.Entities[common.Args[2].Ref].Records[0]
		// Only the companion crease off the seam moves. On y=1/4, both
		// published model pieces and the complete original pcurves are exact.
		roof.Args[11].Items[1].Text, roof.Args[11].Items[2].Text = "0.05224609375", "0.11328125"
		decimal := func(x float64) string {
			value, err := mappedChainStepDecimal(mappedChainStepRat(x))
			if err != nil {
				t.Fatal(err)
			}
			return value
		}
		for i, row := range roof.Args[3].Items {
			for j, pole := range row.Items {
				x := [4]float64{0, 107.0 / 2048, 29.0 / 256, 1}[j]
				y := float64(i)
				z := x*y + .125*max(0, x-29.0/256) + .125*(y-.25)*max(0, x-107.0/2048)
				p := f.Entities[pole.Ref].Records[0].Args[1].Items
				p[0].Text, p[2].Text = decimal(x), decimal(z)
			}
		}
		bad, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(serializeWrappedProofFile(f)), Limits{}, o)
		if !errors.Is(err, geom.ErrConvergence) || !strings.Contains(err.Error(), "separate inferred support stations must be distinct") ||
			len(bad.Crossings) != 0 || len(bad.Native.Solid.Edges()) != 0 {
			t.Fatal("two independently inferred coincident events cannot qualify as distinct crossings", err)
		}
	})
}

func TestOCCTMappedChainRectangularUnsplitThreeSupportKnotsMixedBasesLive(t *testing.T) {
	author, reader := os.Getenv("PLAINSOLID_OCCT_MAPPED_CHAIN_MIXED_BASIS_AUTHOR"), os.Getenv("PLAINSOLID_OCCT_MAPPED_CHAIN_STEP")
	if author == "" || reader == "" {
		t.Skip("optional pinned OCCT mixed-basis author and STEP consumer")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	path := filepath.Join(t.TempDir(), "occt-authored-unsplit-mixed-bases.step")
	if out, err := exec.CommandContext(ctx, author, "write-three-knots-mixed-unsplit", path).CombinedOutput(); err != nil {
		t.Fatalf("pinned OCCT author failed: %v\n%s", err, out)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	verifyUnsplitMixedBasisMappedChain(t, ctx, raw)
	canonical := canonicalUnsplitMixedBasisSTEP(t, ctx, raw)
	verifyIndependentThreeKnotOCCTTransfer(t, ctx, reader, raw, canonical)
}
