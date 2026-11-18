package step

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/internal/slowtest"
	"slices"
	"testing"
	"time"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/collision"
	"plainsolid/geom"
)

const threeJoinedRoofPath = "testdata/occt_mapped_chain_three_joined_independent_roofs.step"
const threeJoinedMissingJointPath = "testdata/occt_mapped_chain_three_joined_independent_missing_joint.step"

const threeJoinedVolume = 0.6404509967881544
const threeJoinedArea = 4.655759693110457
const threeJoinedSTEPVolume = 0.64045501353686718
const threeJoinedSTEPArea = 4.655823835734676

const asymmetricJoinedRoofPath = "testdata/occt_mapped_chain_asymmetric_joined_independent_roofs.step"
const asymmetricJoinedMissingJointPath = "testdata/occt_mapped_chain_asymmetric_joined_independent_missing_joint.step"
const asymmetricJoinedVolume = 0.64313717250617142
const asymmetricJoinedArea = 4.651017456483614
const asymmetricJoinedSTEPVolume = 0.64314146076604906
const asymmetricJoinedSTEPArea = 4.6510825384330969

type threeJoinedRoofShape uint8

const (
	threeJoinedShared threeJoinedRoofShape = iota
	threeJoinedAsymmetric
	threeJoinedKinked
)

func threeJoinedRoofOptions() MappedChainShellSTEPOptions {
	o := joinedRationalCellOptions()
	o.AllowThreeJoinedSupportKnotCells = true
	return o
}

