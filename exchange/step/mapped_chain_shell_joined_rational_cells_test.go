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
	"strings"
	"testing"
	"time"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/collision"
	"plainsolid/geom"
)

const joinedRationalCellsPath = "testdata/occt_mapped_chain_joined_rational_cells.step"
const joinedRationalMissingJointPath = "testdata/occt_mapped_chain_joined_rational_missing_joint.step"
const joinedIndependentRoofsPath = "testdata/occt_mapped_chain_joined_independent_roofs.step"
const joinedIndependentMissingJointPath = "testdata/occt_mapped_chain_joined_independent_missing_joint.step"
const joinedIndependentFalseWeightPath = "testdata/occt_mapped_chain_joined_independent_false_weight.step"

// Independent integration of the two exact rational charts gives these
// references. OCCT's STEP transferrer reports its own numerical properties.
const joinedRationalVolume = 0.635266080676929
const joinedRationalArea = 4.679335100741437
const joinedRationalOCCTVolume = 0.635267017379662
const joinedRationalOCCTArea = 4.679345521098269
const joinedIndependentRoofVolume = 0.640508273389408
const joinedIndependentRoofArea = 4.65185532305444
const joinedIndependentRoofOCCTVolume = 0.640509222445185
const joinedIndependentRoofOCCTArea = 4.651865358117015

func joinedRationalCellOptions() MappedChainShellSTEPOptions {
	return MappedChainShellSTEPOptions{AllowMultiSpanRoofs: true, AllowTwoInteriorRoofKnots: true,
		AllowUnsplitSupportKnot: true, AllowMixedOwnerBases: true, AllowJoinedSupportKnotCells: true}
}

