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

const orderedJoinedRoofPath = "testdata/occt_mapped_chain_ordered_joined_independent_roofs.step"
const orderedJoinedMovedJointPath = "testdata/occt_mapped_chain_ordered_joined_moved_joint.step"
const orderedJoinedVolume = 0.6448671346810797
const orderedJoinedArea = 4.6513347531185012

func orderedJoinedRoofOptions() MappedChainShellSTEPOptions {
	o := joinedRationalCellOptions()
	o.AllowOrderedJoinedSupportKnotCells = true
	return o
}

func orderedJoinedModelPoint(t float64) geom.Vec3 {
	ends := [5]float64{.25, 11.0 / 32, 19.0 / 64, 25.0 / 64, .25}
	cell := min(3, int(4*t))
	x := .75 * t
	y := ends[cell] + (4*t-float64(cell))*(ends[cell+1]-ends[cell])
	return geom.Vec3{X: x, Y: y, Z: x * y}
}

func verifyOrderedJoinedSource(t *testing.T, ctx context.Context, raw []byte) []byte {
	t.Helper()
	o := orderedJoinedRoofOptions()
	m, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(raw), Limits{}, o)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Chain) != 4 || len(m.Crossings) != 4 || len(m.SupportJoints) != 3 || m.SupportJoint != nil {
		t.Fatal("every original model cell and joint must retain its own ordered source evidence")
	}
	north := 0
	if !m.Shell.IndependentOwnerBases()[0] {
		north = 1
	}
	south := 1 - north
	if m.Shell.IndependentOwnerBases()[south] {
		t.Fatal("the single original south basis must remain shared by all coedges")
	}
	var ownerStops [2][4][3]int64
	ownerStops[north] = [4][3]int64{{2, 68, 167}, {700, 706, 715}, {850, 859, 862}, {900, 909, 915}}
	ownerStops[south] = [4][3]int64{{181, 82, 16}, {16, 6, 1}, {1, -1, -2}, {-2, -6, -7}}
	var weights [2][4]*big.Rat
	weights[north] = [4]*big.Rat{big.NewRat(4, 3), big.NewRat(8, 3), big.NewRat(24, 1), big.NewRat(24, 1)}
	weights[south] = [4]*big.Rat{big.NewRat(3, 1), big.NewRat(8, 1), big.NewRat(16, 1), big.NewRat(64, 1)}
	wantEvents := make([]trimmed.MappedSplineChainSupportEvent, 7)
	for i, station := range []*big.Rat{big.NewRat(1, 12), big.NewRat(1, 4), big.NewRat(3, 10),
		big.NewRat(1, 2), big.NewRat(19, 36), big.NewRat(3, 4), big.NewRat(13, 17)} {
		wantEvents[i].ModelStation = station
		wantEvents[i].OriginalJoint = i%2 == 1
	}
	for _, i := range []int{0, 1, 3, 4} {
		wantEvents[i].OwnersChangingCells[north] = true
	}
	for _, i := range []int{1, 2, 5, 6} {
		wantEvents[i].OwnersChangingCells[south] = true
	}
	verifyOrderedJoinedEvents(t, m.Shell, wantEvents)
	// Verify the published event view cannot modify the proof's exact stations.
	view := m.Shell.JoinedSupportEvents()
	view[0].ModelStation.SetInt64(0)
	verifyOrderedJoinedEvents(t, m.Shell, wantEvents)

	f, err := Parse(bytes.NewReader(raw), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	edges, vertices, faces := m.Native.Solid.Edges(), m.Native.Solid.Vertices(), m.Native.Solid.Faces()
	if len(edges) != 30 || len(vertices) != 25 || len(faces) != 7 || len(m.Native.ChainEdges) != 8 {
		t.Fatal("exact repair must add four shared vertices and retain the authored seven-face solid")
	}
	agreement, err := m.Native.Solid.ValidateEdgeAgreement(ctx, edgecheck.Options{})
	if err != nil || !agreement.Complete || len(agreement.Uses) != 60 {
		t.Fatal("every original and repaired edge requires complete paired incidence proof", err)
	}
	for i, cell := range m.Chain {
		lo, hi := int64(8*i), int64(8*(i+1))
		a, b := orderedJoinedModelPoint(float64(i)/4), orderedJoinedModelPoint(float64(i+1)/4)
		ratio := int64(2 << i)
		verifyJoinedOriginalModelBezier(t, f, cell.ModelCurveID, lo, hi, ratio, a, b, (a.Y+b.Y)/2)
		cut, event := m.Crossings[i], wantEvents[2*i]
		rawStation := new(big.Rat).Mul(big.NewRat(32, 1), event.ModelStation)
		pair := [2]int{m.Native.ChainEdges[2*i], m.Native.ChainEdges[2*i+1]}
		if cut.EdgeID != cell.EdgeID || cut.StoredModelKnot || cut.ModelStation.Cmp(event.ModelStation) != 0 ||
			cut.SourceModelParameter.Cmp(rawStation) != 0 || cut.OwnersChangingCells != event.OwnersChangingCells ||
			cut.NativeEdges != pair || edges[pair[0]].End != edges[pair[1]].Start ||
			vertices[edges[pair[0]].End] != orderedJoinedModelPoint(float64(2*i+1)/8) {
			t.Fatal("each interior event must repair its own model cell at the exact original paired station", i, cut)
		}
		for owner, stops := range ownerStops {
			if cut.RawParameters[owner].Cmp(big.NewRat(stops[i][1], 1)) != 0 {
				t.Fatal("the original raw owner cut has changed", owner, i)
			}
			basis, err := originalMappedChainStepOwner(f, cell.Owners[owner], true)
			if err != nil {
				t.Fatal(err)
			}
			local := [3]*big.Rat{new(big.Rat), big.NewRat(1, ratio+1), big.NewRat(1, 1)}
			for part := range 2 {
				q := faces[owner].Outer[2*i+part].Curve.(trimmed.MappedSplineUV)
				if !sameMixedMappedChainBasis(basis.Curve, q.Source().Curve) {
					t.Fatal("both restrictions must retain their original coedge basis", owner, i, part)
				}
				mid := new(big.Rat).Quo(new(big.Rat).Add(local[part], local[part+1]), big.NewRat(2, 1))
				n := new(big.Rat).Mul(weights[owner][i], mid)
				d := new(big.Rat).Add(new(big.Rat).Sub(big.NewRat(1, 1), mid), n)
				wantMid := new(big.Rat).Add(big.NewRat(stops[i][0], 1),
					new(big.Rat).Mul(big.NewRat(stops[i][2]-stops[i][0], 1), new(big.Rat).Quo(n, d)))
				for sample, want := range []*big.Rat{big.NewRat(stops[i][part], 1), wantMid, big.NewRat(stops[i][part+1], 1)} {
					got, err := q.AuthoredParameter(big.NewRat(int64(sample), 2))
					if err != nil || got.Cmp(want) != 0 {
						t.Fatal("a paired restriction must use its original projective raw parameter map", owner, i, part, sample, got, err)
					}
				}
			}
		}
	}
	for i, joint := range m.SupportJoints {
		event := wantEvents[2*i+1]
		pair := [2]int{m.Native.ChainEdges[2*i+1], m.Native.ChainEdges[2*i+2]}
		if joint.SourceEdges != [2]int64{m.Chain[i].EdgeID, m.Chain[i+1].EdgeID} ||
			joint.ModelStation.Cmp(event.ModelStation) != 0 || joint.OwnersChangingCells != event.OwnersChangingCells ||
			joint.NativeEdges != pair || edges[pair[0]].End != edges[pair[1]].Start {
			t.Fatal("each retained original joint must keep its event, raw topology and adjacent native coedges", i, joint)
		}
		source, err := mappedChainStepVertex(f, joint.SourceVertexID)
		if err != nil {
			t.Fatal(err)
		}
		p, err := geom.RepresentedExactPoint(source)
		if err != nil || p != orderedJoinedModelPoint(float64(i+1)/4) || p != vertices[edges[pair[0]].End] {
			t.Fatal("the retained joint must use the actual original vertex", i, p, err)
		}
		for owner, rawSides := range joint.OwnerParameters {
			if rawSides[0].Cmp(big.NewRat(ownerStops[owner][i][2], 1)) != 0 ||
				rawSides[1].Cmp(big.NewRat(ownerStops[owner][i+1][0], 1)) != 0 ||
				joint.SourceModelParameters[owner].Cmp(big.NewRat(int64(8*(i+1)), 1)) != 0 {
				t.Fatal("the raw endpoints of the retained original joints must remain exact", i, owner)
			}
		}
	}
	verifyOrderedJoinedRoofs(t, ctx, m.Native, north)
	mass, err := m.Native.Solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
	if err != nil || math.Abs(mass.Volume-orderedJoinedVolume) > 1e-8 || math.Abs(mass.Area-orderedJoinedArea) > 1e-8 {
		t.Fatal("the native roof material must match the independent OCCT source", mass, err)
	}
	var recipe, replay, canonical bytes.Buffer
	nativeOpts := trimmed.MappedChainShellNativeOptions{}
	if err := m.Shell.WriteJSON(ctx, &recipe, nativeOpts); err != nil || !bytes.Contains(recipe.Bytes(), []byte(`"Boundary":{"Version":13,`)) {
		t.Fatal("four original cells must use the ordered v13 receipt", err)
	}
	native, err := trimmed.ReadMappedSplineChainShellJSON(ctx, bytes.NewReader(recipe.Bytes()), nativeOpts)
	if err != nil {
		t.Fatal(err)
	}
	verifyOrderedJoinedEvents(t, native, wantEvents)
	body, err := native.BuildRectangularSolid(ctx, 1e-9, o.Geometry)
	if err != nil {
		t.Fatal(err)
	}
	verifyOrderedJoinedRoofs(t, ctx, body, north)
	if err := native.WriteJSON(ctx, &replay, nativeOpts); err != nil || !bytes.Equal(recipe.Bytes(), replay.Bytes()) {
		t.Fatal("v13 must reprove and retain the same native receipt", err)
	}
	if err := WriteMappedChainRectangularOriginalShell(ctx, &canonical, m, Limits{}, o); err != nil {
		t.Fatal(err)
	}
	strict, err := ReadMappedChainRectangularShell(ctx, bytes.NewReader(canonical.Bytes()), Limits{}, o)
	if err != nil || len(strict.Chain) != 4 || len(strict.Crossings) != 4 || len(strict.SupportJoints) != 3 {
		t.Fatal("strict STEP must reprove the original four-cell source", err)
	}
	verifyOriginalMappedChainSTEPGraph(t, m, strict)
	verifyOriginalRationalRoofSurfaceProfiles(t, m, strict, []int64{1, 1, 2, 4, 7}, []int64{7, 4, 2, 1, 1})
	verifyOrderedJoinedEvents(t, strict.Shell, wantEvents)
	verifyOrderedJoinedRoofs(t, ctx, strict.Native, north)
	for i, joint := range strict.SupportJoints {
		original := m.SupportJoints[i]
		if joint.SourceEdges != [2]int64{strict.Chain[i].EdgeID, strict.Chain[i+1].EdgeID} ||
			joint.NativeEdges != original.NativeEdges || joint.OwnersChangingCells != original.OwnersChangingCells ||
			joint.ModelStation.Cmp(original.ModelStation) != 0 {
			t.Fatal("strict STEP must retain the ancestry of every original joint", i, joint)
		}
		for owner, raw := range joint.OwnerParameters {
			if joint.SourceModelParameters[owner].Cmp(original.SourceModelParameters[owner]) != 0 ||
				raw[0].Cmp(original.OwnerParameters[owner][0]) != 0 || raw[1].Cmp(original.OwnerParameters[owner][1]) != 0 {
				t.Fatal("strict STEP must retain both raw endpoints on both original owners", i, owner)
			}
		}
	}
	replay.Reset()
	if err := strict.Shell.WriteJSON(ctx, &replay, nativeOpts); err != nil || !bytes.Equal(recipe.Bytes(), replay.Bytes()) {
		t.Fatal("strict STEP must reconstruct the exact original source recipe and event order", err)
	}
	replay.Reset()
	if err := WriteMappedChainRectangularOriginalShell(ctx, &replay, strict, Limits{}, o); err != nil || !bytes.Equal(canonical.Bytes(), replay.Bytes()) {
		t.Fatal("canonical original STEP must re-export byte-for-byte", err)
	}
	return canonical.Bytes()
}