func verifyThreeJoinedRoofSource(t *testing.T, ctx context.Context, raw []byte, shape threeJoinedRoofShape) []byte {
	t.Helper()
	asymmetric, kinked := shape != threeJoinedShared, shape == threeJoinedKinked
	o := threeJoinedRoofOptions()
	m, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(raw), Limits{}, o)
	if err != nil {
		t.Fatal(err)
	}
	modelVersion, sourceEdges, sourceVertices := 11, 24, 19
	volume, area := threeJoinedVolume, threeJoinedArea
	if asymmetric {
		modelVersion, sourceEdges, sourceVertices = 12, 23, 18
		volume, area = asymmetricJoinedVolume, asymmetricJoinedArea
	}
	if kinked {
		volume, area = kinkedJoinedVolume, kinkedJoinedArea
	}
	if len(m.Chain) != 3 || len(m.Crossings) != 3 || len(m.SupportJoints) != 2 || m.SupportJoint != nil {
		t.Fatal("three original Béziers must retain one interior cut per cell and both original support joints")
	}
	bases := m.Shell.IndependentOwnerBases()
	if bases[0] == bases[1] {
		t.Fatal("the north roof must retain independent bases and the south roof must retain its shared basis")
	}
	north := 0
	if !bases[0] {
		north = 1
	}
	var rawStops [2][3][3]int64
	rawStops[north] = [3][3]int64{{2, 68, 167}, {700, 706, 715}, {850, 859, 862}}
	rawStops[1-north] = [3][3]int64{{181, 82, 16}, {16, 6, 1}, {1, -1, -2}}
	var weights [2][3]*big.Rat
	weights[north] = [3]*big.Rat{big.NewRat(4, 3), big.NewRat(8, 3), big.NewRat(24, 1)}
	weights[1-north] = [3]*big.Rat{big.NewRat(3, 1), big.NewRat(8, 1), big.NewRat(16, 1)}
	sourceStations := [4]int64{0, 8, 16, 32}
	geometryStations := [7]float64{0, .125, .25, .375, .5, .75, 1}
	split := [3]*big.Rat{big.NewRat(1, 3), big.NewRat(1, 5), big.NewRat(1, 9)}
	edges, vertices, faces := m.Native.Solid.Edges(), m.Native.Solid.Vertices(), m.Native.Solid.Faces()
	f, err := Parse(bytes.NewReader(raw), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Native.ChainEdges) != 6 || len(m.Native.ChainStations) != 7 ||
		len(vertices) != sourceVertices+3 || len(edges) != sourceEdges+3 || len(faces) != 7 {
		t.Fatal("the rebuilt solid must preserve all original vertices and add only the three paired split vertices")
	}
	agreement, err := m.Native.Solid.ValidateEdgeAgreement(ctx, edgecheck.Options{})
	if err != nil || !agreement.Complete || len(agreement.Uses) != 2*len(edges) {
		t.Fatal("the original and repaired incidences require full continuous geometry agreement", err)
	}
	for i, cell := range m.Chain {
		verifyThreeJoinedOriginalModel(t, f, cell.ModelCurveID, sourceStations[i], sourceStations[i+1], int64(2<<i),
			geometryStations[2*i], geometryStations[2*i+2], kinked)
		cut := m.Crossings[i]
		which := [2]bool{i != 2, i == 2}
		if north != 0 {
			which[0], which[1] = which[1], which[0]
		}
		rawCut := new(big.Rat).Add(big.NewRat(sourceStations[i], 1), new(big.Rat).Mul(big.NewRat(sourceStations[i+1]-sourceStations[i], 1), split[i]))
		if cut.EdgeID != cell.EdgeID || cut.StoredModelKnot || cut.OwnersChangingCells != which ||
			cut.SourceModelParameter.Cmp(rawCut) != 0 || cut.ModelStation.Cmp(new(big.Rat).Quo(rawCut, big.NewRat(32, 1))) != 0 ||
			cut.NativeEdges != [2]int{m.Native.ChainEdges[2*i], m.Native.ChainEdges[2*i+1]} ||
			edges[cut.NativeEdges[0]].End != edges[cut.NativeEdges[1]].Start ||
			vertices[edges[cut.NativeEdges[0]].End] != threeJoinedPhysicalModelPoint(geometryStations[2*i+1], kinked) {
			t.Fatal("the interior cut must preserve exact raw model station, owner and paired physical point", i, cut)
		}
		for owner, stops := range rawStops {
			if cut.RawParameters[owner].Cmp(big.NewRat(stops[i][1], 1)) != 0 {
				t.Fatal("the root must lie at both exact raw owner stations", owner, i, cut.RawParameters[owner])
			}
			source, err := originalMappedChainStepOwner(f, cell.Owners[owner], true)
			if err != nil {
				t.Fatal(err)
			}
			ends := [3]*big.Rat{new(big.Rat), split[i], big.NewRat(1, 1)}
			for child := range 2 {
				q := faces[owner].Outer[2*i+child].Curve.(trimmed.MappedSplineUV)
				if !sameMixedMappedChainBasis(source.Curve, q.Source().Curve) {
					t.Fatal("repaired coedges must retain the original raw support basis", owner, i, child)
				}
				mid := new(big.Rat).Quo(new(big.Rat).Add(ends[child], ends[child+1]), big.NewRat(2, 1))
				numerator := new(big.Rat).Mul(weights[owner][i], mid)
				denominator := new(big.Rat).Add(new(big.Rat).Sub(big.NewRat(1, 1), mid), numerator)
				wantMid := new(big.Rat).Add(big.NewRat(stops[i][0], 1),
					new(big.Rat).Mul(big.NewRat(stops[i][2]-stops[i][0], 1), new(big.Rat).Quo(numerator, denominator)))
				want := [3]*big.Rat{big.NewRat(stops[i][child], 1), wantMid, big.NewRat(stops[i][child+1], 1)}
				for sample, raw := range want {
					got, err := q.AuthoredParameter(big.NewRat(int64(sample), 2))
					if err != nil || got.Cmp(raw) != 0 {
						t.Fatal("the original projective clock must hold at both cell ends and inside the raw restriction", owner, i, child, sample, got, err)
					}
				}
			}
		}
	}
	jointMasks := [][2]bool{{true, true}, {true, true}}
	if asymmetric {
		jointMasks[1][1-north] = false
	}
	if got := m.Shell.OriginalJointSupportChanges(); !slices.Equal(got, jointMasks) {
		t.Fatal("native source proof must retain the original owner masks", got)
	}
	for i, joint := range m.SupportJoints {
		if joint.OwnersChangingCells != jointMasks[i] || joint.SourceEdges != [2]int64{m.Chain[i].EdgeID, m.Chain[i+1].EdgeID} ||
			joint.ModelStation.Cmp(big.NewRat(sourceStations[i+1], 32)) != 0 ||
			joint.NativeEdges != [2]int{m.Native.ChainEdges[2*i+1], m.Native.ChainEdges[2*i+2]} ||
			edges[joint.NativeEdges[0]].End != edges[joint.NativeEdges[1]].Start {
			t.Fatal("original shared model joints must retain topology and their own stations", i, joint)
		}
		wantPoint := threeJoinedPhysicalModelPoint(geometryStations[2*i+2], kinked)
		sourcePoint, err := mappedChainStepVertex(f, joint.SourceVertexID)
		if err != nil {
			t.Fatal(err)
		}
		point, err := geom.RepresentedExactPoint(sourcePoint)
		if err != nil || point != wantPoint || vertices[edges[joint.NativeEdges[0]].End] != point {
			t.Fatal("the shared joint must be the actual original vertex", i, point, err)
		}
		for side, raw := range joint.SourceModelParameters {
			if raw.Cmp(big.NewRat(sourceStations[i+1], 1)) != 0 {
				t.Fatal("the two raw model joint endpoints must be preserved", i, side, raw)
			}
		}
		for owner, boundary := range joint.OwnerParameters {
			if boundary[0].Cmp(big.NewRat(rawStops[owner][i][2], 1)) != 0 ||
				boundary[1].Cmp(big.NewRat(rawStops[owner][i+1][0], 1)) != 0 {
				t.Fatal("the two raw endpoints must survive in original order on each roof", i, owner, boundary)
			}
		}
	}
	if kinked {
		verifyKinkedJoinedRoofs(t, ctx, m.Native, north)
	} else {
		verifyThreeJoinedRoofCreases(t, ctx, m, north, asymmetric)
	}
	mass, err := m.Native.Solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
	if err != nil || math.Abs(mass.Volume-volume) > 1e-8 || math.Abs(mass.Area-area) > 1e-8 {
		t.Fatal("native material must agree with the independent original OCCT body", mass, err)
	}
	var recipe, replay, canonical bytes.Buffer
	nativeOpts := trimmed.MappedChainShellNativeOptions{}
	if err := m.Shell.WriteJSON(ctx, &recipe, nativeOpts); err != nil ||
		!bytes.Contains(recipe.Bytes(), fmt.Appendf(nil, `"Boundary":{"Version":%d,`, modelVersion)) {
		t.Fatal("the native version must preserve all original source cells and their joint semantics", modelVersion, err)
	}
	reopened, err := trimmed.ReadMappedSplineChainShellJSON(ctx, bytes.NewReader(recipe.Bytes()), nativeOpts)
	if err != nil || !slices.Equal(reopened.OriginalJointSupportChanges(), jointMasks) {
		t.Fatal("native reload must re-prove the earlier original joint owner masks", err)
	}
	native, err := reopened.BuildRectangularSolid(ctx, 1e-9, o.Geometry)
	if err != nil || len(native.ChainStations) != len(m.Native.ChainStations) {
		t.Fatal("native source-only reload must rederive three cuts while retaining both source joints", err)
	}
	for i, actual := range native.ChainStations {
		if actual.Cmp(m.Native.ChainStations[i]) != 0 {
			t.Fatal("native replay must preserve all original and repaired model stations", i)
		}
	}
	if kinked {
		verifyKinkedJoinedRoofs(t, ctx, native, north)
	}
	if err := reopened.WriteJSON(ctx, &replay, nativeOpts); err != nil || !bytes.Equal(recipe.Bytes(), replay.Bytes()) {
		t.Fatal("native source replay must be byte stable", err)
	}
	if err := WriteMappedChainRectangularOriginalShell(ctx, &canonical, m, Limits{}, o); err != nil {
		t.Fatal(err)
	}
	strict, err := ReadMappedChainRectangularShell(ctx, bytes.NewReader(canonical.Bytes()), Limits{}, o)
	if err != nil || len(strict.Chain) != 3 || len(strict.Crossings) != 3 || len(strict.SupportJoints) != 2 ||
		strict.Shell.IndependentOwnerBases() != bases {
		t.Fatal("canonical STEP must reprove the exact original three-cell partition", err)
	}
	verifyOriginalMappedChainSTEPGraph(t, m, strict)
	if kinked {
		verifyKinkedJoinedRoofs(t, ctx, strict.Native, north)
		verifyOriginalRationalRoofSurfaceProfiles(t, m, strict, []int64{1, 1, 2, 4, 7}, []int64{7, 4, 2, 1, 1})
	} else {
		verifyOriginalRationalRoofSurfaces(t, m, strict, true)
	}
	for i, joint := range strict.SupportJoints {
		original := m.SupportJoints[i]
		if joint.OwnersChangingCells != jointMasks[i] || joint.SourceEdges != [2]int64{strict.Chain[i].EdgeID, strict.Chain[i+1].EdgeID} ||
			joint.ModelStation.Cmp(original.ModelStation) != 0 || joint.NativeEdges != original.NativeEdges {
			t.Fatal("canonical STEP must preserve both original model joints", i)
		}
		for owner := range 2 {
			if joint.SourceModelParameters[owner].Cmp(original.SourceModelParameters[owner]) != 0 {
				t.Fatal("canonical STEP must preserve both one-sided model endpoints", i, owner)
			}
			for end := range 2 {
				if joint.OwnerParameters[owner][end].Cmp(original.OwnerParameters[owner][end]) != 0 {
					t.Fatal("canonical STEP must preserve both one-sided owner endpoints", i, owner, end)
				}
			}
		}
	}
	replay.Reset()
	if err := strict.Shell.WriteJSON(ctx, &replay, nativeOpts); err != nil || !bytes.Equal(recipe.Bytes(), replay.Bytes()) {
		t.Fatal("canonical STEP must rebuild the same source-only recipe", err)
	}
	replay.Reset()
	if err := WriteMappedChainRectangularOriginalShell(ctx, &replay, strict, Limits{}, o); err != nil || !bytes.Equal(canonical.Bytes(), replay.Bytes()) {
		t.Fatal("strict original-source STEP must rewrite byte-for-byte", err)
	}
	return canonical.Bytes()
}

