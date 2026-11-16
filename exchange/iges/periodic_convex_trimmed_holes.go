package iges

import (
	"context"
	"fmt"
	"io"
	"math/big"

	"plainsolid/geom"
)

// PeriodicConvexTrimmedFaceWithHoles retains two strictly disjoint
// clockwise affine holes in their original type-144 contour order.
type PeriodicConvexTrimmedFaceWithHoles struct {
	PeriodicConvexTrimmedFace
	Holes [2]PeriodicConvexTrimHole
}

// ReadPeriodicConvexTrimmedFaceWithHoles proves one outer quadrilateral
// and two strictly interior, disjoint affine inner quadrilaterals on a
// common certified periodic knot cell. It retains all twelve complete
// directed UV/model pairs.
func ReadPeriodicConvexTrimmedFaceWithHoles(ctx context.Context, r io.Reader, l Limits) (PeriodicConvexTrimmedFaceWithHoles, error) {
	source, err := readPeriodicTrimSourceForAxes(ctx, r, l, 4, periodicTrimAnyAxes, []int{4, 4})
	if err != nil {
		return PeriodicConvexTrimmedFaceWithHoles{}, err
	}
	return provePeriodicConvexTrimmedFaceWithHoles(ctx, source)
}

func provePeriodicConvexTrimmedFaceWithHoles(ctx context.Context, source periodicTrimSource) (PeriodicConvexTrimmedFaceWithHoles, error) {
	var zero PeriodicConvexTrimmedFaceWithHoles
	outer, err := provePeriodicConvexOuter(ctx, &source)
	if err != nil {
		return zero, err
	}
	var holes [2]PeriodicConvexTrimHole
	for i, owner := range source.holes {
		holes[i], source.remaining, err = provePeriodicConvexHole(ctx, source.graph, outer, owner, source.remaining)
		if err != nil {
			return zero, err
		}
	}
	if !periodicConvexWiresStrictlyDisjoint(periodicAffineWireVertices(holes[0].Edges), periodicAffineWireVertices(holes[1].Edges)) {
		return zero, fmt.Errorf("iges: convex trim holes #%d and #%d overlap or meet: %w", holes[0].ContourID, holes[1].ContourID, geom.ErrUnrepresentable)
	}
	if len(source.graph.claimed) != len(source.graph.records) {
		return zero, geom.ErrUnrepresentable
	}
	return PeriodicConvexTrimmedFaceWithHoles{PeriodicConvexTrimmedFace: outer, Holes: holes}, nil
}

// Normals to the sides of both convex polygons supply the separating
// axes. Strictly disjoint projected intervals exclude all shared points.
func periodicConvexWiresStrictlyDisjoint(a, b [4][2]*big.Rat) bool {
	for _, wire := range [2][4][2]*big.Rat{a, b} {
		for i, p := range wire {
			q := wire[(i+1)%len(wire)]
			normal := [2]*big.Rat{new(big.Rat).Sub(p[1], q[1]), new(big.Rat).Sub(q[0], p[0])}
			left, right := periodicUVProjection(a, normal), periodicUVProjection(b, normal)
			if left.Upper.Cmp(right.Lower) < 0 || right.Upper.Cmp(left.Lower) < 0 {
				return true
			}
		}
	}
	return false
}

func periodicUVProjection(vertices [4][2]*big.Rat, normal [2]*big.Rat) geom.ExactParameterInterval {
	var bounds geom.ExactParameterInterval
	for i, p := range vertices {
		station := new(big.Rat).Add(new(big.Rat).Mul(p[0], normal[0]), new(big.Rat).Mul(p[1], normal[1]))
		if i == 0 || station.Cmp(bounds.Lower) < 0 {
			bounds.Lower = station
		}
		if i == 0 || station.Cmp(bounds.Upper) > 0 {
			bounds.Upper = station
		}
	}
	return bounds
}
