//go:build research

package trimmed

import (
	"context"
	"math/big"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

// AlgebraicGraphVerticalMaterial is an immutable query on a closed clipped
// stock whose two caps are graphs over XY and whose remaining supports
// have identically zero XY Jacobian.
type AlgebraicGraphVerticalMaterial struct {
	caps      [2]AlgebraicGraphCapClip
	seams     []AlgebraicGraphCapPairSeam
	lower     int
	maxBits   int
	lineReady bool
}

// AlgebraicGraphVerticalFiber contains the exact heights and cap roles at a
// rational XY station. BoundaryOnly holds on the projected perimeter, where
// the closed fiber consists entirely of boundary, including a collapsed tip.
type AlgebraicGraphVerticalFiber struct {
	Heights      [2]*big.Rat
	Caps         [2]int
	Found        bool
	BoundaryOnly bool
}

// PrepareVerticalMaterial proves a closed exact boundary, two opposing cap
// orientations over XY, and zero projected Jacobian on every retained stock
// surface. Each query separately classifies both projected cap endpoints.
func (c AlgebraicGraphCapPair) PrepareVerticalMaterial(ctx context.Context, o RationalGraphShellSectionOptions, roots geom.ExactCurvePlaneEventOptions) (AlgebraicGraphVerticalMaterial, error) {
	var zero AlgebraicGraphVerticalMaterial
	if ctx == nil || roots.MaxOperations < 0 || roots.MaxSubdivisions < 0 || roots.MaxCoefficientBits < 0 {
		return zero, geom.ErrInvalid
	}
	if roots.MaxOperations == 0 {
		roots.MaxOperations = 2000000
	}
	if roots.MaxSubdivisions == 0 {
		roots.MaxSubdivisions = 8192
	}
	if roots.MaxCoefficientBits == 0 {
		roots.MaxCoefficientBits = 16384
	}
	if roots.MaxCoefficientBits < 64 {
		return zero, geom.ErrInvalid
	}
	b := capClipBudget{ctx, roots.MaxOperations, roots.MaxSubdivisions, roots.MaxCoefficientBits}
	exact, e := c.exactIndexedBoundary(o, &b)
	if e != nil {
		return zero, e
	}
	a, z := c.caps[0].regions.plane, c.caps[1].regions.plane
	if a.dropped != 2 || z.dropped != 2 || a.orientation == z.orientation {
		return zero, geom.ErrUnrepresentable
	}
	if e := c.proveZeroStockXYFlux(exact, o, &b, true); e != nil {
		return zero, e
	}
	lower := 0
	if a.orientation > 0 {
		lower = 1
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	// Fibers need only the flux test. An arbitrary line needs the
	// stronger property that every wall is swept continuously between
	// both complete cap graphs, with no stacked chart at intermediate Z.
	// Leave the existing fiber scope unchanged when this proof is absent.
	lineReady, e := c.proveVerticalLineWalls(exact, o, &b)
	if e != nil {
		return zero, e
	}
	return AlgebraicGraphVerticalMaterial{caps: c.caps, seams: c.seams, lower: lower, maxBits: roots.MaxCoefficientBits, lineReady: lineReady}, nil
}

// VerticalFiber proves exact material on the vertical line through xy. Both
// original cap masks must agree. For an interior projection there can be no
// additional material interval: any regular vertical crossing is on one of
// the two caps, while the retained original supports have zero XY Jacobian.
// A perimeter result is accepted only where both caps agree and the
// corresponding interval is the closed limit of interior fibers.
func (m AlgebraicGraphVerticalMaterial) VerticalFiber(ctx context.Context, xy [2]*big.Rat) (AlgebraicGraphVerticalFiber, error) {
	var zero AlgebraicGraphVerticalFiber
	if ctx == nil || m.caps[0].recipe == nil || xy[0] == nil || xy[1] == nil {
		return zero, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	for _, x := range xy {
		if x.Num().BitLen() > m.maxBits || x.Denom().BitLen() > m.maxBits {
			return zero, geom.ErrConvergence
		}
	}
	out := AlgebraicGraphVerticalFiber{Caps: [2]int{m.lower, m.lower ^ 1}}
	var loc [2]trimvalidate.Location
	for i, owner := range out.Caps {
		cap := m.caps[owner]
		plane := cap.regions.plane
		z := new(big.Rat).Set(plane.offset)
		for j, q := range xy {
			z.Sub(z, new(big.Rat).Mul(plane.normal[j], q))
		}
		if z.Num().BitLen() > m.maxBits || z.Denom().BitLen() > m.maxBits {
			return zero, geom.ErrConvergence
		}
		var e error
		loc[i], e = cap.LocateRational(ctx, [3]*big.Rat{xy[0], xy[1], z})
		if e != nil {
			return zero, e
		}
		out.Heights[i] = z
	}
	if loc[0] == trimvalidate.Outside && loc[1] == trimvalidate.Outside {
		return zero, nil
	}
	if loc[0] == trimvalidate.Outside || loc[1] == trimvalidate.Outside || out.Heights[0].Cmp(out.Heights[1]) > 0 {
		return zero, geom.ErrConvergence
	}
	out.Found = true
	out.BoundaryOnly = loc[0] == trimvalidate.Boundary || loc[1] == trimvalidate.Boundary
	if loc[0] != loc[1] {
		return zero, geom.ErrConvergence
	}
	return out, nil
}

// ClassifyRational compares a rational world point with its exact fiber.
func (m AlgebraicGraphVerticalMaterial) ClassifyRational(ctx context.Context, p [3]*big.Rat) (parametric.Location, error) {
	if p[2] == nil {
		return parametric.Outside, geom.ErrInvalid
	}
	if m.maxBits > 0 && (p[2].Num().BitLen() > m.maxBits || p[2].Denom().BitLen() > m.maxBits) {
		return parametric.Outside, geom.ErrConvergence
	}
	q, e := m.VerticalFiber(ctx, [2]*big.Rat{p[0], p[1]})
	if e != nil || !q.Found {
		return parametric.Outside, e
	}
	a, z := p[2].Cmp(q.Heights[0]), p[2].Cmp(q.Heights[1])
	if a < 0 || z > 0 {
		return parametric.Outside, nil
	}
	if q.BoundaryOnly || a == 0 || z == 0 {
		return parametric.Boundary, nil
	}
	return parametric.Inside, nil
}