func verifyOrderedJoinedEvents(t *testing.T, shell trimmed.MappedSplineChainShell, want []trimmed.MappedSplineChainSupportEvent) {
	t.Helper()
	actual := shell.JoinedSupportEvents()
	if !slices.EqualFunc(actual, want, func(a, b trimmed.MappedSplineChainSupportEvent) bool {
		return a.ModelStation.Cmp(b.ModelStation) == 0 && a.OwnersChangingCells == b.OwnersChangingCells && a.OriginalJoint == b.OriginalJoint
	}) {
		t.Fatal("the certificate must preserve each original joint and each inserted interior event in exact order", actual)
	}
}

func verifyOrderedJoinedRoofs(t *testing.T, ctx context.Context, native trimmed.MappedChainRectangularSolid, north int) {
	t.Helper()
	faces := native.Solid.Faces()
	for _, tc := range []struct {
		face, axis int
		widthKnots []float64
		x, y, z    *big.Rat
	}{
		{north, 0, []float64{0, 3.0 / 32, 3.0 / 16, 3.0 / 8, 15.0 / 32, .75}, big.NewRat(15, 32), big.NewRat(1, 2), big.NewRat(21795, 90112)},
		{1 - north, 1, []float64{0, 3.0 / 16, 9.0 / 32, 9.0 / 16, 21.0 / 32, .75}, big.NewRat(9, 32), big.NewRat(1, 8), big.NewRat(12303, 253952)},
	} {
		face := faces[tc.face]
		patch, err := geom.NewSurfacePatch(face.Surface.(*geom.BSplineSurface), geom.SurfacePatchOptions{})
		if err != nil {
			t.Fatal(err)
		}
		ku, kv := patch.KnotBreaks()
		if !slices.Equal([2][]float64{ku, kv}[tc.axis], tc.widthKnots) {
			t.Fatal("each unchanged owner must stay in its original span at its unilateral joint", tc.face)
		}
		u, v := tc.x, tc.y
		if tc.axis == 1 {
			u, v = v, u
		}
		point, err := patch.EvaluateRational(ctx, u, v, geom.SurfacePatchEvaluationOptions{})
		if err != nil || point.Point[0].Cmp(tc.x) != 0 || point.Point[1].Cmp(tc.y) != 0 || point.Point[2].Cmp(tc.z) != 0 {
			t.Fatal("the original weights must retain the affine projection and exact off-seam height", tc.face, point.Point, err)
		}
		for i, want := range [3][2][2]*big.Rat{
			{{big.NewRat(1, 2), big.NewRat(7, 16)}, {big.NewRat(-1, 4), big.NewRat(19, 64)}},
			{{big.NewRat(-1, 4), big.NewRat(13, 64)}, {big.NewRat(1, 2), big.NewRat(31, 64)}},
			{{big.NewRat(1, 2), big.NewRat(43, 64)}, {big.NewRat(-3, 4), big.NewRat(-1, 32)}},
		} {
			verifyJoinedOneSidedTangents(t, ctx, face, 2*i+1, want)
		}
	}
}