func verifyJoinedRationalSource(t *testing.T, ctx context.Context, raw []byte, mirroredSouth bool) []byte {
	t.Helper()
	o := joinedRationalCellOptions()
	m, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(raw), Limits{}, o)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Chain) != 2 || !mappedChainJoinedSupportCells(m.Chain, m.Crossings) || m.SupportJoint == nil || len(m.Planes) != 5 {
		t.Fatal("the two adjacent original edges must keep their shared joint and distinct interior repairs")
	}
	bases := m.Shell.IndependentOwnerBases()
	if bases[0] == bases[1] {
		t.Fatal("the original two owner organizations must survive")
	}
	north := 0
	if !bases[0] {
		north = 1
	}
	south := 1 - north
	verifyJoinedRationalMaterial(t, ctx, m.Native, mirroredSouth)
	var ownerStops [2][2][3]int64
	ownerStops[north], ownerStops[south] = [2][3]int64{{2, 32, 167}, {700, 706, 715}}, [2][3]int64{{181, 126, 16}, {16, 6, 1}}
	cutRaw := [2]*big.Rat{big.NewRat(8, 7), big.NewRat(64, 5)}
	cutModel := [2]*big.Rat{big.NewRat(1, 28), big.NewRat(2, 5)}
	edges, vertices, faces := m.Native.Solid.Edges(), m.Native.Solid.Vertices(), m.Native.Solid.Faces()
	for i, cut := range m.Crossings {
		if cut.EdgeID != m.Chain[i].EdgeID || cut.SourceModelParameter.Cmp(cutRaw[i]) != 0 || cut.ModelStation.Cmp(cutModel[i]) != 0 ||
			cut.RawParameters[north].Cmp(big.NewRat(ownerStops[north][i][1], 1)) != 0 ||
			cut.RawParameters[south].Cmp(big.NewRat(ownerStops[south][i][1], 1)) != 0 ||
			cut.OwnersChangingCells[north] != (i == 0) || cut.OwnersChangingCells[south] != (i == 1) ||
			cut.NativeEdges != [2]int{m.Native.ChainEdges[2*i], m.Native.ChainEdges[2*i+1]} {
			t.Fatal("each original model curve must preserve its own exact asynchronous support event", i, cut)
		}
		s := [2]float64{.0625, .625}[i]
		x, y := .75*s, .25+.75*s*(1-s)
		a, b := edges[cut.NativeEdges[0]], edges[cut.NativeEdges[1]]
		if a.End != b.Start || vertices[a.End] != (geom.Vec3{X: x, Y: y, Z: x * y}) {
			t.Fatal("both repaired children must share one exact original model point", i)
		}
	}
	joint := m.SupportJoint
	jointPoint := geom.Vec3{X: 3.0 / 16, Y: 25.0 / 64, Z: 75.0 / 1024}
	if joint.ModelStation.Cmp(big.NewRat(1, 4)) != 0 || joint.SourceEdges != [2]int64{m.Chain[0].EdgeID, m.Chain[1].EdgeID} ||
		joint.SourceModelParameters[0].Cmp(big.NewRat(8, 1)) != 0 || joint.SourceModelParameters[1].Cmp(big.NewRat(8, 1)) != 0 ||
		joint.NativeEdges != [2]int{m.Native.ChainEdges[1], m.Native.ChainEdges[2]} ||
		edges[joint.NativeEdges[0]].End != edges[joint.NativeEdges[1]].Start || vertices[edges[joint.NativeEdges[0]].End] != jointPoint {
		t.Fatal("the original shared vertex must remain distinct from both new support cuts", joint)
	}
	for owner, stops := range ownerStops {
		if joint.OwnerParameters[owner][0].Cmp(big.NewRat(stops[0][2], 1)) != 0 ||
			joint.OwnerParameters[owner][1].Cmp(big.NewRat(stops[1][0], 1)) != 0 {
			t.Fatal("each original owner must retain both adjacent raw endpoints at the joint", owner, joint)
		}
	}
	f, err := Parse(bytes.NewReader(raw), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	sourceJoint, err := mappedChainStepVertex(f, joint.SourceVertexID)
	if err != nil {
		t.Fatal(err)
	}
	q, err := geom.RepresentedExactPoint(sourceJoint)
	if err != nil || q != jointPoint {
		t.Fatal("the reported support joint must own the actual source vertex", err)
	}
	verifyOriginalNonplanarModelBezier(t, f, m.Chain[0].ModelCurveID, true)
	verifyJoinedSecondRationalModel(t, f, m.Chain[1].ModelCurveID)

	modelCuts := [2]*big.Rat{big.NewRat(1, 7), big.NewRat(1, 5)}
	var weights [2][2]*big.Rat
	weights[north], weights[south] = [2]*big.Rat{big.NewRat(4, 3), big.NewRat(8, 3)}, [2]*big.Rat{big.NewRat(3, 1), big.NewRat(8, 1)}
	for owner, stops := range ownerStops {
		for cell, model := range m.Chain {
			original, err := originalMappedChainStepOwner(f, model.Owners[owner], true)
			if err != nil {
				t.Fatal(err)
			}
			for child := range 2 {
				q := faces[owner].Outer[2*cell+child].Curve.(trimmed.MappedSplineUV)
				if !sameMixedMappedChainBasis(q.Source().Curve, original.Curve) {
					t.Fatal("both native halves must retain the basis of their own original coedge", owner, cell, child)
				}
				for endpoint, want := range stops[cell][child : child+2] {
					got, err := q.AuthoredParameter(big.NewRat(int64(endpoint), 1))
					if err != nil || got.Cmp(big.NewRat(want, 1)) != 0 {
						t.Fatal("the paired child restrictions must retain the exact original raw endpoints", owner, cell, child, endpoint, err)
					}
				}
				modelEnds := [3]*big.Rat{new(big.Rat), modelCuts[cell], big.NewRat(1, 1)}
				mid := new(big.Rat).Quo(new(big.Rat).Add(modelEnds[child], modelEnds[child+1]), big.NewRat(2, 1))
				numerator := new(big.Rat).Mul(weights[owner][cell], mid)
				denominator := new(big.Rat).Add(new(big.Rat).Sub(big.NewRat(1, 1), mid), numerator)
				want := new(big.Rat).Mul(big.NewRat(stops[cell][2]-stops[cell][0], 1), new(big.Rat).Quo(numerator, denominator))
				want.Add(want, big.NewRat(stops[cell][0], 1))
				got, err := q.AuthoredParameter(big.NewRat(1, 2))
				if err != nil || got.Cmp(want) != 0 {
					t.Fatal("both distinct owner clocks on each original model curve must hold at interior parameters", owner, cell, child, err)
				}
			}
		}
	}
	northRoof := faces[north].Surface.(*geom.BSplineSurface)
	southRoof := faces[south].Surface.(*geom.BSplineSurface)
	southSlope := big.NewRat(-765, 1664)
	if mirroredSouth {
		southSlope = big.NewRat(-765, 4736)
		verifyRationalRoofSection(t, ctx, rationalRoofSectionWitness{northRoof, big.NewRat(1, 2), big.NewRat(363, 1792),
			[4]*big.Rat{new(big.Rat), big.NewRat(-141, 512), big.NewRat(231, 512), big.NewRat(9, 8)}, false, false})
		verifyRationalRoofSection(t, ctx, rationalRoofSectionWitness{southRoof, big.NewRat(1, 8), big.NewRat(213, 18944),
			[4]*big.Rat{new(big.Rat), big.NewRat(3, 512), big.NewRat(231, 512), big.NewRat(9, 32)}, true, true})
	}
	verifyMappedRoofSlopeJump(t, ctx, northRoof, 0, 3.0/16, big.NewRat(1, 2), big.NewRat(-45, 128))
	verifyMappedRoofSlopeJump(t, ctx, southRoof, 1, 3.0/16, big.NewRat(1, 8), southSlope)
	verifyMappedRoofSlopeJump(t, ctx, northRoof, 0, 3.0/16, big.NewRat(25, 64), new(big.Rat))
	verifyMappedRoofSlopeJump(t, ctx, southRoof, 1, 3.0/16, big.NewRat(25, 64), new(big.Rat))

	var recipe, canonical, replay bytes.Buffer
	nativeOptions := trimmed.MappedChainShellNativeOptions{}
	if err := m.Shell.WriteJSON(ctx, &recipe, nativeOptions); err != nil || !bytes.Contains(recipe.Bytes(), []byte(`"Boundary":{"Version":11,`)) {
		t.Fatal("v11 must identify the existing joint and both new interior support events", err)
	}
	shell, err := trimmed.ReadMappedSplineChainShellJSON(ctx, bytes.NewReader(recipe.Bytes()), nativeOptions)
	if err != nil {
		t.Fatal(err)
	}
	native, err := shell.BuildRectangularSolid(ctx, 1e-9, o.Geometry)
	if err != nil {
		t.Fatal(err)
	}
	verifyJoinedRationalMaterial(t, ctx, native, mirroredSouth)
	for i, want := range []*big.Rat{new(big.Rat), cutModel[0], joint.ModelStation, cutModel[1], big.NewRat(1, 1)} {
		if native.ChainStations[i].Cmp(want) != 0 {
			t.Fatal("source-only native replay must derive all old and new model stations exactly", i)
		}
	}
	if err := shell.WriteJSON(ctx, &replay, nativeOptions); err != nil || !bytes.Equal(recipe.Bytes(), replay.Bytes()) {
		t.Fatal("native source station replay must retain the same bytes", err)
	}
	if err := WriteMappedChainRectangularOriginalShell(ctx, &canonical, m, Limits{}, o); err != nil {
		t.Fatal(err)
	}
	strict, err := ReadMappedChainRectangularShell(ctx, bytes.NewReader(canonical.Bytes()), Limits{}, o)
	if err != nil || strict.SupportJoint == nil || len(strict.Chain) != 2 || len(strict.Crossings) != 2 || strict.Shell.IndependentOwnerBases() != bases {
		t.Fatal("strict STEP must re-prove both original rational curves and the original joint", err)
	}
	verifyOriginalMappedChainSTEPGraph(t, m, strict)
	verifyOriginalRationalRoofSurfaces(t, m, strict, mirroredSouth)
	transferredJoint := strict.SupportJoint
	if transferredJoint.SourceEdges != [2]int64{strict.Chain[0].EdgeID, strict.Chain[1].EdgeID} ||
		transferredJoint.NativeEdges != joint.NativeEdges || transferredJoint.ModelStation.Cmp(joint.ModelStation) != 0 {
		t.Fatal("strict canonical STEP must retain the original joint topology and normalized station")
	}
	for owner := range 2 {
		if transferredJoint.SourceModelParameters[owner].Cmp(joint.SourceModelParameters[owner]) != 0 {
			t.Fatal("strict STEP must retain both original raw model endpoints at the joint", owner)
		}
		for end := range 2 {
			if transferredJoint.OwnerParameters[owner][end].Cmp(joint.OwnerParameters[owner][end]) != 0 {
				t.Fatal("strict STEP must retain both original raw owner endpoints at the joint", owner, end)
			}
		}
	}
	replay.Reset()
	if err := strict.Shell.WriteJSON(ctx, &replay, nativeOptions); err != nil || !bytes.Equal(recipe.Bytes(), replay.Bytes()) {
		t.Fatal("canonical STEP must recreate the same native v11 source recipe", err)
	}
	replay.Reset()
	if err := WriteMappedChainRectangularOriginalShell(ctx, &replay, strict, Limits{}, o); err != nil || !bytes.Equal(canonical.Bytes(), replay.Bytes()) {
		t.Fatal("the canonical original graph must re-export byte-for-byte", err)
	}
	return canonical.Bytes()
}