func threeJoinedPhysicalModelPoint(t float64, kinked bool) geom.Vec3 {
	x, y := .75*t, .25+.75*t*(1-t)
	if kinked {
		y = .25 + .375*min(t, 1-t)
	}
	return geom.Vec3{X: x, Y: y, Z: x * y}
}

func verifyThreeJoinedOriginalModel(t *testing.T, f *File, id, lo, hi, ratio int64, a, b float64, kinked bool) {
	t.Helper()
	pa, pb := threeJoinedPhysicalModelPoint(a, kinked), threeJoinedPhysicalModelPoint(b, kinked)
	ym := .25 + .75*((a+b)/2-a*b)
	if kinked {
		ym = (pa.Y + pb.Y) / 2
	}
	verifyJoinedOriginalModelBezier(t, f, id, lo, hi, ratio, pa, pb, ym)
}

func verifyJoinedOriginalModelBezier(t *testing.T, f *File, id, lo, hi, ratio int64, pa, pb geom.Vec3, ym float64) {
	t.Helper()
	s, err := readMappedChainStepSpline(f, id, 3, 16, 17, true)
	if err != nil || s.degree != 3 || len(s.poles) != 4 || len(s.knots) != 8 {
		t.Fatal("each original model edge must store a single exact rational cubic", err)
	}
	poles := [4]geom.Vec3{pa,
		{X: (2*pa.X + pb.X) / 3, Y: (pa.Y + 2*ym) / 3, Z: (2*pa.X*ym + pb.X*pa.Y) / 3},
		{X: (pa.X + 2*pb.X) / 3, Y: (2*ym + pb.Y) / 3, Z: (pa.X*pb.Y + 2*pb.X*ym) / 3}, pb}
	w := int64(1)
	for i, pole := range poles {
		if s.knots[i].Cmp(big.NewRat(lo, 1)) != 0 || s.knots[i+4].Cmp(big.NewRat(hi, 1)) != 0 ||
			s.weights[i].Cmp(big.NewRat(w, 1)) != 0 {
			t.Fatal("the original polynomial degree, raw model range and rational clock must survive", id, i)
		}
		for axis, value := range []float64{pole.X, pole.Y, pole.Z} {
			if s.poles[i][axis].Cmp(mappedChainStepRat(value)) != 0 {
				t.Fatal("the original model poles must coincide exactly with the authored nonplanar control net", id, i, axis)
			}
		}
		w *= ratio
	}
}

