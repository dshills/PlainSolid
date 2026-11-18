package step

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/internal/slowtest"
	"strings"
	"testing"
	"time"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

const nonplanarRationalRoofsPath = "testdata/occt_mapped_chain_nonplanar_rational_roofs.step"
const nonplanarFalseRoofWeightPath = "testdata/occt_mapped_chain_nonplanar_false_roof_weight.step"

// Independent quadrature of the rational graph over the two original roof
// regions gives the material volume and area. The live OCCT transfer uses
// its own recorded area for the corresponding trims.
const nonplanarRationalRoofVolume = 0.639786408392377
const nonplanarRationalRoofArea = 4.651450389464032
const nonplanarRationalRoofOCCTArea = 4.651450379865176

type rationalRoofSectionWitness struct {
	surface    *geom.BSplineSurface
	y, z       *big.Rat
	zControls  [4]*big.Rat
	transposed bool
	mirrored   bool
}

func verifyNonplanarRationalRoofPoints(t *testing.T, ctx context.Context, north, south *geom.BSplineSurface) {
	t.Helper()
	verifyRationalRoofSection(t, ctx, rationalRoofSectionWitness{north, big.NewRat(1, 2), big.NewRat(177, 896),
		[4]*big.Rat{new(big.Rat), big.NewRat(-39, 256), big.NewRat(93, 256), big.NewRat(9, 8)}, false, false})
	verifyRationalRoofSection(t, ctx, rationalRoofSectionWitness{south, big.NewRat(1, 8), big.NewRat(87, 6656),
		[4]*big.Rat{new(big.Rat), big.NewRat(-15, 512), big.NewRat(141, 512), big.NewRat(9, 8)}, true, false})
}