func verifyJoinedSecondRationalModel(t *testing.T, f *File, id int64) {
	t.Helper()
	s, err := readMappedChainStepSpline(f, id, 3, 16, 17, true)
	if err != nil || s.degree != 3 || len(s.poles) != 4 || len(s.knots) != 8 {
		t.Fatal("the second original model curve must remain one exact rational cubic", err)
	}
	poles := [4][3]float64{
		{3.0 / 16, 25.0 / 64, 75.0 / 1024}, {3.0 / 8, 31.0 / 64, 21.0 / 128},
		{9.0 / 16, 7.0 / 16, 9.0 / 32}, {.75, .25, 3.0 / 16},
	}
	for i, pole := range poles {
		if s.knots[i].Cmp(big.NewRat(8, 1)) != 0 || s.knots[i+4].Cmp(big.NewRat(32, 1)) != 0 || s.weights[i].Cmp(big.NewRat([4]int64{1, 4, 16, 64}[i], 1)) != 0 {
			t.Fatal("the second cell must retain its own original model domain and projective weights", i)
		}
		for axis, value := range pole {
			if s.poles[i][axis].Cmp(mappedChainStepRat(value)) != 0 {
				t.Fatal("the second original nonplanar model poles must remain exact", i, axis)
			}
		}
	}
}

