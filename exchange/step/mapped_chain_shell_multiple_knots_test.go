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
	"strings"
	"testing"
	"time"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

type mappedChainMultipleKnotFixture struct {
	name, path, author string
	modelStations      []int64
	volume, area       float64
}

var mappedChainMultipleKnotFixtures = []mappedChainMultipleKnotFixture{
	{"five", "testdata/occt_mapped_chain_five_unsplit_mixed_bases.step", "write-five-knots-mixed-unsplit",
		[]int64{1, 2, 3, 4, 6}, 8256786393.0 / 8589934592, 6.359242666249162},
	{"six", "testdata/occt_mapped_chain_six_unsplit_mixed_bases.step", "write-six-knots-mixed-unsplit",
		[]int64{1, 2, 3, 4, 5, 6}, 65858599023.0 / 68719476736, 6.342952742830787},
}

func multipleKnotMappedChainOptions() MappedChainShellSTEPOptions {
	return MappedChainShellSTEPOptions{AllowMultiSpanRoofs: true, AllowTwoInteriorRoofKnots: true,
		AllowUnsplitSupportKnot: true, AllowUpToSixSupportKnots: true, AllowMixedOwnerBases: true}
}

func verifyMultipleKnotMappedChain(t *testing.T, ctx context.Context, tc mappedChainMultipleKnotFixture, raw []byte) []byte {
	t.Helper()
	o := multipleKnotMappedChainOptions()
	m, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(raw), Limits{}, o)
	if err != nil {
		t.Fatal(err)
	}
	n := len(tc.modelStations)
	bases := m.Shell.IndependentOwnerBases()
	if bases[0] == bases[1] || len(m.Chain) != 3 || len(m.Crossings) != n || m.Crossing != nil {
		t.Fatal("all original mixed coedges and the complete common partition must survive")
	}
	north := 0
	if !bases[0] {
		north = 1
	}
	verifyMultipleKnotMaterial(t, ctx, tc, m.Native)
	edges, vertices, faces := m.Native.Solid.Edges(), m.Native.Solid.Vertices(), m.Native.Solid.Faces()
	xNumerators := []int64{407, 856, 1341, 1856, 2395, 2952}
	for i, rawStation := range tc.modelStations {
		cut := m.Crossings[i]
		if cut.EdgeID != m.Chain[0].EdgeID || cut.SourceModelParameter.Cmp(big.NewRat(rawStation, 1)) != 0 ||
			cut.ModelStation.Cmp(big.NewRat(rawStation, 32)) != 0 || cut.StoredModelKnot != (rawStation == 4) ||
			cut.OwnersChangingCells[north] != (rawStation <= 4) || cut.OwnersChangingCells[1-north] != (rawStation >= 4) ||
			cut.RawParameters[north].Cmp(multipleKnotNorthRaw(rawStation)) != 0 ||
			cut.RawParameters[1-north].Cmp(big.NewRat(48-4*rawStation, 1)) != 0 ||
			cut.NativeEdges != ([2]int{m.Native.ChainEdges[i], m.Native.ChainEdges[i+1]}) {
			t.Fatal("lost an exact stored or inferred owner event", i, cut)
		}
		x := float64(xNumerators[rawStation-1]) / 16384
		point := geom.Vec3{X: x, Y: .25, Z: x/4 + max(0, x-29.0/256)/8}
		a, b := edges[cut.NativeEdges[0]], edges[cut.NativeEdges[1]]
		if a.End != b.Start || vertices[a.End] != point {
			t.Fatal("original owner restrictions must share the same exact model vertex", i, vertices[a.End])
		}
	}
	file, err := Parse(bytes.NewReader(raw), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	model, err := readMappedChainStepSpline(file, m.Chain[0].ModelCurveID, 3, 16, 33, true)
	if err != nil || model.degree != 3 || len(model.poles) != 7 || model.knots[0].Sign() != 0 || model.knots[10].Cmp(big.NewRat(8, 1)) != 0 {
		t.Fatal("the original unsplit model must keep only its two cubic branches on [0,8]", err)
	}
	for _, knot := range model.knots[4:7] {
		if knot.Cmp(big.NewRat(4, 1)) != 0 {
			t.Fatal("the other exact support events must stay inferred from the original support charts")
		}
	}
	for owner, u := range m.Chain[0].Owners {
		original, err := originalMappedChainStepOwner(file, u, true)
		if err != nil {
			t.Fatal(err)
		}
		if owner == north && !slices.Equal(original.Curve.Weights(), []float64{1, 3, 9, 27}) {
			t.Fatal("the authored rational north coefficients must survive exactly")
		}
		stations := append(append([]int64{0}, tc.modelStations...), 8)
		for i := 0; i < len(stations)-1; i++ {
			q := faces[owner].Outer[i].Curve.(trimmed.MappedSplineUV)
			if !sameMixedMappedChainBasis(q.Source().Curve, original.Curve) {
				t.Fatal("all repaired native edges must restrict their own original basis", owner, i)
			}
			for end, station := range stations[i : i+2] {
				want := big.NewRat(48-4*station, 1)
				if owner == north {
					want = multipleKnotNorthRaw(station)
				}
				raw, err := q.AuthoredParameter(big.NewRat(int64(end), 1))
				if err != nil || want.Cmp(raw) != 0 {
					t.Fatal("the repaired restrictions lost an exact raw parameter", owner, i, end, err)
				}
			}
		}
	}
	verifyMappedRoofSlopeJump(t, ctx, faces[north].Surface.(*geom.BSplineSurface), 0, 1341.0/16384, big.NewRat(1, 2), big.NewRat(1, 16))
	if n == 6 {
		verifyMappedRoofSlopeJump(t, ctx, faces[1-north].Surface.(*geom.BSplineSurface), 1, 2395.0/16384, big.NewRat(1, 8), big.NewRat(-1, 32))
	}
	var native, canonical, replay bytes.Buffer
	if err := m.Shell.WriteJSON(ctx, &native, trimmed.MappedChainShellNativeOptions{}); err != nil {
		t.Fatal(err)
	}
	var recipe struct {
		Boundary struct {
			Version int
			Owners  []struct{ Weights []string }
		}
	}
	if err := json.Unmarshal(native.Bytes(), &recipe); err != nil || recipe.Boundary.Version != 8 || len(recipe.Boundary.Owners) != 2 ||
		len(recipe.Boundary.Owners[north].Weights) != 3 || len(recipe.Boundary.Owners[1-north].Weights) != 3 ||
		recipe.Boundary.Owners[north].Weights[0] != "1/3" || recipe.Boundary.Owners[1-north].Weights[0] != "1" {
		t.Fatal("one v8 format must preserve both original owner maps at either count", err)
	}
	loaded, err := trimmed.ReadMappedSplineChainShellJSON(ctx, bytes.NewReader(native.Bytes()), trimmed.MappedChainShellNativeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	reconstructed, err := loaded.BuildRectangularSolid(ctx, 1e-9, o.Geometry)
	if err != nil {
		t.Fatal(err)
	}
	verifyMultipleKnotMaterial(t, ctx, tc, reconstructed)
	if err := WriteMappedChainRectangularOriginalShell(ctx, &canonical, m, Limits{}, o); err != nil {
		t.Fatal(err)
	}
	strict, err := ReadMappedChainRectangularShell(ctx, bytes.NewReader(canonical.Bytes()), Limits{}, o)
	if err != nil || len(strict.Chain) != 3 || len(strict.Crossings) != n || strict.Shell.IndependentOwnerBases() != bases {
		t.Fatal("strict canonical reopen must retain and re-prove all original model and owner edges", err)
	}
	verifyOriginalMappedChainSTEPGraph(t, m, strict)
	if err := strict.Shell.WriteJSON(ctx, &replay, trimmed.MappedChainShellNativeOptions{}); err != nil || !bytes.Equal(native.Bytes(), replay.Bytes()) {
		t.Fatal("strict canonical reopen must recover the same source-only native v8 record", err)
	}
	replay.Reset()
	if err := WriteMappedChainRectangularOriginalShell(ctx, &replay, strict, Limits{}, o); err != nil || !bytes.Equal(canonical.Bytes(), replay.Bytes()) {
		t.Fatal("the canonical original graph must replay byte-for-byte", err)
	}
	return canonical.Bytes()
}

func multipleKnotNorthRaw(station int64) *big.Rat {
	return new(big.Rat).Add(big.NewRat(2, 1), big.NewRat(4620*station, 24-2*station))
}

func verifyMultipleKnotMaterial(t *testing.T, ctx context.Context, tc mappedChainMultipleKnotFixture, native trimmed.MappedChainRectangularSolid) {
	t.Helper()
	solid := native.Solid
	n := len(tc.modelStations)
	edges := 18 + 2*n
	if len(native.ChainEdges) != 3+n || len(solid.Edges()) != edges || len(solid.Faces()) != 7 || len(solid.Vertices()) != edges-5 {
		t.Fatal("the new roof creases and repaired edges need their original closed topology", n)
	}
	proof, err := solid.ValidateEdgeAgreement(ctx, edgecheck.Options{})
	if err != nil || !proof.Complete || len(proof.Uses) != 2*edges {
		t.Fatal("every coedge must agree over its complete original model restriction", err)
	}
	mass, err := solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
	if err != nil || math.Abs(mass.Volume-tc.volume) > 1e-8 || math.Abs(mass.Area-tc.area) > 1e-8 {
		t.Fatal("native material must match the independent roof fixture", tc.name, mass, err)
	}
}

func TestOCCTMappedChainRectangularMultipleSupportKnots(t *testing.T) {
	for _, tc := range mappedChainMultipleKnotFixtures {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := os.ReadFile(tc.path)
			if err != nil {
				t.Fatal(err)
			}
			verifyMultipleKnotMappedChain(t, context.Background(), tc, raw)
		})
	}
}