func verifyThreeJoinedRoofCreases(t *testing.T, ctx context.Context, m MappedChainSTEPModel, north int, asymmetric bool) {
	t.Helper()
	faces := m.Native.Solid.Faces()
	one, yLimit := big.NewRat(1, 1), big.NewRat(3, 4)
	knotsByRoof := [2][]float64{{.125, .25, .375, .5}, {.25, .5, .75}}
	if asymmetric {
		knotsByRoof[1] = []float64{.25, .75}
	}
	for owner, knots := range knotsByRoof {
		face, y, axis := north, big.NewRat(1, 2), 0
		if owner == 1 {
			face, y, axis = 1-north, big.NewRat(1, 8), 1
		}
		v := new(big.Rat).Quo(y, yLimit)
		if owner == 1 {
			v.Sub(one, v)
		}
		den := new(big.Rat).Add(one, new(big.Rat).Mul(big.NewRat(3, 1), new(big.Rat).Mul(v, v)))
		roof := faces[face].Surface.(*geom.BSplineSurface)
		patch, err := geom.NewSurfacePatch(roof, geom.SurfacePatchOptions{})
		if err != nil {
			t.Fatal(err)
		}
		uBreaks, vBreaks := patch.KnotBreaks()
		actual := [2][]float64{uBreaks, vBreaks}[axis]
		expected := []float64{0}
		for _, knot := range knots {
			expected = append(expected, .75*knot)
		}
		if !slices.Equal(actual, append(expected, .75)) {
			t.Fatal("the untouched original span at the unilateral joint must have no authored break", owner, actual)
		}
		for _, knot := range knots {
			p := threeJoinedPhysicalModelPoint(knot, false)
			got := new(big.Rat).Mul(big.NewRat(-16, 1), y)
			got.Mul(got, new(big.Rat).Sub(yLimit, y))
			got.Mul(got, new(big.Rat).Sub(y, mappedChainStepRat(p.Y)))
			got.Quo(got, den)
			verifyMappedRoofSlopeJump(t, ctx, roof, axis, p.X, y, got)
			verifyMappedRoofSlopeJump(t, ctx, roof, axis, p.X, mappedChainStepRat(p.Y), new(big.Rat))
		}
	}
}