func TestOCCTMappedChainRectangularOrderedJoinedRoofs(t *testing.T) {
	slowtest.Skip(t)
	verifyOrderedJoinedSource(t, context.Background(), readInferredSharedMappedChainFixture(t, orderedJoinedRoofPath))
}

func TestOCCTMappedChainRectangularOrderedJoinedRoofsClosedDisplay(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	raw := readInferredSharedMappedChainFixture(t, orderedJoinedRoofPath)
	m, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(raw), Limits{}, orderedJoinedRoofOptions())
	if err != nil {
		t.Fatal(err)
	}
	fill := nonplanarInferredDisplayOptions()
	fill.Span.MaxDegree = 4
	fill.MaxResidualOperations = 512 << 20
	fill.Face.BoundaryModelBudget = .065
	fill.Face.Interior.Deflection = .36
	fill.Face.Interior.MaxSubdivisions = 64
	verifyJoinedClosedDisplay(t, ctx, m, fill)
}

func TestOCCTMappedChainRectangularOrderedJoinedRoofsRejectsMovedJoint(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	raw := readInferredSharedMappedChainFixture(t, orderedJoinedRoofPath)
	older := threeJoinedRoofOptions()
	older.AllowUpToSixSupportKnots = true
	if bad, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(raw), Limits{}, older); !errors.Is(err, geom.ErrConvergence) ||
		len(bad.SupportJoints) != 0 || len(bad.Native.ChainEdges) != 0 {
		t.Fatal("the old joined proof must still require the new four-cell opt-in", err)
	}
	bad := readInferredSharedMappedChainFixture(t, orderedJoinedMovedJointPath)
	rejectOrderedJoinedMovedJoint(t, ctx, raw, bad)
}

