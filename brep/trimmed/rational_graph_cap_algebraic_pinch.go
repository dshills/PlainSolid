package trimmed

import (
	"context"
	"math/big"
	"slices"

	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

// AlgebraicGraphVerticalBoundaryRun is the intersection of an exact
// vertical line with the retained stock boundary. Extent is in the
// caller's unnormalized line parameter. Original cap endpoint owners are
// reported only when those endpoints lie inside the requested window.
// A clipped run that contains just an endpoint has equal bounds.
type AlgebraicGraphVerticalBoundaryRun struct {
	Found                         bool
	Extent                        [2]*big.Rat
	Caps, StockFaces              [2]int
	ExactOperations, Subdivisions int
}

// IntersectRationalVerticalBoundary proves a coincident wall run or a
// collapsed common-cap fiber. Exterior and interior XY stations return
// Found=false; use IntersectRationalLine for ordinary interior fibers.
func (m AlgebraicGraphVerticalMaterial) IntersectRationalVerticalBoundary(ctx context.Context,
	origin, direction [3]*big.Rat, extent [2]*big.Rat, opts geom.ExactCurvePlaneEventOptions) (AlgebraicGraphVerticalBoundaryRun, error) {
	var zero AlgebraicGraphVerticalBoundaryRun
	if ctx == nil || m.caps[0].recipe == nil || opts.MaxOperations < 0 || opts.MaxCoefficientBits < 0 || opts.MaxSubdivisions < 0 {
		return zero, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	if !m.lineReady {
		return zero, geom.ErrUnrepresentable
	}
	if opts.MaxOperations == 0 {
		opts.MaxOperations = 2000000
	}
	if opts.MaxCoefficientBits == 0 {
		opts.MaxCoefficientBits = m.maxBits
	}
	if opts.MaxSubdivisions == 0 {
		opts.MaxSubdivisions = 8192
	}
	if opts.MaxCoefficientBits < 64 {
		return zero, geom.ErrInvalid
	}
	b := capClipBudget{ctx, opts.MaxOperations, opts.MaxSubdivisions, opts.MaxCoefficientBits}
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
	if extent[0].Cmp(extent[1]) >= 0 || direction[0].Sign() != 0 || direction[1].Sign() != 0 {
		return zero, geom.ErrDomain
	}
	if direction[2].Sign() == 0 {
		return zero, geom.ErrDegenerate
	}
	times, ownerOrder, loc, err := m.verticalLineStation(origin, direction, &b)
	if err != nil {
		return zero, err
	}
	if loc != trimvalidate.Boundary {
		return zero, nil
	}
	a, z := times[0], times[1]
	if a.Cmp(extent[1]) > 0 || z.Cmp(extent[0]) < 0 {
		return zero, nil
	}
	owners, err := m.exactVerticalBoundaryOwners([2]*big.Rat{origin[0], origin[1]}, a.Cmp(z) == 0, &b)
	if err != nil {
		return zero, err
	}
	if a.Cmp(extent[0]) < 0 {
		a, ownerOrder[0] = extent[0], -1
	}
	if z.Cmp(extent[1]) > 0 {
		z, ownerOrder[1] = extent[1], -1
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	return AlgebraicGraphVerticalBoundaryRun{Found: true, Extent: [2]*big.Rat{new(big.Rat).Set(a), new(big.Rat).Set(z)},
		Caps: ownerOrder, StockFaces: owners, ExactOperations: opts.MaxOperations - b.work, Subdivisions: opts.MaxSubdivisions - b.subdiv}, nil
}

func (m AlgebraicGraphVerticalMaterial) verticalLineStation(origin, direction [3]*big.Rat,
	b *capClipBudget) ([2]*big.Rat, [2]int, trimvalidate.Location, error) {
	var times [2]*big.Rat
	owners := [2]int{m.lower, m.lower ^ 1}
	if direction[2].Sign() < 0 {
		owners[0], owners[1] = owners[1], owners[0]
	}
	var loc [2]trimvalidate.Location
	for i, owner := range owners {
		if err := b.charge(25, 0); err != nil {
			return times, owners, trimvalidate.Unknown, err
		}
		p := origin
		cap := m.caps[owner]
		eq := cap.regions.plane
		p[2] = new(big.Rat).Sub(origin[2], new(big.Rat).Sub(eq.dot(origin), eq.offset))
		var err error
		loc[i], err = cap.locateRationalBudget(p, b)
		if err != nil {
			return times, owners, trimvalidate.Unknown, err
		}
		times[i] = new(big.Rat).Quo(new(big.Rat).Sub(p[2], origin[2]), direction[2])
		if x := times[i]; max(x.Num().BitLen(), x.Denom().BitLen()) > b.bits {
			return times, owners, trimvalidate.Unknown, geom.ErrConvergence
		}
	}
	if loc[0] != loc[1] || loc[0] != trimvalidate.Outside && times[0].Cmp(times[1]) > 0 {
		return times, owners, trimvalidate.Unknown, geom.ErrConvergence
	}
	return times, owners, loc[0], nil
}

// exactVerticalBoundaryOwners finds the original wall occurrences at the
// same XY station on the independently sewed cap trims. If the two heights
// coincide, it also matches the two coedges of the generated cap seam.
func (m AlgebraicGraphVerticalMaterial) exactVerticalBoundaryOwners(xy [2]*big.Rat, pinch bool, b *capClipBudget) ([2]int, error) {
	empty := [2]int{-1, -1}
	q := big.NewRat
	planes := [2][4]*big.Rat{
		{q(1, 1), nil, nil, new(big.Rat).Neg(xy[0])},
		{nil, q(1, 1), nil, new(big.Rat).Neg(xy[1])},
	}
	var faces [2][]int
	seams := [2]AlgebraicGraphCapSeamOwner{{-1, -1}, {-1, -1}}
	for i, cap := range m.caps {
		for wi, wire := range cap.wires {
			for ui, use := range wire {
				h, err := use.Cell.IntersectRawPlanePair(b.ctx, planes, b.limits())
				if err != nil {
					return empty, err
				}
				if err = b.charge(h.Operations, h.Subdivisions); err != nil {
					return empty, err
				}
				if !h.Complete || h.Coincident || len(h.Events) > 1 {
					return empty, geom.ErrConvergence
				}
				if len(h.Events) == 0 {
					continue
				}
				if use.Seam >= 0 {
					if !pinch || seams[i].Wire >= 0 || h.Events[0].EndpointMask() > 2 {
						return empty, geom.ErrConvergence
					}
					seams[i] = AlgebraicGraphCapSeamOwner{wi, ui}
					continue
				}
				end := h.Events[0].EndpointMask()
				if use.Source.Face < 0 || pinch && end == 0 || end > 2 {
					return empty, geom.ErrConvergence
				}
				if !slices.Contains(faces[i], use.Source.Face) {
					faces[i] = append(faces[i], use.Source.Face)
				}
			}
		}
		slices.Sort(faces[i])
	}
	if len(faces[0]) > len(empty) || !slices.Equal(faces[0], faces[1]) {
		return empty, geom.ErrConvergence
	}
	if !pinch {
		if len(faces[0]) == 0 {
			return empty, geom.ErrConvergence
		}
		copy(empty[:], faces[0])
		return empty, nil
	}
	if seams[0].Wire < 0 || seams[1].Wire < 0 {
		return empty, geom.ErrConvergence
	}
	paired := false
	for _, seam := range m.seams {
		if err := b.charge(1, 0); err != nil {
			return empty, err
		}
		if seam.Owners == seams {
			paired = true
			break
		}
	}
	if !paired {
		return empty, geom.ErrConvergence
	}
	copy(empty[:], faces[0])
	return empty, nil
}