func TestOCCTMappedChainRectangularMultipleSupportKnotsClosedDisplay(t *testing.T) {
	ctx := context.Background()
	raw, err := os.ReadFile(mappedChainMultipleKnotFixtures[1].path)
	if err != nil {
		t.Fatal(err)
	}
	m, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(raw), Limits{}, multipleKnotMappedChainOptions())
	if err != nil {
		t.Fatal(err)
	}
	verifyMappedChainCutDisplay(t, ctx, m)
}

func TestOCCTMappedChainRectangularMultipleSupportKnotsRejectsOverlap(t *testing.T) {
	ctx := context.Background()
	raw, err := os.ReadFile(mappedChainMultipleKnotFixtures[1].path)
	if err != nil {
		t.Fatal(err)
	}
	o := multipleKnotMappedChainOptions()
	m, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(raw), Limits{}, o)
	if err != nil {
		t.Fatal(err)
	}
	old := o
	old.AllowUpToSixSupportKnots, old.AllowFourSupportKnots = false, true
	if rejected, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(raw), Limits{}, old); err == nil || len(rejected.Crossings) != 0 {
		t.Fatal("the old four-cut gate must retain its original scope", err)
	}
	f, err := Parse(bytes.NewReader(raw), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	south := 0
	if m.Shell.IndependentOwnerBases()[0] {
		south = 1
	}
	face := f.Entities[m.FaceIDs[south]].Records[0]
	roof := f.Entities[face.Args[2].Ref].Records[0]
	const coincident = 1341.0 / 16384
	roof.Args[11].Items[1].Text, roof.Args[11].Items[2].Text = "0.08184814453125", "0.11328125"
	for i, row := range roof.Args[3].Items {
		for j, pole := range row.Items {
			x, y := [5]float64{0, coincident, 29.0 / 256, 369.0 / 2048, 1}[j], float64(i)
			z := x*y + .125*max(0, x-29.0/256) + .125*(y-.25)*max(0, x-369.0/2048) + .25*(y-.25)*max(0, x-coincident)
			p := f.Entities[pole.Ref].Records[0].Args[1].Items
			for axis, value := range []float64{x, y, z} {
				p[axis].Text, err = mappedChainStepDecimal(mappedChainStepRat(value))
				if err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	bad, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(serializeWrappedProofFile(f)), Limits{}, o)
	if !errors.Is(err, geom.ErrConvergence) || !strings.Contains(err.Error(), "separate inferred support stations must be distinct") ||
		len(bad.Crossings) != 0 || len(bad.Native.ChainEdges) != 0 {
		t.Fatal("two owner-only knots coinciding on an unchanged seam must reject atomically", err)
	}
}

func TestOCCTMappedChainRectangularMultipleSupportKnotsNativeBound(t *testing.T) {
	ctx := context.Background()
	raw, err := os.ReadFile(mappedChainMultipleKnotFixtures[1].path)
	if err != nil {
		t.Fatal(err)
	}
	m, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(raw), Limits{}, multipleKnotMappedChainOptions())
	if err != nil {
		t.Fatal(err)
	}
	south := 0
	if m.Shell.IndependentOwnerBases()[0] {
		south = 1
	}
	// Insert another exact south chart knot at model station 7/32 without
	// changing its bilinear geometry. The raw north and south stops there
	// would be 3236 and 20, but both roofs would then have four knots.
	xs := []float64{0, 29.0 / 256, 2395.0 / 16384, 369.0 / 2048, 3521.0 / 16384, 1}
	var poles []geom.Vec3
	for _, y := range []float64{0, 1} {
		for _, x := range xs {
			z := x*y + .125*max(0, x-29.0/256) + .125*(y-.25)*max(0, x-369.0/2048) + .25*(y-.25)*max(0, x-2395.0/16384)
			poles = append(poles, geom.Vec3{X: x, Y: y, Z: z})
		}
	}
	knots := append(append([]float64{0}, xs...), 1)
	chart, err := geom.NewBSplineSurface(1, 1, 2, len(xs), poles, []float64{0, 0, 1, 1}, knots, nil)
	if err != nil {
		t.Fatal(err)
	}
	var recipe bytes.Buffer
	if err := m.Shell.WriteJSON(ctx, &recipe, trimmed.MappedChainShellNativeOptions{}); err != nil {
		t.Fatal(err)
	}
	var shell, boundary map[string]json.RawMessage
	var owners []map[string]json.RawMessage
	if err := json.Unmarshal(recipe.Bytes(), &shell); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(shell["Boundary"], &boundary); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(boundary["Owners"], &owners); err != nil {
		t.Fatal(err)
	}
	owners[south]["Surface"], err = parametric.EncodeSurface(chart)
	if err != nil {
		t.Fatal(err)
	}
	boundary["Owners"], err = json.Marshal(owners)
	if err != nil {
		t.Fatal(err)
	}
	shell["Boundary"], err = json.Marshal(boundary)
	if err != nil {
		t.Fatal(err)
	}
	raw, err = json.Marshal(shell)
	if err != nil {
		t.Fatal(err)
	}
	bad, err := trimmed.ReadMappedSplineChainShellJSON(ctx, bytes.NewReader(raw), trimmed.MappedChainShellNativeOptions{})
	if !errors.Is(err, geom.ErrConvergence) || bad.Topology().Faces != 0 || !strings.Contains(err.Error(), "crossings disagree in model stations") {
		t.Fatal("an exact seventh event lies outside the native v8 bound", err)
	}
}

func TestOCCTMappedChainRectangularMultipleSupportKnotsLive(t *testing.T) {
	author, reader := os.Getenv("PLAINSOLID_OCCT_MAPPED_CHAIN_MULTIPLE_AUTHOR"), os.Getenv("PLAINSOLID_OCCT_MAPPED_CHAIN_STEP")
	if author == "" || reader == "" {
		t.Skip("optional pinned OCCT multiple-knots author and STEP consumer")
	}
	for _, tc := range mappedChainMultipleKnotFixtures {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			path := filepath.Join(t.TempDir(), "occt-authored-multiple-support-crossings.step")
			if out, err := exec.CommandContext(ctx, author, tc.author, path).CombinedOutput(); err != nil {
				t.Fatalf("pinned OCCT author failed: %v\n%s", err, out)
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			canonical := verifyMultipleKnotMappedChain(t, ctx, tc, raw)
			verifyPinnedMappedChainTransfer(t, ctx, reader, raw, canonical, tc.volume, tc.area)
		})
	}
}
