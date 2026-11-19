package step

import (
	"bytes"
	"context"
	"errors"
	"math/big"
	"plainsolid/internal/slowtest"
	"slices"
	"testing"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

const kinkedJoinedRoofPath = "testdata/occt_mapped_chain_kinked_joined_independent_roofs.step"
const kinkedJoinedMovedSupportPath = "testdata/occt_mapped_chain_kinked_joined_independent_moved_support.step"
const kinkedJoinedVolume = 0.64344524546000759
const kinkedJoinedArea = 4.6427728126414856

func verifyKinkedJoinedRoofs(t *testing.T, ctx context.Context, native trimmed.MappedChainRectangularSolid, north int) {
	t.Helper()
	faces := native.Solid.Faces()
	x := big.NewRat(3, 8)
	for j, tc := range []struct {
		face, axis int
		widthKnots []float64
		y, z       *big.Rat
	}{
		{north, 0, []float64{0, 3.0 / 32, 3.0 / 16, 9.0 / 32, 3.0 / 8, .75}, big.NewRat(1, 2), big.NewRat(2139, 11264)},
		{1 - north, 1, []float64{0, 3.0 / 16, 9.0 / 16, .75}, big.NewRat(1, 8), big.NewRat(15279, 253952)},
	} {
		face := faces[tc.face]
		roof := face.Surface.(*geom.BSplineSurface)
		patch, err := geom.NewSurfacePatch(roof, geom.SurfacePatchOptions{})
		if err != nil {
			t.Fatal(err)
		}
		ku, kv := patch.KnotBreaks()
		if !slices.Equal([2][]float64{ku, kv}[tc.axis], tc.widthKnots) {
			t.Fatal("the south chart must retain one original span across the kink and the north chart must retain its support joint", tc.face)
		}
		u, v := x, tc.y
		if tc.axis == 1 {
			u, v = v, u
		}
		point, err := patch.EvaluateRational(ctx, u, v, geom.SurfacePatchEvaluationOptions{})
		if err != nil || point.Point[0].Cmp(x) != 0 || point.Point[1].Cmp(tc.y) != 0 || point.Point[2].Cmp(tc.z) != 0 {
			t.Fatal("the original weights must preserve the affine projection and exact off-seam rational height", tc.face, point.Point, err)
		}
		if j == 0 {
			verifyMappedRoofSlopeJump(t, ctx, roof, 0, .375, tc.y, big.NewRat(-45, 1408))
		}
		verifyJoinedOneSidedTangents(t, ctx, face, 3, [2][2]*big.Rat{
			{big.NewRat(1, 2), big.NewRat(5, 8)}, {big.NewRat(-1, 2), big.NewRat(1, 4)},
		})
	}
}

// Check both endpoint tangents of the original lifted paths in this chart.
func verifyJoinedOneSidedTangents(t *testing.T, ctx context.Context, face trimmed.Face, incoming int, expected [2][2]*big.Rat) {
	t.Helper()
	for side, want := range expected {
		lift, err := trimmed.ExactTrimModelBezier(ctx, face.Surface, face.Outer[incoming+side].Curve, false, geom.SurfacePatchOptions{})
		if err != nil {
			t.Fatal(err)
		}
		net, _, err := lift.BezierControlNet(ctx, geom.SurfacePatchOptions{})
		if err != nil || len(net.Control) < 2 {
			t.Fatal("each one-sided lift must be an exact original restriction", incoming, side, err)
		}
		a, b := net.Control[0], net.Control[1]
		if side == 0 {
			a, b = net.Control[len(net.Control)-2], net.Control[len(net.Control)-1]
		}
		difference := func(axis int) *big.Rat {
			return new(big.Rat).Sub(new(big.Rat).Quo(b[axis], b[3]), new(big.Rat).Quo(a[axis], a[3]))
		}
		dx := difference(0)
		if dx.Sign() <= 0 {
			t.Fatal("both original paths must advance along the shell seam", incoming, side)
		}
		for axis, slope := range want {
			if got := new(big.Rat).Quo(difference(axis+1), dx); got.Cmp(slope) != 0 {
				t.Fatal("the exact lift must retain the distinct sided tangent at the unilateral joint", incoming, side, axis, got)
			}
		}
	}
}

func TestOCCTMappedChainRectangularKinkedJoinedIndependentRoofs(t *testing.T) {
	slowtest.Skip(t)
	raw := readInferredSharedMappedChainFixture(t, kinkedJoinedRoofPath)
	verifyThreeJoinedRoofSource(t, context.Background(), raw, threeJoinedKinked)
}

func TestOCCTMappedChainRectangularKinkedJoinedIndependentRoofsClosedDisplay(t *testing.T) {
	slowtest.Skip(t)
	verifyThreeJoinedRoofDisplay(t, readInferredSharedMappedChainFixture(t, kinkedJoinedRoofPath), 4, .24)
}

func TestOCCTMappedChainRectangularKinkedJoinedIndependentRoofsRejectsMovedSupport(t *testing.T) {
	t.Parallel()
	raw := readInferredSharedMappedChainFixture(t, kinkedJoinedRoofPath)
	bad := readInferredSharedMappedChainFixture(t, kinkedJoinedMovedSupportPath)
	ctx := context.Background()
	source, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(raw), Limits{}, threeJoinedRoofOptions())
	if err != nil {
		t.Fatal(err)
	}
	rejectKinkedJoinedMovedSupport(t, ctx, source, bad)
}

func rejectKinkedJoinedMovedSupport(t *testing.T, ctx context.Context, source MappedChainSTEPModel, bad []byte) {
	t.Helper()
	o := threeJoinedRoofOptions()
	m, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(bad), Limits{}, o)
	if !errors.Is(err, geom.ErrConvergence) || len(m.Native.ChainEdges) != 0 || len(m.SupportJoints) != 0 || len(m.Crossings) != 0 {
		t.Fatal("moving the north support knot away from the actual material kink must reject atomically", err)
	}
	rejectChangedNativeJoinedRoof(t, ctx, source, bad, true, "support knot must belong")
}

func TestOCCTMappedChainRectangularKinkedJoinedIndependentRoofsLive(t *testing.T) {
	t.Parallel()
	verifyPinnedThreeJoinedRoofs(t, threeJoinedKinked)
}

// Compare the live author to the fixture with the full native and strict
// source assertions. STEP headers contain timestamps and output filenames.
func requirePinnedMappedChainData(t *testing.T, raw []byte, fixture string) {
	t.Helper()
	data := func(raw []byte) []byte {
		t.Helper()
		_, body, ok := bytes.Cut(raw, []byte("\nDATA;\n"))
		if !ok {
			t.Fatal("the authored STEP file must contain its exact DATA section")
		}
		lines := bytes.Split(body, []byte("\n"))
		for i, line := range lines {
			lines[i] = bytes.TrimRight(line, " \t\r")
		}
		return bytes.Join(lines, []byte("\n"))
	}
	if !bytes.Equal(data(raw), data(readInferredSharedMappedChainFixture(t, fixture))) {
		t.Fatal("the live pinned author must reproduce the exact audited STEP graph", fixture)
	}
}
