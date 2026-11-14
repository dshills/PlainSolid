package trimmed

import (
	"context"
	"math/big"
	"slices"

	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

// AlgebraicGraphStockLinePiece is a closed interval of retained material.
// BoundaryOnly means no relative interior of this interval is in the solid
// interior. Caps and StockFaces are the original owners of that relative
// interior, or the owners of the point for a singleton.
type AlgebraicGraphStockLinePiece struct {
	Extent           [2]*geom.ExactAlgebraicRoot
	BoundaryOnly     bool
	Caps, StockFaces []int
}

// AlgebraicGraphStockLineCoverage is a partition into span closures and
// point contacts. Their union is the exact closed material along the line.
// A boundary point can coincide with an endpoint of a neighboring span.
type AlgebraicGraphStockLineCoverage struct {
	Spans, Points                 []AlgebraicGraphStockLinePiece
	ExactOperations, Subdivisions int
}

// CoverRationalLine proves the exact finite material along a line. It
// clips projected cap coverage by both affine cap halfspaces. Lines
// coincident with a cap or wall, vertical fibers, and singular projected
// contacts are allowed.
func (m AlgebraicGraphVerticalMaterial) CoverRationalLine(ctx context.Context, origin, direction [3]*big.Rat,
	extent [2]*big.Rat, opts geom.ExactCurvePlaneEventOptions) (AlgebraicGraphStockLineCoverage, error) {
	var zero AlgebraicGraphStockLineCoverage
	if m.caps[0].recipe == nil {
		return zero, geom.ErrInvalid
	}
	if !m.lineReady {
		return zero, geom.ErrUnrepresentable
	}
	if opts.MaxCoefficientBits == 0 {
		opts.MaxCoefficientBits = m.maxBits
	}
	b, err := newCapClipBudget(ctx, opts)
	if err != nil {
		return zero, err
	}
	work, splits := b.work, b.subdiv
	for _, row := range [][]*big.Rat{origin[:], direction[:], extent[:]} {
		for _, x := range row {
			if x == nil {
				return zero, geom.ErrInvalid
			}
			if max(x.Num().BitLen(), x.Denom().BitLen()) > b.bits {
				return zero, geom.ErrConvergence
			}
		}
	}
	if extent[0].Cmp(extent[1]) >= 0 {
		return zero, geom.ErrDomain
	}
	if direction[0].Sign() == 0 && direction[1].Sign() == 0 {
		if direction[2].Sign() == 0 {
			return zero, geom.ErrDegenerate
		}
		out, err := m.coverVerticalLine(origin, direction, extent, &b)
		if err != nil {
			return zero, err
		}
		out.ExactOperations, out.Subdivisions = work-b.work, splits-b.subdiv
		return out, nil
	}
	var residual [2][]*big.Rat
	for i, c := range m.caps {
		p := c.regions.plane
		s := big.NewRat(int64(p.orientation), 1)
		residual[i] = []*big.Rat{
			new(big.Rat).Mul(s, new(big.Rat).Sub(p.dot(origin), p.offset)),
			new(big.Rat).Mul(s, p.dot(direction)),
		}
		if e := b.charge(24, 0); e != nil {
			return zero, e
		}
		for _, x := range residual[i] {
			if max(x.Num().BitLen(), x.Denom().BitLen()) > b.bits {
				return zero, geom.ErrConvergence
			}
		}
		if residual[i][1].Sign() == 0 && residual[i][0].Sign() > 0 {
			return zero, nil
		}
	}
	a, d := origin, direction
	p := m.caps[m.lower].regions.plane
	a[2] = new(big.Rat).Sub(origin[2], new(big.Rat).Sub(p.dot(origin), p.offset))
	d[2] = new(big.Rat).Sub(direction[2], p.dot(direction))
	projected, e := m.caps[m.lower].CoverRationalLine(ctx, a, d, extent, b.limits())
	if e != nil {
		return zero, e
	}
	if e = b.charge(projected.ExactOperations, projected.Subdivisions); e != nil {
		return zero, e
	}
	cmp := func(a, z *geom.ExactAlgebraicRoot) (int, error) {
		o, err := a.Compare(ctx, z, b.limits())
		if err != nil {
			return 0, err
		}
		if err = b.charge(o.Operations, o.Subdivisions); err != nil {
			return 0, err
		}
		if !o.Proven {
			return 0, geom.ErrConvergence
		}
		return o.Order, nil
	}
	makeRoot := func(t *big.Rat) (*geom.ExactAlgebraicRoot, error) {
		r, n, e := geom.NewExactRationalRoot(ctx, t, b.limits())
		if e != nil {
			return nil, e
		}
		return r, b.charge(n, 0)
	}
	var window [2]*geom.ExactAlgebraicRoot
	for i, t := range extent {
		window[i], e = makeRoot(t)
		if e != nil {
			return zero, e
		}
	}
	for _, line := range residual {
		if line[1].Sign() == 0 {
			continue
		}
		t, e := makeRoot(new(big.Rat).Quo(new(big.Rat).Neg(line[0]), line[1]))
		if e != nil {
			return zero, e
		}
		if line[1].Sign() < 0 {
			if k, e := cmp(t, window[0]); e != nil {
				return zero, e
			} else if k > 0 {
				window[0] = t
			}
		} else if k, e := cmp(t, window[1]); e != nil {
			return zero, e
		} else if k < 0 {
			window[1] = t
		}
	}
	if k, e := cmp(window[0], window[1]); e != nil {
		return zero, e
	} else if k > 0 {
		return zero, nil
	}
	var out AlgebraicGraphStockLineCoverage
	// Points from separate interval closures are merged by exact line
	// identity and accumulate all original owners.
	add := func(dst *[]AlgebraicGraphStockLinePiece, p AlgebraicGraphStockLinePiece, merge bool) error {
		lo, hi := 0, len(*dst)
		for lo < hi {
			k := lo + (hi-lo)/2
			o, err := cmp((*dst)[k].Extent[0], p.Extent[0])
			if err != nil {
				return err
			}
			switch {
			case o == 0:
				if !merge {
					return geom.ErrConvergence
				}
				other := &(*dst)[k]
				other.Caps = sortedUniqueStockOwners(append(other.Caps, p.Caps...))
				other.StockFaces = sortedUniqueStockOwners(append(other.StockFaces, p.StockFaces...))
				return nil
			case o < 0:
				lo = k + 1
			default:
				hi = k
			}
		}
		*dst = append(*dst, AlgebraicGraphStockLinePiece{})
		copy((*dst)[lo+1:], (*dst)[lo:])
		(*dst)[lo] = p
		return nil
	}
	at := func(r *geom.ExactAlgebraicRoot, wall []int) error {
		p := AlgebraicGraphStockLinePiece{Extent: [2]*geom.ExactAlgebraicRoot{r, r}, BoundaryOnly: true,
			StockFaces: slices.Clone(wall)}
		for i, poly := range residual {
			s, err := r.SignPolynomial(ctx, poly, b.limits())
			if err != nil {
				return err
			}
			if err = b.charge(s.Operations, s.Subdivisions); err != nil {
				return err
			}
			if s.Sign > 0 {
				return nil
			}
			if s.Sign == 0 {
				p.Caps = append(p.Caps, i)
			}
		}
		if len(p.Caps) == 0 && len(wall) == 0 {
			return nil
		}
		return add(&out.Points, p, true)
	}
	span := func(r [2]*geom.ExactAlgebraicRoot, owner int) error {
		if k, e := cmp(r[0], window[0]); e != nil {
			return e
		} else if k < 0 {
			r[0] = window[0]
		}
		if k, e := cmp(r[1], window[1]); e != nil {
			return e
		} else if k > 0 {
			r[1] = window[1]
		}
		k, e := cmp(r[0], r[1])
		if e != nil || k > 0 {
			return e
		}
		p := AlgebraicGraphStockLinePiece{Extent: r}
		if owner >= 0 {
			p.StockFaces = []int{owner}
		}
		if k == 0 {
			return at(r[0], p.StockFaces)
		}
		for i, poly := range residual {
			if poly[0].Sign() == 0 && poly[1].Sign() == 0 {
				p.Caps = append(p.Caps, i)
			}
		}
		p.BoundaryOnly = len(p.Caps)+len(p.StockFaces) > 0
		if !p.BoundaryOnly && owner == -2 {
			return geom.ErrConvergence // a generated seam must be on both cap planes
		}
		if e := add(&out.Spans, p, false); e != nil {
			return e
		}
		for _, point := range r {
			if err := at(point, p.StockFaces); err != nil {
				return err
			}
		}
		return nil
	}
	for _, r := range projected.Interior {
		if e := span(r, -1); e != nil {
			return zero, e
		}
	}
	wire := m.caps[m.lower].wires
	for _, run := range projected.Boundary {
		u := wire[run.Wire][run.Use]
		owner := u.Source.Face
		if u.Seam >= 0 {
			owner = -2
		} else if owner < 0 {
			return zero, geom.ErrConvergence
		}
		if e := span(run.Extent, owner); e != nil {
			return zero, e
		}
	}
	for _, point := range projected.Points {
		var walls []int
		for _, from := range point.Owners {
			u := wire[from.Wire][from.Use]
			if u.Seam >= 0 {
				continue
			}
			if u.Source.Face < 0 {
				return zero, geom.ErrConvergence
			}
			walls = append(walls, u.Source.Face)
		}
		if e := at(point.Parameter, sortedUniqueStockOwners(walls)); e != nil {
			return zero, e
		}
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	out.ExactOperations, out.Subdivisions = work-b.work, splits-b.subdiv
	return out, nil
}

func sortedUniqueStockOwners(owners []int) []int {
	slices.Sort(owners)
	return slices.Compact(owners)
}

func (m AlgebraicGraphVerticalMaterial) coverVerticalLine(origin, direction [3]*big.Rat, extent [2]*big.Rat,
	b *capClipBudget) (AlgebraicGraphStockLineCoverage, error) {
	var out AlgebraicGraphStockLineCoverage
	times, caps, where, err := m.verticalLineStation(origin, direction, b)
	if err != nil || where == trimvalidate.Outside {
		return out, err
	}
	if times[0].Cmp(extent[1]) > 0 || times[1].Cmp(extent[0]) < 0 {
		return out, nil
	}
	a, z := times[0], times[1]
	var walls []int
	if where == trimvalidate.Boundary {
		found, err := m.exactVerticalBoundaryOwners([2]*big.Rat{origin[0], origin[1]}, a.Cmp(z) == 0, b)
		if err != nil {
			return out, err
		}
		for _, face := range found {
			if face >= 0 {
				walls = append(walls, face)
			}
		}
	}
	if a.Cmp(extent[0]) < 0 {
		a = extent[0]
	}
	if z.Cmp(extent[1]) > 0 {
		z = extent[1]
	}
	bounds := [2]*geom.ExactAlgebraicRoot{}
	for i, t := range [2]*big.Rat{a, z} {
		r, work, err := geom.NewExactRationalRoot(b.ctx, t, b.limits())
		if err != nil {
			return AlgebraicGraphStockLineCoverage{}, err
		}
		if err = b.charge(work, 0); err != nil {
			return AlgebraicGraphStockLineCoverage{}, err
		}
		bounds[i] = r
	}
	if a.Cmp(z) < 0 {
		out.Spans = []AlgebraicGraphStockLinePiece{{Extent: bounds, BoundaryOnly: len(walls) != 0, StockFaces: slices.Clone(walls)}}
	}
	for i, t := range [2]*big.Rat{a, z} {
		p := AlgebraicGraphStockLinePiece{Extent: [2]*geom.ExactAlgebraicRoot{bounds[i], bounds[i]}, BoundaryOnly: true}
		for j, h := range times {
			if h.Cmp(t) == 0 {
				p.Caps = append(p.Caps, caps[j])
			}
		}
		// A finite clip on an already covered wall is not an original
		// vertex. Only cap endpoints or a singleton require a point.
		if len(p.Caps) > 0 || a.Cmp(z) == 0 {
			p.StockFaces = slices.Clone(walls)
			out.Points = append(out.Points, p)
		}
		if a.Cmp(z) == 0 {
			break
		}
	}
	return out, b.ctx.Err()
}
