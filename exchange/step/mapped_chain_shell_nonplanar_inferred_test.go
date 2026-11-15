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
	"slices"
	"testing"
	"time"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

const nonplanarInferredMappedChainPath = "testdata/occt_mapped_chain_nonplanar_inferred_mixed.step"
const nonplanarRationalInferredMappedChainPath = "testdata/occt_mapped_chain_nonplanar_rational_inferred_mixed.step"
const nonplanarIndependentOwnerClocksPath = "testdata/occt_mapped_chain_nonplanar_independent_owner_clocks.step"

// Integrate the common graph xy and both bicubic perturbations over their
// original roof domains split by y = 1/4 + 3u(1-u)/4, with u = 4x/3.
const nonplanarInferredMappedChainVolume = 10127995838792649.0 / 15762598695796736
const nonplanarInferredMappedChainArea = 4.649146934610298

// Pinned OCCT evaluates the original and canonical rational STEP trims at
// this area. The exact material has the same graph as the polynomial source.
const nonplanarRationalInferredOCCTArea = 4.649146910413742
const nonplanarIndependentOwnerClocksOCCTArea = 4.649146925043418

type nonplanarMappedChainVariant uint8

const (
	nonplanarPolynomialClock nonplanarMappedChainVariant = iota
	nonplanarRationalClock
	nonplanarIndependentOwnerClocks
	nonplanarRationalRoofs
)