func verifyJoinedRationalMaterial(t *testing.T, ctx context.Context, native trimmed.MappedChainRectangularSolid, mirroredSouth bool) {
	t.Helper()
	s := native.Solid
	if len(native.ChainEdges) != 4 || len(s.Faces()) != 7 || len(s.Edges()) != 22 || len(s.Vertices()) != 17 {
		t.Fatal("the original join and two new support vertices must form one closed native solid")
	}
	proof, err := s.ValidateEdgeAgreement(ctx, edgecheck.Options{})
	if err != nil || !proof.Complete || len(proof.Uses) != 44 {
		t.Fatal("all original and repaired incidences must have complete continuous agreement", err)
	}
	volume, area := joinedRationalVolume, joinedRationalArea
	if mirroredSouth {
		volume, area = joinedIndependentRoofVolume, joinedIndependentRoofArea
	}
	mass, err := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
	if err != nil || math.Abs(mass.Volume-volume) > 1e-8 || math.Abs(mass.Area-area) > 1e-8 {
		t.Fatal("the native material must match integration of the exact original roof charts", mass, err)
	}
}

func TestOCCTMappedChainRectangularJoinedRationalSupportCells(t *testing.T) {
	verifyJoinedRationalSource(t, context.Background(), readInferredSharedMappedChainFixture(t, joinedRationalCellsPath), false)
}

func TestOCCTMappedChainRectangularJoinedRationalSupportCellsClosedDisplay(t *testing.T) {
	verifyJoinedRationalDisplay(t, readInferredSharedMappedChainFixture(t, joinedRationalCellsPath))
}

func verifyJoinedRationalDisplay(t *testing.T, raw []byte) {
	t.Helper()
	ctx := context.Background()
	m, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(raw), Limits{}, joinedRationalCellOptions())
	if err != nil {
		t.Fatal(err)
	}
	fill := nonplanarInferredDisplayOptions()
	fill.Face.BoundaryModelBudget, fill.Face.Interior.Deflection = .03, .14
	display := verifyMappedChainCutDisplayWithClosedOptions(t, ctx, m, trimmed.MappedChainClosedMeshOptions{
		Boundary:  trimmed.MappedChainPolylineOptions{Deflection: .01875},
		Curved:    [2]trimmed.MixedExactMultiSpanMeshOptions{fill, fill},
		Collision: collision.Options{MaxPairs: 4_000_000},
	})
	if len(display.ChainJoints) != 1 {
		t.Fatal("the display must retain both original occurrences of the one shared joint")
	}
	joint := m.SupportJoint
	a, b := display.ChainJoints[0][0], display.ChainJoints[0][1]
	if a.Vertex != b.Vertex || a.ModelParameter.Cmp(joint.ModelStation) != 0 || b.ModelParameter.Cmp(joint.ModelStation) != 0 ||
		a.AuthoredParts != [2]int{0, 0} || b.AuthoredParts != [2]int{1, 1} {
		t.Fatal("one welded display vertex must own both original coedge events")
	}
	for owner, raw := range joint.OwnerParameters {
		if a.AuthoredParameters[owner].Cmp(raw[0]) != 0 || b.AuthoredParameters[owner].Cmp(raw[1]) != 0 {
			t.Fatal("both exact one-sided original raw parameters must appear in the display ledger", owner)
		}
	}
}