func verifyRationalRoofSection(t *testing.T, ctx context.Context, tc rationalRoofSectionWitness) {
	t.Helper()
	x := big.NewRat(3, 8)
	p, err := geom.NewSurfacePatch(tc.surface, geom.SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	u, v := x, tc.y
	fixedAxis := 0
	if tc.transposed {
		u, v = v, u
		fixedAxis = 1
	}
	section, err := p.IsoCurve(fixedAxis, .375, geom.SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	net, _, err := section.BezierControlNet(ctx, geom.SurfacePatchOptions{})
	if err != nil || net.Start != 0 || net.End != .75 || len(net.Control) != 4 {
		t.Fatal("each original owner has one full exact rational transverse section", err)
	}
	yControls := [4]*big.Rat{new(big.Rat), big.NewRat(1, 4), big.NewRat(1, 2), big.NewRat(3, 1)}
	profile := [4]int64{1, 1, 2, 4}
	if tc.mirrored {
		yControls = [4]*big.Rat{new(big.Rat), big.NewRat(1, 1), big.NewRat(1, 2), big.NewRat(3, 4)}
		profile = [4]int64{4, 2, 1, 1}
	}
	scale := new(big.Rat).Quo(net.Control[0][3], big.NewRat(profile[0], 1))
	for i, c := range net.Control {
		weight := big.NewRat(profile[i], 1)
		want := [4]*big.Rat{new(big.Rat).Mul(x, weight), yControls[i], tc.zControls[i], weight}
		for axis, value := range want {
			if c[axis].Cmp(new(big.Rat).Mul(value, scale)) != 0 {
				t.Fatal("the exact homogeneous transverse section must retain its off-seam rational profile", fixedAxis, i, axis, c[axis])
			}
		}
	}
	q, err := p.EvaluateRational(ctx, u, v, geom.SurfacePatchEvaluationOptions{})
	if err != nil || q.Point[0].Cmp(x) != 0 || q.Point[1].Cmp(tc.y) != 0 || q.Point[2].Cmp(tc.z) != 0 {
		t.Fatal("each original roof must give its exact rational off-seam height on the affine XY chart", tc.z, q.Point, err)
	}
}

func verifyOriginalRationalRoofSurfaces(t *testing.T, source, strict MappedChainSTEPModel, mirroredSouth bool) {
	t.Helper()
	north := []int64{1, 1, 2, 4}
	south := north
	if mirroredSouth {
		south = []int64{4, 2, 1, 1}
	}
	verifyOriginalRationalRoofSurfaceProfiles(t, source, strict, north, south)
}

func verifyOriginalRationalRoofSurfaceProfiles(t *testing.T, source, strict MappedChainSTEPModel, northWeights, southWeights []int64) {
	t.Helper()
	a, err := Parse(bytes.NewBufferString(source.OriginalSTEP()), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	b, err := Parse(bytes.NewBufferString(strict.OriginalSTEP()), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	for owner := range 2 {
		i := rationalRoofSupportID(t, a, source.FaceIDs[owner])
		j := rationalRoofSupportID(t, b, strict.FaceIDs[owner])
		weights, _ := component(a.Entities[i], "RATIONAL_B_SPLINE_SURFACE")
		north := source.Shell.IndependentOwnerBases()[owner]
		profile := southWeights
		if north {
			profile = northWeights
		}
		for row, values := range weights.Args[0].Items {
			for col, value := range values.Items {
				index := row
				if north {
					index = col
				}
				w, err := affineSplineSourceRat(value)
				if err != nil || index >= len(profile) || w.Cmp(big.NewRat(profile[index], 1)) != 0 {
					t.Fatal("the full original weight net must retain the transverse rational profile", owner, row, col, err)
				}
			}
		}
		before, err := decodeMappedChainStepSurface(a, i, 4)
		if err != nil {
			t.Fatal(err)
		}
		after, err := decodeMappedChainStepSurface(b, j, 4)
		if err != nil {
			t.Fatal(err)
		}
		want, err := parametric.EncodeSurface(before)
		if err != nil {
			t.Fatal(err)
		}
		got, err := parametric.EncodeSurface(after)
		if err != nil || !bytes.Equal(want, got) {
			t.Fatal("strict STEP must retain every original rational support coefficient and knot", owner, err)
		}
	}
}

func rationalRoofSupportID(t *testing.T, f *File, faceID int64) int64 {
	t.Helper()
	face, err := simple(f, faceID, "ADVANCED_FACE", 4)
	if err != nil {
		t.Fatal(err)
	}
	id := face.Args[2].Ref
	if _, ok := component(f.Entities[id], "RATIONAL_B_SPLINE_SURFACE"); !ok {
		t.Fatal("each authored roof must retain a genuinely weighted original STEP support", id)
	}
	return id
}

func TestOCCTMappedChainRectangularNonplanarRationalRoofs(t *testing.T) {
	slowtest.Skip(t)
	raw := readInferredSharedMappedChainFixture(t, nonplanarRationalRoofsPath)
	verifyNonplanarInferredMappedChain(t, context.Background(), raw, nonplanarRationalRoofs)
}

func TestOCCTMappedChainRectangularNonplanarRationalRoofsClosedDisplay(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	raw := readInferredSharedMappedChainFixture(t, nonplanarRationalRoofsPath)
	m, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(raw), Limits{}, inferredMixedMappedChainOptions())
	if err != nil {
		t.Fatal(err)
	}
	fill := nonplanarInferredDisplayOptions()
	fill.Face.BoundaryModelBudget, fill.Face.Interior.Deflection = .02, .10
	verifyMappedChainCutDisplayWithOptions(t, ctx, m, fill, .0125)
}

func TestOCCTMappedChainRectangularNonplanarRationalRoofsRejectsFalseWeight(t *testing.T) {
	valid := readInferredSharedMappedChainFixture(t, nonplanarRationalRoofsPath)
	bad := readInferredSharedMappedChainFixture(t, nonplanarFalseRoofWeightPath)
	rejectNonplanarFalseRoofWeight(t, context.Background(), valid, bad)
}

func rejectNonplanarFalseRoofWeight(t *testing.T, ctx context.Context, valid, bad []byte) {
	t.Helper()
	rejectFalseRationalRoofWeight(t, ctx, valid, bad, inferredMixedMappedChainOptions(), 2)
}

func rejectFalseRationalRoofWeight(t *testing.T, ctx context.Context, valid, bad []byte, o MappedChainShellSTEPOptions, row int) {
	t.Helper()
	m, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(valid), Limits{}, o)
	if err != nil {
		t.Fatal(err)
	}
	south := 0
	if m.Shell.IndependentOwnerBases()[0] {
		south = 1
	}
	a, err := Parse(bytes.NewReader(valid), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	b, err := Parse(bytes.NewReader(bad), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	id := rationalRoofSupportID(t, a, m.FaceIDs[south])
	original, _ := component(a.Entities[id], "RATIONAL_B_SPLINE_SURFACE")
	changed, ok := component(b.Entities[id], "RATIONAL_B_SPLINE_SURFACE")
	if !ok {
		t.Fatal("the independently authored control must retain the original support entity")
	}
	from, err := affineSplineSourceRat(original.Args[0].Items[row].Items[7])
	if err != nil {
		t.Fatal(err)
	}
	to, err := affineSplineSourceRat(changed.Args[0].Items[row].Items[7])
	if err != nil || from.Cmp(big.NewRat(2, 1)) != 0 || to.Cmp(big.NewRat(262145, 131072)) != 0 {
		t.Fatal("the source control must alter one exactly represented interior weight", from, to, err)
	}
	changed.Args[0].Items[row].Items[7] = original.Args[0].Items[row].Items[7]
	if !bytes.Equal(serializeWrappedProofFile(a), serializeWrappedProofFile(b)) {
		t.Fatal("the actual pinned control must differ only at the specified original support weight")
	}
	malformed, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(bad), Limits{}, o)
	if !errors.Is(err, geom.ErrConvergence) || !strings.Contains(err.Error(), fmt.Sprintf("owner #%d", m.FaceIDs[south])) ||
		malformed.SupportJoint != nil || len(malformed.Crossings) != 0 || len(malformed.Native.ChainEdges) != 0 {
		t.Fatal("the false south support weight must reject the original owner proof atomically", err)
	}
}

func TestOCCTMappedChainRectangularNonplanarRationalRoofsLive(t *testing.T) {
	author, reader := os.Getenv("PLAINSOLID_OCCT_MAPPED_CHAIN_RATIONAL_ROOFS_AUTHOR"), os.Getenv("PLAINSOLID_OCCT_MAPPED_CHAIN_STEP")
	if author == "" || reader == "" {
		t.Skip("optional pinned OCCT source author and STEP consumer")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	authorFile := func(mode string) ([]byte, string) {
		path := filepath.Join(t.TempDir(), mode+".step")
		if output, err := exec.CommandContext(ctx, author, mode, path).CombinedOutput(); err != nil {
			t.Fatalf("pinned OCCT author failed: %v\n%s", err, output)
		}
		return readInferredSharedMappedChainFixture(t, path), path
	}
	raw, _ := authorFile("write-nonplanar-rational-roofs")
	canonical := verifyNonplanarInferredMappedChain(t, ctx, raw, nonplanarRationalRoofs)
	verifyPinnedMappedChainTransfer(t, ctx, reader, raw, canonical, nonplanarRationalRoofVolume, nonplanarRationalRoofOCCTArea)
	falseWeight, path := authorFile("write-nonplanar-false-roof-weight")
	if output, err := exec.CommandContext(ctx, reader, "read", path).CombinedOutput(); err != nil {
		t.Fatalf("pinned OCCT failed to reopen the altered support control: %v\n%s", err, output)
	}
	rejectNonplanarFalseRoofWeight(t, ctx, raw, falseWeight)
}