func TestOCCTMappedChainRectangularThreeJoinedIndependentRoofs(t *testing.T) {
	slowtest.Skip(t)
	verifyThreeJoinedRoofSource(t, context.Background(), readInferredSharedMappedChainFixture(t, threeJoinedRoofPath), threeJoinedShared)
}

func TestOCCTMappedChainRectangularThreeJoinedIndependentRoofsClosedDisplay(t *testing.T) {
	slowtest.Skip(t)
	verifyThreeJoinedRoofDisplay(t, readInferredSharedMappedChainFixture(t, threeJoinedRoofPath), 3, .14)
}

func verifyThreeJoinedRoofDisplay(t *testing.T, raw []byte, maxDegree int, deflection float64) {
	t.Helper()
	ctx := context.Background()
	m, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(raw), Limits{}, threeJoinedRoofOptions())
	if err != nil {
		t.Fatal(err)
	}
	fill := nonplanarInferredDisplayOptions()
	fill.Span.MaxDegree = maxDegree
	fill.Face.BoundaryModelBudget, fill.Face.Interior.Deflection = .03, deflection
	verifyJoinedClosedDisplay(t, ctx, m, fill)
}

func verifyJoinedClosedDisplay(t *testing.T, ctx context.Context, m MappedChainSTEPModel, fill trimmed.MixedExactMultiSpanMeshOptions) {
	t.Helper()
	display := verifyMappedChainCutDisplayWithClosedOptions(t, ctx, m, trimmed.MappedChainClosedMeshOptions{
		Boundary:  trimmed.MappedChainPolylineOptions{Deflection: .01875},
		Curved:    [2]trimmed.MixedExactMultiSpanMeshOptions{fill, fill},
		Collision: collision.Options{MaxPairs: 4_000_000},
	})
	if len(display.ChainJoints) != len(m.SupportJoints) {
		t.Fatal("the closed display must retain both original coedge occurrences at every welded joint")
	}
	for i, pair := range display.ChainJoints {
		joint := m.SupportJoints[i]
		a, b := pair[0], pair[1]
		if a.Vertex != b.Vertex || a.ModelParameter.Cmp(joint.ModelStation) != 0 || b.ModelParameter.Cmp(joint.ModelStation) != 0 ||
			a.AuthoredParts != [2]int{i, i} || b.AuthoredParts != [2]int{i + 1, i + 1} {
			t.Fatal("each welded joint must retain its original ordered model adjacency", i)
		}
		for owner, raw := range joint.OwnerParameters {
			if a.AuthoredParameters[owner].Cmp(raw[0]) != 0 || b.AuthoredParameters[owner].Cmp(raw[1]) != 0 {
				t.Fatal("the joint must retain both one-sided owner endpoints", i, owner)
			}
		}
	}
}