func verifyNonplanarInferredMappedChain(t *testing.T, ctx context.Context, raw []byte, variant nonplanarMappedChainVariant) []byte {
	t.Helper()
	rationalModel := variant != nonplanarPolynomialClock
	o := inferredMixedMappedChainOptions()
	m, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(raw), Limits{}, o)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Chain) != 3 || len(m.Crossings) != 3 || !allMappedChainCrossingsOnOneEdge(m.Crossings) || len(m.Planes) != 5 {
		t.Fatal("the original nonplanar model Bézier must have all three support events and five planar closings")
	}
	bases := m.Shell.IndependentOwnerBases()
	if bases[0] == bases[1] {
		t.Fatal("one whole owner basis and three independent bases must remain distinct")
	}
	north := 0
	if !bases[0] {
		north = 1
	}
	south := 1 - north
	verifyNonplanarInferredMappedChainMaterial(t, ctx, m.Shell, m.Native, variant)
	edges, vertices, faces := m.Native.Solid.Edges(), m.Native.Solid.Vertices(), m.Native.Solid.Faces()
	stations := [3]int64{2, 4, 6}
	modelRaw := [3]*big.Rat{big.NewRat(2, 1), big.NewRat(4, 1), big.NewRat(6, 1)}
	if rationalModel {
		modelRaw = [3]*big.Rat{big.NewRat(8, 7), big.NewRat(8, 3), big.NewRat(24, 5)}
	}
	var stops [2][5]int64
	stops[north], stops[south] = [5]int64{2, 17, 37, 65, 107}, [5]int64{48, 40, 32, 24, 16}
	var clocks [2]*big.Rat
	clocks[north], clocks[south] = big.NewRat(1, 2), big.NewRat(1, 1)
	switch variant {
	case nonplanarRationalClock:
		clocks[north], clocks[south] = big.NewRat(1, 1), big.NewRat(2, 1)
	case nonplanarIndependentOwnerClocks, nonplanarRationalRoofs:
		clocks[north], clocks[south] = big.NewRat(4, 3), big.NewRat(3, 1)
		stops[north], stops[south] = [5]int64{2, 32, 68, 112, 167}, [5]int64{181, 126, 82, 46, 16}
	}
	for i, cut := range m.Crossings {
		p := float64(stations[i]) / 32
		x, y := .75*p, .25+.75*p*(1-p)
		modelStation := new(big.Rat).Quo(modelRaw[i], big.NewRat(32, 1))
		if cut.EdgeID != m.Chain[0].EdgeID || cut.StoredModelKnot ||
			cut.SourceModelParameter.Cmp(modelRaw[i]) != 0 || cut.ModelStation.Cmp(modelStation) != 0 ||
			cut.RawParameters[north].Cmp(big.NewRat(stops[north][i+1], 1)) != 0 ||
			cut.RawParameters[south].Cmp(big.NewRat(stops[south][i+1], 1)) != 0 ||
			cut.OwnersChangingCells[north] != (i != 1) || cut.OwnersChangingCells[south] != (i != 2) ||
			cut.NativeEdges != [2]int{m.Native.ChainEdges[i], m.Native.ChainEdges[i+1]} {
			t.Fatal("the joint and owner-only events must retain their exact raw and model ownership", i, cut)
		}
		if rationalModel {
			if _, exact := cut.SourceModelParameter.Float64(); exact {
				t.Fatal("the rational original must retain a non-dyadic model station", i)
			}
		}
		a, b := edges[cut.NativeEdges[0]], edges[cut.NativeEdges[1]]
		if a.End != b.Start || vertices[a.End] != (geom.Vec3{X: x, Y: y, Z: x * y}) {
			t.Fatal("both native children must retain one exact point on the original nonplanar trace", i)
		}
	}
	f, err := Parse(bytes.NewReader(raw), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	verifyOriginalNonplanarModelBezier(t, f, m.Chain[0].ModelCurveID, rationalModel)
	modelCuts := [5]*big.Rat{new(big.Rat), nil, nil, nil, big.NewRat(1, 1)}
	for i, station := range modelRaw {
		modelCuts[i+1] = new(big.Rat).Quo(station, big.NewRat(8, 1))
	}
	for owner, use := range m.Chain[0].Owners {
		original, err := originalMappedChainStepOwner(f, use, true)
		if err != nil {
			t.Fatal(err)
		}
		if variant == nonplanarIndependentOwnerClocks || variant == nonplanarRationalRoofs {
			weights := []float64{8, 12, 18, 27}
			if owner == south {
				weights = []float64{1, 2, 4, 8, 16, 32, 64, 96, 144, 216}
			}
			if !slices.Equal(original.Curve.Weights(), weights) || original.Reversed != (owner == south) {
				t.Fatal("both independently authored source clock coefficients and their direction must survive", owner)
			}
		}
		for child := range 4 {
			q := faces[owner].Outer[child].Curve.(trimmed.MappedSplineUV)
			if !sameMixedMappedChainBasis(q.Source().Curve, original.Curve) {
				t.Fatal("the exact chart restriction must retain its entire original owner basis", owner, child)
			}
			for end, expected := range stops[owner][child : child+2] {
				rawParameter, err := q.AuthoredParameter(big.NewRat(int64(end), 1))
				if err != nil || rawParameter.Cmp(big.NewRat(expected, 1)) != 0 {
					t.Fatal("the four intervals must retain exact raw limits on both original owner maps", owner, child, end, err)
				}
			}
			// Independently evaluate the original projective law at the
			// complete model interval's corresponding interior parameter.
			mid := new(big.Rat).Quo(new(big.Rat).Add(modelCuts[child], modelCuts[child+1]), big.NewRat(2, 1))
			numerator := new(big.Rat).Mul(clocks[owner], mid)
			denominator := new(big.Rat).Add(new(big.Rat).Sub(big.NewRat(1, 1), mid), numerator)
			width := big.NewRat(stops[owner][4]-stops[owner][0], 1)
			want := new(big.Rat).Mul(new(big.Rat).Quo(numerator, denominator), width)
			want.Add(want, big.NewRat(stops[owner][0], 1))
			got, err := q.AuthoredParameter(big.NewRat(1, 2))
			if err != nil || got.Cmp(want) != 0 {
				t.Fatal("a child must preserve the exact original clock between its raw endpoints", owner, child, got, want, err)
			}
		}
	}
	northRoof, southRoof := faces[north].Surface.(*geom.BSplineSurface), faces[south].Surface.(*geom.BSplineSurface)
	northScale, southScale := big.NewRat(1, 1), big.NewRat(1, 1)
	if variant == nonplanarRationalRoofs {
		verifyNonplanarRationalRoofPoints(t, ctx, northRoof, southRoof)
		northScale, southScale = big.NewRat(6, 7), big.NewRat(24, 13)
	}
	verifyMappedRoofSlopeJump(t, ctx, northRoof, 0, 3.0/64, big.NewRat(1, 2), new(big.Rat).Mul(northScale, big.NewRat(2743, 4096)))
	verifyMappedRoofSlopeJump(t, ctx, northRoof, 0, 9.0/64, big.NewRat(1, 2), new(big.Rat).Mul(northScale, big.NewRat(-2085, 4096)))
	verifyMappedRoofSlopeJump(t, ctx, southRoof, 1, 3.0/64, big.NewRat(1, 8), new(big.Rat).Mul(southScale, big.NewRat(-6055, 16384)))
	verifyMappedRoofSlopeJump(t, ctx, southRoof, 1, 3.0/32, big.NewRat(1, 8), new(big.Rat).Mul(southScale, big.NewRat(3975, 8192)))
	for i, station := range stations {
		p := float64(station) / 32
		x, y := .75*p, .25+.75*p*(1-p)
		if i != 1 {
			verifyMappedRoofSlopeJump(t, ctx, northRoof, 0, x, mappedChainStepRat(y), new(big.Rat))
		}
		if i != 2 {
			verifyMappedRoofSlopeJump(t, ctx, southRoof, 1, x, mappedChainStepRat(y), new(big.Rat))
		}
	}

	var recipe, canonical, replay bytes.Buffer
	nativeOptions := trimmed.MappedChainShellNativeOptions{}
	if err := m.Shell.WriteJSON(ctx, &recipe, nativeOptions); err != nil || !bytes.Contains(recipe.Bytes(), []byte(`"Boundary":{"Version":7,`)) {
		t.Fatal("the existing native v7 proof must rederive the three nonplanar source events", err)
	}
	opened, err := trimmed.ReadMappedSplineChainShellJSON(ctx, bytes.NewReader(recipe.Bytes()), nativeOptions)
	if err != nil {
		t.Fatal(err)
	}
	native, err := opened.BuildRectangularSolid(ctx, 1e-9, o.Geometry)
	if err != nil {
		t.Fatal(err)
	}
	verifyNonplanarInferredMappedChainMaterial(t, ctx, opened, native, variant)
	for i, cut := range m.Crossings {
		if native.ChainStations[i+1].Cmp(cut.ModelStation) != 0 {
			t.Fatal("native reload must rederive the exact original model station", i)
		}
	}
	if err := opened.WriteJSON(ctx, &replay, nativeOptions); err != nil || !bytes.Equal(recipe.Bytes(), replay.Bytes()) {
		t.Fatal("native v7 reload must retain the original source recipe bytes", err)
	}
	if err := WriteMappedChainRectangularOriginalShell(ctx, &canonical, m, Limits{}, o); err != nil {
		t.Fatal(err)
	}
	strict, err := ReadMappedChainRectangularShell(ctx, bytes.NewReader(canonical.Bytes()), Limits{}, o)
	if err != nil || len(strict.Chain) != 3 || len(strict.Crossings) != 3 || strict.Shell.IndependentOwnerBases() != bases {
		t.Fatal("strict canonical reopening must preserve the unsplit nonplanar source edge and both original owners", err)
	}
	verifyOriginalMappedChainSTEPGraph(t, m, strict)
	if variant == nonplanarRationalRoofs {
		verifyOriginalRationalRoofSurfaces(t, m, strict, false)
	}
	replay.Reset()
	if err := strict.Shell.WriteJSON(ctx, &replay, nativeOptions); err != nil || !bytes.Equal(recipe.Bytes(), replay.Bytes()) {
		t.Fatal("strict STEP must reconstruct the identical original native recipe", err)
	}
	replay.Reset()
	if err := WriteMappedChainRectangularOriginalShell(ctx, &replay, strict, Limits{}, o); err != nil || !bytes.Equal(canonical.Bytes(), replay.Bytes()) {
		t.Fatal("canonical STEP must replay the same original nonplanar graph byte-for-byte", err)
	}
	return canonical.Bytes()
}