func rejectOrderedJoinedMovedJoint(t *testing.T, ctx context.Context, raw, changed []byte) {
	t.Helper()
	const diagnosis = "original joint 1 has no transverse support change"
	o := orderedJoinedRoofOptions()
	bad, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(changed), Limits{}, o)
	if !errors.Is(err, geom.ErrConvergence) || !strings.Contains(err.Error(), diagnosis) ||
		len(bad.SupportJoints) != 0 || len(bad.Crossings) != 0 || len(bad.Native.ChainEdges) != 0 {
		t.Fatal("the four interior roots must still reject a missing transverse original joint atomically", err)
	}
	rejectChangedJoinedRationalNativeSupportWithOptions(t, ctx, raw, changed, o, true, diagnosis)
}

func TestOCCTMappedChainRectangularOrderedJoinedRoofsRejectsFalseReceipt(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	raw := readInferredSharedMappedChainFixture(t, orderedJoinedRoofPath)
	m, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(raw), Limits{}, orderedJoinedRoofOptions())
	if err != nil {
		t.Fatal(err)
	}
	o := trimmed.MappedChainShellNativeOptions{}
	var recipe bytes.Buffer
	if err := m.Shell.WriteJSON(ctx, &recipe, o); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, original, changed string }{
		{"false joint mask", `"ModelStation":"1/4","Owners":3`, `"ModelStation":"1/4","Owners":2`},
		{"false joint kind", `"ModelStation":"1/4","Owners":3,"Kind":"joint"`, `"ModelStation":"1/4","Owners":3,"Kind":"interior"`},
		{"moved interior station", `"ModelStation":"19/36"`, `"ModelStation":"19/37"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if bytes.Count(recipe.Bytes(), []byte(tc.original)) != 1 {
				t.Fatal("each forged receipt value must correspond to an actual recorded source event")
			}
			altered := bytes.Replace(recipe.Bytes(), []byte(tc.original), []byte(tc.changed), 1)
			shell, err := trimmed.ReadMappedSplineChainShellJSON(ctx, bytes.NewReader(altered), o)
			if !errors.Is(err, geom.ErrConvergence) || shell.Topology().Faces != 0 {
				t.Fatal("reopen must recompute the ordered event evidence and reject a forged declaration", err)
			}
		})
	}
}

func TestOCCTMappedChainRectangularOrderedJoinedRoofsLive(t *testing.T) {
	t.Parallel()
	author := os.Getenv("PLAINSOLID_OCCT_MAPPED_CHAIN_ORDERED_JOINED_ROOFS_AUTHOR")
	reader := os.Getenv("PLAINSOLID_OCCT_MAPPED_CHAIN_STEP")
	if author == "" || reader == "" {
		t.Skip("optional pinned OCCT source author and STEP consumer")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Second)
	defer cancel()
	authorFile := func(suffix string) ([]byte, string) {
		t.Helper()
		mode := "write-ordered-joined-independent-roofs" + suffix
		path := filepath.Join(t.TempDir(), mode+".step")
		if data, err := exec.CommandContext(ctx, author, mode, path).CombinedOutput(); err != nil {
			t.Fatalf("pinned OCCT author failed: %v\n%s", err, data)
		}
		return readInferredSharedMappedChainFixture(t, path), path
	}
	raw, _ := authorFile("")
	canonical := verifyOrderedJoinedSource(t, ctx, raw)
	verifyPinnedMappedChainTransfer(t, ctx, reader, raw, canonical, orderedJoinedVolume, orderedJoinedArea)
	data, err := exec.CommandContext(ctx, author, "measure-ordered-joined-independent-roofs").CombinedOutput()
	if err != nil {
		t.Fatalf("could not measure the original OCCT body before STEP: %v\n%s", err, data)
	}
	var props struct {
		Faces, Edges, Vertices int
		Volume, Area           float64
	}
	if err := json.Unmarshal(data, &props); err != nil || props.Faces != 7 || props.Edges != 26 || props.Vertices != 21 ||
		math.Abs(props.Volume-orderedJoinedVolume) > 1e-8 || math.Abs(props.Area-orderedJoinedArea) > 1e-8 {
		t.Fatal("the original pinned authored solid must agree with the exact rebuilt material", props, err)
	}
	bad, path := authorFile("-moved-joint")
	data, err = exec.CommandContext(ctx, reader, "read", path).CombinedOutput()
	if err != nil {
		t.Fatalf("pinned OCCT did not reopen the independently moved support arrangement: %v\n%s", err, data)
	}
	var parsed struct{ Faces, Solids int }
	if err := json.Unmarshal(data, &parsed); err != nil || parsed.Faces != 7 || parsed.Solids != 1 {
		t.Fatal("the real changed source joint control must remain a pinned seven-face solid", parsed, err)
	}
	rejectOrderedJoinedMovedJoint(t, ctx, raw, bad)
}