func TestOCCTMappedChainRectangularThreeJoinedIndependentRoofsRejectsMissingJoint(t *testing.T) {
	ctx := context.Background()
	raw := readInferredSharedMappedChainFixture(t, threeJoinedRoofPath)
	older := joinedRationalCellOptions()
	older.AllowUpToSixSupportKnots = true
	if got, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(raw), Limits{}, older); !errors.Is(err, geom.ErrConvergence) ||
		len(got.Native.ChainEdges) != 0 || len(got.SupportJoints) != 0 {
		t.Fatal("the earlier wider chart bound must still require the three joined cell gate", err)
	}
	bad := readInferredSharedMappedChainFixture(t, threeJoinedMissingJointPath)
	rejectThreeJoinedRoofMissingJoint(t, ctx, raw, bad)
}

func rejectThreeJoinedRoofMissingJoint(t *testing.T, ctx context.Context, raw, bad []byte) {
	t.Helper()
	got, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(bad), Limits{}, threeJoinedRoofOptions())
	if !errors.Is(err, geom.ErrConvergence) || len(got.Native.ChainEdges) != 0 || len(got.SupportJoints) != 0 || len(got.Crossings) != 0 {
		t.Fatal("moving one original support knot off the second shared joint must reject atomically", err)
	}
	rejectChangedJoinedRationalNativeSupportWithOptions(t, ctx, raw, bad, threeJoinedRoofOptions(), false, "support knot must belong")
}

func TestOCCTMappedChainRectangularThreeJoinedIndependentRoofsLive(t *testing.T) {
	verifyPinnedThreeJoinedRoofs(t, threeJoinedShared)
}