func verifyOriginalNonplanarModelBezier(t *testing.T, f *File, modelID int64, rationalModel bool) {
	t.Helper()
	model, err := readMappedChainStepSpline(f, modelID, 3, 16, 17, true)
	if err != nil || model.degree != 3 || len(model.poles) != 4 || len(model.knots) != 8 {
		t.Fatal("the original model must contain one unsplit cubic", err)
	}
	poles := [4][3]float64{
		{0, .25, 0}, {1.0 / 16, 5.0 / 16, 1.0 / 64},
		{1.0 / 8, 23.0 / 64, 11.0 / 256}, {3.0 / 16, 25.0 / 64, 75.0 / 1024},
	}
	weights := [4]int64{1, 1, 1, 1}
	if rationalModel {
		weights = [4]int64{1, 2, 4, 8}
	}
	for i, pole := range poles {
		if model.knots[i].Sign() != 0 || model.knots[i+4].Cmp(big.NewRat(8, 1)) != 0 || model.weights[i].Cmp(big.NewRat(weights[i], 1)) != 0 {
			t.Fatal("the authored cubic must retain its exact raw interval and original weights", i)
		}
		for axis, value := range pole {
			if model.poles[i][axis].Cmp(mappedChainStepRat(value)) != 0 {
				t.Fatal("the full original nonplanar coefficients must agree exactly", i, axis)
			}
		}
	}
	var d [3][3]*big.Rat
	for i := range d {
		for axis := range d[i] {
			d[i][axis] = new(big.Rat).Sub(model.poles[i+1][axis], model.poles[0][axis])
		}
	}
	determinant := new(big.Rat)
	for i := range 3 {
		j, k := (i+1)%3, (i+2)%3
		minor := new(big.Rat).Sub(new(big.Rat).Mul(d[1][j], d[2][k]), new(big.Rat).Mul(d[1][k], d[2][j]))
		determinant.Add(determinant, minor.Mul(minor, d[0][i]))
	}
	if determinant.Cmp(big.NewRat(9, 1048576)) != 0 {
		t.Fatal("the four authored model poles must have the expected nonzero affine determinant", determinant)
	}
}

