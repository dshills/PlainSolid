package trimmed

import (
	"context"
	"errors"
	"math/big"
	"reflect"
	"testing"

	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
	"plainsolid/intersect"
)

func nativeThreeCrossingsTangencyFixture(t *testing.T, axis int) (*geom.BSplineSurface, *geom.BSplineSurface, *geom.BSplineSurface) {
	t.Helper()
	f := []*big.Rat{big.NewRat(1, 4), big.NewRat(3, 8), big.NewRat(-3, 8), big.NewRat(1, 8)}
	// Subtract f to get -(u-1/8)^2(u-3/8)(u-5/8)(u-7/8).
	// The original roof product f*g has degree eight and fits the same
	// exact dyadic stock representation as earlier branch fixtures.
	g := []*big.Rat{
		big.NewRat(8297, 32768), big.NewRat(1255, 4096), big.NewRat(35, 256),
		big.NewRat(-47, 32), big.NewRat(17, 8), big.NewRat(-1, 1),
	}
	return twoPolynomialRoofFixture(t, axis, f, g)
}

func TestNativePolynomialThreeCrossingsTangencyQuinticTrimLimit(t *testing.T) {
	ctx := context.Background()
	for axis := 0; axis < 2; axis++ {
		base, a, b := nativeThreeCrossingsTangencyFixture(t, axis)
		for order, roofs := range [][2]*geom.BSplineSurface{{a, b}, {b, a}} {
			patches, e := prepareSharedBaseGraphContact(ctx, base, roofs[0], roofs[1], intersect.SplinePlaneOptions{})
			if e != nil {
				t.Fatal("exact identity chart and common positive base", axis, order, e)
			}
			proof, e := geom.DiscoverSplineGraphPolynomialThreeCrossingsTangency(ctx, patches[0], patches[1], axis, geom.GraphPointContactOptions{})
			if e != nil || !proof.Proven || proof.ContactOrder != [4]int{2, 1, 1, 1} {
				t.Fatal("solid stocks retain the complete quintic contact arrangement", axis, order, e)
			}
			uv, _, e := proof.RepresentedVertices()
			if e != nil {
				t.Fatal("all required terminal and contact coordinates are lossless", axis, order, e)
			}
			for i, source := range proof.Edges {
				var controls [][3]*big.Rat
				for _, p := range source.Controls {
					controls = append(controls, [3]*big.Rat{p[0], p[1], big.NewRat(1, 1)})
				}
				// The generic exact loop checker already represents these
				// controls. The native trim and its splitter stop at quartic.
				if _, e := trimvalidate.ExactBezier(controls); e != nil {
					t.Fatal("the exact validation descriptor accepts the original restriction", i, e)
				}
				curve, e := NewExactBezierUV(controls)
				if source.Branch == 0 {
					if e != nil || len(controls) != 4 || curve.ends != ([2]geom.Vec2{uv[source.Start], uv[source.End]}) {
						t.Fatal("ordinary cubic restrictions still fit the native trim", i, e)
					}
					continue
				}
				if len(controls) != 6 || !errors.Is(e, geom.ErrInvalid) || len(curve.HomogeneousControls()) != 0 {
					t.Fatal("the exact quintic restriction fails specifically at native trim construction", i, e)
				}
				if source.Start >= 4 && source.End >= 4 {
					checkQuinticSplitPoint(t, patches, proof, source)
				}
			}
			if _, e := polynomialNodePieces(proof.Edges[:], uv[:]); !errors.Is(e, geom.ErrInvalid) {
				t.Fatal("native branch assembly must fail at the six-control trim", axis, order, e)
			}
		}
	}
}

func checkQuinticSplitPoint(t *testing.T, patches [2]*geom.SurfacePatch, proof geom.SplineGraphPolynomialThreeCrossingsTangency, source geom.SplineGraphPolynomialPiece) {
	t.Helper()
	axis := proof.FreeAxis
	first, last := proof.Vertices[source.Start].Parameters[axis], proof.Vertices[source.End].Parameters[axis]
	free := new(big.Rat).Quo(new(big.Rat).Add(first, last), big.NewRat(2, 1))
	dependent := fourPolynomialCutBranchValue(proof.Power[source.Branch], free)
	var uv [2]*big.Rat
	uv[axis], uv[1-axis] = free, dependent
	ctx := context.Background()
	a, e := patches[0].EvaluateRational(ctx, uv[0], uv[1], geom.SurfacePatchEvaluationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	b, e := patches[1].EvaluateRational(ctx, uv[0], uv[1], geom.SurfacePatchEvaluationOptions{})
	if e != nil || !reflect.DeepEqual(a.Point, b.Point) {
		t.Fatal("both original supports lift the midpoint identically", e)
	}
	if _, e := geom.RepresentedExactPoint(a.Point); e != nil {
		t.Fatal("the eventual split also has lossless model coordinates", e)
	}
}