func TestOCCTMappedChainRectangularJoinedRationalSupportCellsRejectsMissingJoint(t *testing.T) {
	raw := readInferredSharedMappedChainFixture(t, joinedRationalCellsPath)
	badSource := readInferredSharedMappedChainFixture(t, joinedRationalMissingJointPath)
	rejectJoinedRationalMissingJoint(t, context.Background(), raw, badSource)
}

func rejectJoinedRationalMissingJoint(t *testing.T, ctx context.Context, raw, badSource []byte) {
	t.Helper()
	o := joinedRationalCellOptions()
	bad, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(badSource), Limits{}, o)
	if !errors.Is(err, geom.ErrConvergence) || len(bad.Crossings) != 0 || bad.SupportJoint != nil || len(bad.Native.ChainEdges) != 0 {
		t.Fatal("the genuine different south support arrangement must reject without publishing a repaired shell", err)
	}
	rejectChangedJoinedRationalNativeSupport(t, ctx, raw, badSource, "did not cross support at original joint")
}

func rejectChangedJoinedRationalNativeSupport(t *testing.T, ctx context.Context, raw, badSource []byte, diagnosis string) {
	t.Helper()
	rejectChangedJoinedRationalNativeSupportWithOptions(t, ctx, raw, badSource, joinedRationalCellOptions(), false, diagnosis)
}

func rejectChangedJoinedRationalNativeSupportWithOptions(t *testing.T, ctx context.Context, raw, badSource []byte, o MappedChainShellSTEPOptions, north bool, diagnosis string) {
	t.Helper()
	m, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(raw), Limits{}, o)
	if err != nil {
		t.Fatal(err)
	}
	rejectChangedNativeJoinedRoof(t, ctx, m, badSource, north, diagnosis)
}