func verifyNonplanarInferredMappedChainMaterial(t *testing.T, ctx context.Context, shell trimmed.MappedSplineChainShell, m trimmed.MappedChainRectangularSolid, variant nonplanarMappedChainVariant) {
	t.Helper()
	s := m.Solid
	if len(m.ChainEdges) != 6 || len(s.Faces()) != 7 || len(s.Edges()) != 24 || len(s.Vertices()) != 19 {
		t.Fatal("the nonplanar Bézier must preserve complete closed topology", len(s.Faces()), len(s.Edges()), len(s.Vertices()))
	}
	embedding, err := shell.CertifyRectangularEmbedding(ctx, geom.SurfacePatchOptions{})
	if err != nil || embedding.FloorHeight.Cmp(big.NewRat(-1, 1)) != 0 || embedding.Clearance.Sign() <= 0 {
		t.Fatal("the original planar walls and floor must close the positive material", err)
	}
	for _, axis := range embedding.Footprint {
		if axis[0].Sign() != 0 || axis[1].Cmp(big.NewRat(3, 4)) != 0 {
			t.Fatal("the original planar closings must retain the exact footprint", embedding.Footprint)
		}
	}
	proof, err := s.ValidateEdgeAgreement(ctx, edgecheck.Options{})
	if err != nil || !proof.Complete || len(proof.Uses) != 48 {
		t.Fatal("all original and rebuilt incidences must prove continuous equality", err)
	}
	volume, area := nonplanarInferredMappedChainVolume, nonplanarInferredMappedChainArea
	if variant == nonplanarRationalRoofs {
		volume, area = nonplanarRationalRoofVolume, nonplanarRationalRoofArea
	}
	mass, err := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
	if err != nil || math.Abs(mass.Volume-volume) > 1e-8 || math.Abs(mass.Area-area) > 1e-8 {
		t.Fatal("native material must match the independent nonplanar model", mass, err)
	}
}

func TestOCCTMappedChainRectangularNonplanarInferredMixedSupportKnots(t *testing.T) {
	verifyNonplanarInferredMappedChain(t, context.Background(), readInferredSharedMappedChainFixture(t, nonplanarInferredMappedChainPath), nonplanarPolynomialClock)
}

func TestOCCTMappedChainRectangularNonplanarInferredMixedSupportKnotsClosedDisplay(t *testing.T) {
	ctx := context.Background()
	raw := readInferredSharedMappedChainFixture(t, nonplanarInferredMappedChainPath)
	m, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(raw), Limits{}, inferredMixedMappedChainOptions())
	if err != nil {
		t.Fatal(err)
	}
	verifyMappedChainCutDisplayWithOptions(t, ctx, m, nonplanarInferredDisplayOptions(), .005)
}

func nonplanarInferredDisplayOptions() trimmed.MixedExactMultiSpanMeshOptions {
	return trimmed.MixedExactMultiSpanMeshOptions{
		Span: trimmed.MultiSpanTriangleBoundOptions{MaxDegree: 3},
		Face: trimmed.MixedExactCurvedMeshOptions{
			BoundaryModelBudget: .005, Interior: trimmed.CurvedQuadraticTrimMeshOptions{Deflection: .04},
		},
	}
}