func verifyPinnedThreeJoinedRoofs(t *testing.T, shape threeJoinedRoofShape) {
	t.Helper()
	asymmetric := shape != threeJoinedShared
	authorEnv, mode := "PLAINSOLID_OCCT_MAPPED_CHAIN_THREE_JOINED_ROOFS_AUTHOR", "three-joined-independent-roofs"
	negativeMode := "-missing-south-joint"
	edges, vertices := 24, 19
	volume, area, stepVolume, stepArea := threeJoinedVolume, threeJoinedArea, threeJoinedSTEPVolume, threeJoinedSTEPArea
	if asymmetric {
		authorEnv, mode = "PLAINSOLID_OCCT_MAPPED_CHAIN_ASYMMETRIC_JOINED_ROOFS_AUTHOR", "asymmetric-joined-independent-roofs"
		negativeMode = "-missing-shared-joint"
		edges, vertices = 23, 18
		volume, area, stepVolume, stepArea = asymmetricJoinedVolume, asymmetricJoinedArea, asymmetricJoinedSTEPVolume, asymmetricJoinedSTEPArea
	}
	if shape == threeJoinedKinked {
		authorEnv, mode = "PLAINSOLID_OCCT_MAPPED_CHAIN_KINKED_JOINED_ROOFS_AUTHOR", "kinked-joined-independent-roofs"
		negativeMode = "-moved-unilateral-support"
		volume, area, stepVolume, stepArea = kinkedJoinedVolume, kinkedJoinedArea, kinkedJoinedVolume, kinkedJoinedArea
	}
	author := os.Getenv(authorEnv)
	reader := os.Getenv("PLAINSOLID_OCCT_MAPPED_CHAIN_STEP")
	if author == "" || reader == "" {
		t.Skip("optional pinned OCCT source author and STEP consumer")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	authorFile := func(suffix string) ([]byte, string) {
		t.Helper()
		path := filepath.Join(t.TempDir(), "three-joined"+suffix+".step")
		if data, err := exec.CommandContext(ctx, author, "write-"+mode+suffix, path).CombinedOutput(); err != nil {
			t.Fatalf("pinned OCCT author failed: %v\n%s", err, data)
		}
		return readInferredSharedMappedChainFixture(t, path), path
	}
	raw, _ := authorFile("")
	var canonical []byte
	var kinkedSource MappedChainSTEPModel
	if shape == threeJoinedKinked {
		requirePinnedMappedChainData(t, raw, kinkedJoinedRoofPath)
		opts := threeJoinedRoofOptions()
		var err error
		kinkedSource, err = ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(raw), Limits{}, opts)
		if err != nil {
			t.Fatal(err)
		}
		var output bytes.Buffer
		if err := WriteMappedChainRectangularOriginalShell(ctx, &output, kinkedSource, Limits{}, opts); err != nil {
			t.Fatal(err)
		}
		canonical = output.Bytes()
	} else {
		canonical = verifyThreeJoinedRoofSource(t, ctx, raw, shape)
	}
	verifyPinnedMappedChainTransfer(t, ctx, reader, raw, canonical, stepVolume, stepArea)
	data, err := exec.CommandContext(ctx, author, "measure-"+mode).CombinedOutput()
	if err != nil {
		t.Fatalf("could not measure independent OCCT material before STEP: %v\n%s", err, data)
	}
	var material struct {
		Faces, Edges, Vertices int
		Volume, Area           float64
	}
	if err := json.Unmarshal(data, &material); err != nil || material.Faces != 7 || material.Edges != edges || material.Vertices != vertices ||
		math.Abs(material.Volume-volume) > 1e-8 || math.Abs(material.Area-area) > 1e-8 {
		t.Fatal("the pinned original closed body and native material must agree", material, err)
	}
	t.Logf("OCCT before STEP volume %.15g, area %.15g", material.Volume, material.Area)
	bad, path := authorFile(negativeMode)
	data, err = exec.CommandContext(ctx, reader, "read", path).CombinedOutput()
	if err != nil {
		t.Fatalf("pinned OCCT could not reopen the independently moved support knot: %v\n%s", err, data)
	}
	var reopened struct{ Faces, Solids int }
	if err := json.Unmarshal(data, &reopened); err != nil || reopened.Faces != 7 || reopened.Solids != 1 {
		t.Fatal("the moved joint control must still be an independently valid seven-face solid", reopened, err)
	}
	switch shape {
	case threeJoinedKinked:
		requirePinnedMappedChainData(t, bad, kinkedJoinedMovedSupportPath)
		rejectKinkedJoinedMovedSupport(t, ctx, kinkedSource, bad)
	case threeJoinedAsymmetric:
		rejectAsymmetricJoinedRoofMissingJoint(t, ctx, raw, bad)
	default:
		rejectThreeJoinedRoofMissingJoint(t, ctx, raw, bad)
	}
}