func rejectChangedNativeJoinedRoof(t *testing.T, ctx context.Context, m MappedChainSTEPModel, badSource []byte, north bool, diagnosis string) {
	t.Helper()
	owner := 0
	if m.Shell.IndependentOwnerBases()[0] != north {
		owner = 1
	}
	changedFile, err := Parse(bytes.NewReader(badSource), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	changedID := rationalRoofSupportID(t, changedFile, m.FaceIDs[owner])
	changed, err := decodeMappedChainStepSurface(changedFile, changedID, 4)
	if err != nil {
		t.Fatal(err)
	}
	before, err := parametric.EncodeSurface(m.Native.Solid.Faces()[owner].Surface)
	if err != nil {
		t.Fatal(err)
	}
	after, err := parametric.EncodeSurface(changed)
	if err != nil {
		t.Fatal(err)
	}
	var recipe bytes.Buffer
	nativeOptions := trimmed.MappedChainShellNativeOptions{}
	if err := m.Shell.WriteJSON(ctx, &recipe, nativeOptions); err != nil || !bytes.Contains(recipe.Bytes(), before) {
		t.Fatal("the exact original owner chart must be present in the native recipe", err)
	}
	altered := bytes.Replace(recipe.Bytes(), before, after, 1)
	reopened, err := trimmed.ReadMappedSplineChainShellJSON(ctx, bytes.NewReader(altered), nativeOptions)
	if !errors.Is(err, geom.ErrConvergence) || !strings.Contains(err.Error(), diagnosis) || reopened.Topology().Faces != 0 {
		t.Fatal("native reproof must reject the substituted source chart atomically", err)
	}
}

func TestOCCTMappedChainRectangularJoinedRationalSupportCellsLive(t *testing.T) {
	verifyPinnedJoinedRationalSource(t, os.Getenv("PLAINSOLID_OCCT_MAPPED_CHAIN_JOINED_RATIONAL_AUTHOR"), false)
}

func TestOCCTMappedChainRectangularJoinedIndependentRoofs(t *testing.T) {
	raw := readInferredSharedMappedChainFixture(t, joinedIndependentRoofsPath)
	verifyJoinedRationalSource(t, context.Background(), raw, true)
}

func TestOCCTMappedChainRectangularJoinedIndependentRoofsClosedDisplay(t *testing.T) {
	verifyJoinedRationalDisplay(t, readInferredSharedMappedChainFixture(t, joinedIndependentRoofsPath))
}

func TestOCCTMappedChainRectangularJoinedIndependentRoofsRejectsMissingJoint(t *testing.T) {
	raw := readInferredSharedMappedChainFixture(t, joinedIndependentRoofsPath)
	bad := readInferredSharedMappedChainFixture(t, joinedIndependentMissingJointPath)
	rejectJoinedRationalMissingJoint(t, context.Background(), raw, bad)
}

func TestOCCTMappedChainRectangularJoinedIndependentRoofsRejectsFalseWeight(t *testing.T) {
	raw := readInferredSharedMappedChainFixture(t, joinedIndependentRoofsPath)
	bad := readInferredSharedMappedChainFixture(t, joinedIndependentFalseWeightPath)
	ctx := context.Background()
	rejectFalseRationalRoofWeight(t, ctx, raw, bad, joinedRationalCellOptions(), 1)
	rejectChangedJoinedRationalNativeSupport(t, ctx, raw, bad, "")
}

func TestOCCTMappedChainRectangularJoinedIndependentRoofsLive(t *testing.T) {
	verifyPinnedJoinedRationalSource(t, os.Getenv("PLAINSOLID_OCCT_MAPPED_CHAIN_INDEPENDENT_JOINED_ROOFS_AUTHOR"), true)
}

func verifyPinnedJoinedRationalSource(t *testing.T, author string, mirroredSouth bool) {
	t.Helper()
	reader := os.Getenv("PLAINSOLID_OCCT_MAPPED_CHAIN_STEP")
	if author == "" || reader == "" {
		t.Skip("optional pinned OCCT source author and STEP consumer")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	stem := "joined-rational-cells"
	volume, area := joinedRationalVolume, joinedRationalArea
	occtVolume, occtArea := joinedRationalOCCTVolume, joinedRationalOCCTArea
	if mirroredSouth {
		stem = "joined-independent-roofs"
		volume, area = joinedIndependentRoofVolume, joinedIndependentRoofArea
		occtVolume, occtArea = joinedIndependentRoofOCCTVolume, joinedIndependentRoofOCCTArea
	}
	authorFile := func(suffix string) ([]byte, string) {
		mode := "write-" + stem + suffix
		path := filepath.Join(t.TempDir(), mode+".step")
		if output, err := exec.CommandContext(ctx, author, mode, path).CombinedOutput(); err != nil {
			t.Fatalf("pinned OCCT author failed: %v\n%s", err, output)
		}
		return readInferredSharedMappedChainFixture(t, path), path
	}
	raw, _ := authorFile("")
	canonical := verifyJoinedRationalSource(t, ctx, raw, mirroredSouth)
	verifyPinnedMappedChainTransfer(t, ctx, reader, raw, canonical, occtVolume, occtArea)
	beforeSTEP, err := exec.CommandContext(ctx, author, "measure-"+stem).CombinedOutput()
	if err != nil {
		t.Fatalf("pinned OCCT could not measure the original body before STEP transfer: %v\n%s", err, beforeSTEP)
	}
	var material struct {
		Volume, Area float64
		Faces        int
	}
	if err := json.Unmarshal(beforeSTEP, &material); err != nil || material.Faces != 7 ||
		math.Abs(material.Volume-volume) > 1e-8 || math.Abs(material.Area-area) > 5e-7 {
		t.Fatal("the original constructed OCCT body must agree with independently integrated source material", material, err)
	}
	t.Logf("OCCT before STEP volume %.15g, area %.15g", material.Volume, material.Area)
	movedJoint, path := authorFile("-missing-south-joint")
	if output, err := exec.CommandContext(ctx, reader, "read", path).CombinedOutput(); err != nil {
		t.Fatalf("pinned OCCT could not reopen the different support arrangement: %v\n%s", err, output)
	}
	bad, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(movedJoint), Limits{}, joinedRationalCellOptions())
	if !errors.Is(err, geom.ErrConvergence) || bad.SupportJoint != nil || len(bad.Crossings) != 0 || len(bad.Native.ChainEdges) != 0 {
		t.Fatal("the live different chart must reject without publishing the original joint or repaired topology", err)
	}
	if mirroredSouth {
		falseWeight, path := authorFile("-false-south-weight")
		if output, err := exec.CommandContext(ctx, reader, "read", path).CombinedOutput(); err != nil {
			t.Fatalf("pinned OCCT could not reopen the changed support weight: %v\n%s", err, output)
		}
		rejectFalseRationalRoofWeight(t, ctx, raw, falseWeight, joinedRationalCellOptions(), 1)
	}
}