func TestOCCTMappedChainRectangularNonplanarInferredMixedSupportKnotsRejectsChangedPole(t *testing.T) {
	ctx := context.Background()
	raw := readInferredSharedMappedChainFixture(t, nonplanarInferredMappedChainPath)
	o := inferredMixedMappedChainOptions()
	m, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(raw), Limits{}, o)
	if err != nil {
		t.Fatal(err)
	}
	f, err := Parse(bytes.NewReader(raw), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	basis, _, _, err := mappedChainStepSplineParts(f, m.Chain[0].ModelCurveID)
	if err != nil {
		t.Fatal(err)
	}
	pole := f.Entities[basis[1].Items[2].Ref].Records[0].Args[1].Items
	// Only one interior model Z coordinate changes. Both source charts,
	// original endpoint vertices and raw knots retain their exact values.
	pole[2].Text = "0.043212890625"
	bad, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(serializeWrappedProofFile(f)), Limits{}, o)
	if !errors.Is(err, geom.ErrConvergence) || len(bad.Crossings) != 0 || len(bad.Native.ChainEdges) != 0 {
		t.Fatal("a changed exact 3D coefficient must prevent publishing any repaired original edge", err)
	}
}

func TestOCCTMappedChainRectangularNonplanarInferredMixedSupportKnotsLive(t *testing.T) {
	author, reader := os.Getenv("PLAINSOLID_OCCT_MAPPED_CHAIN_NONPLANAR_AUTHOR"), os.Getenv("PLAINSOLID_OCCT_MAPPED_CHAIN_STEP")
	if author == "" || reader == "" {
		t.Skip("optional pinned OCCT source author and STEP consumer")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	path := filepath.Join(t.TempDir(), "nonplanar-original.step")
	if output, err := exec.CommandContext(ctx, author, "write-nonplanar-inferred-mixed", path).CombinedOutput(); err != nil {
		t.Fatalf("pinned OCCT author failed: %v\n%s", err, output)
	}
	raw := readInferredSharedMappedChainFixture(t, path)
	canonical := verifyNonplanarInferredMappedChain(t, ctx, raw, nonplanarPolynomialClock)
	verifyPinnedMappedChainTransfer(t, ctx, reader, raw, canonical, nonplanarInferredMappedChainVolume, nonplanarInferredMappedChainArea)
}

func TestOCCTMappedChainRectangularNonplanarRationalInferredMixedSupportKnots(t *testing.T) {
	verifyNonplanarInferredMappedChain(t, context.Background(), readInferredSharedMappedChainFixture(t, nonplanarRationalInferredMappedChainPath), nonplanarRationalClock)
}

func TestOCCTMappedChainRectangularNonplanarRationalInferredMixedSupportKnotsClosedDisplay(t *testing.T) {
	ctx := context.Background()
	raw := readInferredSharedMappedChainFixture(t, nonplanarRationalInferredMappedChainPath)
	m, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(raw), Limits{}, inferredMixedMappedChainOptions())
	if err != nil {
		t.Fatal(err)
	}
	verifyMappedChainCutDisplayWithOptions(t, ctx, m, nonplanarInferredDisplayOptions(), .005)
}

func TestOCCTMappedChainRectangularNonplanarRationalInferredMixedSupportKnotsRejectsFalseWeight(t *testing.T) {
	ctx := context.Background()
	raw := readInferredSharedMappedChainFixture(t, nonplanarRationalInferredMappedChainPath)
	o := inferredMixedMappedChainOptions()
	m, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(raw), Limits{}, o)
	if err != nil {
		t.Fatal(err)
	}
	f, err := Parse(bytes.NewReader(raw), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	_, _, weights, err := mappedChainStepSplineParts(f, m.Chain[0].ModelCurveID)
	if err != nil || len(weights) != 4 {
		t.Fatal("the originally authored 3D spline must retain four rational weights", err)
	}
	// Leave the exact original poles, endpoint vertices, raw domain and both
	// owner maps fixed while invalidating the original projective clock.
	weights[2].Text = "3."
	bad, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(serializeWrappedProofFile(f)), Limits{}, o)
	if !errors.Is(err, geom.ErrConvergence) || len(bad.Crossings) != 0 || len(bad.Native.ChainEdges) != 0 {
		t.Fatal("a false 3D weight must reject atomically even with exact endpoints and raw limits", err)
	}
}

