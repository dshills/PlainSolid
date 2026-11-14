package trimmed

import (
	"context"
	"fmt"
	"math/big"

	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

// AlgebraicGraphVerticalDistance is a boundary minimum on a vertical stock.
// Cap is an original cap role for a filled interior projection or -1.
// Wire/Use identify a lower-cap trim curve for a nearest wall or seam;
// Wall is its original stock face or -1 for a generated seam. A nonnegative
// HeightCap records the cap constraining the witness; otherwise its height
// equals the query height. ModelParameter identifies the exact XY point
// on the referenced cap curve. Cap interior witnesses instead supply Point.
type AlgebraicGraphVerticalDistance struct {
	SquaredDistance, ModelParameter *geom.ExactAlgebraicRoot
	Point                           [3]*big.Rat
	Cap, Wall, Wire, Use, HeightCap int
	ExactOperations, Subdivisions   int
}

// NearestBoundaryRational finds the exact distance to the prepared caps and
// vertical sweep walls, including curved quadratic footprints. The best
// wall point at each footprint clamps Z to the two actual cap heights.
// Stationary points of the three resulting quadratic rational metrics,
// their switching locations, and original curve endpoints are considered.
func (m AlgebraicGraphVerticalMaterial) NearestBoundaryRational(ctx context.Context, point [3]*big.Rat,
	opts geom.ExactCurvePlaneEventOptions) (AlgebraicGraphVerticalDistance, error) {
	var zero AlgebraicGraphVerticalDistance
	b, e := newCapClipBudget(ctx, opts)
	if e != nil {
		return zero, e
	}
	if m.caps[0].recipe == nil {
		return zero, geom.ErrInvalid
	}
	if !m.lineReady {
		return zero, geom.ErrUnrepresentable
	}
	work, splits := b.work, b.subdiv
	for _, q := range point {
		if q == nil {
			return zero, geom.ErrInvalid
		}
		if max(q.Num().BitLen(), q.Denom().BitLen()) > b.bits {
			return zero, geom.ErrConvergence
		}
	}
	out := AlgebraicGraphVerticalDistance{}
	accept := func(q AlgebraicGraphVerticalDistance) error {
		if out.SquaredDistance != nil {
			c, e := q.SquaredDistance.Compare(ctx, out.SquaredDistance, b.limits())
			if e != nil {
				return e
			}
			if e = b.charge(c.Operations, c.Subdivisions); e != nil {
				return e
			}
			if !c.Proven {
				return geom.ErrConvergence
			}
			if c.Order >= 0 {
				return nil
			}
		}
		out = q
		return nil
	}
	var planes [2]polynomialPlaneEquation
	for i, cap := range m.caps {
		p := cap.regions.plane
		planes[i] = p
		norm := new(big.Rat)
		for _, q := range p.normal {
			norm.Add(norm, new(big.Rat).Mul(q, q))
		}
		d := new(big.Rat).Sub(p.dot(point), p.offset)
		s := new(big.Rat).Quo(d, norm)
		var witness [3]*big.Rat
		for j, x := range point {
			witness[j] = new(big.Rat).Sub(x, new(big.Rat).Mul(s, p.normal[j]))
		}
		loc, e := cap.locateRationalBudget(witness, &b)
		if e != nil {
			return zero, e
		}
		if loc != trimvalidate.Inside {
			continue
		}
		q, used, e := geom.NewExactRationalRoot(ctx, new(big.Rat).Mul(d, s), b.limits())
		if e != nil {
			return zero, e
		}
		if e = b.charge(used, 0); e != nil {
			return zero, e
		}
		if e = accept(AlgebraicGraphVerticalDistance{SquaredDistance: q, Point: witness, Cap: i, Wall: -1, Wire: -1, Use: -1, HeightCap: i}); e != nil {
			return zero, e
		}
	}
	base := m.caps[m.lower]
	axes := [2]int{0, 1}
	var height [2][3]*big.Rat
	for i, p := range planes {
		height[i] = [3]*big.Rat{p.offset, new(big.Rat).Neg(p.normal[0]), new(big.Rat).Neg(p.normal[1])}
	}
	for wi, wire := range base.wires {
		for ui, use := range wire {
			if e := ctx.Err(); e != nil {
				return zero, e
			}
			// Positive means the query is above the graph at a point.
			// The footprint uses the original model XY polynomials even
			// when the lower cap has varying Z.
			h, used, e := use.Cell.HomogeneousPower(ctx, geom.SurfacePatchEvaluationOptions{MaxOperations: b.work, MaxCoefficientBits: b.bits})
			if e != nil {
				return zero, e
			}
			if e = b.charge(used, 0); e != nil {
				return zero, e
			}
			var forms [2][]*big.Rat
			var breaks []*geom.ExactAlgebraicRoot
			for side, p := range planes {
				n := max(len(h[0]), len(h[1]), len(h[3]))
				forms[side] = make([]*big.Rat, n)
				bias := new(big.Rat).Sub(point[2], p.offset)
				for k := range n {
					x := new(big.Rat)
					if k < len(h[3]) {
						x.Mul(bias, h[3][k])
					}
					for j := 0; j < 2; j++ {
						if k < len(h[j]) {
							x.Add(x, new(big.Rat).Mul(p.normal[j], h[j][k]))
						}
					}
					forms[side][k] = x
					if max(x.Num().BitLen(), x.Denom().BitLen()) > b.bits {
						return zero, geom.ErrConvergence
					}
				}
				if p.normal[0].Sign() == 0 && p.normal[1].Sign() == 0 {
					// A horizontal height constraint is constant. If
					// equality holds it creates no isolated switch.
					continue
				}
				raw := [4]*big.Rat{p.normal[0], p.normal[1], new(big.Rat), bias}
				events, e := use.Cell.IntersectRawPlane(ctx, raw, b.limits())
				if e != nil {
					return zero, fmt.Errorf("trim %d/%d height %d: %w", wi, ui, side, e)
				}
				if e = b.charge(events.Operations, events.Subdivisions); e != nil {
					return zero, e
				}
				if !events.Complete {
					return zero, geom.ErrConvergence
				}
				for _, v := range events.Events {
					breaks = append(breaks, v.Root())
				}
			}
			for _, a := range breaks {
				// A switch is already a valid candidate of either
				// adjacent height function. A zero-length cell factory
				// is unnecessary: evaluate the exact metric at its root.
				if e := m.addVerticalDistanceCandidate(ctx, point, axes, height, forms, use, wi, ui, a, nil, &b, accept); e != nil {
					return zero, fmt.Errorf("trim %d/%d height switch: %w", wi, ui, e)
				}
			}
			for mode := -1; mode < 2; mode++ {
				var graph *[3]*big.Rat
				if mode >= 0 {
					graph = &height[mode]
				}
				set, e := use.Cell.AffineHeightCurveDistanceCandidates(ctx, axes, graph, point, b.limits())
				if e != nil {
					return zero, fmt.Errorf("trim %d/%d distance mode %d: %w", wi, ui, mode, e)
				}
				if e = b.charge(set.Operations, set.Subdivisions); e != nil {
					return zero, e
				}
				for _, a := range set.Candidates {
					actual, e := m.verticalDistanceMode(ctx, a.Parameter, forms, &b)
					if e != nil {
						return zero, fmt.Errorf("trim %d/%d exact height mode: %w", wi, ui, e)
					}
					if mode != actual {
						continue
					}
					if e = m.addVerticalDistanceCandidate(ctx, point, axes, height, forms, use, wi, ui, a.Parameter, a.SquaredDistance, &b, accept); e != nil {
						return zero, e
					}
				}
			}
		}
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	if out.SquaredDistance == nil {
		return zero, geom.ErrDomain
	}
	out.ExactOperations, out.Subdivisions = work-b.work, splits-b.subdiv
	return out, nil
}

func (m AlgebraicGraphVerticalMaterial) verticalDistanceMode(ctx context.Context, a *geom.ExactAlgebraicRoot, forms [2][]*big.Rat, b *capClipBudget) (int, error) {
	var sign [2]int
	for i, f := range forms {
		p, e := a.SignPolynomial(ctx, f, b.limits())
		if e != nil {
			return -1, e
		}
		if e = b.charge(p.Operations, p.Subdivisions); e != nil {
			return -1, e
		}
		sign[i] = p.Sign
	}
	if sign[m.lower] < 0 {
		return m.lower, nil
	}
	if sign[m.lower^1] > 0 {
		return m.lower ^ 1, nil
	}
	return -1, nil
}

func (m AlgebraicGraphVerticalMaterial) addVerticalDistanceCandidate(ctx context.Context, point [3]*big.Rat, axes [2]int,
	height [2][3]*big.Rat, forms [2][]*big.Rat, use AlgebraicGraphCapUse, wi, ui int, a, value *geom.ExactAlgebraicRoot,
	b *capClipBudget, accept func(AlgebraicGraphVerticalDistance) error) error {
	mode, e := m.verticalDistanceMode(ctx, a, forms, b)
	if e != nil {
		return e
	}
	if value == nil {
		var graph *[3]*big.Rat
		if mode >= 0 {
			graph = &height[mode]
		}
		q, ops, e := use.Cell.AffineHeightCurveSquaredDistance(ctx, axes, graph, point, a, b.limits())
		if e != nil {
			return e
		}
		if e = b.charge(ops, 0); e != nil {
			return e
		}
		value = q
	}
	wall := -1
	if use.Seam < 0 {
		wall = use.Source.Face
	}
	heightOwner := mode
	if heightOwner < 0 {
		// On a switching station the clamp has the same value as the
		// query. Report its cap owner when known instead of labeling it
		// a strict wall-interior height.
		for i, form := range forms {
			z, e := a.SignPolynomial(ctx, form, b.limits())
			if e != nil {
				return e
			}
			if e = b.charge(z.Operations, z.Subdivisions); e != nil {
				return e
			}
			if z.Sign == 0 {
				heightOwner = i
				break
			}
		}
	}
	return accept(AlgebraicGraphVerticalDistance{SquaredDistance: value, ModelParameter: a, Cap: -1, Wall: wall, Wire: wi, Use: ui, HeightCap: heightOwner})
}