func TestOCCTMappedChainRectangularNonplanarRationalInferredMixedSupportKnotsLive(t *testing.T) {
	author, reader := os.Getenv("PLAINSOLID_OCCT_MAPPED_CHAIN_NONPLANAR_RATIONAL_AUTHOR"), os.Getenv("PLAINSOLID_OCCT_MAPPED_CHAIN_STEP")
	if author == "" || reader == "" {
		t.Skip("optional pinned OCCT source author and STEP consumer")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	path := filepath.Join(t.TempDir(), "rational-nonplanar-original.step")
	if output, err := exec.CommandContext(ctx, author, "write-nonplanar-rational-inferred-mixed", path).CombinedOutput(); err != nil {
		t.Fatalf("pinned OCCT author failed: %v\n%s", err, output)
	}
	raw := readInferredSharedMappedChainFixture(t, path)
	canonical := verifyNonplanarInferredMappedChain(t, ctx, raw, nonplanarRationalClock)
	verifyPinnedMappedChainTransfer(t, ctx, reader, raw, canonical, nonplanarInferredMappedChainVolume, nonplanarRationalInferredOCCTArea)
}

func TestOCCTMappedChainRectangularNonplanarIndependentOwnerClocks(t *testing.T) {
	raw := readInferredSharedMappedChainFixture(t, nonplanarIndependentOwnerClocksPath)
	verifyNonplanarInferredMappedChain(t, context.Background(), raw, nonplanarIndependentOwnerClocks)
}

func TestOCCTMappedChainRectangularNonplanarIndependentOwnerClocksClosedDisplay(t *testing.T) {
	ctx := context.Background()
	raw := readInferredSharedMappedChainFixture(t, nonplanarIndependentOwnerClocksPath)
	m, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(raw), Limits{}, inferredMixedMappedChainOptions())
	if err != nil {
		t.Fatal(err)
	}
	verifyMappedChainCutDisplayWithOptions(t, ctx, m, nonplanarInferredDisplayOptions(), .005)
}

func TestOCCTMappedChainRectangularNonplanarIndependentOwnerClocksRejectsFalseOwnerWeight(t *testing.T) {
	ctx := context.Background()
	raw := readInferredSharedMappedChainFixture(t, nonplanarIndependentOwnerClocksPath)
	o := inferredMixedMappedChainOptions()
	m, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(raw), Limits{}, o)
	if err != nil {
		t.Fatal(err)
	}
	north := 0
	if !m.Shell.IndependentOwnerBases()[0] {
		north = 1
	}
	for _, control := range []struct {
		name         string
		owner, index int
		weight       string
	}{
		{"independent north basis", north, 1, "11."},
		{"whole south basis", 1 - north, 7, "95."},
	} {
		t.Run(control.name, func(t *testing.T) {
			f, err := Parse(bytes.NewReader(raw), Limits{})
			if err != nil {
				t.Fatal(err)
			}
			_, _, weights, err := mappedChainStepSplineParts(f, m.Chain[0].Owners[control.owner].BasisID)
			if err != nil {
				t.Fatal(err)
			}
			// Leave the weighted 3D model and all raw endpoint values fixed.
			weights[control.index].Text = control.weight
			bad, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(serializeWrappedProofFile(f)), Limits{}, o)
			if !errors.Is(err, geom.ErrUnrepresentable) || len(bad.Crossings) != 0 || len(bad.Native.ChainEdges) != 0 {
				t.Fatal("each original basis requires its own exact parameter law", err)
			}
		})
	}
}

func TestOCCTMappedChainRectangularNonplanarIndependentOwnerClocksLive(t *testing.T) {
	author, reader := os.Getenv("PLAINSOLID_OCCT_MAPPED_CHAIN_INDEPENDENT_OWNER_CLOCKS_AUTHOR"), os.Getenv("PLAINSOLID_OCCT_MAPPED_CHAIN_STEP")
	if author == "" || reader == "" {
		t.Skip("optional pinned OCCT source author and STEP consumer")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	path := filepath.Join(t.TempDir(), "nonplanar-independent-owner-clocks.step")
	if output, err := exec.CommandContext(ctx, author, "write-nonplanar-independent-owner-clocks", path).CombinedOutput(); err != nil {
		t.Fatalf("pinned OCCT author failed: %v\n%s", err, output)
	}
	raw := readInferredSharedMappedChainFixture(t, path)
	canonical := verifyNonplanarInferredMappedChain(t, ctx, raw, nonplanarIndependentOwnerClocks)
	verifyPinnedMappedChainTransfer(t, ctx, reader, raw, canonical, nonplanarInferredMappedChainVolume, nonplanarIndependentOwnerClocksOCCTArea)
}
